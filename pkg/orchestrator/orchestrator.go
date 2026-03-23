// Package orchestrator drives SAW protocol execution: it advances the
// 10-state machine, creates per-agent git worktrees, launches agents
// concurrently via the Anthropic API, merges completed worktrees, runs
// post-merge verification, and updates the IMPL doc status table.
// State mutations always go through TransitionTo — never set state directly.
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/retryctx"
	bedrockbackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"
	cliclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/cli"
	openaibackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/openai"
	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types" // kept for types.FailureType constants (E19 routing)
	"github.com/blackwell-systems/scout-and-wave-go/pkg/worktree"
)

func init() {
	// Wire up protocol.ValidateInvariants. Agent B will migrate the signature
	// from *types.IMPLDoc to *protocol.IMPLManifest. Until then, use an adapter
	// that calls protocol.ValidateManifestInvariants (new manifest-based entry point).
	// If that doesn't exist yet, wrap the legacy function.
	SetValidateInvariantsFunc(validateManifestInvariantsAdapter)
}

// validateManifestInvariantsAdapter wraps protocol.ValidateInvariants for the
// *protocol.IMPLManifest signature. This adapter is temporary — once Agent B
// changes protocol.ValidateInvariants to accept *IMPLManifest directly, this
// can be replaced with a direct reference.
func validateManifestInvariantsAdapter(manifest *protocol.IMPLManifest) error {
	if manifest == nil {
		return nil
	}
	// Perform I1 validation directly using manifest data (same logic as protocol.ValidateInvariants).
	for _, wave := range manifest.Waves {
		seen := make(map[string]string) // "repo:file" -> agent ID
		for _, agent := range wave.Agents {
			for _, file := range agent.Files {
				// Build composite key: use repo from ownership table if available.
				repo := ""
				for _, fo := range manifest.FileOwnership {
					if fo.File == file && fo.Agent == agent.ID {
						repo = fo.Repo
						break
					}
				}
				key := repo + ":" + file
				if prev, ok := seen[key]; ok {
					return fmt.Errorf(
						"I1 violation in Wave %d: file %q claimed by both Agent %s and Agent %s",
						wave.Number, file, prev, agent.ID,
					)
				}
				seen[key] = agent.ID
			}
		}
	}
	return nil
}

// defaultAgentTimeout is the maximum time RunWave waits per agent for a
// completion report. Package-level so tests can lower it.
var defaultAgentTimeout = 30 * time.Minute

// defaultAgentPollInterval is how often RunWave polls for completion reports.
var defaultAgentPollInterval = 10 * time.Second

// validateInvariantsFunc is replaced by pkg/protocol via SetValidateInvariantsFunc.
// Default no-op for Wave 1 compilation.
var validateInvariantsFunc = func(doc *protocol.IMPLManifest) error { return nil }

// SetValidateInvariantsFunc allows pkg/protocol to inject the real implementation
// without a direct import cycle.
func SetValidateInvariantsFunc(f func(doc *protocol.IMPLManifest) error) {
	validateInvariantsFunc = f
}

// reportMu serializes concurrent writes to the IMPL doc's completion_reports.
// Without this, parallel agents in the same wave race on Load→Set→Save and
// the last writer wins (overwriting earlier reports).
var reportMu sync.Mutex

// retryCountMap tracks how many times each agent has been retried in this wave run.
// Key format: "<slug>:<waveNum>:<agentLetter>" e.g. "my-feature:1:A". Transient state only.
var retryCountMap sync.Map

// retryPrefixMap stores the retry prompt prefix for injection AFTER E23 extraction.
// Key format same as retryCountMap. Set by executeRetryLoop, read in launchAgent.
// Using a sync.Map avoids needing to modify types.AgentSpec.
var retryPrefixMap sync.Map

// MaxTransientRetries is the retry limit per E19 for transient failures.
const MaxTransientRetries = 2

// MaxFixableRetries is the retry limit per E19 for fixable failures.
// Must match protocol.MaxRetries(FailureFixable) in pkg/protocol/failure.go.
const MaxFixableRetries = 2

// MaxTimeoutRetries is the retry limit per E19 for timeout failures (retry once with scope-reduction note).
const MaxTimeoutRetries = 1

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

// implSlug loads the IMPL manifest and returns the feature slug.
// Returns empty string if the manifest can't be loaded (backward compat).
func (o *Orchestrator) implSlug() string {
	if o.implDocPath == "" {
		return ""
	}
	manifest, err := protocol.Load(o.implDocPath)
	if err != nil {
		return ""
	}
	return manifest.FeatureSlug
}

// waitForCompletionFunc is a seam for tests: wraps agent.WaitForCompletion.
var waitForCompletionFunc = func(implDocPath, agentLetter string, timeout, pollInterval time.Duration) (*protocol.CompletionReport, error) {
	return agent.WaitForCompletion(implDocPath, agentLetter, timeout, pollInterval)
}

