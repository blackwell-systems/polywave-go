package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	bedrockbackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/analyzer"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/commands"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/hooks"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/retryctx"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/suitability"
	"gopkg.in/yaml.v3"
)

// RunScout executes a Scout agent, calling onChunk for each output fragment.
// Returns when the agent finishes. Cancellable via ctx.
// I6 enforcement: Post-execution validation checks that Scout only wrote to docs/IMPL/IMPL-*.yaml.
func RunScout(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
	if opts.Feature == "" {
		return fmt.Errorf("engine.RunScout: Feature is required")
	}
	if opts.RepoPath == "" {
		return fmt.Errorf("engine.RunScout: RepoPath is required")
	}
	if opts.IMPLOutPath == "" {
		return fmt.Errorf("engine.RunScout: IMPLOutPath is required")
	}

	// Record start time for I6 validation (detect files written during execution)
	startTime := time.Now()

	// E40: Emit scout_launch after validation passes.
	implSlug := implSlugFromIMPLPath(opts.IMPLOutPath)
	opts.ObsEmitter.Emit(ctx, observability.NewScoutLaunchEvent(implSlug))

	// Resolve SAW repo path.
	sawRepo := opts.SAWRepoPath
	if sawRepo == "" {
		sawRepo = os.Getenv("SAW_REPO")
	}
	if sawRepo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("engine.RunScout: cannot determine home directory: %w", err)
		}
		sawRepo = filepath.Join(home, "code", "scout-and-wave")
	}

	// Load scout.md prompt (L1: no fallback — missing file is a fatal error).
	scoutMdPath := filepath.Join(sawRepo, "implementations", "claude-code", "prompts", "scout.md")
	scoutMdBytes, err := os.ReadFile(scoutMdPath)
	if err != nil {
		return fmt.Errorf("engine.RunScout: scout.md not found at %s: %w", scoutMdPath, err)
	}

	// Run automation tools (H1a, H2, H3) before launching Scout.
	automationContext := runScoutAutomation(opts.RepoPath, opts.Feature)

	// Load program contracts if ProgramManifestPath is set.
	programContractsSection := ""
	if opts.ProgramManifestPath != "" {
		manifest, parseErr := protocol.ParseProgramManifest(opts.ProgramManifestPath)
		if parseErr != nil {
			// Non-fatal: log warning and continue without program contracts
			fmt.Fprintf(os.Stderr, "engine.RunScout: failed to parse PROGRAM manifest (continuing without contracts): %v\n", parseErr)
		} else {
			programContractsSection = buildProgramContractsSection(manifest, opts.RepoPath)
		}
	}

	// E17: Prepend docs/CONTEXT.md if present, so Scout has project memory.
	contextMD := readContextMD(opts.RepoPath)
	var prompt string
	if contextMD != "" {
		prompt = fmt.Sprintf("## Project Memory (docs/CONTEXT.md)\n\n%s\n\n%s%s\n\n## Feature\n%s\n\n%s\n\n## IMPL Output Path\n%s\n",
			contextMD, programContractsSection, string(scoutMdBytes), opts.Feature, automationContext, opts.IMPLOutPath)
	} else {
		prompt = fmt.Sprintf("%s%s\n\n## Feature\n%s\n\n%s\n\n## IMPL Output Path\n%s\n",
			programContractsSection, string(scoutMdBytes), opts.Feature, automationContext, opts.IMPLOutPath)
	}

	var execErr error

	if opts.UseStructuredOutput {
		if strings.HasPrefix(opts.ScoutModel, "bedrock:") {
			_, execErr = runScoutStructuredBedrock(ctx, opts, prompt, onChunk)
		} else {
			_, execErr = runScoutStructured(ctx, opts, prompt, onChunk)
		}
	} else {
		b, bErr := orchestrator.NewBackendFromModel(opts.ScoutModel)
		if bErr != nil {
			return fmt.Errorf("engine.RunScout: backend init: %w", bErr)
		}
		runner := agent.NewRunner(b, nil)
		spec := &protocol.Agent{ID: "scout", Task: prompt}
		_, execErr = runner.ExecuteStreamingWithTools(ctx, spec, opts.RepoPath, onChunk, nil)
	}

	// I6 enforcement: Validate Scout only wrote to docs/IMPL/IMPL-*.yaml
	if execErr == nil {
		if err := hooks.ValidateScoutWrites(opts.RepoPath, opts.IMPLOutPath, startTime); err != nil {
			return fmt.Errorf("engine.RunScout: %w", err)
		}
		// E40: Emit scout_complete on success.
		opts.ObsEmitter.Emit(ctx, observability.NewScoutCompleteEvent(implSlug))
	}

	return execErr
}

// implSlugFromIMPLPath derives an IMPL slug from a file path such as
// /path/to/IMPL-my-feature.yaml → "my-feature".
// This is a runner-local helper mirroring implSlugFromPath in finalize.go.
func implSlugFromIMPLPath(path string) string {
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = base[:len(base)-len(ext)]
	}
	const prefix = "IMPL-"
	if len(base) > len(prefix) && base[:len(prefix)] == prefix {
		return base[len(prefix):]
	}
	return base
}

