// Package orchestrator drives SAW protocol execution: it advances the
// 10-state machine, creates per-agent git worktrees, launches agents
// concurrently via the Anthropic API, merges completed worktrees, runs
// post-merge verification, and updates the IMPL doc status table.
// State mutations always go through TransitionTo — never set state directly.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	cliclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/cli"
	openaibackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/openai"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/worktree"
)

func init() {
	SetParseIMPLDocFunc(protocol.ParseIMPLDoc)
	SetValidateInvariantsFunc(protocol.ValidateInvariants)
}

// defaultAgentTimeout is the maximum time RunWave waits per agent for a
// completion report. Package-level so tests can lower it.
var defaultAgentTimeout = 30 * time.Minute

// defaultAgentPollInterval is how often RunWave polls for completion reports.
var defaultAgentPollInterval = 10 * time.Second

// parseIMPLDocFunc is replaced at runtime by pkg/protocol once that package
// is compiled. The default no-op implementation returns an empty IMPLDoc so
// that the orchestrator package compiles independently during Wave 1 parallel
// execution (Agent A owns pkg/protocol and may not be merged yet).
var parseIMPLDocFunc = func(path string) (*types.IMPLDoc, error) {
	return &types.IMPLDoc{}, nil
}

// validateInvariantsFunc is replaced by pkg/protocol via SetValidateInvariantsFunc.
// Default no-op for Wave 1 compilation.
var validateInvariantsFunc = func(doc *types.IMPLDoc) error { return nil }

// SetValidateInvariantsFunc allows pkg/protocol to inject the real implementation
// without a direct import cycle.
func SetValidateInvariantsFunc(f func(doc *types.IMPLDoc) error) {
	validateInvariantsFunc = f
}

// mergeWaveFunc is replaced by merge.go via init().
// Default no-op for compilation.
var mergeWaveFunc = func(o *Orchestrator, waveNum int) error { return nil }

// runVerificationFunc is replaced by verification.go via init().
// Default no-op for compilation.
var runVerificationFunc = func(o *Orchestrator, testCommand string) error { return nil }

// worktreeCreatorFunc is a seam for tests: it creates a worktree for wave/agent
// and returns the worktree path. Tests can replace this to avoid real git ops.
var worktreeCreatorFunc = func(wm *worktree.Manager, waveNum int, agentLetter string) (string, error) {
	return wm.Create(waveNum, agentLetter)
}

// waitForCompletionFunc is a seam for tests: wraps agent.WaitForCompletion.
var waitForCompletionFunc = func(implDocPath, agentLetter string, timeout, pollInterval time.Duration) (*types.CompletionReport, error) {
	return agent.WaitForCompletion(implDocPath, agentLetter, timeout, pollInterval)
}

// BackendConfig carries backend selection + credentials for newBackendFunc.
type BackendConfig struct {
	Kind      string // "api" | "cli" | "auto" | "openai" | "anthropic"
	APIKey    string
	Model     string
	MaxTokens int
	MaxTurns  int

	// OpenAIKey is the API key for the OpenAI-compatible backend.
	// Falls back to OPENAI_API_KEY env var if empty.
	OpenAIKey string

	// BaseURL is an optional endpoint override used when Kind == "openai"
	// or when the provider prefix is "openai".
	BaseURL string
}

// parseProviderPrefix splits a provider-qualified model string.
// Input "openai:gpt-4o" returns ("openai", "gpt-4o").
// Input "cli:kimi" returns ("cli", "kimi").
// Input "anthropic:claude-opus-4-6" returns ("anthropic", "claude-opus-4-6").
// Input "gpt-4o" (no colon) returns ("", "gpt-4o").
func parseProviderPrefix(model string) (provider, bareModel string) {
	idx := strings.Index(model, ":")
	if idx < 0 {
		return "", model
	}
	return model[:idx], model[idx+1:]
}