// prioritizeAgentsFunc is replaced by pkg/engine/scheduler.go via SetPrioritizeAgentsFunc.
// Default implementation returns agents in declaration order (no reordering).
var prioritizeAgentsFunc = func(manifest *protocol.IMPLManifest, waveNum int) []string {
	// Fallback: return agents in declaration order if scheduler not available yet
	for _, wave := range manifest.Waves {
		if wave.Number == waveNum {
			order := make([]string, len(wave.Agents))
			for i, a := range wave.Agents {
				order[i] = a.ID
			}
			return order
		}
	}
	return []string{}
}

// SetPrioritizeAgentsFunc allows pkg/engine to inject the real scheduler implementation
// without a direct import cycle.
func SetPrioritizeAgentsFunc(f func(manifest *protocol.IMPLManifest, waveNum int) []string) {
	prioritizeAgentsFunc = f
}

// slicesEqual compares two string slices for equality.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// BackendConfig carries backend selection + credentials for newBackendFunc.
type BackendConfig struct {
	Kind      string // "api" | "cli" | "auto" | "openai" | "anthropic" | "bedrock" | "ollama" | "lmstudio"
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

	// Constraints, if non-nil, configures SAW protocol invariant enforcement
	// (I1 ownership, I2 freeze, I5 commit tracking, I6 role restriction).
	Constraints *tools.Constraints
}

// validateModelName ensures model name contains only safe characters and is
// within reasonable length limits. Returns error if validation fails.
func validateModelName(model string) error {
	if model == "" {
		return nil // empty is allowed (falls back to defaults)
	}
	if len(model) > 200 {
		return fmt.Errorf("model name too long (max 200 chars)")
	}
	// Allow alphanumeric, hyphens, dots, colons, underscores, slashes (for paths like us.anthropic.X)
	for _, ch := range model {
		if !(ch >= 'a' && ch <= 'z') &&
			!(ch >= 'A' && ch <= 'Z') &&
			!(ch >= '0' && ch <= '9') &&
			ch != '-' && ch != '.' && ch != ':' && ch != '_' && ch != '/' {
			return fmt.Errorf("model name contains invalid character: %q", ch)
		}
	}
	return nil
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

// expandBedrockModelID converts short Bedrock model names to full region-prefixed IDs.
// "claude-sonnet-4-5" → "us.anthropic.claude-sonnet-4-5-20250929-v1:0"
// "claude-opus-4-6" → "us.anthropic.claude-opus-4-6-20250519-v1:0"
// "claude-haiku-4-5" → "us.anthropic.claude-haiku-4-5-20251001-v1:0"
// Already-expanded IDs are returned unchanged.
func expandBedrockModelID(shortName string) string {
	// If already a full Bedrock ID (contains region prefix), return as-is
	if strings.Contains(shortName, ".anthropic.") {
		return shortName
	}
	
	// Map of short names to Bedrock inference profile IDs (from aws bedrock list-inference-profiles)
	// Use us. prefix for single-region, global. for cross-region routing
	mapping := map[string]string{
		"claude-sonnet-4-5":          "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		"claude-sonnet-4-6":          "us.anthropic.claude-sonnet-4-6",
		"claude-opus-4-6":            "us.anthropic.claude-opus-4-6-v1",
		"claude-haiku-4-5":           "us.anthropic.claude-haiku-4-5-20251001-v1:0",
		"claude-haiku-4-5-20251001":  "us.anthropic.claude-haiku-4-5-20251001-v1:0",
	}
	
	if fullID, ok := mapping[shortName]; ok {
		return fullID
	}
	
	// Unknown short name: return as-is (may be a custom model or different region)
	return shortName
}

// newBackendFunc constructs a backend.Backend from config. Seam for tests.
var newBackendFunc = func(cfg BackendConfig) (backend.Backend, error) {
	// Validate model name before processing to prevent injection attacks.
	if err := validateModelName(cfg.Model); err != nil {
		return nil, fmt.Errorf("orchestrator: invalid model name: %w", err)
	}
	
	// Parse any provider prefix from the model string (e.g. "openai:gpt-4o").
	provider, bareModel := parseProviderPrefix(cfg.Model)

	// Determine effective kind: explicit prefix overrides cfg.Kind.
	effectiveKind := cfg.Kind
	if provider != "" {
		effectiveKind = provider
	}

	bcfg := backend.Config{
		Model:       bareModel,
		MaxTokens:   cfg.MaxTokens,
		MaxTurns:    cfg.MaxTurns,
		Constraints: cfg.Constraints,
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
	case "ollama":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434/v1"
		}
		return openaibackend.New(backend.Config{
			Model:     bareModel,
			MaxTokens: cfg.MaxTokens,
			MaxTurns:  cfg.MaxTurns,
			BaseURL:   baseURL,
			// Ollama does not require an API key; pass empty string.
		}), nil
	case "lmstudio":
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:1234/v1"
		}
		return openaibackend.New(backend.Config{
			Model:     bareModel,
			MaxTokens: cfg.MaxTokens,
			MaxTurns:  cfg.MaxTurns,
			BaseURL:   baseURL,
		}), nil
	case "bedrock":
		// bedrock: prefix uses AWS Bedrock SDK with expanded Bedrock model IDs.
		// Uses AWS credentials from default chain (~/.aws/credentials, env vars, IAM role).
		// Use cli: prefix if you want to shell out to the claude CLI command instead.
		fullID := expandBedrockModelID(bareModel)
		return bedrockbackend.New(backend.Config{
			Model:     fullID,
			MaxTokens: cfg.MaxTokens,
			MaxTurns:  cfg.MaxTurns,
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
		return nil, fmt.Errorf("orchestrator: unknown backend kind %q; valid values: api, cli, auto, openai, anthropic, bedrock, ollama, lmstudio", effectiveKind)
	}
}

// NewBackendFromModel creates a backend.Backend from a model string that may
// contain a provider prefix (e.g. "bedrock:claude-sonnet-4-6", "openai:gpt-4o").
// This is the exported entry point for engine code that needs provider routing
// without constructing a full BackendConfig.
func NewBackendFromModel(model string) (backend.Backend, error) {
	return newBackendFunc(BackendConfig{Model: model})
}

// newRunnerFunc is a seam for tests: constructs the agent.Runner used by RunWave.
// Tests can replace this to inject a fake Backend without real API calls.
var newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
	return agent.NewRunner(b, wm)
}