// RunPlanner executes a Planner agent to produce a PROGRAM manifest.
// Mirrors RunScout but reads agents/planner.md and writes docs/PROGRAM-*.yaml.
func RunPlanner(ctx context.Context, opts RunPlannerOpts, onChunk func(string)) error {
	if opts.Description == "" {
		return fmt.Errorf("engine.RunPlanner: Description is required")
	}
	if opts.RepoPath == "" {
		return fmt.Errorf("engine.RunPlanner: RepoPath is required")
	}
	if opts.ProgramOutPath == "" {
		return fmt.Errorf("engine.RunPlanner: ProgramOutPath is required")
	}

	sawRepo := opts.SAWRepoPath
	if sawRepo == "" {
		sawRepo = os.Getenv("SAW_REPO")
	}
	if sawRepo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("engine.RunPlanner: cannot determine home directory: %w", err)
		}
		sawRepo = filepath.Join(home, "code", "scout-and-wave")
	}

	// Load planner.md prompt with fallback.
	plannerMdPath := filepath.Join(sawRepo, "agents", "planner.md")
	plannerMdBytes, err := os.ReadFile(plannerMdPath)
	if err != nil {
		plannerMdBytes = []byte("You are a Planner agent. Analyze the project requirements and produce a PROGRAM manifest at the specified output path. Decompose the project into features organized into dependency-ordered tiers. Define cross-feature program contracts for shared types/APIs.")
	}

	// E17: Prepend docs/CONTEXT.md if present.
	contextMD := readContextMD(opts.RepoPath)
	var prompt string
	if contextMD != "" {
		prompt = fmt.Sprintf("## Project Memory (docs/CONTEXT.md)\n\n%s\n\n%s\n\n## Project Description\n%s\n\n## PROGRAM Output Path\n%s\n",
			contextMD, string(plannerMdBytes), opts.Description, opts.ProgramOutPath)
	} else {
		prompt = fmt.Sprintf("%s\n\n## Project Description\n%s\n\n## PROGRAM Output Path\n%s\n",
			string(plannerMdBytes), opts.Description, opts.ProgramOutPath)
	}

	model := opts.PlannerModel
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	b, bErr := orchestrator.NewBackendFromModel(model)
	if bErr != nil {
		return fmt.Errorf("engine.RunPlanner: backend init: %w", bErr)
	}
	runner := agent.NewRunner(b, nil)
	spec := &protocol.Agent{ID: "planner", Task: prompt}
	_, execErr := runner.ExecuteStreamingWithTools(ctx, spec, opts.RepoPath, onChunk, nil)
	return execErr
}

// runScoutStructured runs the Scout agent via the API backend with structured
// output enabled. It applies opts.OutputSchemaOverride if non-nil, otherwise
// calls protocol.GenerateScoutSchema(). The API response JSON is unmarshalled
// directly into a protocol.IMPLManifest and saved to opts.IMPLOutPath.
func runScoutStructured(ctx context.Context, opts RunScoutOpts, prompt string, onChunk func(string)) (*protocol.IMPLManifest, error) {
	// Resolve schema: use override if provided, otherwise generate from protocol.
	var schema map[string]any
	if opts.OutputSchemaOverride != nil {
		schema = opts.OutputSchemaOverride
	} else {
		var err error
		schema, err = protocol.GenerateScoutSchema()
		if err != nil {
			return nil, fmt.Errorf("runScoutStructured: generate schema: %w", err)
		}
	}

	// Build API client with structured output config.
	apiClient := apiclient.New("", backend.Config{Model: opts.ScoutModel})
	apiClient.WithOutputConfig(schema)

	// Run the agent and collect output.
	var chunkCallback func(string)
	if onChunk != nil {
		chunkCallback = onChunk
	}
	jsonStr, err := apiClient.RunStreaming(ctx, "", prompt, opts.RepoPath, chunkCallback)
	if err != nil {
		return nil, fmt.Errorf("runScoutStructured: API run: %w", err)
	}

	// Unmarshal the structured JSON response into IMPLManifest.
	var manifest protocol.IMPLManifest
	if err := json.Unmarshal([]byte(jsonStr), &manifest); err != nil {
		return nil, fmt.Errorf("runScoutStructured: unmarshal manifest: %w", err)
	}

	// Persist to disk.
	if err := protocol.Save(&manifest, opts.IMPLOutPath); err != nil {
		return nil, fmt.Errorf("runScoutStructured: save manifest: %w", err)
	}

	return &manifest, nil
}

// runScoutStructuredBedrock runs the Scout agent via the Bedrock backend with
// structured output enabled. It is the Bedrock-specific counterpart of
// runScoutStructured (which uses the API backend). The function:
//  1. Strips the "bedrock:" provider prefix from opts.ScoutModel to get the bare model ID.
//  2. Builds a Bedrock client with WithOutputConfig set to the scout schema.
//  3. Calls client.Run() (non-streaming Converse API) so the full JSON is returned at once.
//  4. Unmarshals the JSON response into protocol.IMPLManifest and persists it via protocol.Save().
//
// If onChunk is non-nil the raw JSON response is forwarded as a single chunk so callers
// (e.g. SSE streams) can surface progress without requiring a streaming call.
func runScoutStructuredBedrock(ctx context.Context, opts RunScoutOpts, prompt string, onChunk func(string)) (*protocol.IMPLManifest, error) {
	// Strip provider prefix "bedrock:" to get bare model ID.
	bareModel := strings.TrimPrefix(opts.ScoutModel, "bedrock:")
	if bareModel == "" {
		return nil, fmt.Errorf("runScoutStructuredBedrock: model is required")
	}

	// Resolve schema: use override if provided, otherwise generate from protocol.
	var schema map[string]any
	if opts.OutputSchemaOverride != nil {
		schema = opts.OutputSchemaOverride
	} else {
		var err error
		schema, err = protocol.GenerateScoutSchema()
		if err != nil {
			return nil, fmt.Errorf("runScoutStructuredBedrock: generate schema: %w", err)
		}
	}

	// Build Bedrock client with structured output config.
	bedrockClient := bedrockbackend.New(backend.Config{Model: bareModel})
	bedrockClient.WithOutputConfig(schema)

	// Run non-streaming Converse request (structured output returns complete JSON).
	jsonStr, err := bedrockClient.Run(ctx, "", prompt, opts.RepoPath)
	if err != nil {
		// Provide a helpful hint when AWS credentials are not configured.
		if strings.Contains(err.Error(), "no EC2 IMDS role found") ||
			strings.Contains(err.Error(), "failed to refresh cached credentials") ||
			strings.Contains(err.Error(), "NoCredentialProviders") ||
			strings.Contains(err.Error(), "AWS config failed to load") {
			return nil, fmt.Errorf("runScoutStructuredBedrock: AWS credentials not configured: %w\n"+
				"Hint: run `aws configure` or set AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY environment variables", err)
		}
		return nil, fmt.Errorf("runScoutStructuredBedrock: Bedrock run: %w", err)
	}

	// Forward full JSON response as a single chunk for SSE visibility.
	if onChunk != nil {
		onChunk(jsonStr)
	}

	// Unmarshal the structured JSON response into IMPLManifest.
	var manifest protocol.IMPLManifest
	if err := json.Unmarshal([]byte(jsonStr), &manifest); err != nil {
		return nil, fmt.Errorf("runScoutStructuredBedrock: unmarshal manifest: %w", err)
	}

	// Persist to disk.
	if err := protocol.Save(&manifest, opts.IMPLOutPath); err != nil {
		return nil, fmt.Errorf("runScoutStructuredBedrock: save manifest: %w", err)
	}

	return &manifest, nil
}

