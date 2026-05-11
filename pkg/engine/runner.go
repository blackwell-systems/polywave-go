package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/agent"
	"github.com/blackwell-systems/polywave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/polywave-go/pkg/agent/backend/api"
	bedrockbackend "github.com/blackwell-systems/polywave-go/pkg/agent/backend/bedrock"
	"github.com/blackwell-systems/polywave-go/pkg/analyzer"
	"github.com/blackwell-systems/polywave-go/pkg/commands"
	"github.com/blackwell-systems/polywave-go/pkg/hooks"
	"github.com/blackwell-systems/polywave-go/pkg/journal"
	"github.com/blackwell-systems/polywave-go/pkg/observability"
	"github.com/blackwell-systems/polywave-go/pkg/orchestrator"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
	"github.com/blackwell-systems/polywave-go/pkg/retry"
	"github.com/blackwell-systems/polywave-go/pkg/suitability"
	"gopkg.in/yaml.v3"
)

// loggerFrom is a nil-safe helper that returns slog.Default() when l is nil.
// Defined once here for the entire engine package; all other engine files
// (finalize.go, finalize_steps.go, integration_runner.go) call it directly.
func loggerFrom(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}

// RunScout executes a Scout agent, calling onChunk for each output fragment.
// Returns when the agent finishes. Cancellable via ctx.
// I6 enforcement: Post-execution validation checks that Scout only wrote to docs/IMPL/IMPL-*.yaml.
func RunScout(ctx context.Context, opts RunScoutOpts, onChunk func(string)) result.Result[ScoutData] {
	if opts.Feature == "" {
		return result.NewFailure[ScoutData]([]result.PolywaveError{
			result.NewFatal(result.CodeScoutInvalidOpts, "engine.RunScout: Feature is required"),
		})
	}
	if opts.RepoPath == "" {
		return result.NewFailure[ScoutData]([]result.PolywaveError{
			result.NewFatal(result.CodeScoutInvalidOpts, "engine.RunScout: RepoPath is required"),
		})
	}
	if opts.IMPLOutPath == "" {
		return result.NewFailure[ScoutData]([]result.PolywaveError{
			result.NewFatal(result.CodeScoutInvalidOpts, "engine.RunScout: IMPLOutPath is required"),
		})
	}

	// Record start time for I6 validation (detect files written during execution)
	startTime := time.Now()

	// E40: Emit scout_launch after validation passes.
	implSlug := protocol.ExtractIMPLSlug(opts.IMPLOutPath, nil)
	if opts.ObsEmitter != nil {
		if r := opts.ObsEmitter.EmitSync(ctx, observability.NewScoutLaunchEvent(implSlug)); !r.IsSuccess() {
			loggerFrom(opts.Logger).Warn("engine: scout_launch emit failed", "slug", implSlug, "err", r.Errors)
		}
	}

	// Resolve Polywave repo path.
	polywaveRepo := opts.PolywaveRepoPath
	if polywaveRepo == "" {
		polywaveRepo = os.Getenv("POLYWAVE_REPO")
	}
	if polywaveRepo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return result.NewFailure[ScoutData]([]result.PolywaveError{
				result.NewFatal(result.CodeScoutRunFailed, "engine.RunScout: cannot determine home directory").WithCause(err),
			})
		}
		polywaveRepo = filepath.Join(home, "code", "polywave")
	}

	// Load scout.md prompt (L1: no fallback — missing file is a fatal error).
	scoutMdPath := filepath.Join(polywaveRepo, "implementations", "claude-code", "prompts", "agents", "scout.md")
	scoutMdRes := LoadTypePromptWithRefs(scoutMdPath)
	if scoutMdRes.IsFatal() {
		return result.NewFailure[ScoutData]([]result.PolywaveError{
			result.NewFatal(result.CodeScoutRunFailed, fmt.Sprintf("engine.RunScout: scout.md not found at %s", scoutMdPath)),
		})
	}
	scoutMdContent := scoutMdRes.GetData()

	// Run automation tools (H1a, H2, H3) before launching Scout.
	automationContext := runScoutAutomation(ctx, opts.RepoPath, opts.Feature)

	// Load program contracts if ProgramManifestPath is set.
	programContractsSection := ""
	if opts.ProgramManifestPath != "" {
		manifest, parseErr := protocol.ParseProgramManifest(opts.ProgramManifestPath)
		if parseErr != nil {
			// Non-fatal: log warning and continue without program contracts
			loggerFrom(opts.Logger).Warn("engine.RunScout: failed to parse PROGRAM manifest", "err", parseErr)
		} else {
			programContractsSection = buildProgramContractsSection(manifest, opts.RepoPath)
		}
	}

	// E17: Prepend docs/CONTEXT.md if present, so Scout has project memory.
	contextMD := readContextMD(opts.RepoPath)
	var prompt string
	if contextMD != "" {
		prompt = fmt.Sprintf("## Project Memory (docs/CONTEXT.md)\n\n%s\n\n%s%s\n\n## Feature\n%s\n\n%s\n\n## IMPL Output Path\n%s\n",
			contextMD, programContractsSection, scoutMdContent, opts.Feature, automationContext, opts.IMPLOutPath)
	} else {
		prompt = fmt.Sprintf("%s%s\n\n## Feature\n%s\n\n%s\n\n## IMPL Output Path\n%s\n",
			programContractsSection, scoutMdContent, opts.Feature, automationContext, opts.IMPLOutPath)
	}

	var execErr error

	if opts.UseStructuredOutput {
		if strings.HasPrefix(opts.ScoutModel, "bedrock:") {
			_, execErr = runScoutStructuredBedrock(ctx, opts, prompt, onChunk)
		} else {
			_, execErr = runScoutStructured(ctx, opts, prompt, onChunk)
		}
	} else {
		bRes := orchestrator.NewBackendFromModel(opts.ScoutModel)
		if bRes.IsFatal() {
			return result.NewFailure[ScoutData](bRes.Errors)
		}
		b := bRes.GetData()
		runner := agent.NewRunner(b)
		spec := &protocol.Agent{ID: "scout", Task: prompt}
		_, execErr = runner.ExecuteStreamingWithTools(ctx, spec, opts.RepoPath, onChunk, nil)
	}

	if execErr != nil {
		if ctx.Err() != nil {
			return result.NewFailure[ScoutData]([]result.PolywaveError{
				{Code: result.CodeContextCancelled, Message: "engine.RunScout: context cancelled", Severity: "fatal", Cause: execErr},
			})
		}
		return result.NewFailure[ScoutData]([]result.PolywaveError{
			result.NewFatal(result.CodeScoutRunFailed, "engine.RunScout: agent execution failed").WithCause(execErr),
		})
	}

	// I6 enforcement: Validate Scout only wrote to docs/IMPL/IMPL-*.yaml
	validateRes := hooks.ValidateScoutWrites(opts.RepoPath, opts.IMPLOutPath, startTime)
	if validateRes.IsFatal() {
		msg := "engine.RunScout: scout boundary validation failed"
		if len(validateRes.Errors) > 0 {
			msg = "engine.RunScout: " + validateRes.Errors[0].Message
		}
		return result.NewFailure[ScoutData]([]result.PolywaveError{
			result.NewFatal(result.CodeScoutBoundaryViolation, msg),
		})
	}
	if validateRes.IsPartial() {
		data := validateRes.GetData()
		if len(data.UnexpectedWrites) > 0 {
			return result.NewFailure[ScoutData]([]result.PolywaveError{
				result.NewFatal(result.CodeScoutBoundaryViolation,
					fmt.Sprintf("engine.RunScout: I6 VIOLATION: Scout wrote files outside permitted boundaries: %s",
						strings.Join(data.UnexpectedWrites, ", "))),
			})
		}
	}

	// E40: Emit scout_complete on success.
	if opts.ObsEmitter != nil {
		opts.ObsEmitter.Emit(ctx, observability.NewScoutCompleteEvent(implSlug))
	}

	return result.NewSuccess(ScoutData{
		IMPLOutPath: opts.IMPLOutPath,
		Feature:     opts.Feature,
	})
}