// Orchestrator drives SAW protocol wave coordination.
// State mutations must go through TransitionTo — never set o.state directly.
type Orchestrator struct {
	state          protocol.ProtocolState
	implDoc        *protocol.IMPLManifest
	repoPath       string
	currentWave    int
	implDocPath    string
	eventPublisher EventPublisher
	defaultModel   string            // optional default model for wave agents (e.g. "claude-haiku-4-5")
	worktreePaths  map[string]string // agent letter -> pre-computed worktree abs path (for multi-repo)
}

// SetDefaultModel sets the fallback model used for wave agents that have no
// per-agent model: field in the IMPL doc. Empty string means use the CLI/API default.
func (o *Orchestrator) SetDefaultModel(model string) {
	o.defaultModel = model
}

// SetWorktreePaths provides pre-computed worktree paths for multi-repo execution.
// Keys are agent letters, values are absolute worktree paths. When set,
// launchAgent uses these instead of computing paths from o.repoPath.
func (o *Orchestrator) SetWorktreePaths(paths map[string]string) {
	o.worktreePaths = paths
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
	var doc *protocol.IMPLManifest
	if implDocPath != "" {
		var err error
		doc, err = protocol.Load(implDocPath)
		if err != nil {
			return nil, fmt.Errorf("orchestrator.New: %w", err)
		}
	}
	return &Orchestrator{
		state:       protocol.StateScoutPending,
		implDoc:     doc,
		repoPath:    repoPath,
		implDocPath: implDocPath,
	}, nil
}

// newFromDoc creates an Orchestrator directly from a pre-parsed IMPLManifest.
// Used in tests to avoid file I/O.
func newFromDoc(doc *protocol.IMPLManifest, repoPath, implDocPath string) *Orchestrator {
	return &Orchestrator{
		state:       protocol.StateScoutPending,
		implDoc:     doc,
		repoPath:    repoPath,
		implDocPath: implDocPath,
	}
}

// State returns the current protocol state.
func (o *Orchestrator) State() protocol.ProtocolState {
	return o.state
}

// IMPLDoc returns the parsed IMPL document.
func (o *Orchestrator) IMPLDoc() *protocol.IMPLManifest {
	return o.implDoc
}

// RepoPath returns the repository root path.
func (o *Orchestrator) RepoPath() string {
	return o.repoPath
}