// StartWave executes a full wave run (all waves in the IMPL doc).
// Publishes lifecycle events via onEvent. Blocks until all waves complete
// or a fatal error occurs.
func StartWave(ctx context.Context, opts RunWaveOpts, onEvent func(Event)) error {
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.StartWave: IMPLPath is required")
	}
	if opts.RepoPath == "" {
		return fmt.Errorf("engine.StartWave: RepoPath is required")
	}

	publish := func(event string, data interface{}) {
		onEvent(Event{Event: event, Data: data})
	}

	publish("run_started", map[string]string{"slug": opts.Slug, "impl_path": opts.IMPLPath})

	orch, err := orchestrator.New(opts.RepoPath, opts.IMPLPath)
	if err != nil {
		publish("run_failed", map[string]string{"error": err.Error()})
		return fmt.Errorf("engine.StartWave: %w", err)
	}

	if opts.WaveModel != "" {
		orch.SetDefaultModel(opts.WaveModel)
	}

	// Wire orchestrator events to the onEvent callback.
	orch.SetEventPublisher(func(ev orchestrator.OrchestratorEvent) {
		onEvent(Event{Event: ev.Event, Data: ev.Data})
	})

	// Run scaffold if needed.
	scaffoldModel := opts.ScaffoldModel
	if scaffoldModel == "" {
		scaffoldModel = opts.WaveModel
	}
	if err := RunScaffold(ctx, opts.IMPLPath, opts.RepoPath, "", scaffoldModel, onEvent); err != nil {
		publish("run_failed", map[string]string{"error": err.Error()})
		return fmt.Errorf("engine.StartWave: scaffold: %w", err)
	}

	waves := orch.IMPLDoc().Waves
	totalAgents := 0
	for _, w := range waves {
		totalAgents += len(w.Agents)
	}

	for i, wave := range waves {
		waveNum := wave.Number

		// Pre-create worktrees via protocol (handles multi-repo from file ownership).
		wtRes := protocol.CreateWorktrees(opts.IMPLPath, waveNum, opts.RepoPath)
		if !wtRes.IsSuccess() {
			errMsg := fmt.Sprintf("%v", wtRes.Errors)
			publish("run_failed", map[string]string{"error": errMsg})
			return fmt.Errorf("engine.StartWave: CreateWorktrees wave %d: %v", waveNum, wtRes.Errors)
		}
		wtResult := wtRes.GetData()
		// Pass pre-computed worktree paths to orchestrator so launchAgent
		// uses the correct (potentially cross-repo) paths.
		wtPaths := make(map[string]string, len(wtResult.Worktrees))
		for _, wt := range wtResult.Worktrees {
			wtPaths[wt.Agent] = wt.Path
		}
		orch.SetWorktreePaths(wtPaths)

		if err := orch.RunWave(waveNum); err != nil {
			publish("run_failed", map[string]string{"error": err.Error()})
			return fmt.Errorf("engine.StartWave: RunWave %d: %w", waveNum, err)
		}

		// E20: Post-wave stub scan (informational only).
		if doc := orch.IMPLDoc(); doc != nil {
			manifest, err := protocol.Load(opts.IMPLPath)
			if err == nil {
				stubReports := make(map[string]*protocol.CompletionReport)
				for _, ag := range wave.Agents {
					if protoReport, ok := manifest.CompletionReports[ag.ID]; ok {
						cr := protoReport // copy to take address
						stubReports[ag.ID] = &cr
					}
				}
				_ = orchestrator.RunStubScan(opts.IMPLPath, waveNum, stubReports, "")
			}
		}

		// E21: Post-wave quality gates before merge.
		if doc := orch.IMPLDoc(); doc != nil && doc.QualityGates != nil {
			results, err := orchestrator.RunQualityGates(opts.RepoPath, doc.QualityGates)
			for _, r := range results {
				onEvent(Event{Event: "quality_gate_result", Data: r})
			}
			if err != nil {
				return fmt.Errorf("engine.StartWave: quality gate wave %d: %w", waveNum, err)
			}
		}

		if err := orch.MergeWave(waveNum); err != nil {
			publish("run_failed", map[string]string{"error": err.Error()})
			return fmt.Errorf("engine.StartWave: MergeWave %d: %w", waveNum, err)
		}

		testCmd := orch.IMPLDoc().TestCommand
		if testCmd != "" {
			if err := orch.RunVerification(testCmd); err != nil {
				publish("run_failed", map[string]string{"error": err.Error()})
				return fmt.Errorf("engine.StartWave: RunVerification %d: %w", waveNum, err)
			}
		}

		// E25: Post-wave integration validation
		manifest, loadErr := protocol.Load(opts.IMPLPath)
		if loadErr == nil {
			integrationReport, intErr := protocol.ValidateIntegration(manifest, waveNum, opts.RepoPath)
			if intErr == nil && integrationReport != nil && !integrationReport.Valid {
				publish("integration_gaps_detected", map[string]interface{}{
					"wave":   waveNum,
					"gaps":   len(integrationReport.Gaps),
					"report": integrationReport,
				})

				// E26: Launch integration agent to wire gaps
				intModel := opts.IntegrationModel
				if intModel == "" {
					intModel = opts.WaveModel
				}
				intAgentErr := RunIntegrationAgent(ctx, RunIntegrationAgentOpts{
					IMPLPath: opts.IMPLPath,
					RepoPath: opts.RepoPath,
					WaveNum:  waveNum,
					Report:   integrationReport,
					Model:    intModel,
				}, func(ev Event) { onEvent(ev) })
				if intAgentErr != nil {
					// Non-fatal: log but don't abort wave
					publish("integration_agent_warning", map[string]string{
						"error": intAgentErr.Error(),
					})
				}
			}
		}

		if err := orch.UpdateIMPLStatus(waveNum); err != nil {
			// Non-fatal: log but don't abort.
			publish("update_status_failed", map[string]string{
				"wave":  opts.Slug,
				"error": err.Error(),
			})
		}

		// Between waves, pause at gate (no-op in engine layer — callers handle gating).
		_ = i
	}

	// E18: Update project memory after final wave completes.
	slug := opts.Slug
	if slug == "" {
		slug = filepath.Base(filepath.Dir(opts.IMPLPath))
	}
	entry := orchestrator.ContextMDEntry{
		Slug:    slug,
		ImplDoc: opts.IMPLPath,
		Waves:   len(waves),
		Agents:  totalAgents,
	}
	if err := orchestrator.UpdateContextMD(opts.RepoPath, entry); err != nil {
		// Non-fatal: log but don't abort.
		fmt.Fprintf(os.Stderr, "engine: E18 UpdateContextMD failed: %v\n", err)
	}

	publish("run_complete", orchestrator.RunCompletePayload{
		Status: "success",
		Waves:  len(waves),
		Agents: totalAgents,
	})
	return nil
}

