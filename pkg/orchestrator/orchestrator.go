// Package orchestrator drives SAW protocol execution: it advances the
// 10-state machine, creates per-agent git worktrees, launches agents
// concurrently via the Anthropic API, merges completed worktrees, runs
// post-merge verification, and updates the IMPL doc status table.
// State mutations always go through TransitionTo — never set state directly.
package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/retry"
	bedrockbackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"
	cliclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/cli"
	openaibackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/openai"
	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
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
func validateManifestInvariantsAdapter(manifest *protocol.IMPLManifest) result.Result[ValidateData] {
	if manifest == nil {
		return result.NewSuccess(ValidateData{WavesChecked: 0})
	}
	// Perform I1 validation directly using manifest data (same logic as protocol.ValidateInvariants).
	wavesChecked := 0
	for _, wave := range manifest.Waves {
		wavesChecked++
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
					return result.NewFailure[ValidateData]([]result.SAWError{
						result.NewFatal(result.CodeDisjointOwnership, fmt.Sprintf(
							"I1 violation in Wave %d: file %q claimed by both Agent %s and Agent %s",
							wave.Number, file, prev, agent.ID,
						)),
					})
				}
				seen[key] = agent.ID
			}
		}
	}
	return result.NewSuccess(ValidateData{WavesChecked: wavesChecked})
}

// defaultAgentTimeout is the maximum time RunWave waits per agent for a
// completion report. Package-level so tests can lower it.
var defaultAgentTimeout = 30 * time.Minute

// defaultAgentPollInterval is how often RunWave polls for completion reports.
var defaultAgentPollInterval = 10 * time.Second

// validateInvariantsFunc is replaced by pkg/protocol via SetValidateInvariantsFunc.
// Default no-op for Wave 1 compilation.
var validateInvariantsFunc = func(doc *protocol.IMPLManifest) result.Result[ValidateData] {
	return result.NewSuccess(ValidateData{})
}