// RunPlanner executes a Planner agent to produce a PROGRAM manifest.
// Mirrors RunScout but reads agents/planner.md and writes docs/PROGRAM/PROGRAM-*.yaml.
func RunPlanner(ctx context.Context, opts RunPlannerOpts, onChunk func(string)) result.Result[PlannerData] {
	if opts.Description == "" {
		return result.NewFailure[PlannerData]([]result.PolywaveError{
			result.NewFatal(result.CodePlannerInvalidOpts, "engine.RunPlanner: Description is required"),
		})
	}
	if opts.RepoPath == "" {
		return result.NewFailure[PlannerData]([]result.PolywaveError{
			result.NewFatal(result.CodePlannerInvalidOpts, "engine.RunPlanner: RepoPath is required"),
		})
	}
	if opts.ProgramOutPath == "" {
		return result.NewFailure[PlannerData]([]result.PolywaveError{
			result.NewFatal(result.CodePlannerInvalidOpts, "engine.RunPlanner: ProgramOutPath is required"),
		})
	}

	polywaveRepo := opts.PolywaveRepoPath
	if polywaveRepo == "" {
		polywaveRepo = os.Getenv("POLYWAVE_REPO")
	}
	if polywaveRepo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return result.NewFailure[PlannerData]([]result.PolywaveError{
				result.NewFatal(result.CodePlannerFailed, "engine.RunPlanner: cannot determine home directory").WithCause(err),
			})
		}
		polywaveRepo = filepath.Join(home, "code", "polywave")
	}

	// Load planner.md prompt with fallback.
	plannerMdPath := filepath.Join(polywaveRepo, "implementations", "claude-code", "prompts", "agents", "planner.md")
	plannerMdContent := "You are a Planner agent. Analyze the project requirements and produce a PROGRAM manifest at the specified output path. Decompose the project into features organized into dependency-ordered tiers. Define cross-feature program contracts for shared types/APIs."
	if plannerMdRes := LoadTypePromptWithRefs(plannerMdPath); plannerMdRes.IsSuccess() {
		plannerMdContent = plannerMdRes.GetData()
	}

	// E17: Prepend docs/CONTEXT.md if present.
	contextMD := readContextMD(opts.RepoPath)
	var prompt string
	if contextMD != "" {
		prompt = fmt.Sprintf("## Project Memory (docs/CONTEXT.md)\n\n%s\n\n%s\n\n## Project Description\n%s\n\n## PROGRAM Output Path\n%s\n",
			contextMD, plannerMdContent, opts.Description, opts.ProgramOutPath)
	} else {
		prompt = fmt.Sprintf("%s\n\n## Project Description\n%s\n\n## PROGRAM Output Path\n%s\n",
			plannerMdContent, opts.Description, opts.ProgramOutPath)
	}

	model := opts.PlannerModel
	if model == "" {
		model = "claude-sonnet-4-6"
	}

	bRes := orchestrator.NewBackendFromModel(model)
	if bRes.IsFatal() {
		return result.NewFailure[PlannerData](bRes.Errors)
	}
	b := bRes.GetData()
	runner := agent.NewRunner(b)
	spec := &protocol.Agent{ID: "planner", Task: prompt}
	_, execErr := runner.ExecuteStreamingWithTools(ctx, spec, opts.RepoPath, onChunk, nil)
	if execErr != nil {
		if ctx.Err() != nil {
			return result.NewFailure[PlannerData]([]result.PolywaveError{
				{Code: result.CodeContextCancelled, Message: "engine.RunPlanner: context cancelled", Severity: "fatal", Cause: execErr},
			})
		}
		return result.NewFailure[PlannerData]([]result.PolywaveError{
			result.NewFatal(result.CodePlannerFailed, "engine.RunPlanner: agent execution failed").WithCause(execErr),
		})
	}
	return result.NewSuccess(PlannerData{
		ProgramOutPath: opts.ProgramOutPath,
		Description:    opts.Description,
	})
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
	if saveRes := protocol.Save(ctx, &manifest, opts.IMPLOutPath); saveRes.IsFatal() {
		saveErrMsg := "save failed"
		if len(saveRes.Errors) > 0 {
			saveErrMsg = saveRes.Errors[0].Message
		}
		return nil, fmt.Errorf("runScoutStructured: save manifest: %s", saveErrMsg)
	}

	return &manifest, nil
}