// gateChannels stores per-slug gate channels used to pause the wave loop between waves.
var gateChannels sync.Map

// RunScaffold checks for pending scaffold files and runs a Scaffold agent if needed.
// The model parameter is optional; if empty, the backend uses its default model.
func RunScaffold(ctx context.Context, implPath, repoPath, sawRepoPath, model string, onEvent func(Event)) error {
	publish := func(event string, data interface{}) {
		onEvent(Event{Event: event, Data: data})
	}

	// Load YAML manifest to get scaffold files.
	manifest, err := protocol.Load(implPath)
	if err != nil {
		return fmt.Errorf("engine.RunScaffold: load manifest: %w", err)
	}

	scaffolds := manifest.Scaffolds
	if len(scaffolds) == 0 {
		return nil
	}

	// If all scaffold files already exist, skip.
	allExist := true
	for _, sf := range scaffolds {
		absPath := sf.FilePath
		if !filepath.IsAbs(absPath) {
			absPath = filepath.Join(repoPath, absPath)
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			allExist = false
			break
		}
	}
	if allExist {
		return nil
	}

	publish("scaffold_started", map[string]string{"impl_path": implPath})

	// Locate scaffold-agent.md prompt.
	sawRepo := sawRepoPath
	if sawRepo == "" {
		sawRepo = os.Getenv("SAW_REPO")
	}
	if sawRepo == "" {
		home, _ := os.UserHomeDir()
		sawRepo = filepath.Join(home, "code", "scout-and-wave")
	}
	scaffoldMdPath := filepath.Join(sawRepo, "implementations", "claude-code", "prompts", "agents", "scaffold-agent.md")
	scaffoldMdBytes, err := os.ReadFile(scaffoldMdPath)
	if err != nil {
		scaffoldMdBytes = []byte("You are a Scaffold Agent. Create the stub files defined in the IMPL doc Scaffolds section.")
	}

	prompt := fmt.Sprintf("%s\n\n## IMPL Doc Path\n%s\n", string(scaffoldMdBytes), implPath)

	b, err := orchestrator.NewBackendFromModel(model)
	if err != nil {
		publish("scaffold_failed", map[string]string{"error": err.Error()})
		return fmt.Errorf("engine.RunScaffold: backend init: %w", err)
	}
	runner := agent.NewRunner(b, nil)
	spec := &protocol.Agent{ID: "scaffold", Task: prompt}

	onChunk := func(chunk string) {
		publish("scaffold_output", map[string]string{"chunk": chunk})
	}

	if _, execErr := runner.ExecuteStreamingWithTools(ctx, spec, repoPath, onChunk, nil); execErr != nil {
		publish("scaffold_failed", map[string]string{"error": execErr.Error()})
		return fmt.Errorf("engine.RunScaffold: scaffold agent failed: %w", execErr)
	}

	// E22: Scaffold build verification — compile to catch scaffold errors early.
	if err := runScaffoldBuildVerification(repoPath, onEvent); err != nil {
		return fmt.Errorf("engine.RunScaffold: build verification failed: %w", err)
	}

	publish("scaffold_complete", map[string]string{"impl_path": implPath})
	return nil
}

