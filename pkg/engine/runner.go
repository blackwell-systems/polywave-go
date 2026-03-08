package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/cli"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

// RunScout executes a Scout agent, calling onChunk for each output fragment.
// Returns when the agent finishes. Cancellable via ctx.
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

	// Load scout.md prompt with fallback.
	scoutMdPath := filepath.Join(sawRepo, "implementations", "claude-code", "prompts", "scout.md")
	scoutMdBytes, err := os.ReadFile(scoutMdPath)
	if err != nil {
		scoutMdBytes = []byte("You are a Scout agent. Analyze the codebase and produce an IMPL doc.")
	}

	// E17: Prepend docs/CONTEXT.md if present, so Scout has project memory.
	contextMD := readContextMD(opts.RepoPath)
	var prompt string
	if contextMD != "" {
		prompt = fmt.Sprintf("## Project Memory (docs/CONTEXT.md)\n\n%s\n\n%s\n\n## Feature\n%s\n\n## IMPL Output Path\n%s\n",
			contextMD, string(scoutMdBytes), opts.Feature, opts.IMPLOutPath)
	} else {
		prompt = fmt.Sprintf("%s\n\n## Feature\n%s\n\n## IMPL Output Path\n%s\n",
			string(scoutMdBytes), opts.Feature, opts.IMPLOutPath)
	}

	b := cli.New("", backend.Config{})
	runner := agent.NewRunner(b, nil)
	spec := &types.AgentSpec{Letter: "scout", Prompt: prompt}

	_, execErr := runner.ExecuteStreaming(ctx, spec, opts.RepoPath, onChunk)
	return execErr
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

	// Wire orchestrator events to the onEvent callback.
	orch.SetEventPublisher(func(ev orchestrator.OrchestratorEvent) {
		onEvent(Event{Event: ev.Event, Data: ev.Data})
	})

	// Run scaffold if needed.
	if err := RunScaffold(ctx, opts.IMPLPath, opts.RepoPath, "", onEvent); err != nil {
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

		if err := orch.RunWave(waveNum); err != nil {
			publish("run_failed", map[string]string{"error": err.Error()})
			return fmt.Errorf("engine.StartWave: RunWave %d: %w", waveNum, err)
		}

		// E20: Post-wave stub scan (informational only).
		if doc := orch.IMPLDoc(); doc != nil {
			stubReports := make(map[string]*types.CompletionReport)
			for _, ag := range wave.Agents {
				if r, err := protocol.ParseCompletionReport(opts.IMPLPath, ag.Letter); err == nil {
					stubReports[ag.Letter] = r
				}
			}
			_ = orchestrator.RunStubScan(opts.IMPLPath, waveNum, stubReports, "")
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
func RunScaffold(ctx context.Context, implPath, repoPath, sawRepoPath string, onEvent func(Event)) error {
	publish := func(event string, data interface{}) {
		onEvent(Event{Event: event, Data: data})
	}

	// Parse IMPL doc to get scaffold files.
	doc, err := protocol.ParseIMPLDoc(implPath)
	if err != nil {
		return fmt.Errorf("engine.RunScaffold: parse IMPL doc: %w", err)
	}

	scaffolds := doc.ScaffoldsDetail
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

	b := cli.New("", backend.Config{})
	runner := agent.NewRunner(b, nil)
	spec := &types.AgentSpec{Letter: "scaffold", Prompt: prompt}

	onChunk := func(chunk string) {
		publish("scaffold_output", map[string]string{"chunk": chunk})
	}

	if _, execErr := runner.ExecuteStreaming(ctx, spec, repoPath, onChunk); execErr != nil {
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

// runScaffoldBuildVerification runs `go build ./...` in repoPath to catch
// scaffold errors early (E22). If no go.mod exists in repoPath, returns nil
// (skip silently — not a Go project).
func runScaffoldBuildVerification(repoPath string, onEvent func(Event)) error {
	// Guard: only run for Go projects.
	if _, err := os.Stat(filepath.Join(repoPath, "go.mod")); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build ./... failed: %w\n%s", err, string(out))
	}
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
	orch.SetEventPublisher(func(ev orchestrator.OrchestratorEvent) {
		onEvent(Event{Event: ev.Event, Data: ev.Data})
	})
	return orch.RunWave(waveNum)
}

// MergeWave merges the agent branches for the given wave into the repo's main branch.
func MergeWave(ctx context.Context, opts RunMergeOpts) error {
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.MergeWave: IMPLPath is required")
	}
	orch, err := orchestrator.New(opts.RepoPath, opts.IMPLPath)
	if err != nil {
		return fmt.Errorf("engine.MergeWave: %w", err)
	}
	return orch.MergeWave(opts.WaveNum)
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

// ParseIMPLDoc parses an IMPL doc and returns the structured representation.
// Delegates to pkg/protocol.ParseIMPLDoc.
func ParseIMPLDoc(path string) (*types.IMPLDoc, error) {
	return protocol.ParseIMPLDoc(path)
}

// ParseCompletionReport parses an agent's completion report from the IMPL doc.
// Delegates to pkg/protocol.ParseCompletionReport.
// Maps protocol.ErrReportNotFound to engine.ErrReportNotFound so callers can
// use errors.Is(err, engine.ErrReportNotFound) without importing pkg/protocol.
func ParseCompletionReport(implDocPath, agentLetter string) (*types.CompletionReport, error) {
	report, err := protocol.ParseCompletionReport(implDocPath, agentLetter)
	if errors.Is(err, protocol.ErrReportNotFound) {
		return nil, ErrReportNotFound
	}
	return report, err
}

// UpdateIMPLStatus ticks status checkboxes for completed agents.
// Delegates to pkg/protocol.UpdateIMPLStatus.
func UpdateIMPLStatus(implDocPath string, completedLetters []string) error {
	return protocol.UpdateIMPLStatus(implDocPath, completedLetters)
}

// ValidateInvariants validates disjoint file ownership invariants.
// Delegates to pkg/protocol.ValidateInvariants.
func ValidateInvariants(doc *types.IMPLDoc) error {
	return protocol.ValidateInvariants(doc)
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