// newBackendFunc constructs a backend.Backend from config. Seam for tests.
var newBackendFunc = func(cfg BackendConfig) (backend.Backend, error) {
	// Parse any provider prefix from the model string (e.g. "openai:gpt-4o").
	provider, bareModel := parseProviderPrefix(cfg.Model)

	// Determine effective kind: explicit prefix overrides cfg.Kind.
	effectiveKind := cfg.Kind
	if provider != "" {
		effectiveKind = provider
	}

	bcfg := backend.Config{
		Model:     bareModel,
		MaxTokens: cfg.MaxTokens,
		MaxTurns:  cfg.MaxTurns,
	}
	switch effectiveKind {
	case "openai":
		apiKey := cfg.OpenAIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		return openaibackend.New(backend.Config{
			Model:     bareModel,
			MaxTokens: cfg.MaxTokens,
			MaxTurns:  cfg.MaxTurns,
			APIKey:    apiKey,
			BaseURL:   cfg.BaseURL,
		}), nil
	case "anthropic":
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		return apiclient.New(apiKey, backend.Config{
			Model:     bareModel,
			MaxTokens: cfg.MaxTokens,
			MaxTurns:  cfg.MaxTurns,
		}), nil
	case "api":
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		return apiclient.New(apiKey, bcfg), nil
	case "cli":
		binaryPath := os.Getenv("SAW_CLI_BINARY")
		return cliclient.New(binaryPath, backend.Config{
			Model:      bareModel,
			MaxTokens:  cfg.MaxTokens,
			MaxTurns:   cfg.MaxTurns,
			BinaryPath: binaryPath,
		}), nil
	case "auto", "":
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey != "" {
			return apiclient.New(apiKey, bcfg), nil
		}
		return cliclient.New("", bcfg), nil
	default:
		return nil, fmt.Errorf("orchestrator: unknown backend kind %q; valid values: api, cli, auto, openai, anthropic", effectiveKind)
	}
}

// newRunnerFunc is a seam for tests: constructs the agent.Runner used by RunWave.
// Tests can replace this to inject a fake Backend without real API calls.
var newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
	return agent.NewRunner(b, wm)
}

// Orchestrator drives SAW protocol wave coordination.
// State mutations must go through TransitionTo — never set o.state directly.
type Orchestrator struct {
	state          types.State
	implDoc        *types.IMPLDoc
	repoPath       string
	currentWave    int
	implDocPath    string
	eventPublisher EventPublisher
	defaultModel   string // optional default model for wave agents (e.g. "claude-haiku-4-5")
}

// SetDefaultModel sets the fallback model used for wave agents that have no
// per-agent model: field in the IMPL doc. Empty string means use the CLI/API default.
func (o *Orchestrator) SetDefaultModel(model string) {
	o.defaultModel = model
}

// publish sends ev to the registered EventPublisher, if any.
// It is a no-op when no publisher has been set.
func (o *Orchestrator) publish(ev OrchestratorEvent) {
	if o.eventPublisher != nil {
		o.eventPublisher(ev)
	}
}

// New creates an Orchestrator by loading the IMPL doc at implDocPath.
// Initial state is ScoutPending.
func New(repoPath string, implDocPath string) (*Orchestrator, error) {
	doc, err := parseIMPLDocFunc(implDocPath)
	if err != nil {
		return nil, fmt.Errorf("orchestrator.New: failed to parse IMPL doc %q: %w", implDocPath, err)
	}
	return &Orchestrator{
		state:       types.ScoutPending,
		implDoc:     doc,
		repoPath:    repoPath,
		implDocPath: implDocPath,
	}, nil
}

// newFromDoc creates an Orchestrator directly from a pre-parsed IMPLDoc.
// Used in tests to avoid the pkg/protocol dependency.
func newFromDoc(doc *types.IMPLDoc, repoPath, implDocPath string) *Orchestrator {
	return &Orchestrator{
		state:       types.ScoutPending,
		implDoc:     doc,
		repoPath:    repoPath,
		implDocPath: implDocPath,
	}
}

// State returns the current protocol state.
func (o *Orchestrator) State() types.State {
	return o.state
}

// IMPLDoc returns the parsed IMPL document.
func (o *Orchestrator) IMPLDoc() *types.IMPLDoc {
	return o.implDoc
}

// RepoPath returns the repository root path.
func (o *Orchestrator) RepoPath() string {
	return o.repoPath
}

// TransitionTo advances the state machine to newState.
// It returns a descriptive error if the transition is not permitted.
func (o *Orchestrator) TransitionTo(newState types.State) error {
	if !isValidTransition(o.state, newState) {
		return fmt.Errorf(
			"orchestrator: invalid state transition from %s to %s",
			o.state, newState,
		)
	}
	o.state = newState
	return nil
}