// readContextMD reads docs/CONTEXT.md from repoPath and returns its contents (E17).
// Returns empty string if the file does not exist or cannot be read.
func readContextMD(repoPath string) string {
	p := filepath.Join(repoPath, "docs", "CONTEXT.md")
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// runScaffoldBuildVerification verifies the scaffold files compile correctly (E22).
// For Go projects it runs dependency resolution then a 2-pass build
// (scaffold packages first, then full project). For other project types it
// attempts basic detection and runs the appropriate install/build command.
// Signature accepts *protocol.IMPLManifest so the caller can pass doc for Pass 1 package
// derivation; passing nil skips Pass 1 and only runs Pass 2.
func runScaffoldBuildVerification(repoPath string, onEvent func(Event)) error {
	return runScaffoldBuildVerificationWithDoc(context.Background(), repoPath, nil, onEvent)
}

// runScaffoldBuildVerificationWithDoc is the full implementation of E22 scaffold
// build verification. It is separated from runScaffoldBuildVerification so that
// the engine layer can pass an *protocol.IMPLManifest for scaffold-package derivation.
func runScaffoldBuildVerificationWithDoc(ctx context.Context, repoPath string, doc *protocol.IMPLManifest, onEvent func(Event)) error {
	// ── Go project ────────────────────────────────────────────────────────────
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); err == nil {
		// Dependency resolution: go get ./...
		getCmd := exec.CommandContext(ctx, "go", "get", "./...")
		getCmd.Dir = repoPath
		if out, err := getCmd.CombinedOutput(); err != nil {
			// Log but don't fail — some projects don't need go get.
			fmt.Fprintf(os.Stderr, "scaffold build: go get ./... (non-fatal): %v\n%s", err, string(out))
		}

		// go mod tidy
		tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
		tidyCmd.Dir = repoPath
		if out, err := tidyCmd.CombinedOutput(); err != nil {
			// Log but don't fail.
			fmt.Fprintf(os.Stderr, "scaffold build: go mod tidy (non-fatal): %v\n%s", err, string(out))
		}

		// Pass 1 (scaffold-only): build only the packages containing scaffold files.
		if doc != nil && len(doc.Scaffolds) > 0 {
			pkgSet := make(map[string]bool)
			for _, sf := range doc.Scaffolds {
				dir := filepath.Dir(sf.FilePath)
				if dir == "." || dir == "" {
					pkgSet["./"] = true
				} else {
					pkgSet["./"+dir] = true
				}
			}
			scaffoldPkgs := make([]string, 0, len(pkgSet))
			for pkg := range pkgSet {
				scaffoldPkgs = append(scaffoldPkgs, pkg)
			}
			args := append([]string{"build"}, scaffoldPkgs...)
			pass1Cmd := exec.CommandContext(ctx, "go", args...)
			pass1Cmd.Dir = repoPath
			if out, err := pass1Cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("go build (scaffold-only) failed: %w\n%s", err, string(out))
			}
		}

		// Pass 2 (full project): go build ./...
		pass2Cmd := exec.CommandContext(ctx, "go", "build", "./...")
		pass2Cmd.Dir = repoPath
		if out, err := pass2Cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("go build ./... failed: %w\n%s", err, string(out))
		}
		return nil
	}

	// ── Rust project ──────────────────────────────────────────────────────────
	if _, err := os.Stat(filepath.Join(repoPath, "Cargo.toml")); err == nil {
		fetchCmd := exec.CommandContext(ctx, "cargo", "fetch")
		fetchCmd.Dir = repoPath
		if out, err := fetchCmd.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "scaffold build: cargo fetch (non-fatal): %v\n%s", err, string(out))
		}
		buildCmd := exec.CommandContext(ctx, "cargo", "build")
		buildCmd.Dir = repoPath
		if out, err := buildCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("cargo build failed: %w\n%s", err, string(out))
		}
		return nil
	}

	// ── Node/TypeScript project ────────────────────────────────────────────────
	if _, err := os.Stat(filepath.Join(repoPath, "package.json")); err == nil {
		// Use yarn if yarn.lock exists, else npm.
		installArgs := []string{"install"}
		npmBin := "npm"
		if _, yarnErr := os.Stat(filepath.Join(repoPath, "yarn.lock")); yarnErr == nil {
			npmBin = "yarn"
		}
		installCmd := exec.CommandContext(ctx, npmBin, installArgs...)
		installCmd.Dir = repoPath
		if out, err := installCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s install failed: %w\n%s", npmBin, err, string(out))
		}
		// Skip TypeScript compilation — requires project-specific config.
		return nil
	}

	// Unrecognized project type — skip silently.
	fmt.Fprintf(os.Stderr, "scaffold build verification: unrecognized project type, skipping\n")
	return nil
}

// RunSingleWave runs the agents for one wave number without merging or verifying.
// The caller is responsible for calling MergeWave and RunVerification afterwards.
func RunSingleWave(ctx context.Context, opts RunWaveOpts, waveNum int, onEvent func(Event)) error {
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.RunSingleWave: IMPLPath is required")
	}
	orch, err := orchestrator.New(opts.RepoPath, opts.IMPLPath)
	if err != nil {
		return fmt.Errorf("engine.RunSingleWave: %w", err)
	}
	if opts.WaveModel != "" {
		orch.SetDefaultModel(opts.WaveModel)
	}
	orch.SetEventPublisher(func(ev orchestrator.OrchestratorEvent) {
		onEvent(Event{Event: ev.Event, Data: ev.Data})
	})

	// Use protocol.CreateWorktrees for cross-repo awareness. It reads the
	// file_ownership repo: field and creates worktrees in sibling repos when
	// needed. Feed the resulting paths into the orchestrator so launchAgent
	// uses the correct worktree for each agent.
	wtRes := protocol.CreateWorktrees(opts.IMPLPath, waveNum, opts.RepoPath)
	if !wtRes.IsSuccess() {
		return fmt.Errorf("engine.RunSingleWave: create worktrees: %v", wtRes.Errors)
	}
	wtResult := wtRes.GetData()
	if len(wtResult.Worktrees) > 0 {
		paths := make(map[string]string, len(wtResult.Worktrees))
		for _, wt := range wtResult.Worktrees {
			paths[wt.Agent] = wt.Path
		}
		orch.SetWorktreePaths(paths)
	}

	return orch.RunWave(waveNum)
}