// runScoutStructuredBedrock runs the Scout agent via the Bedrock backend with
// structured output enabled. It is the Bedrock-specific counterpart of
// runScoutStructured (which uses the API backend). The function:
//  1. Strips the "bedrock:" provider prefix from opts.ScoutModel to get the bare model ID.
//  2. Builds a Bedrock client with WithOutputConfig set to the scout schema.
//  3. Calls client.Run() (non-streaming Converse API) so the full JSON is returned at once.
//  4. Unmarshals the JSON response into protocol.IMPLManifest and persists it via protocol.Save(ctx, ).
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
	if saveRes := protocol.Save(ctx, &manifest, opts.IMPLOutPath); saveRes.IsFatal() {
		saveErrMsg := "save failed"
		if len(saveRes.Errors) > 0 {
			saveErrMsg = saveRes.Errors[0].Message
		}
		return nil, fmt.Errorf("runScoutStructuredBedrock: save manifest: %s", saveErrMsg)
	}

	return &manifest, nil
}

// StartWave executes a full wave run (all waves in the IMPL doc).
// Publishes lifecycle events via onEvent. Blocks until all waves complete
// or a fatal error occurs.
func StartWave(ctx context.Context, opts RunWaveOpts, onEvent func(Event)) result.Result[StartWaveData] {
	if opts.IMPLPath == "" {
		return result.NewFailure[StartWaveData]([]result.PolywaveError{
			result.NewFatal(result.CodeWaveInvalidOpts, "engine.StartWave: IMPLPath is required"),
		})
	}
	if opts.RepoPath == "" {
		return result.NewFailure[StartWaveData]([]result.PolywaveError{
			result.NewFatal(result.CodeWaveInvalidOpts, "engine.StartWave: RepoPath is required"),
		})
	}

	publish := func(event string, data interface{}) {
		onEvent(Event{Event: event, Data: data})
	}

	fatalf := func(code, msg string, cause error) result.Result[StartWaveData] {
		publish("run_failed", RunFailedPayload{Error: msg})
		sawErr := result.NewFatal(code, msg)
		if cause != nil {
			sawErr = sawErr.WithCause(cause)
		}
		return result.NewFailure[StartWaveData]([]result.PolywaveError{sawErr})
	}

	publish("run_started", RunStartedPayload{Slug: opts.Slug, IMPLPath: opts.IMPLPath})

	orchRes := orchestrator.New(ctx, opts.RepoPath, opts.IMPLPath)
	if orchRes.IsFatal() {
		return result.NewFailure[StartWaveData](orchRes.Errors)
	}
	orch := orchRes.GetData()

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
	scaffoldRes := RunScaffold(RunScaffoldOpts{
		Ctx:      ctx,
		ImplPath: opts.IMPLPath,
		RepoPath: opts.RepoPath,
		Model:    scaffoldModel,
		OnEvent:  onEvent,
	})
	if scaffoldRes.IsFatal() {
		msg := "engine.StartWave: scaffold failed"
		if len(scaffoldRes.Errors) > 0 {
			msg = "engine.StartWave: scaffold: " + scaffoldRes.Errors[0].Message
		}
		return fatalf(result.CodeWaveFailed, msg, nil)
	}

	waves := orch.IMPLDoc().Waves
	totalAgents := 0
	for _, w := range waves {
		totalAgents += len(w.Agents)
	}

	for i, wave := range waves {
		waveNum := wave.Number

		// Pre-create worktrees via protocol (handles multi-repo from file ownership).
		wtRes := protocol.CreateWorktrees(ctx, opts.IMPLPath, waveNum, opts.RepoPath, nil)
		if !wtRes.IsSuccess() {
			errMsg := fmt.Sprintf("%v", wtRes.Errors)
			return fatalf(result.CodeWaveFailed, fmt.Sprintf("engine.StartWave: CreateWorktrees wave %d: %s", waveNum, errMsg), nil)
		}
		wtResult := wtRes.GetData()
		// Pass pre-computed worktree paths to orchestrator so launchAgent
		// uses the correct (potentially cross-repo) paths.
		wtPaths := make(map[string]string, len(wtResult.Worktrees))
		for _, wt := range wtResult.Worktrees {
			wtPaths[wt.Agent] = wt.Path
		}
		orch.SetWorktreePaths(wtPaths)

		// E20: Post-wave stub scan (informational only).
		if doc := orch.IMPLDoc(); doc != nil {
			manifest, err := protocol.Load(ctx, opts.IMPLPath)
			if err == nil {
				stubReports := make(map[string]*protocol.CompletionReport)
				for _, ag := range wave.Agents {
					if protoReport, ok := manifest.CompletionReports[ag.ID]; ok {
						cr := protoReport // copy to take address
						stubReports[ag.ID] = &cr
					}
				}
				_ = orchestrator.RunStubScan(ctx, opts.IMPLPath, waveNum, stubReports, "", loggerFrom(opts.Logger))
			}
		}

		// E21: Post-wave quality gates before merge.
		if doc := orch.IMPLDoc(); doc != nil && doc.QualityGates != nil {
			gateRes := protocol.RunPreMergeGates(ctx, doc, waveNum, opts.RepoPath, "", nil, nil)
			if gateRes.IsSuccess() || gateRes.IsPartial() {
				gatesData := gateRes.GetData()
				for _, r := range gatesData.Gates {
					onEvent(Event{Event: "quality_gate_result", Data: r})
				}
				for _, gate := range gatesData.Gates {
					if gate.Required && !gate.Passed {
						return fatalf(result.CodeWaveFailed,
							fmt.Sprintf("engine.StartWave: required gate %q failed in wave %d", gate.Type, waveNum), nil)
					}
				}
			} else {
				return fatalf(result.CodeWaveFailed,
					fmt.Sprintf("engine.StartWave: quality gate wave %d: %v", waveNum, gateRes.Errors), nil)
			}
		}

		if err := runOneWave(ctx, orch, opts, waveNum, i, len(waves), publish, nil); err != nil {
			return fatalf(result.CodeWaveFailed, err.Error(), err)
		}
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
	if ctxRes := orchestrator.UpdateContextMD(ctx, opts.RepoPath, entry); ctxRes.IsFatal() {
		// Non-fatal: log but don't abort.
		errMsg := "UpdateContextMD failed"
		if len(ctxRes.Errors) > 0 {
			errMsg = ctxRes.Errors[0].Message
		}
		loggerFrom(opts.Logger).Warn("engine: E18 UpdateContextMD failed", "err", errMsg)
	}

	publish("run_complete", orchestrator.RunCompletePayload{
		Status: "success",
		Waves:  len(waves),
		Agents: totalAgents,
	})
	return result.NewSuccess(StartWaveData{
		Slug:   slug,
		Waves:  len(waves),
		Agents: totalAgents,
	})
}

// RunScaffold checks for pending scaffold files and runs a Scaffold agent if needed.
// The model parameter is optional; if empty, the backend uses its default model.
func RunScaffold(opts RunScaffoldOpts) result.Result[ScaffoldData] {
	ctx := opts.Ctx
	implPath := opts.ImplPath
	repoPath := opts.RepoPath
	polywaveRepoPath := opts.PolywaveRepoPath
	model := opts.Model
	onEvent := opts.OnEvent

	publish := func(event string, data interface{}) {
		onEvent(Event{Event: event, Data: data})
	}

	// Load YAML manifest to get scaffold files.
	manifest, err := protocol.Load(ctx, implPath)
	if err != nil {
		return result.NewFailure[ScaffoldData]([]result.PolywaveError{
			result.NewFatal(result.CodeScaffoldRunFailed, "engine.RunScaffold: load manifest failed").WithCause(err),
		})
	}

	scaffolds := manifest.Scaffolds
	if len(scaffolds) == 0 {
		return result.NewSuccess(ScaffoldData{IMPLPath: implPath, ScaffoldsFound: 0})
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
		return result.NewSuccess(ScaffoldData{IMPLPath: implPath, ScaffoldsFound: len(scaffolds)})
	}

	publish("scaffold_started", ScaffoldStartedPayload{IMPLPath: implPath})

	// Locate scaffold-agent.md prompt.
	polywaveRepo := polywaveRepoPath
	if polywaveRepo == "" {
		polywaveRepo = os.Getenv("POLYWAVE_REPO")
	}
	if polywaveRepo == "" {
		home, _ := os.UserHomeDir()
		polywaveRepo = filepath.Join(home, "code", "polywave")
	}
	scaffoldMdPath := filepath.Join(polywaveRepo, "implementations", "claude-code", "prompts", "agents", "scaffold-agent.md")
	scaffoldMdContent := "You are a Scaffold Agent. Create the stub files defined in the IMPL doc Scaffolds section."
	if scaffoldMdRes := LoadTypePromptWithRefs(scaffoldMdPath); scaffoldMdRes.IsSuccess() {
		scaffoldMdContent = scaffoldMdRes.GetData()
	}

	prompt := fmt.Sprintf("%s\n\n## IMPL Doc Path\n%s\n", scaffoldMdContent, implPath)

	bRes := orchestrator.NewBackendFromModel(model)
	if bRes.IsFatal() {
		publish("scaffold_failed", ScaffoldFailedPayload{Error: bRes.Errors[0].Message})
		return result.NewFailure[ScaffoldData](bRes.Errors)
	}
	b := bRes.GetData()
	runner := agent.NewRunner(b)
	spec := &protocol.Agent{ID: "scaffold", Task: prompt}

	onChunk := func(chunk string) {
		publish("scaffold_output", ScaffoldOutputPayload{Chunk: chunk})
	}

	if _, execErr := runner.ExecuteStreamingWithTools(ctx, spec, repoPath, onChunk, nil); execErr != nil {
		publish("scaffold_failed", ScaffoldFailedPayload{Error: execErr.Error()})
		return result.NewFailure[ScaffoldData]([]result.PolywaveError{
			result.NewFatal(result.CodeScaffoldRunFailed, "engine.RunScaffold: scaffold agent failed").WithCause(execErr),
		})
	}

	// E22: Scaffold build verification — compile to catch scaffold errors early.
	if err := runScaffoldBuildVerification(repoPath, onEvent); err != nil {
		return result.NewFailure[ScaffoldData]([]result.PolywaveError{
			result.NewFatal(result.CodeScaffoldRunFailed, "engine.RunScaffold: build verification failed").WithCause(err),
		})
	}

	publish("scaffold_complete", ScaffoldCompletePayload{IMPLPath: implPath})
	return result.NewSuccess(ScaffoldData{IMPLPath: implPath, ScaffoldsFound: len(scaffolds)})
}

// LoadTypePromptWithRefs reads the agent type prompt at promptPath and
// prepends any reference files from the adjacent references/ directory.
// Reference files must match the pattern "<type-stem>-*.md" where
// type-stem is the base filename without the ".md" extension.
// Each reference file is prepended with a dedup marker:
//
//	<!-- injected: references/<filename> -->
//
// Missing refsDir is not an error; returns core content only.
func LoadTypePromptWithRefs(promptPath string) result.Result[string] {
	coreBytes, err := os.ReadFile(promptPath)
	if err != nil {
		return result.NewFailure[string]([]result.PolywaveError{
			result.NewFatal(result.CodeScoutRunFailed, fmt.Sprintf("LoadTypePromptWithRefs: cannot read prompt file %s: %v", promptPath, err)),
		})
	}
	coreContent := string(coreBytes)

	refsDir := filepath.Join(filepath.Dir(filepath.Dir(promptPath)), "references")
	if _, statErr := os.Stat(refsDir); os.IsNotExist(statErr) {
		return result.NewSuccess(coreContent)
	}

	stem := strings.TrimSuffix(filepath.Base(promptPath), ".md")
	matches, globErr := filepath.Glob(filepath.Join(refsDir, stem+"-*.md"))
	if globErr != nil {
		// Glob only errors on malformed patterns; safe to skip
		return result.NewSuccess(coreContent)
	}

	var injectedParts []string
	for _, refPath := range matches {
		filename := filepath.Base(refPath)
		marker := "<!-- injected: references/" + filename + " -->"

		if strings.Contains(coreContent, marker) {
			continue // already injected — skip to avoid duplication
		}

		refBytes, readErr := os.ReadFile(refPath)
		if readErr != nil {
			slog.Default().Warn("LoadTypePromptWithRefs: cannot read reference file", "path", refPath, "err", readErr)
			continue
		}

		injectedParts = append(injectedParts, marker+"\n"+string(refBytes)+"\n\n")
	}

	if len(injectedParts) == 0 {
		return result.NewSuccess(coreContent)
	}
	return result.NewSuccess(strings.Join(injectedParts, "") + "\n\n" + coreContent)
}

// readContextMD reads docs/CONTEXT.md from repoPath and returns its contents (E17).
// Returns empty string if the file does not exist or cannot be read.
func readContextMD(repoPath string) string {
	p := protocol.ContextMDPath(repoPath)
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
			loggerFrom(nil).Warn("scaffold build: go get ./... (non-fatal)", "err", err, "output", string(out))
		}

		// go mod tidy
		tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
		tidyCmd.Dir = repoPath
		if out, err := tidyCmd.CombinedOutput(); err != nil {
			// Log but don't fail.
			loggerFrom(nil).Warn("scaffold build: go mod tidy (non-fatal)", "err", err, "output", string(out))
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
			loggerFrom(nil).Warn("scaffold build: cargo fetch (non-fatal)", "err", err, "output", string(out))
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
	loggerFrom(nil).Debug("scaffold build verification: unrecognized project type, skipping")
	return nil
}

// RunSingleWave runs the agents for one wave number without merging or verifying.
// The caller is responsible for calling MergeWave and RunVerification afterwards.
func RunSingleWave(ctx context.Context, opts RunWaveOpts, waveNum int, onEvent func(Event)) result.Result[WaveData] {
	if opts.IMPLPath == "" {
		return result.NewFailure[WaveData]([]result.PolywaveError{
			result.NewFatal(result.CodeWaveInvalidOpts, "engine.RunSingleWave: IMPLPath is required"),
		})
	}
	orchRes := orchestrator.New(ctx, opts.RepoPath, opts.IMPLPath)
	if orchRes.IsFatal() {
		return result.NewFailure[WaveData](orchRes.Errors)
	}
	orch := orchRes.GetData()
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
	wtRes := protocol.CreateWorktrees(ctx, opts.IMPLPath, waveNum, opts.RepoPath, nil)
	if !wtRes.IsSuccess() {
		return result.NewFailure[WaveData]([]result.PolywaveError{
			result.NewFatal(result.CodeWaveFailed, fmt.Sprintf("engine.RunSingleWave: create worktrees: %v", wtRes.Errors)),
		})
	}
	wtResult := wtRes.GetData()
	if len(wtResult.Worktrees) > 0 {
		paths := make(map[string]string, len(wtResult.Worktrees))
		for _, wt := range wtResult.Worktrees {
			paths[wt.Agent] = wt.Path
		}
		orch.SetWorktreePaths(paths)
	}

	if runRes := orch.RunWave(ctx, waveNum); runRes.IsFatal() {
		if ctx.Err() != nil {
			return result.NewFailure[WaveData]([]result.PolywaveError{
				{Code: result.CodeContextCancelled, Message: "engine.RunSingleWave: context cancelled", Severity: "fatal"},
			})
		}
		msg := "RunWave failed"
		if len(runRes.Errors) > 0 {
			msg = runRes.Errors[0].Message
		}
		return result.NewFailure[WaveData]([]result.PolywaveError{
			result.NewFatal(result.CodeWaveFailed, msg),
		})
	}
	return result.NewSuccess(WaveData{IMPLPath: opts.IMPLPath, WaveNum: waveNum})
}

// RunSingleAgent runs exactly one agent from the specified wave. This is used
// for single-agent reruns (e.g. retrying a failed agent). If promptPrefix is
// non-empty it is prepended to the agent's task prompt before execution.
func RunSingleAgent(ctx context.Context, opts RunWaveOpts, waveNum int, agentLetter string, promptPrefix string, onEvent func(Event)) result.Result[AgentData] {
	if opts.IMPLPath == "" {
		return result.NewFailure[AgentData]([]result.PolywaveError{
			result.NewFatal(result.CodeAgentRunInvalidOpts, "engine.RunSingleAgent: IMPLPath is required"),
		})
	}
	orchRes := orchestrator.New(ctx, opts.RepoPath, opts.IMPLPath)
	if orchRes.IsFatal() {
		return result.NewFailure[AgentData](orchRes.Errors)
	}
	orch := orchRes.GetData()
	if opts.WaveModel != "" {
		orch.SetDefaultModel(opts.WaveModel)
	}
	orch.SetEventPublisher(func(ev orchestrator.OrchestratorEvent) {
		onEvent(Event{Event: ev.Event, Data: ev.Data})
	})

	// Auto-inject retry context when no explicit promptPrefix is provided
	// and the agent has a prior non-complete completion report.
	if promptPrefix == "" {
		manifest, loadErr := protocol.Load(ctx, opts.IMPLPath)
		if loadErr == nil {
			if report, ok := manifest.CompletionReports[agentLetter]; ok && report.Status != protocol.StatusComplete {
				rcResult := retry.BuildRetryAttempt(ctx, opts.IMPLPath, agentLetter, 1)
				if rcResult.IsFatal() {
					if len(rcResult.Errors) > 0 {
						loggerFrom(opts.Logger).Debug("engine.RunSingleAgent: retry context (best-effort)", "err", rcResult.Errors[0])
					}
				} else {
					rc := rcResult.GetData()
					if rc != nil && rc.PromptText != "" {
						promptPrefix = rc.PromptText
					}
				}
			}
		}
	}

	// Journal context recovery: prepend prior session context if available.
	// Non-fatal: errors are logged and promptPrefix is used unchanged.
	ji := NewJournalIntegration(opts.RepoPath, func(msg string, args ...interface{}) {
		loggerFrom(opts.Logger).Info(msg, args...)
	})
	enrichedPrefix, _, jiErr := ji.PrepareAgentContext(waveNum, agentLetter, promptPrefix)
	if jiErr != nil {
		loggerFrom(opts.Logger).Debug("engine.RunSingleAgent: journal context skipped", "err", jiErr)
	} else {
		promptPrefix = enrichedPrefix
	}

	if agentRes := orch.RunAgent(ctx, waveNum, agentLetter, promptPrefix); agentRes.IsFatal() {
		if ctx.Err() != nil {
			return result.NewFailure[AgentData]([]result.PolywaveError{
				{Code: result.CodeContextCancelled, Message: "engine.RunSingleAgent: context cancelled", Severity: "fatal"},
			})
		}
		msg := "RunAgent failed"
		if len(agentRes.Errors) > 0 {
			msg = agentRes.Errors[0].Message
		}
		return result.NewFailure[AgentData]([]result.PolywaveError{
			result.NewFatal(result.CodeAgentRunFailed, msg),
		})
	}
	return result.NewSuccess(AgentData{IMPLPath: opts.IMPLPath, WaveNum: waveNum, AgentLetter: agentLetter})
}

// MergeWave merges the agent branches for the given wave into the repo's main branch.
// After successful merge, archives journals for all agents in the wave.
func MergeWave(ctx context.Context, opts RunMergeOpts) result.Result[MergeData] {
	if opts.IMPLPath == "" {
		return result.NewFailure[MergeData]([]result.PolywaveError{
			result.NewFatal(result.CodeMergeWaveInvalidOpts, "engine.MergeWave: IMPLPath is required"),
		})
	}
	orchRes := orchestrator.New(ctx, opts.RepoPath, opts.IMPLPath)
	if orchRes.IsFatal() {
		return result.NewFailure[MergeData](orchRes.Errors)
	}
	orch := orchRes.GetData()

	// Merge the wave
	if mergeRes := orch.MergeWave(ctx, opts.WaveNum); mergeRes.IsFatal() {
		msg := "MergeWave failed"
		if len(mergeRes.Errors) > 0 {
			msg = mergeRes.Errors[0].Message
		}
		return result.NewFailure[MergeData]([]result.PolywaveError{
			result.NewFatal(result.CodeMergeWaveFailed, fmt.Sprintf("engine.MergeWave: %s", msg)),
		})
	}

	// Archive journals for all agents in this wave (non-fatal)
	var warnings []result.PolywaveError
	doc := orch.IMPLDoc()
	if doc != nil {
		for _, wave := range doc.Waves {
			if wave.Number == opts.WaveNum {
				for _, ag := range wave.Agents {
					agentPath := fmt.Sprintf("wave%d/agent-%s", opts.WaveNum, ag.ID)
					obsRes := journal.NewObserver(opts.RepoPath, agentPath)
					if obsRes.IsSuccess() {
						observer := obsRes.GetData()
						if archRes := observer.Archive(); archRes.IsFatal() {
							// Log but don't fail the merge
							loggerFrom(opts.Logger).Warn("engine: failed to archive journal for agent", "agent", ag.ID, "err", archRes.Errors[0].Message)
							warnings = append(warnings, result.NewWarning(result.CodeJournalArchiveFailed,
								fmt.Sprintf("failed to archive journal for agent %s: %s", ag.ID, archRes.Errors[0].Message)))
						}
					}
				}
				break
			}
		}
	}

	data := MergeData{IMPLPath: opts.IMPLPath, WaveNum: opts.WaveNum}
	if len(warnings) > 0 {
		return result.NewPartial(data, warnings)
	}
	return result.NewSuccess(data)
}

// RunVerification runs post-merge verification (go vet + test command).
func RunVerification(ctx context.Context, opts RunVerificationOpts) result.Result[VerificationData] {
	testCmd := opts.TestCommand
	if testCmd == "" {
		testCmd = "go test ./..."
	}
	orchRes := orchestrator.New(ctx, opts.RepoPath, "")
	if orchRes.IsFatal() {
		return result.NewFailure[VerificationData](orchRes.Errors)
	}
	orch := orchRes.GetData()
	if verRes := orch.RunVerification(ctx, testCmd); verRes.IsFatal() {
		if ctx.Err() != nil {
			return result.NewFailure[VerificationData]([]result.PolywaveError{
				{Code: result.CodeContextCancelled, Message: "engine.RunVerification: context cancelled", Severity: "fatal"},
			})
		}
		msg := "RunVerification failed"
		if len(verRes.Errors) > 0 {
			msg = verRes.Errors[0].Message
		}
		return result.NewFailure[VerificationData]([]result.PolywaveError{
			result.NewFatal(result.CodeEngineVerificationFailed, msg),
		})
	}
	return result.NewSuccess(VerificationData{RepoPath: opts.RepoPath, TestCommand: testCmd})
}

// UpdateIMPLStatus ticks status checkboxes for completed agents.
// Delegates to pkg/protocol.UpdateIMPLStatus.
func UpdateIMPLStatus(implDocPath string, completedLetters []string) result.Result[UpdateData] {
	res := protocol.UpdateIMPLStatus(implDocPath, completedLetters)
	if res.IsFatal() {
		msg := "engine.UpdateIMPLStatus: update failed"
		if len(res.Errors) > 0 {
			msg = res.Errors[0].Message
		}
		return result.NewFailure[UpdateData]([]result.PolywaveError{
			result.NewFatal(result.CodeUpdateStatusFailed, msg),
		})
	}
	return result.NewSuccess(UpdateData{
		IMPLDocPath:      implDocPath,
		CompletedLetters: completedLetters,
	})
}

// ValidateInvariants validates disjoint file ownership invariants.
// Delegates to pkg/protocol.ValidateInvariants.
func ValidateInvariants(manifest *protocol.IMPLManifest) result.Result[ValidateData] {
	if errs := protocol.ValidateInvariants(manifest); len(errs) > 0 {
		return result.NewFailure[ValidateData](errs)
	}
	return result.NewSuccess(ValidateData{Valid: true})
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
	obsRes := journal.NewObserver(ji.repoPath, agentPath)
	if obsRes.IsFatal() {
		ji.logger("Failed to create journal observer", "agent", agentID, "error", obsRes.Errors[0].Message)
		return originalPrompt, nil, fmt.Errorf("create journal observer: %s", obsRes.Errors[0].Message)
	}
	observer = obsRes.GetData()

	// Sync from Claude Code session logs (incremental tail)
	syncRes := observer.Sync()
	if syncRes.IsFatal() {
		ji.logger("Failed to sync journal", "agent", agentID, "error", syncRes.Errors[0].Message)
		// Non-fatal: continue without journal recovery
		return originalPrompt, observer, nil
	}

	// If journal has events, generate context and prepend to prompt
	if syncRes.GetData() != nil && syncRes.GetData().NewToolUses > 0 {
		ctxRes := observer.GenerateContext()
		if ctxRes.IsFatal() {
			ji.logger("Failed to generate context", "agent", agentID, "error", ctxRes.Errors[0].Message)
			return originalPrompt, observer, nil
		}
		contextMd := ctxRes.GetData()

		ji.logger("Recovered session context", "agent", agentID, "tool_calls", syncRes.GetData().NewToolUses)
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
				if sr := observer.Sync(); sr.IsFatal() {
					ji.logger("Periodic sync failed", "error", sr.Errors[0].Message)
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

	if r := observer.Checkpoint(name); r.IsFatal() {
		ji.logger("Checkpoint failed", "name", name, "error", r.Errors[0].Message)
		return fmt.Errorf("checkpoint %s: %s", name, r.Errors[0].Message)
	}

	ji.logger("Checkpoint created", "name", name)
	return nil
}

// ArchiveJournal compresses the journal directory to .tar.gz after wave merge succeeds.
func (ji *JournalIntegration) ArchiveJournal(observer *journal.JournalObserver) error {
	if observer == nil {
		return nil // no-op if observer wasn't created
	}

	if r := observer.Archive(); r.IsFatal() {
		ji.logger("Archive failed", "error", r.Errors[0].Message)
		return fmt.Errorf("archive journal: %s", r.Errors[0].Message)
	}

	ji.logger("Journal archived")
	return nil
}

// runScoutAutomation runs the 5 scout automation tools (H1a, H2, H3) and
// returns a markdown section to inject into the Scout prompt. All tools are
// best-effort: failures are logged but don't block Scout execution.
func runScoutAutomation(ctx context.Context, repoPath string, featureDescription string) string {
	var sections []string

	// H2: Extract build/test/lint commands
	commandsResult := commands.ExtractCommands(ctx, repoPath)
	if commandsResult.IsFatal() {
		sections = append(sections, "### Build/Test Commands (H2)\nNot detected")
	} else {
		// Cannot use protocol.SaveYAML: marshaling to []byte for inline string formatting, not to a file.
		commandsYAML, err := yaml.Marshal(commandsResult.GetData().CommandSet)
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
		suitResult := suitability.AnalyzeSuitability(requirementsFile, repoPath)
		if suitResult.IsFatal() {
			sections = append(sections, "### Pre-Implementation Status (H1a)\nSkipped - no requirements file")
		} else if suitResult.IsSuccess() {
			data := suitResult.GetData()
			// Cannot use protocol.SaveYAML: marshaling to []byte for inline string formatting, not to a file.
			suitYAML, err := yaml.Marshal(data)
			if err != nil {
				sections = append(sections, "### Pre-Implementation Status (H1a)\nSkipped - no requirements file")
			} else {
				sections = append(sections, fmt.Sprintf("### Pre-Implementation Status (H1a)\n```yaml\n%s```", string(suitYAML)))
				// Extract target files from suitability results
				for _, item := range data.PreImplementation.ItemStatus {
					if item.File != "" {
						targetFiles = append(targetFiles, item.File)
					}
				}
			}
		} else {
			sections = append(sections, "### Pre-Implementation Status (H1a)\nSkipped - no requirements file")
		}
	} else {
		sections = append(sections, "### Pre-Implementation Status (H1a)\nSkipped - no requirements file")
	}

	// H3: Analyze dependencies
	// Baseline hotfix: AnalyzeDeps deleted in analyzer-review-fixes; Agent I (wave 4) owns final form.
	var depsResult *analyzer.Output
	var depsErr error
	graphResult := analyzer.BuildGraph(ctx, repoPath, targetFiles)
	if graphResult.IsFatal() {
		depsErr = fmt.Errorf("%s", graphResult.Errors[0].Message)
	} else {
		depsResult = analyzer.ToOutput(graphResult.GetData())
	}
	_ = depsErr // used below
	if depsErr != nil {
		sections = append(sections, fmt.Sprintf("### Dependency Analysis (H3)\nAnalysis failed: %v", depsErr))
	} else {
		// Cannot use protocol.SaveYAML: marshaling to []byte for inline string formatting, not to a file.
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

// runOneWave is the per-wave body shared by StartWave and startWaveWithGate.
// publish forwards event notifications. waveIdx is the 0-based index into the
// waves slice; totalWaves is len(waves). gateCh is nil when called from StartWave.
// When non-nil, after each wave except the last (waveIdx < totalWaves-1),
// runOneWave waits for a bool from gateCh: true = proceed, false = abort.
func runOneWave(
	ctx context.Context,
	orch *orchestrator.Orchestrator,
	opts RunWaveOpts,
	waveNum int,
	waveIdx int,
	totalWaves int,
	publish func(event string, data interface{}),
	gateCh <-chan bool,
) error {
	if runRes := orch.RunWave(ctx, waveNum); runRes.IsFatal() {
		errMsg := "RunWave failed"
		if len(runRes.Errors) > 0 {
			errMsg = runRes.Errors[0].Message
		}
		return fmt.Errorf("engine.runOneWave: RunWave %d: %s", waveNum, errMsg)
	}

	if mergeRes := orch.MergeWave(ctx, waveNum); mergeRes.IsFatal() {
		errMsg := "MergeWave failed"
		if len(mergeRes.Errors) > 0 {
			errMsg = mergeRes.Errors[0].Message
		}
		return fmt.Errorf("engine.runOneWave: MergeWave %d: %s", waveNum, errMsg)
	}

	testCmd := orch.IMPLDoc().TestCommand
	if testCmd != "" {
		if verRes := orch.RunVerification(ctx, testCmd); verRes.IsFatal() {
			errMsg := "RunVerification failed"
			if len(verRes.Errors) > 0 {
				errMsg = verRes.Errors[0].Message
			}
			return fmt.Errorf("engine.runOneWave: RunVerification %d: %s", waveNum, errMsg)
		}
	}

	// E25: Post-wave integration validation
	manifest, loadErr := protocol.Load(ctx, opts.IMPLPath)
	if loadErr == nil {
		integrationReport, intErr := protocol.ValidateIntegration(manifest, waveNum, opts.RepoPath)
		if intErr == nil && integrationReport != nil && !integrationReport.Valid {
			publish("integration_gaps_detected", IntegrationGapsPayload{
				Wave:   waveNum,
				Gaps:   len(integrationReport.Gaps),
				Report: integrationReport,
			})

			// E26: Launch integration agent to wire gaps
			intModel := opts.IntegrationModel
			if intModel == "" {
				intModel = opts.WaveModel
			}
			intAgentRes := RunIntegrationAgent(ctx, RunIntegrationAgentOpts{
				IMPLPath: opts.IMPLPath,
				RepoPath: opts.RepoPath,
				WaveNum:  waveNum,
				Report:   integrationReport,
				Model:    intModel,
			}, func(ev Event) {
				publish(ev.Event, ev.Data)
			})
			if intAgentRes.IsFatal() {
				// Non-fatal: log but don't abort wave
				msg := "integration agent failed"
				if len(intAgentRes.Errors) > 0 {
					msg = intAgentRes.Errors[0].Message
				}
				publish("integration_agent_warning", IntegrationAgentWarningPayload{Error: msg})
			}
		}
	}

	if statusRes := orch.UpdateIMPLStatus(ctx, waveNum); statusRes.IsFatal() {
		// Non-fatal: log but don't abort.
		errMsg := "UpdateIMPLStatus failed"
		if len(statusRes.Errors) > 0 {
			errMsg = statusRes.Errors[0].Message
		}
		publish("update_status_failed", UpdateStatusFailedPayload{Slug: opts.Slug, Error: errMsg})
	}

	// Gate wait: pause between waves when gateCh is provided.
	if waveIdx < totalWaves-1 && gateCh != nil {
		publish("wave_gate_pending", WaveGatePendingPayload{Wave: waveNum, NextWave: waveNum + 1, Slug: opts.Slug})
		select {
		case ok := <-gateCh:
			if !ok {
				publish("run_failed", RunFailedPayload{Error: "gate cancelled"})
				return fmt.Errorf("startWaveWithGate: gate cancelled at wave %d", waveNum)
			}
			publish("wave_gate_resolved", WaveGateResolvedPayload{Wave: waveNum, Action: "proceed", Slug: opts.Slug})
		case <-time.After(30 * time.Minute):
			publish("run_failed", RunFailedPayload{Error: "gate timed out"})
			return fmt.Errorf("startWaveWithGate: gate timed out at wave %d", waveNum)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