// RunWave executes all agents in wave waveNum concurrently. Each agent receives
// its own git worktree and the backend handles all LLM interaction internally.
// RunWave blocks until all agents complete (or one fails), then returns.
func (o *Orchestrator) RunWave(waveNum int) error {
	if o.implDoc == nil {
		return fmt.Errorf("orchestrator.RunWave: no IMPL doc loaded")
	}
	// I1: Validate disjoint file ownership before any worktrees are created.
	if err := validateInvariantsFunc(o.implDoc); err != nil {
		return fmt.Errorf("orchestrator.RunWave: invariant violation: %w", err)
	}
	// Find the wave in the doc.
	var wave *types.Wave
	for i := range o.implDoc.Waves {
		if o.implDoc.Waves[i].Number == waveNum {
			wave = &o.implDoc.Waves[i]
			break
		}
	}
	if wave == nil && len(o.implDoc.Waves) > 0 {
		return fmt.Errorf("orchestrator.RunWave: wave %d not found in IMPL doc", waveNum)
	}
	o.currentWave = waveNum

	// Nothing to do if there are no waves defined.
	if wave == nil {
		return nil
	}

	// Build the worktree manager and default agent runner.
	wm := worktree.New(o.repoPath)
	defaultBackend, err := newBackendFunc(BackendConfig{Kind: "auto", Model: o.defaultModel})
	if err != nil {
		return fmt.Errorf("orchestrator.RunWave: failed to create backend: %w", err)
	}
	defaultRunner := newRunnerFunc(defaultBackend, wm)

	// Launch all agents concurrently and collect the first error.
	eg, ctx := errgroup.WithContext(context.Background())

	for _, spec := range wave.Agents {
		agentSpec := spec // capture loop variable
		eg.Go(func() error {
			runner := defaultRunner
			// Per-agent model override: create a separate backend for this agent.
			if agentSpec.Model != "" && agentSpec.Model != o.defaultModel {
				b2, err2 := newBackendFunc(BackendConfig{Kind: "auto", Model: agentSpec.Model})
				if err2 != nil {
					return fmt.Errorf("orchestrator: agent %s: create backend: %w", agentSpec.Letter, err2)
				}
				runner = newRunnerFunc(b2, wm)
			}
			return o.launchAgent(ctx, runner, wm, waveNum, agentSpec)
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	// All agents in the wave completed successfully.
	o.publish(OrchestratorEvent{
		Event: "wave_complete",
		Data: WaveCompletePayload{
			Wave:        waveNum,
			MergeStatus: "pending",
		},
	})

	return nil
}

// wtIMPLPath derives the IMPL doc path inside the agent's worktree.
// implDocPath is the main repo IMPL doc path (e.g. /repo/docs/IMPL/IMPL-foo.md).
// wtPath is the worktree root (e.g. /repo/.claude/worktrees/wave1-agent-A).
// repoPath is the repo root (e.g. /repo).
// Result: /repo/.claude/worktrees/wave1-agent-A/docs/IMPL/IMPL-foo.md
func wtIMPLPath(repoPath, implDocPath, wtPath string) string {
	rel, err := filepath.Rel(repoPath, implDocPath)
	if err != nil {
		return implDocPath // fallback to main repo path
	}
	return filepath.Join(wtPath, rel)
}

// AgentBlockedPayload is the event data published when an agent reports partial or blocked status (E19).
type AgentBlockedPayload struct {
	Agent       string             `json:"agent"`
	Wave        int                `json:"wave"`
	Status      string             `json:"status"`
	FailureType string             `json:"failure_type"`
	Action      OrchestratorAction `json:"action"`
}

// launchAgent creates a worktree for one agent, calls Execute, then
// polls WaitForCompletion. Returns the first non-nil error encountered.
func (o *Orchestrator) launchAgent(
	ctx context.Context,
	runner *agent.Runner,
	wm *worktree.Manager,
	waveNum int,
	agentSpec types.AgentSpec,
) error {
	// a. Create the worktree.
	wtPath, err := worktreeCreatorFunc(wm, waveNum, agentSpec.Letter)
	if err != nil {
		o.publish(OrchestratorEvent{
			Event: "agent_failed",
			Data: AgentFailedPayload{
				Agent:       agentSpec.Letter,
				Wave:        waveNum,
				Status:      "failed",
				FailureType: "worktree_creation",
				Message:     err.Error(),
			},
		})
		return fmt.Errorf("orchestrator: agent %s: create worktree: %w", agentSpec.Letter, err)
	}

	// Publish agent_started after the worktree is ready.
	o.publish(OrchestratorEvent{
		Event: "agent_started",
		Data: AgentStartedPayload{
			Agent: agentSpec.Letter,
			Wave:  waveNum,
			Files: agentSpec.FilesOwned,
		},
	})

	// E23: Construct per-agent context payload instead of passing full IMPL doc prompt.
	if payload, err := protocol.ExtractAgentContext(o.implDocPath, agentSpec.Letter); err == nil {
		agentSpec.Prompt = protocol.FormatAgentContextPayload(payload)
	} else {
		// Fallback: use existing prompt from agentSpec (already set from IMPL doc parse).
		fmt.Fprintf(os.Stderr, "orchestrator: E23 context extraction failed for agent %s: %v (falling back to full prompt)\n", agentSpec.Letter, err)
	}

	// b. Execute the agent via the backend, streaming output chunks as SSE events.
	if _, err := runner.ExecuteStreaming(ctx, &agentSpec, wtPath, func(chunk string) {
		o.publish(OrchestratorEvent{
			Event: "agent_output",
			Data: AgentOutputPayload{
				Agent: agentSpec.Letter,
				Wave:  waveNum,
				Chunk: chunk,
			},
		})
	}); err != nil {
		o.publish(OrchestratorEvent{
			Event: "agent_failed",
			Data: AgentFailedPayload{
				Agent:       agentSpec.Letter,
				Wave:        waveNum,
				Status:      "failed",
				FailureType: "execute",
				Message:     err.Error(),
			},
		})
		return fmt.Errorf("orchestrator: agent %s: Execute: %w", agentSpec.Letter, err)
	}

	// c. Poll for the completion report in the agent's worktree IMPL doc.
	// Agents write their completion reports into the worktree copy of the IMPL doc,
	// not the main repo copy — so we must poll wtIMPLPath, not o.implDocPath.
	report, err := waitForCompletionFunc(wtIMPLPath(o.repoPath, o.implDocPath, wtPath), agentSpec.Letter, defaultAgentTimeout, defaultAgentPollInterval)
	if err != nil {
		o.publish(OrchestratorEvent{
			Event: "agent_failed",
			Data: AgentFailedPayload{
				Agent:       agentSpec.Letter,
				Wave:        waveNum,
				Status:      "failed",
				FailureType: "completion_timeout",
				Message:     err.Error(),
			},
		})
		return fmt.Errorf("orchestrator: agent %s: %w", agentSpec.Letter, err)
	}

	// Publish agent_complete after a successful completion report.
	status := ""
	if report != nil {
		status = string(report.Status)
	}
	o.publish(OrchestratorEvent{
		Event: "agent_complete",
		Data: AgentCompletePayload{
			Agent:  agentSpec.Letter,
			Wave:   waveNum,
			Status: status,
			Branch: fmt.Sprintf("saw/wave%d-agent-%s", waveNum, agentSpec.Letter),
		},
	})

	// E19: If agent reported partial or blocked, route the failure and publish an event.
	// This does NOT relaunch the agent — that is a future follow-on task.
	if report != nil && (report.Status == types.StatusPartial || report.Status == types.StatusBlocked) {
		action := RouteFailure(report.FailureType)
		o.publish(OrchestratorEvent{
			Event: "agent_blocked",
			Data: AgentBlockedPayload{
				Agent:       agentSpec.Letter,
				Wave:        waveNum,
				Status:      string(report.Status),
				FailureType: string(report.FailureType),
				Action:      action,
			},
		})
	}

	return nil
}

// MergeWave merges the worktrees for wave waveNum.
// Implementation is provided by merge.go via mergeWaveFunc.
func (o *Orchestrator) MergeWave(waveNum int) error {
	return mergeWaveFunc(o, waveNum)
}

// RunVerification runs the post-merge test command.
// Implementation is provided by verification.go via runVerificationFunc.
func (o *Orchestrator) RunVerification(testCommand string) error {
	return runVerificationFunc(o, testCommand)
}

// UpdateIMPLStatus ticks the Status table checkboxes in the IMPL doc for all
// agents in waveNum that reported status: complete. Non-fatal: returns nil
// if no Status section found. Returns error only on file I/O failure.
func (o *Orchestrator) UpdateIMPLStatus(waveNum int) error {
	// 1. Find wave in o.implDoc.Waves by waveNum. If not found, return nil.
	var wave *types.Wave
	for i := range o.implDoc.Waves {
		if o.implDoc.Waves[i].Number == waveNum {
			wave = &o.implDoc.Waves[i]
			break
		}
	}
	if wave == nil {
		return nil
	}

	// 2. For each agent in the wave, call protocol.ParseCompletionReport.
	//    If ErrReportNotFound or status != StatusComplete, skip.
	var completedLetters []string
	for _, agentSpec := range wave.Agents {
		report, err := protocol.ParseCompletionReport(o.implDocPath, agentSpec.Letter)
		if err != nil {
			if errors.Is(err, protocol.ErrReportNotFound) {
				continue
			}
			// Non-fatal: skip agents whose reports cannot be parsed.
			continue
		}
		if report.Status != types.StatusComplete {
			continue
		}
		completedLetters = append(completedLetters, agentSpec.Letter)
	}

	// 4. If no complete agents, return nil.
	if len(completedLetters) == 0 {
		return nil
	}

	// 5. Call protocol.UpdateIMPLStatus to tick checkboxes.
	return protocol.UpdateIMPLStatus(o.implDocPath, completedLetters)
}