// RunSingleAgent runs exactly one agent from the specified wave. This is used
// for single-agent reruns (e.g. retrying a failed agent). If promptPrefix is
// non-empty it is prepended to the agent's task prompt before execution.
func RunSingleAgent(ctx context.Context, opts RunWaveOpts, waveNum int, agentLetter string, promptPrefix string, onEvent func(Event)) error {
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.RunSingleAgent: IMPLPath is required")
	}
	orch, err := orchestrator.New(opts.RepoPath, opts.IMPLPath)
	if err != nil {
		return fmt.Errorf("engine.RunSingleAgent: %w", err)
	}
	if opts.WaveModel != "" {
		orch.SetDefaultModel(opts.WaveModel)
	}
	orch.SetEventPublisher(func(ev orchestrator.OrchestratorEvent) {
		onEvent(Event{Event: ev.Event, Data: ev.Data})
	})

	// Auto-inject retry context when no explicit promptPrefix is provided
	// and the agent has a prior non-complete completion report.
	if promptPrefix == "" {
		manifest, loadErr := protocol.Load(opts.IMPLPath)
		if loadErr == nil {
			if report, ok := manifest.CompletionReports[agentLetter]; ok && report.Status != "complete" {
				rc, rcErr := retryctx.BuildRetryContext(opts.IMPLPath, agentLetter, 1)
				if rcErr != nil {
					fmt.Fprintf(os.Stderr, "engine.RunSingleAgent: retry context (best-effort): %v\n", rcErr)
				} else if rc != nil && rc.PromptText != "" {
					promptPrefix = rc.PromptText
				}
			}
		}
	}

	return orch.RunAgent(waveNum, agentLetter, promptPrefix)
}

// MergeWave merges the agent branches for the given wave into the repo's main branch.
// After successful merge, archives journals for all agents in the wave.
func MergeWave(ctx context.Context, opts RunMergeOpts) error {
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.MergeWave: IMPLPath is required")
	}
	orch, err := orchestrator.New(opts.RepoPath, opts.IMPLPath)
	if err != nil {
		return fmt.Errorf("engine.MergeWave: %w", err)
	}

	// Merge the wave
	if err := orch.MergeWave(opts.WaveNum); err != nil {
		return err
	}

	// Archive journals for all agents in this wave (non-fatal)
	doc := orch.IMPLDoc()
	if doc != nil {
		for _, wave := range doc.Waves {
			if wave.Number == opts.WaveNum {
				for _, agent := range wave.Agents {
					agentPath := fmt.Sprintf("wave%d/agent-%s", opts.WaveNum, agent.ID)
					observer, obsErr := journal.NewObserver(opts.RepoPath, agentPath)
					if obsErr == nil {
						if archErr := observer.Archive(); archErr != nil {
							// Log but don't fail the merge
							fmt.Fprintf(os.Stderr, "engine: failed to archive journal for agent %s: %v\n", agent.ID, archErr)
						}
					}
				}
				break
			}
		}
	}

	return nil
}

// RunVerification runs post-merge verification (go vet + test command).
func RunVerification(ctx context.Context, opts RunVerificationOpts) error {
	testCmd := opts.TestCommand
	if testCmd == "" {
		testCmd = "go test ./..."
	}
	orch, err := orchestrator.New(opts.RepoPath, "")
	if err != nil {
		return fmt.Errorf("engine.RunVerification: %w", err)
	}
	return orch.RunVerification(testCmd)
}

// UpdateIMPLStatus ticks status checkboxes for completed agents.
// Delegates to pkg/protocol.UpdateIMPLStatus.
func UpdateIMPLStatus(implDocPath string, completedLetters []string) error {
	return protocol.UpdateIMPLStatus(implDocPath, completedLetters)
}

// ValidateInvariants validates disjoint file ownership invariants.
// Delegates to pkg/protocol.ValidateInvariants.
func ValidateInvariants(manifest *protocol.IMPLManifest) error {
	return protocol.ValidateInvariants(manifest)
}

// JournalIntegration provides journal lifecycle hooks for wave execution.
// It creates journal observers for agents, syncs session logs, injects context,
// triggers checkpoints at milestones, and archives journals after merge.
type JournalIntegration struct {
	repoPath string
	logger   func(msg string, args ...interface{})
}

// NewJournalIntegration creates a journal integration instance for the given repo.
func NewJournalIntegration(repoPath string, logger func(string, ...interface{})) *JournalIntegration {
	if logger == nil {
		logger = func(string, ...interface{}) {} // no-op logger
	}
	return &JournalIntegration{
		repoPath: repoPath,
		logger:   logger,
	}
}

// PrepareAgentContext creates a journal observer, syncs from session logs,
// and generates context markdown to prepend to the agent prompt.
// Returns the enriched prompt and the observer (for later checkpoint/archive calls).
// Non-fatal: returns original prompt if journal operations fail.
func (ji *JournalIntegration) PrepareAgentContext(waveNum int, agentID string, originalPrompt string) (enrichedPrompt string, observer *journal.JournalObserver, err error) {
	// Create journal observer
	agentPath := fmt.Sprintf("wave%d/agent-%s", waveNum, agentID)
	observer, err = journal.NewObserver(ji.repoPath, agentPath)
	if err != nil {
		ji.logger("Failed to create journal observer", "agent", agentID, "error", err)
		return originalPrompt, nil, fmt.Errorf("create journal observer: %w", err)
	}

	// Sync from Claude Code session logs (incremental tail)
	result, err := observer.Sync()
	if err != nil {
		ji.logger("Failed to sync journal", "agent", agentID, "error", err)
		// Non-fatal: continue without journal recovery
		return originalPrompt, observer, nil
	}

	// If journal has events, generate context and prepend to prompt
	if result != nil && result.NewToolUses > 0 {
		contextMd, err := observer.GenerateContext()
		if err != nil {
			ji.logger("Failed to generate context", "agent", agentID, "error", err)
			return originalPrompt, observer, nil
		}

		ji.logger("Recovered session context", "agent", agentID, "tool_calls", result.NewToolUses)
		enrichedPrompt = contextMd + "\n\n---\n\n" + originalPrompt
		return enrichedPrompt, observer, nil
	}

	return originalPrompt, observer, nil
}