// SetValidateInvariantsFunc allows pkg/protocol to inject the real implementation
// without a direct import cycle.
func SetValidateInvariantsFunc(f func(doc *protocol.IMPLManifest) result.Result[ValidateData]) {
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
// Default failure: if merge.go never ran init(), callers get an actionable error.
var mergeWaveFunc = func(ctx context.Context, o *Orchestrator, waveNum int) result.Result[MergeData] {
	return result.NewFailure[MergeData]([]result.SAWError{
		result.NewFatal(result.CodeAgentLaunchFailed,
			"orchestrator: mergeWaveFunc not injected; call SetMergeWaveFunc before merging"),
	})
}

// runVerificationFunc is replaced by verification.go via init().
// Default failure: if verification.go never ran init(), callers get an actionable error.
var runVerificationFunc = func(ctx context.Context, o *Orchestrator, testCommand string) result.Result[VerificationData] {
	return result.NewFailure[VerificationData]([]result.SAWError{
		result.NewFatal(result.CodeAgentLaunchFailed,
			"orchestrator: runVerificationFunc not injected; call SetRunVerificationFunc before verifying"),
	})
}

// worktreeCreatorFunc is a seam for tests: it creates a worktree for wave/agent
// and returns the worktree path. Tests can replace this to avoid real git ops.
var worktreeCreatorFunc = func(wm *worktree.Manager, waveNum int, agentLetter string) (string, error) {
	r := wm.Create(waveNum, agentLetter)
	if r.IsFatal() {
		return "", r.Errors[0]
	}
	return r.GetData().Path, nil
}

// implSlug returns the feature slug from the in-memory IMPL manifest.
// Returns empty string if the manifest was not loaded (backward compat).
func (o *Orchestrator) implSlug(_ context.Context) string {
	if o.implDoc == nil {
		return ""
	}
	return o.implDoc.FeatureSlug
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

	// AnthropicKey is the Anthropic API key sourced from config.
	// Falls back to APIKey, then ANTHROPIC_API_KEY env var if empty.
	AnthropicKey string

	// OpenAIKey is the API key for the OpenAI-compatible backend.
	// Falls back to OPENAI_API_KEY env var if empty.
	OpenAIKey string

	// BedrockRegion is the AWS region for Bedrock (e.g. "us-east-1").
	// If empty, uses AWS SDK default chain.
	BedrockRegion string

	// BedrockAccessKeyID is the AWS access key for Bedrock.
	// If empty, uses AWS SDK default credential chain.
	BedrockAccessKeyID string

	// BedrockSecretAccessKey is the AWS secret key for Bedrock.
	// If empty, uses AWS SDK default credential chain.
	BedrockSecretAccessKey string

	// BedrockSessionToken is an optional AWS session token for temporary credentials.
	BedrockSessionToken string

	// BedrockProfile is an AWS CLI named profile (supports SSO, assume-role).
	BedrockProfile string

	// BaseURL is an optional endpoint override used when Kind == "openai"
	// or when the provider prefix is "openai".
	BaseURL string

	// Constraints, if non-nil, configures SAW protocol invariant enforcement
	// (I1 ownership, I2 freeze, I5 commit tracking, I6 role restriction).
	Constraints *tools.Constraints
}

// validateModelName ensures model name contains only safe characters and is
// within reasonable length limits. Returns result failure if validation fails.
func validateModelName(model string) result.Result[ModelData] {
	if model == "" {
		return result.NewSuccess(ModelData{Model: model}) // empty is allowed (falls back to defaults)
	}
	if len(model) > 200 {
		return result.NewFailure[ModelData]([]result.SAWError{
			result.NewFatal(result.CodeInvalidFieldValue, "model name too long (max 200 chars)"),
		})
	}
	// Allow alphanumeric, hyphens, dots, colons, underscores, slashes (for paths like us.anthropic.X)
	for _, ch := range model {
		if !(ch >= 'a' && ch <= 'z') &&
			!(ch >= 'A' && ch <= 'Z') &&
			!(ch >= '0' && ch <= '9') &&
			ch != '-' && ch != '.' && ch != ':' && ch != '_' && ch != '/' {
			return result.NewFailure[ModelData]([]result.SAWError{
				result.NewFatal(result.CodeInvalidFieldValue,
					fmt.Sprintf("model name contains invalid character: %q", ch)),
			})
		}
	}
	return result.NewSuccess(ModelData{Model: model})
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
	if res := validateModelName(cfg.Model); res.IsFatal() {
		errMsg := "orchestrator: invalid model name"
		if len(res.Errors) > 0 {
			errMsg = fmt.Sprintf("orchestrator: invalid model name: %s", res.Errors[0].Message)
		}
		return nil, errors.New(errMsg)
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
		bcfgBedrock := backend.Config{
			Model:                 fullID,
			MaxTokens:             cfg.MaxTokens,
			MaxTurns:              cfg.MaxTurns,
			BedrockRegion:         cfg.BedrockRegion,
			BedrockAccessKeyID:    cfg.BedrockAccessKeyID,
			BedrockSecretAccessKey: cfg.BedrockSecretAccessKey,
			BedrockSessionToken:   cfg.BedrockSessionToken,
			BedrockProfile:        cfg.BedrockProfile,
		}
		return bedrockbackend.New(bcfgBedrock), nil
	case "anthropic":
		apiKey := cfg.AnthropicKey
		if apiKey == "" {
			apiKey = cfg.APIKey
		}
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		return apiclient.New(apiKey, backend.Config{
			Model:     bareModel,
			MaxTokens: cfg.MaxTokens,
			MaxTurns:  cfg.MaxTurns,
		}), nil
	case "api":
		apiKey := cfg.AnthropicKey
		if apiKey == "" {
			apiKey = cfg.APIKey
		}
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
		apiKey := cfg.AnthropicKey
		if apiKey == "" {
			apiKey = cfg.APIKey
		}
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
func NewBackendFromModel(model string) result.Result[backend.Backend] {
	b, err := newBackendFunc(BackendConfig{Model: model})
	if err != nil {
		return result.NewFailure[backend.Backend]([]result.SAWError{
			result.NewFatal(result.CodeAgentLaunchFailed,
				fmt.Sprintf("orchestrator: NewBackendFromModel: %s", err)),
		})
	}
	return result.NewSuccess(b)
}

// newRunnerFunc is a seam for tests: constructs the agent.Runner used by RunWave.
// Tests can replace this to inject a fake Backend without real API calls.
var newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
	return agent.NewRunner(b)
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
	logger         *slog.Logger     // nil = use slog.Default()
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

// SetLogger sets the logger used by all Orchestrator methods.
// Nil resets to slog.Default() fallback.
func (o *Orchestrator) SetLogger(logger *slog.Logger) {
	o.logger = logger
}

// log returns the configured logger, falling back to slog.Default() if nil.
func (o *Orchestrator) log() *slog.Logger {
	if o.logger == nil {
		return slog.Default()
	}
	return o.logger
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
func New(ctx context.Context, repoPath string, implDocPath string) result.Result[*Orchestrator] {
	var doc *protocol.IMPLManifest
	if implDocPath != "" {
		var err error
		doc, err = protocol.Load(ctx, implDocPath)
		if err != nil {
			return result.NewFailure[*Orchestrator]([]result.SAWError{
				result.NewFatal(result.CodeIMPLParseFailed,
					fmt.Sprintf("orchestrator.New: %s", err)),
			})
		}
	}
	return result.NewSuccess(&Orchestrator{
		state:       protocol.StateScoutPending,
		implDoc:     doc,
		repoPath:    repoPath,
		implDocPath: implDocPath,
	})
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
// It returns a fatal result if the transition is not permitted.
func (o *Orchestrator) TransitionTo(newState protocol.ProtocolState) result.Result[TransitionData] {
	from := o.state
	if !isValidTransition(from, newState) {
		return result.NewFailure[TransitionData]([]result.SAWError{
			result.NewFatal(result.CodeStateTransitionInvalid, fmt.Sprintf(
				"orchestrator: invalid state transition from %s to %s",
				from, newState,
			)),
		})
	}
	o.state = newState
	return result.NewSuccess(TransitionData{
		From: string(from),
		To:   string(newState),
	})
}

// RunWave executes all agents in wave waveNum concurrently. Each agent receives
// its own git worktree and the backend handles all LLM interaction internally.
// RunWave blocks until all agents complete (or one fails), then returns.
func (o *Orchestrator) RunWave(ctx context.Context, waveNum int) result.Result[WaveData] {
	if o.implDoc == nil {
		return result.NewFailure[WaveData]([]result.SAWError{
			result.NewFatal(result.CodeIMPLNotFound, "orchestrator.RunWave: no IMPL doc loaded"),
		})
	}
	// I1: Validate disjoint file ownership before any worktrees are created.
	if res := validateInvariantsFunc(o.implDoc); res.IsFatal() {
		msg := "orchestrator.RunWave: invariant violation"
		if len(res.Errors) > 0 {
			msg = fmt.Sprintf("orchestrator.RunWave: invariant violation: %s", res.Errors[0].Message)
		}
		return result.NewFailure[WaveData]([]result.SAWError{
			result.NewFatal(result.CodeInvariantViolation, msg),
		})
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
		return result.NewFailure[WaveData]([]result.SAWError{
			result.NewFatal(result.CodeWaveNotReady, fmt.Sprintf(
				"orchestrator.RunWave: wave %d not found in IMPL doc", waveNum)),
		})
	}
	o.currentWave = waveNum

	// Nothing to do if there are no waves defined.
	if wave == nil {
		return result.NewSuccess(WaveData{WaveNum: waveNum, AgentCount: 0})
	}

	// Reset per-agent retry counts for this wave run (transient state).
	implSlug := o.implSlug(ctx)
	for _, a := range wave.Agents {
		retryCountMap.Delete(fmt.Sprintf("%s:%d:%s", implSlug, waveNum, a.ID))
	}

	// Build the worktree manager and default agent runner.
	slug := o.implSlug(ctx)
	wm, wmErr := worktree.New(o.repoPath, slug)
	if wmErr != nil {
		return result.NewFailure[WaveData]([]result.SAWError{
			result.NewFatal(result.CodeWorktreeCreateFailed, wmErr.Error()),
		})
	}
	defaultBackend, err := newBackendFunc(BackendConfig{Kind: "auto", Model: o.defaultModel})
	if err != nil {
		return result.NewFailure[WaveData]([]result.SAWError{
			result.NewFatal(result.CodeAgentLaunchFailed,
				fmt.Sprintf("orchestrator.RunWave: failed to create backend: %s", err.Error())),
		})
	}
	defaultRunner := newRunnerFunc(defaultBackend, wm)

	// Launch all agents concurrently and collect the first error.
	eg, ctx := errgroup.WithContext(ctx)

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
			return result.NewFailure[WaveData]([]result.SAWError{
				result.NewFatal(result.CodeAgentLaunchFailed, fmt.Sprintf(
					"orchestrator.RunWave: prioritized agent %s not found in wave %d", agentID, waveNum)),
			})
		}

		eg.Go(func() error {
			// Build per-agent constraints from the IMPL manifest (I1/I2/I5/I6).
			constraints := buildWaveConstraints(ctx, o.implDocPath, protoAgent.ID)

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
		return result.NewFailure[WaveData]([]result.SAWError{
			result.NewFatal(result.CodeAgentLaunchFailed, err.Error()),
		})
	}

	// Best-effort cleanup of any worktrees tracked by this Manager instance.
	// merge.go handles removal of worktrees after merge; this catches any
	// that were created before merge runs or that merge.go missed.
	if cleanupResult := wm.CleanupAll(); cleanupResult.IsFatal() || cleanupResult.IsPartial() {
		for _, e := range cleanupResult.Errors {
			o.log().Warn("orchestrator.RunWave: worktree cleanup warning", "code", e.Code, "msg", e.Message)
		}
	}

	// All agents in the wave completed successfully.
	o.publish(OrchestratorEvent{
		Event: "wave_complete",
		Data: WaveCompletePayload{
			Wave:        waveNum,
			MergeStatus: "pending",
		},
	})

	return result.NewSuccess(WaveData{WaveNum: waveNum, AgentCount: len(wave.Agents)})
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
	slug := o.implSlug(ctx)
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
			o.log().Debug("orchestrator: reusing existing worktree", "agent", agentSpec.ID, "path", wtPath)
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
	manifest, err := protocol.Load(ctx, o.implDocPath)
	if err == nil {
		if contextPayload, extractErr := protocol.ExtractAgentContextFromManifest(ctx, manifest, agentSpec.ID); extractErr == nil {
			if jsonBytes, marshalErr := json.Marshal(contextPayload); marshalErr == nil {
				agentSpec.Task = string(jsonBytes)
			} else {
				o.log().Warn("orchestrator: failed to marshal E23 context", "err", marshalErr)
			}
		} else {
			// Fallback: use existing prompt from agentSpec (already set from IMPL doc parse).
			o.log().Warn("orchestrator: E23 context extraction failed", "agent", agentSpec.ID, "err", extractErr)
		}
	} else {
		o.log().Warn("orchestrator: failed to load manifest for E23 extraction", "err", err)
	}

	// Inject retry prefix after E23 extraction (GAP-4 fix).
	// retryPrefixMap is set by executeRetryLoop; read here after E23 so the prefix
	// is preserved even though E23 overwrites the base prompt.
	retryKey := fmt.Sprintf("%s:%d:%s", o.implSlug(ctx), waveNum, agentSpec.ID)
	if prefix, ok := retryPrefixMap.Load(retryKey); ok {
		if prefixStr, ok2 := prefix.(string); ok2 && prefixStr != "" {
			agentSpec.Task = prefixStr + "\n\n" + agentSpec.Task
			retryPrefixMap.Delete(retryKey) // consume once; recursive retries store fresh
		}
	}

	// Journal context recovery: sync and prepend if prior session has entries.
	// Non-fatal: any error is logged and execution continues with original prompt.
	fullAgentID := fmt.Sprintf("wave%d-agent-%s", waveNum, agentSpec.ID)
	if obsRes := journal.NewObserver(o.repoPath, fullAgentID); obsRes.IsSuccess() {
		observer := obsRes.GetData()
		if syncRes := observer.Sync(); syncRes.IsSuccess() {
			sr := syncRes.GetData()
			if sr != nil && sr.NewToolUses > 0 {
				if ctxRes := observer.GenerateContext(); ctxRes.IsSuccess() {
					agentSpec.Task = ctxRes.GetData() + "\n\n---\n\n" + agentSpec.Task
					o.log().Info("orchestrator: prepended journal context",
						"agent", agentSpec.ID, "tool_uses", sr.NewToolUses)
				}
			}
		}
		// Periodic sync: 30s goroutine tied to agent execution context.
		syncCtx, cancelSync := context.WithCancel(ctx)
		defer cancelSync()
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					observer.Sync() //nolint:errcheck // non-fatal periodic sync
				case <-syncCtx.Done():
					return
				}
			}
		}()
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
		if savedManifest, checkErr := protocol.Load(ctx, o.implDocPath); checkErr == nil {
			if cr, ok := savedManifest.CompletionReports[agentSpec.ID]; ok &&
				(cr.Status == protocol.StatusPartial || cr.Status == protocol.StatusBlocked) {
				report = &cr
			}
		}
	}
	if report == nil {
		// Only auto-synthesize complete if no existing partial/blocked report was found.
		commitSHA, filesChanged, autoErr := autoCommitWorktree(wtPath, waveNum, agentSpec.ID, baseSHA)
		if autoErr != nil {
			o.log().Warn("orchestrator: auto-commit failed", "agent", agentSpec.ID, "err", autoErr)
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
			Status:       protocol.StatusComplete,
			Worktree:     wtPath,
			Branch:       branch,
			Commit:       commitSHA,
			FilesChanged: filesChanged,
			Notes:        notes,
		}
	}

	// Populate dedup stats if available
	if report != nil && runner != nil {
		if stats := runner.DedupStats(); stats != nil {
			report.DedupStats = &protocol.DedupStats{
				Hits:                stats.Hits,
				Misses:              stats.Misses,
				TokensSavedEstimate: stats.TokensSavedEstimate,
			}
		}
	}

	// e. Always persist the completion report to the main branch IMPL doc.
	if report != nil {
		builder := protocol.NewCompletionReport(agentSpec.ID).
			WithStatus(report.Status).
			WithCommit(report.Commit).
			WithFiles(report.FilesChanged, report.FilesCreated).
			WithVerification(report.Verification).
			WithWorktree(report.Worktree).
			WithBranch(report.Branch).
			WithTestsAdded(report.TestsAdded).
			WithNotes(report.Notes).
			WithDedupStats(report.DedupStats).
			WithInterfaceDeviations(report.InterfaceDeviations).
			WithRepo(report.Repo)

		if report.FailureType != "" {
			builder = builder.WithFailureType(report.FailureType)
		}

		if saveErr := protocol.WithCompletionReportLock(ctx, func(ctx context.Context) error {
			manifest, loadErr := protocol.Load(ctx, o.implDocPath)
			if loadErr != nil {
				return loadErr
			}
			if appendErr := builder.AppendToManifest(manifest); appendErr != nil {
				return appendErr
			}
			if saveRes := protocol.Save(ctx, manifest, o.implDocPath); saveRes.IsFatal() {
				if len(saveRes.Errors) > 0 {
					return fmt.Errorf("%s", saveRes.Errors[0].Message)
				}
				return fmt.Errorf("failed to save manifest")
			}
			return nil
		}); saveErr != nil {
			o.log().Warn("orchestrator: failed to save report", "agent", agentSpec.ID, "err", saveErr)
		}
	}

	// BUG-5 fix: Determine whether E19 will trigger an automatic retry before
	// publishing agent_complete. If retry is coming, suppress the publish here —
	// the recursive launchAgent call (from executeRetryLoop) will publish
	// agent_complete with the final settled status.
	willAutoRetry := report != nil &&
		(report.Status == protocol.StatusPartial || report.Status == protocol.StatusBlocked) &&
		(report.FailureType == "transient" || report.FailureType == "fixable" || report.FailureType == "timeout")

	if !willAutoRetry {
		status := string(protocol.StatusComplete)
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
	if report != nil && (report.Status == protocol.StatusPartial || report.Status == protocol.StatusBlocked) {
		var failureType protocol.FailureTypeEnum
		switch report.FailureType {
		case "transient":
			failureType = protocol.FailureTransient
		case "fixable":
			failureType = protocol.FailureFixable
		case "needs_replan":
			failureType = protocol.FailureNeedsReplan
		case "escalate":
			failureType = protocol.FailureEscalate
		case "timeout":
			failureType = protocol.FailureTimeout
		default:
			failureType = protocol.FailureEscalate
		}

		action := RouteFailureWithReactions(failureType, o.implDoc.Reactions)
		o.publish(OrchestratorEvent{
			Event: "agent_blocked",
			Data: AgentBlockedPayload{
				Agent:       agentSpec.ID,
				Wave:        waveNum,
				Status:      string(report.Status),
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
	implSlug := o.implSlug(ctx)
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

	// Build retry context using retry package (enriched prompt with fix guidance).
	var promptPrefix string
	if o.implDocPath != "" {
		rcResult := retry.BuildRetryAttempt(ctx, o.implDocPath, agentSpec.ID, count)
		if rcResult.IsFatal() {
			if len(rcResult.Errors) > 0 {
				o.log().Debug("orchestrator: retry context build (best-effort)", "err", rcResult.Errors[0])
			}
		} else {
			rc := rcResult.GetData()
			if rc != nil && rc.PromptText != "" {
				promptPrefix = rc.PromptText
			}
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
func (o *Orchestrator) RunAgent(ctx context.Context, waveNum int, agentLetter string, promptPrefix string) result.Result[AgentData] {
	if o.implDoc == nil {
		return result.NewFailure[AgentData]([]result.SAWError{
			result.NewFatal(result.CodeIMPLNotFound, "orchestrator.RunAgent: no IMPL doc loaded"),
		})
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
		return result.NewFailure[AgentData]([]result.SAWError{
			result.NewFatal(result.CodeWaveNotReady, fmt.Sprintf(
				"orchestrator.RunAgent: wave %d not found", waveNum)),
		})
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
		return result.NewFailure[AgentData]([]result.SAWError{
			result.NewFatal(result.CodeAgentLaunchFailed, fmt.Sprintf(
				"orchestrator.RunAgent: agent %s not found in wave %d", agentLetter, waveNum)),
		})
	}

	// Use protocol.Agent directly (no agentToSpec adapter needed).
	spec := *protoAgent

	// Apply prompt prefix if provided (e.g. scope hint for timeout reruns).
	if promptPrefix != "" {
		spec.Task = promptPrefix + "\n\n" + spec.Task
	}

	// Build worktree manager and backend.
	wm, wmErr := worktree.New(o.repoPath, o.implSlug(ctx))
	if wmErr != nil {
		return result.NewFailure[AgentData]([]result.SAWError{
			result.NewFatal(result.CodeWorktreeCreateFailed, wmErr.Error()),
		})
	}
	b, err := newBackendFunc(BackendConfig{Kind: "auto", Model: o.defaultModel})
	if err != nil {
		return result.NewFailure[AgentData]([]result.SAWError{
			result.NewFatal(result.CodeAgentLaunchFailed,
				fmt.Sprintf("orchestrator.RunAgent: create backend: %s", err.Error())),
		})
	}
	if spec.Model != "" && spec.Model != o.defaultModel {
		b, err = newBackendFunc(BackendConfig{Kind: "auto", Model: spec.Model})
		if err != nil {
			return result.NewFailure[AgentData]([]result.SAWError{
				result.NewFatal(result.CodeAgentLaunchFailed, fmt.Sprintf(
					"orchestrator.RunAgent: create backend for model %s: %s", spec.Model, err.Error())),
			})
		}
	}
	runner := newRunnerFunc(b, wm)

	if launchErr := o.launchAgent(ctx, runner, wm, waveNum, spec); launchErr != nil {
		return result.NewFailure[AgentData]([]result.SAWError{
			result.NewFatal(result.CodeAgentLaunchFailed, launchErr.Error()),
		})
	}
	return result.NewSuccess(AgentData{WaveNum: waveNum, AgentLetter: agentLetter})
}

// MergeWave merges the worktrees for wave waveNum.
// Implementation is provided by merge.go via mergeWaveFunc.
func (o *Orchestrator) MergeWave(ctx context.Context, waveNum int) result.Result[MergeData] {
	return mergeWaveFunc(ctx, o, waveNum)
}

// RunVerification runs the post-merge test command.
// Implementation is provided by verification.go via runVerificationFunc.
func (o *Orchestrator) RunVerification(ctx context.Context, testCommand string) result.Result[VerificationData] {
	return runVerificationFunc(ctx, o, testCommand)
}

// UpdateIMPLStatus ticks the Status table checkboxes in the IMPL doc for all
// agents in waveNum that reported status: complete. Non-fatal: returns success
// if no Status section found. Returns failure only on file I/O failure.
func (o *Orchestrator) UpdateIMPLStatus(ctx context.Context, waveNum int) result.Result[UpdateData] {
	// 1. Find wave in o.implDoc.Waves by waveNum. If not found, return success.
	var wave *protocol.Wave
	for i := range o.implDoc.Waves {
		if o.implDoc.Waves[i].Number == waveNum {
			wave = &o.implDoc.Waves[i]
			break
		}
	}
	if wave == nil {
		return result.NewSuccess(UpdateData{WaveNum: waveNum, CompletedAgents: nil})
	}

	// 2. Load manifest and check completion reports.
	//    If report not found or status != StatusComplete, skip.
	manifest, err := protocol.Load(ctx, o.implDocPath)
	if err != nil {
		return result.NewSuccess(UpdateData{WaveNum: waveNum, CompletedAgents: nil}) // Cannot determine completed agents without manifest
	}

	var completedLetters []string
	for _, a := range wave.Agents {
		report, ok := manifest.CompletionReports[a.ID]
		if !ok {
			continue // Report not found
		}
		if report.Status != protocol.StatusComplete {
			continue
		}
		completedLetters = append(completedLetters, a.ID)
	}

	// 4. If no complete agents, return success.
	if len(completedLetters) == 0 {
		return result.NewSuccess(UpdateData{WaveNum: waveNum, CompletedAgents: nil})
	}

	// 5. Call protocol.UpdateIMPLStatus to tick checkboxes.
	res := protocol.UpdateIMPLStatus(o.implDocPath, completedLetters)
	if res.IsFatal() && len(res.Errors) > 0 {
		return result.NewFailure[UpdateData]([]result.SAWError{
			result.NewFatal(result.CodeStatusUpdateFailed, res.Errors[0].Message),
		})
	}
	return result.NewSuccess(UpdateData{WaveNum: waveNum, CompletedAgents: completedLetters})
}

// buildWaveConstraints loads the IMPL manifest and builds per-agent constraints
// for wave-role enforcement (I1 ownership, I2 freeze, I5 commit tracking).
// Returns nil if the manifest can't be loaded (backward compatible, no enforcement).
func buildWaveConstraints(ctx context.Context, implPath string, agentID string) *tools.Constraints {
	manifest, err := protocol.Load(ctx, implPath)
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