// TransitionTo advances the state machine to newState.
// It returns a descriptive error if the transition is not permitted.
func (o *Orchestrator) TransitionTo(newState protocol.ProtocolState) error {
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
	var wave *protocol.Wave
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

	// Reset per-agent retry counts for this wave run (transient state).
	implSlug := o.implSlug()
	for _, a := range wave.Agents {
		retryCountMap.Delete(fmt.Sprintf("%s:%d:%s", implSlug, waveNum, a.ID))
	}

	// Build the worktree manager and default agent runner.
	slug := o.implSlug()
	wm := worktree.New(o.repoPath, slug)
	defaultBackend, err := newBackendFunc(BackendConfig{Kind: "auto", Model: o.defaultModel})
	if err != nil {
		return fmt.Errorf("orchestrator.RunWave: failed to create backend: %w", err)
	}
	defaultRunner := newRunnerFunc(defaultBackend, wm)

	// Launch all agents concurrently and collect the first error.
	eg, ctx := errgroup.WithContext(context.Background())

	// Prioritize agent launch order based on dependency graph critical path depth.
	agentOrder := prioritizeAgentsFunc(o.implDoc, waveNum)

	// Emit SSE event showing reordering (observability).
	originalOrder := make([]string, len(wave.Agents))
	for i, a := range wave.Agents {
		originalOrder[i] = a.ID
	}
	reordered := !slicesEqual(originalOrder, agentOrder)
	o.publish(OrchestratorEvent{
		Event: "agent_prioritized",
		Data: AgentPrioritizedPayload{
			Wave:             waveNum,
			OriginalOrder:    originalOrder,
			PrioritizedOrder: agentOrder,
			Reordered:        reordered,
			Reason:           "critical_path_scheduling",
		},
	})

	// Launch agents in prioritized order instead of declaration order.
	for _, agentID := range agentOrder {
		// Find the protocol.Agent by ID
		var protoAgent protocol.Agent
		found := false
		for _, a := range wave.Agents {
			if a.ID == agentID {
				protoAgent = a
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("orchestrator.RunWave: prioritized agent %s not found in wave %d", agentID, waveNum)
		}

		eg.Go(func() error {
			// Build per-agent constraints from the IMPL manifest (I1/I2/I5/I6).
			constraints := buildWaveConstraints(o.implDocPath, protoAgent.ID)

			runner := defaultRunner
			// Per-agent model override or constraints: create a separate backend.
			model := o.defaultModel
			if protoAgent.Model != "" && protoAgent.Model != o.defaultModel {
				model = protoAgent.Model
			}
			if constraints != nil || model != o.defaultModel {
				b2, err2 := newBackendFunc(BackendConfig{
					Kind:        "auto",
					Model:       model,
					Constraints: constraints,
				})
				if err2 != nil {
					return fmt.Errorf("orchestrator: agent %s: create backend: %w", protoAgent.ID, err2)
				}
				runner = newRunnerFunc(b2, wm)
			}
			return o.launchAgent(ctx, runner, wm, waveNum, protoAgent)
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
	agentSpec protocol.Agent,
) error {
	// a. Determine worktree path: use pre-computed multi-repo path if available,
	// otherwise fall back to single-repo creation.
	slug := o.implSlug()
	branch := protocol.BranchName(slug, waveNum, agentSpec.ID)
	var wtPath string

	if prePath, ok := o.worktreePaths[agentSpec.ID]; ok {
		// Multi-repo: worktree was pre-created by protocol.CreateWorktrees
		wtPath = prePath
		if _, statErr := os.Stat(wtPath); statErr != nil {
			o.publish(OrchestratorEvent{
				Event: "agent_failed",
				Data: AgentFailedPayload{
					Agent:       agentSpec.ID,
					Wave:        waveNum,
					Status:      "failed",
					FailureType: "worktree_missing",
					Message:     fmt.Sprintf("pre-created worktree not found at %s", wtPath),
				},
			})
			return fmt.Errorf("orchestrator: agent %s: pre-created worktree not found at %s", agentSpec.ID, wtPath)
		}
	} else {
		// Single-repo fallback: create worktree on demand
		wtPath = filepath.Join(o.repoPath, ".claude", "worktrees", branch)
		if _, statErr := os.Stat(wtPath); statErr == nil {
			fmt.Fprintf(os.Stderr, "orchestrator: reusing existing worktree for agent %s at %s\n", agentSpec.ID, wtPath)
		} else {
			var createErr error
			wtPath, createErr = worktreeCreatorFunc(wm, waveNum, agentSpec.ID)
			if createErr != nil {
				o.publish(OrchestratorEvent{
					Event: "agent_failed",
					Data: AgentFailedPayload{
						Agent:       agentSpec.ID,
						Wave:        waveNum,
						Status:      "failed",
						FailureType: "worktree_creation",
						Message:     createErr.Error(),
					},
				})
				return fmt.Errorf("orchestrator: agent %s: create worktree: %w", agentSpec.ID, createErr)
			}
		}
	}

	// Capture the worktree HEAD before the agent runs. This is the commit the
	// worktree was created from — we compare against it after execution to detect
	// whether the agent committed its own work (Bedrock agents commit via bash).
	baseSHA, _ := git.RevParse(wtPath, "HEAD")

	// Publish agent_started after the worktree is ready.
	o.publish(OrchestratorEvent{
		Event: "agent_started",
		Data: AgentStartedPayload{
			Agent: agentSpec.ID,
			Wave:  waveNum,
			Files: agentSpec.Files,
		},
	})

	// E23: Construct per-agent context payload instead of passing full IMPL doc prompt.
	manifest, err := protocol.Load(o.implDocPath)
	if err == nil {
		if contextPayload, extractErr := protocol.ExtractAgentContextFromManifest(manifest, agentSpec.ID); extractErr == nil {
			if jsonBytes, marshalErr := json.Marshal(contextPayload); marshalErr == nil {
				agentSpec.Task = string(jsonBytes)
			} else {
				fmt.Fprintf(os.Stderr, "orchestrator: failed to marshal E23 context: %v\n", marshalErr)
			}
		} else {
			// Fallback: use existing prompt from agentSpec (already set from IMPL doc parse).
			fmt.Fprintf(os.Stderr, "orchestrator: E23 context extraction failed for agent %s: %v (falling back to full prompt)\n", agentSpec.ID, extractErr)
		}
	} else {
		fmt.Fprintf(os.Stderr, "orchestrator: failed to load manifest for E23 extraction: %v\n", err)
	}

	// Inject retry prefix after E23 extraction (GAP-4 fix).
	// retryPrefixMap is set by executeRetryLoop; read here after E23 so the prefix
	// is preserved even though E23 overwrites the base prompt.
	retryKey := fmt.Sprintf("%s:%d:%s", o.implSlug(), waveNum, agentSpec.ID)
	if prefix, ok := retryPrefixMap.Load(retryKey); ok {
		if prefixStr, ok2 := prefix.(string); ok2 && prefixStr != "" {
			agentSpec.Task = prefixStr + "\n\n" + agentSpec.Task
			retryPrefixMap.Delete(retryKey) // consume once; recursive retries store fresh
		}
	}

	// b. Execute the agent via the backend, streaming output chunks as SSE events.
	if _, err := runner.ExecuteStreamingWithTools(ctx, &agentSpec, wtPath,
		// onChunk — stream output chunks as SSE events
		func(chunk string) {
			o.publish(OrchestratorEvent{
				Event: "agent_output",
				Data: AgentOutputPayload{
					Agent: agentSpec.ID,
					Wave:  waveNum,
					Chunk: chunk,
				},
			})
		},
		// onToolCall — stream tool invocations and results as SSE events
		func(ev backend.ToolCallEvent) {
			o.publish(OrchestratorEvent{
				Event: "agent_tool_call",
				Data: AgentToolCallPayload{
					Agent:      agentSpec.ID,
					Wave:       waveNum,
					ToolID:     ev.ID,
					ToolName:   ev.Name,
					Input:      ev.Input,
					IsResult:   ev.IsResult,
					IsError:    ev.IsError,
					DurationMs: ev.DurationMs,
				},
			})
		},
	); err != nil {
		failureType := "execute"
		if strings.Contains(err.Error(), "maxTurns") {
			failureType = "timeout"
		}
		o.publish(OrchestratorEvent{
			Event: "agent_failed",
			Data: AgentFailedPayload{
				Agent:       agentSpec.ID,
				Wave:        waveNum,
				Status:      "failed",
				FailureType: failureType,
				Message:     err.Error(),
			},
		})
		return fmt.Errorf("orchestrator: agent %s: Execute: %w", agentSpec.ID, err)
	}

	// c. Check for completion report in the agent's worktree IMPL doc.
	// API/Bedrock backends run synchronously — when ExecuteStreamingWithTools returns,
	// the agent is done. Poll briefly (once) for a completion report in case the agent
	// wrote one, but don't block for 30 minutes waiting for one that may never come.
	report, _ := waitForCompletionFunc(wtIMPLPath(o.repoPath, o.implDocPath, wtPath), agentSpec.ID, 5*time.Second, 2*time.Second)

	// d. Auto-commit and synthesize completion report if the agent didn't write one.
	// This bridges the gap between CLI agents (SAW-protocol-aware, commit + write report)
	// and API/Bedrock agents (vanilla Claude, just write files via tools).
	if report == nil {
		// BUG-4 fix: Before synthesizing "complete", check if a previous retry wrote
		// a partial/blocked report to the main IMPL doc. If so, reuse it rather than
		// masking the failure with a spurious "complete".
		if savedManifest, checkErr := protocol.Load(o.implDocPath); checkErr == nil {
			if cr, ok := savedManifest.CompletionReports[agentSpec.ID]; ok &&
				(cr.Status == "partial" || cr.Status == "blocked") {
				report = &cr
			}
		}
	}
	if report == nil {
		// Only auto-synthesize complete if no existing partial/blocked report was found.
		commitSHA, filesChanged, autoErr := autoCommitWorktree(wtPath, waveNum, agentSpec.ID, baseSHA)
		if autoErr != nil {
			fmt.Fprintf(os.Stderr, "orchestrator: auto-commit failed for agent %s: %v\n", agentSpec.ID, autoErr)
		}

		// Always synthesize a completion report for API agents that didn't write one.
		// commitSHA may be empty if the agent produced no file changes (no-op task);
		// that's still a successful completion — the merge step just has nothing to merge.
		notes := "auto-committed by orchestrator (API agent)"
		if commitSHA == "" {
			// Resolve HEAD as the "commit" so verifyAgentCommits can find the branch.
			commitSHA, _ = git.RevParse(wtPath, "HEAD")
			notes = "no changes produced (API agent)"
		}
		report = &protocol.CompletionReport{
			Status:       "complete",
			Worktree:     wtPath,
			Branch:       branch,
			Commit:       commitSHA,
			FilesChanged: filesChanged,
			Notes:        notes,
		}
	}

	// e. Always persist the completion report to the main branch IMPL doc.
	// Whether the agent wrote it (found in worktree) or we synthesized it,
	// the main branch IMPL doc must have it for merge to proceed.
	if report != nil {
		reportMu.Lock()
		if manifest, loadErr := protocol.Load(o.implDocPath); loadErr == nil {
			if setErr := protocol.SetCompletionReport(manifest, agentSpec.ID, *report); setErr == nil {
				if saveErr := protocol.Save(manifest, o.implDocPath); saveErr != nil {
					fmt.Fprintf(os.Stderr, "orchestrator: failed to save report for agent %s: %v\n", agentSpec.ID, saveErr)
				}
			} else {
				fmt.Fprintf(os.Stderr, "orchestrator: failed to set report for agent %s: %v\n", agentSpec.ID, setErr)
			}
		}
		reportMu.Unlock()
	}

	// BUG-5 fix: Determine whether E19 will trigger an automatic retry before
	// publishing agent_complete. If retry is coming, suppress the publish here —
	// the recursive launchAgent call (from executeRetryLoop) will publish
	// agent_complete with the final settled status.
	willAutoRetry := report != nil &&
		(report.Status == "partial" || report.Status == "blocked") &&
		(report.FailureType == "transient" || report.FailureType == "fixable" || report.FailureType == "timeout")

	if !willAutoRetry {
		status := "complete"
		if report != nil {
			status = string(report.Status)
		}
		o.publish(OrchestratorEvent{
			Event: "agent_complete",
			Data: AgentCompletePayload{
				Agent:  agentSpec.ID,
				Wave:   waveNum,
				Status: status,
				Branch: branch,
			},
		})
	}

	// E19: If agent reported partial or blocked, apply the decision tree.
	if report != nil && (report.Status == "partial" || report.Status == "blocked") {
		var failureType types.FailureType
		switch report.FailureType {
		case "transient":
			failureType = types.FailureTypeTransient
		case "fixable":
			failureType = types.FailureTypeFixable
		case "needs_replan":
			failureType = types.FailureTypeNeedsReplan
		case "escalate":
			failureType = types.FailureTypeEscalate
		case "timeout":
			failureType = types.FailureTypeTimeout
		default:
			failureType = types.FailureTypeEscalate
		}

		action := RouteFailure(failureType)
		o.publish(OrchestratorEvent{
			Event: "agent_blocked",
			Data: AgentBlockedPayload{
				Agent:       agentSpec.ID,
				Wave:        waveNum,
				Status:      report.Status,
				FailureType: report.FailureType,
				Action:      action,
			},
		})

		switch action {
		case ActionRetry, ActionApplyAndRelaunch, ActionRetryWithScope:
			// E19: transient/fixable/timeout — execute retry loop (no human gate).
			if retryErr := o.executeRetryLoop(ctx, runner, wm, waveNum, agentSpec, report); retryErr != nil {
				return retryErr
			}
		case ActionReplan, ActionEscalate:
			// E19: needs_replan/escalate — return error to surface to human.
			return fmt.Errorf("orchestrator: agent %s: %s failure (failure_type=%s): requires human intervention",
				agentSpec.ID, report.Status, report.FailureType)
		}
	}

	return nil
}

// executeRetryLoop implements E19 automatic retry for transient and fixable failures.
// It is called from launchAgent when a completion report has partial/blocked status
// and a failure_type of "transient", "fixable", or "timeout". Returns nil if retry
// eventually succeeds (report.Status == "complete"), or an error if retries are
// exhausted or the failure_type does not warrant automatic retry.
func (o *Orchestrator) executeRetryLoop(
	ctx context.Context,
	runner *agent.Runner,
	wm *worktree.Manager,
	waveNum int,
	agentSpec protocol.Agent,
	report *protocol.CompletionReport,
) error {
	failureType := report.FailureType

	// Determine max retries per E19.
	// timeout uses MaxFixableRetries (1) not MaxTransientRetries (2):
	// failure.go maps timeout → ActionRetryWithScope (retry once with scope-reduction note).
	var maxRetries int
	switch failureType {
	case "transient":
		maxRetries = MaxTransientRetries
	case "timeout":
		maxRetries = MaxTimeoutRetries // E19: timeout retries once with scope-reduction note
	case "fixable":
		maxRetries = MaxFixableRetries
	default:
		// needs_replan or escalate: do not retry automatically.
		return fmt.Errorf("orchestrator: agent %s: failure_type=%q requires human intervention", agentSpec.ID, failureType)
	}

	// Check and increment retry count.
	// Use slug-scoped key to prevent cross-IMPL contamination in concurrent server runs.
	implSlug := o.implSlug()
	key := fmt.Sprintf("%s:%d:%s", implSlug, waveNum, agentSpec.ID)
	count := 0
	if v, ok := retryCountMap.Load(key); ok {
		if n, ok := v.(int); ok {
			count = n
		}
	}
	if count >= maxRetries {
		o.publish(OrchestratorEvent{
			Event: "auto_retry_exhausted",
			Data: AutoRetryExhaustedPayload{
				Agent:       agentSpec.ID,
				Wave:        waveNum,
				FailureType: failureType,
				Attempts:    count,
			},
		})
		return fmt.Errorf("orchestrator: agent %s: auto-retry exhausted after %d attempts (failure_type=%s)", agentSpec.ID, count, failureType)
	}
	count++
	retryCountMap.Store(key, count)

	// Build retry context using retryctx package (enriched prompt with fix guidance).
	var promptPrefix string
	if o.implDocPath != "" {
		rc, rcErr := retryctx.BuildRetryContext(o.implDocPath, agentSpec.ID, count)
		if rcErr != nil {
			fmt.Fprintf(os.Stderr, "orchestrator: retry context build (best-effort): %v\n", rcErr)
		} else if rc != nil && rc.PromptText != "" {
			promptPrefix = rc.PromptText
		}
	}

	// Publish auto_retry_started event.
	o.publish(OrchestratorEvent{
		Event: "auto_retry_started",
		Data: AutoRetryStartedPayload{
			Agent:       agentSpec.ID,
			Wave:        waveNum,
			FailureType: failureType,
			Attempt:     count,
			MaxAttempts: maxRetries,
		},
	})

	// Store retry prefix in retryPrefixMap BEFORE calling launchAgent.
	// DO NOT set retrySpec.Prompt here — E23 extraction in launchAgent will
	// overwrite it (GAP-4 fix). Instead, launchAgent reads from retryPrefixMap
	// after E23 extraction and injects the prefix there.
	if promptPrefix != "" {
		retryPrefixMap.Store(key, promptPrefix)
	}
	retrySpec := agentSpec // Prompt NOT modified here

	// Re-launch agent (recursive call to launchAgent; retry count prevents infinite loop).
	return o.launchAgent(ctx, runner, wm, waveNum, retrySpec)
}

// autoCommitWorktree stages and commits all changes in the agent's worktree.
// baseSHA is the worktree HEAD captured before the agent ran — used to detect
// agent-committed work when the worktree appears clean.
// Returns (commitSHA, filesChanged, error). If no changes were made,
// returns ("", nil, nil).
func autoCommitWorktree(wtPath string, waveNum int, agentLetter string, baseSHA string) (string, []string, error) {
	// Check if there are uncommitted changes.
	status, err := git.StatusPorcelain(wtPath)
	if err != nil {
		return "", nil, fmt.Errorf("checking worktree status: %w", err)
	}
	if status == "" {
		// Worktree is clean — agent may have already committed (via bash tool).
		// Compare current HEAD against the base SHA captured before execution.
		headSHA, _ := git.RevParse(wtPath, "HEAD")
		if baseSHA != "" && headSHA != baseSHA {
			// HEAD moved — agent committed its own work. Get changed files.
			files, _ := git.ChangedFilesSinceRef(wtPath, baseSHA)
			return headSHA, files, nil
		}
		return "", nil, nil
	}

	// If baseSHA wasn't provided (e.g. solo-agent path), resolve HEAD now
	// so we can diff after committing.
	if baseSHA == "" {
		baseSHA, _ = git.RevParse(wtPath, "HEAD")
	}

	// Stage all changes.
	if err := git.AddAll(wtPath); err != nil {
		return "", nil, fmt.Errorf("staging changes: %w", err)
	}

	// Commit. Use legacy format in commit message for readability.
	msg := fmt.Sprintf("feat(wave%d-agent-%s): implement assigned files", waveNum, agentLetter)

	commitSHA, err := git.Commit(wtPath, msg)
	if err != nil {
		return "", nil, fmt.Errorf("committing: %w", err)
	}

	// Determine files changed.
	files, err := git.ChangedFilesSinceRef(wtPath, baseSHA)
	if err != nil {
		return commitSHA, nil, fmt.Errorf("listing changed files: %w", err)
	}

	return commitSHA, files, nil
}

// RunAgent executes a single agent from the specified wave. Unlike RunWave
// (which launches all agents concurrently), RunAgent targets exactly one agent
// by letter. This is used for single-agent reruns. If promptPrefix is non-empty,
// it is prepended to the agent's task prompt (e.g. scope-reduction hints for
// timeout reruns).
func (o *Orchestrator) RunAgent(waveNum int, agentLetter string, promptPrefix string) error {
	if o.implDoc == nil {
		return fmt.Errorf("orchestrator.RunAgent: no IMPL doc loaded")
	}
	// Find the wave.
	var wave *protocol.Wave
	for i := range o.implDoc.Waves {
		if o.implDoc.Waves[i].Number == waveNum {
			wave = &o.implDoc.Waves[i]
			break
		}
	}
	if wave == nil {
		return fmt.Errorf("orchestrator.RunAgent: wave %d not found", waveNum)
	}
	// Find the agent in the wave.
	var protoAgent *protocol.Agent
	for i := range wave.Agents {
		if strings.EqualFold(wave.Agents[i].ID, agentLetter) {
			protoAgent = &wave.Agents[i]
			break
		}
	}
	if protoAgent == nil {
		return fmt.Errorf("orchestrator.RunAgent: agent %s not found in wave %d", agentLetter, waveNum)
	}

	// Use protocol.Agent directly (no agentToSpec adapter needed).
	spec := *protoAgent

	// Apply prompt prefix if provided (e.g. scope hint for timeout reruns).
	if promptPrefix != "" {
		spec.Task = promptPrefix + "\n\n" + spec.Task
	}

	// Build worktree manager and backend.
	wm := worktree.New(o.repoPath, o.implSlug())
	b, err := newBackendFunc(BackendConfig{Kind: "auto", Model: o.defaultModel})
	if err != nil {
		return fmt.Errorf("orchestrator.RunAgent: create backend: %w", err)
	}
	if spec.Model != "" && spec.Model != o.defaultModel {
		b, err = newBackendFunc(BackendConfig{Kind: "auto", Model: spec.Model})
		if err != nil {
			return fmt.Errorf("orchestrator.RunAgent: create backend for model %s: %w", spec.Model, err)
		}
	}
	runner := newRunnerFunc(b, wm)

	return o.launchAgent(context.Background(), runner, wm, waveNum, spec)
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
	var wave *protocol.Wave
	for i := range o.implDoc.Waves {
		if o.implDoc.Waves[i].Number == waveNum {
			wave = &o.implDoc.Waves[i]
			break
		}
	}
	if wave == nil {
		return nil
	}

	// 2. Load manifest and check completion reports.
	//    If report not found or status != StatusComplete, skip.
	manifest, err := protocol.Load(o.implDocPath)
	if err != nil {
		return nil // Cannot determine completed agents without manifest
	}

	var completedLetters []string
	for _, a := range wave.Agents {
		report, ok := manifest.CompletionReports[a.ID]
		if !ok {
			continue // Report not found
		}
		if report.Status != "complete" {
			continue
		}
		completedLetters = append(completedLetters, a.ID)
	}

	// 4. If no complete agents, return nil.
	if len(completedLetters) == 0 {
		return nil
	}

	// 5. Call protocol.UpdateIMPLStatus to tick checkboxes.
	return protocol.UpdateIMPLStatus(o.implDocPath, completedLetters)
}

// buildWaveConstraints loads the IMPL manifest and builds per-agent constraints
// for wave-role enforcement (I1 ownership, I2 freeze, I5 commit tracking).
// Returns nil if the manifest can't be loaded (backward compatible, no enforcement).
func buildWaveConstraints(implPath string, agentID string) *tools.Constraints {
	manifest, err := protocol.Load(implPath)
	if err != nil || manifest == nil {
		return nil
	}

	c := &tools.Constraints{
		AgentRole:    "wave",
		AgentID:      agentID,
		TrackCommits: true,
	}

	// I1: Owned files for this agent
	owned := make(map[string]bool)
	for _, fo := range manifest.FileOwnership {
		if fo.Agent == agentID {
			owned[fo.File] = true
		}
	}
	c.OwnedFiles = owned

	// I2: Frozen paths = scaffold files + interface contract locations
	frozen := make(map[string]bool)
	for _, sf := range manifest.Scaffolds {
		if sf.FilePath != "" {
			frozen[sf.FilePath] = true
		}
	}
	for _, ic := range manifest.InterfaceContracts {
		if ic.Location != "" {
			frozen[ic.Location] = true
		}
	}
	c.FrozenPaths = frozen
	c.FreezeTime = manifest.WorktreesCreatedAt

	return c
}