// StartPeriodicSync launches a background goroutine that syncs the journal every 30 seconds.
// Returns a cancel function to stop the sync goroutine.
func (ji *JournalIntegration) StartPeriodicSync(ctx context.Context, observer *journal.JournalObserver) func() {
	if observer == nil {
		return func() {} // no-op if observer wasn't created
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Sync journal from Claude Code session logs
				if _, err := observer.Sync(); err != nil {
					ji.logger("Periodic sync failed", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return cancel
}

// TriggerCheckpoint creates a named checkpoint snapshot of the journal.
// Checkpoint names follow E23A protocol: 001-isolation, 002-first-edit, 003-tests, 004-pre-report.
func (ji *JournalIntegration) TriggerCheckpoint(observer *journal.JournalObserver, name string) error {
	if observer == nil {
		return nil // no-op if observer wasn't created
	}

	if err := observer.Checkpoint(name); err != nil {
		ji.logger("Checkpoint failed", "name", name, "error", err)
		return fmt.Errorf("checkpoint %s: %w", name, err)
	}

	ji.logger("Checkpoint created", "name", name)
	return nil
}

// ArchiveJournal compresses the journal directory to .tar.gz after wave merge succeeds.
func (ji *JournalIntegration) ArchiveJournal(observer *journal.JournalObserver) error {
	if observer == nil {
		return nil // no-op if observer wasn't created
	}

	if err := observer.Archive(); err != nil {
		ji.logger("Archive failed", "error", err)
		return fmt.Errorf("archive journal: %w", err)
	}

	ji.logger("Journal archived")
	return nil
}

// runScoutAutomation runs the 5 scout automation tools (H1a, H2, H3) and
// returns a markdown section to inject into the Scout prompt. All tools are
// best-effort: failures are logged but don't block Scout execution.
func runScoutAutomation(repoPath string, featureDescription string) string {
	var sections []string

	// H2: Extract build/test/lint commands
	commandsResult, commandsErr := commands.ExtractCommands(repoPath)
	if commandsErr != nil {
		sections = append(sections, "### Build/Test Commands (H2)\nNot detected")
	} else {
		commandsYAML, err := yaml.Marshal(commandsResult)
		if err != nil {
			sections = append(sections, "### Build/Test Commands (H2)\nNot detected")
		} else {
			sections = append(sections, fmt.Sprintf("### Build/Test Commands (H2)\n```yaml\n%s```", string(commandsYAML)))
		}
	}

	// H1a: Analyze suitability (conditional: only if feature references a file)
	var targetFiles []string
	requirementsFile := detectRequirementsFile(repoPath, featureDescription)
	if requirementsFile != "" {
		suitResult, suitErr := suitability.AnalyzeSuitability(requirementsFile, repoPath)
		if suitErr != nil {
			sections = append(sections, "### Pre-Implementation Status (H1a)\nSkipped - no requirements file")
		} else if suitResult == nil {
			sections = append(sections, "### Pre-Implementation Status (H1a)\nSkipped - no requirements file")
		} else {
			suitYAML, err := yaml.Marshal(suitResult)
			if err != nil {
				sections = append(sections, "### Pre-Implementation Status (H1a)\nSkipped - no requirements file")
			} else {
				sections = append(sections, fmt.Sprintf("### Pre-Implementation Status (H1a)\n```yaml\n%s```", string(suitYAML)))
				// Extract target files from suitability results
				for _, item := range suitResult.PreImplementation.ItemStatus {
					if item.File != "" {
						targetFiles = append(targetFiles, item.File)
					}
				}
			}
		}
	} else {
		sections = append(sections, "### Pre-Implementation Status (H1a)\nSkipped - no requirements file")
	}

	// H3: Analyze dependencies
	depsResult, depsErr := analyzer.AnalyzeDeps(repoPath, targetFiles)
	if depsErr != nil {
		sections = append(sections, fmt.Sprintf("### Dependency Analysis (H3)\nAnalysis failed: %v", depsErr))
	} else {
		depsYAML, err := yaml.Marshal(depsResult)
		if err != nil {
			sections = append(sections, fmt.Sprintf("### Dependency Analysis (H3)\nAnalysis failed: %v", err))
		} else {
			sections = append(sections, fmt.Sprintf("### Dependency Analysis (H3)\n```yaml\n%s```", string(depsYAML)))
		}
	}

	// Build final automation section
	header := `## Automation Analysis Results

Use these results to:
- Populate quality_gates and test_command from H2 output
- Adjust agent prompts based on H1a status (DONE = add tests, TODO = implement)
- Use H3 wave_candidate field to assign wave numbers
- Document pre-implementation findings in suitability assessment

`
	return header + strings.Join(sections, "\n\n")
}

// detectRequirementsFile checks if the feature description references a
// requirements file (heuristic: contains ".md" or ".txt" substring that looks
// like a file path). Returns the full path if found, empty string otherwise.
func detectRequirementsFile(repoPath string, featureDescription string) string {
	// Heuristic: look for patterns like "audit.md", "requirements.txt", etc.
	patterns := []string{
		`.md`,
		`.txt`,
	}

	for _, pattern := range patterns {
		if strings.Contains(featureDescription, pattern) {
			// Try to extract file path-like token
			words := strings.Fields(featureDescription)
			for _, word := range words {
				if strings.Contains(word, pattern) {
					// Clean up punctuation
					word = strings.Trim(word, ".,;:\"'")
					// Try absolute path first
					if filepath.IsAbs(word) {
						return word
					}
					// Try relative to repo
					candidate := filepath.Join(repoPath, word)
					if _, err := os.Stat(candidate); err == nil {
						return candidate
					}
				}
			}
		}
	}

	return ""
}

// buildProgramContractsSection extracts frozen program contracts from a PROGRAM
// manifest and formats them as a markdown section to prepend to the Scout prompt.
// Only includes contracts whose FreezeAt references an IMPL in a completed tier.
func buildProgramContractsSection(manifest *protocol.PROGRAMManifest, repoPath string) string {
	if manifest == nil || len(manifest.ProgramContracts) == 0 {
		return ""
	}

	// Build a set of completed IMPL slugs by checking each tier
	completedImpls := make(map[string]bool)
	for _, tier := range manifest.Tiers {
		allComplete := true
		for _, implSlug := range tier.Impls {
			// Find the IMPL in manifest.Impls
			var implStatus string
			for _, impl := range manifest.Impls {
				if impl.Slug == implSlug {
					implStatus = impl.Status
					break
				}
			}
			if implStatus != "complete" {
				allComplete = false
				break
			}
		}
		// If all IMPLs in this tier are complete, mark them
		if allComplete {
			for _, implSlug := range tier.Impls {
				completedImpls[implSlug] = true
			}
		}
	}

	// Extract frozen contracts
	var frozenContracts []protocol.ProgramContract
	for _, contract := range manifest.ProgramContracts {
		if contract.FreezeAt != "" {
			// Parse freeze_at format: "impl:<slug>" or just "<slug>"
			freezeSlug := strings.TrimPrefix(contract.FreezeAt, "impl:")
			if completedImpls[freezeSlug] {
				frozenContracts = append(frozenContracts, contract)
			}
		}
	}

	if len(frozenContracts) == 0 {
		return ""
	}

	// Build markdown section
	var sb strings.Builder
	sb.WriteString("## Program Contracts (Frozen — Do Not Redefine)\n\n")
	sb.WriteString("The following types/APIs are defined by the PROGRAM manifest and are frozen. Use them as-is. Do not redefine or modify them.\n\n")

	for _, contract := range frozenContracts {
		sb.WriteString(fmt.Sprintf("### %s\n", contract.Name))
		if contract.Description != "" {
			sb.WriteString(fmt.Sprintf("%s\n\n", contract.Description))
		}
		sb.WriteString(fmt.Sprintf("Location: %s\n", contract.Location))
		sb.WriteString("```go\n")
		sb.WriteString(contract.Definition)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}

// startWaveWithGate runs waves with an inter-wave gate. gateCh receives true to
// proceed or false to abort. Used internally to support wave-by-wave execution
// with external approval gates.
func startWaveWithGate(ctx context.Context, opts RunWaveOpts, onEvent func(Event), gateCh <-chan bool) error {
	publish := func(event string, data interface{}) {
		onEvent(Event{Event: event, Data: data})
	}

	orch, err := orchestrator.New(opts.RepoPath, opts.IMPLPath)
	if err != nil {
		return fmt.Errorf("startWaveWithGate: %w", err)
	}

	orch.SetEventPublisher(func(ev orchestrator.OrchestratorEvent) {
		onEvent(Event{Event: ev.Event, Data: ev.Data})
	})

	waves := orch.IMPLDoc().Waves
	totalAgents := 0
	for _, w := range waves {
		totalAgents += len(w.Agents)
	}

	for i, wave := range waves {
		waveNum := wave.Number

		if err := orch.RunWave(waveNum); err != nil {
			return err
		}
		if err := orch.MergeWave(waveNum); err != nil {
			return err
		}
		testCmd := orch.IMPLDoc().TestCommand
		if testCmd != "" {
			if err := orch.RunVerification(testCmd); err != nil {
				return err
			}
		}

		// E25: Post-wave integration validation
		manifest, loadErr := protocol.Load(opts.IMPLPath)
		if loadErr == nil {
			integrationReport, intErr := protocol.ValidateIntegration(manifest, waveNum, opts.RepoPath)
			if intErr == nil && integrationReport != nil && !integrationReport.Valid {
				publish("integration_gaps_detected", map[string]interface{}{
					"wave":   waveNum,
					"gaps":   len(integrationReport.Gaps),
					"report": integrationReport,
				})

				// E26: Launch integration agent to wire gaps
				intModel := opts.IntegrationModel
				if intModel == "" {
					intModel = opts.WaveModel
				}
				intAgentErr := RunIntegrationAgent(ctx, RunIntegrationAgentOpts{
					IMPLPath: opts.IMPLPath,
					RepoPath: opts.RepoPath,
					WaveNum:  waveNum,
					Report:   integrationReport,
					Model:    intModel,
				}, func(ev Event) { onEvent(ev) })
				if intAgentErr != nil {
					// Non-fatal: log but don't abort wave
					publish("integration_agent_warning", map[string]string{
						"error": intAgentErr.Error(),
					})
				}
			}
		}

		_ = orch.UpdateIMPLStatus(waveNum)

		if i < len(waves)-1 && gateCh != nil {
			nextWave := waves[i+1].Number
			publish("wave_gate_pending", map[string]interface{}{
				"wave":      waveNum,
				"next_wave": nextWave,
				"slug":      opts.Slug,
			})
			select {
			case ok := <-gateCh:
				if !ok {
					publish("run_failed", map[string]string{"error": "gate cancelled"})
					return fmt.Errorf("startWaveWithGate: gate cancelled at wave %d", waveNum)
				}
				publish("wave_gate_resolved", map[string]interface{}{
					"wave":   waveNum,
					"action": "proceed",
					"slug":   opts.Slug,
				})
			case <-time.After(30 * time.Minute):
				publish("run_failed", map[string]string{"error": "gate timed out"})
				return fmt.Errorf("startWaveWithGate: gate timed out at wave %d", waveNum)
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	publish("run_complete", orchestrator.RunCompletePayload{
		Status: "success",
		Waves:  len(waves),
		Agents: totalAgents,
	})
	return nil
}
