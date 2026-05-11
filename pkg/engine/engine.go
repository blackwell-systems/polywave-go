package engine

import (
	"context"
	"log/slog"

	"github.com/blackwell-systems/polywave-go/pkg/agent/backend"
	"github.com/blackwell-systems/polywave-go/pkg/observability"
	"github.com/blackwell-systems/polywave-go/pkg/orchestrator"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func init() {
	// Inject the structured wave agent runner into the orchestrator package.
	// This breaks the circular import (orchestrator → engine → orchestrator)
	// by using a function-variable seam.
	orchestrator.SetRunWaveAgentStructuredFunc(func(ctx context.Context, implPath, waveModel string, agentSpec protocol.Agent, wtPath string, onChunk func(string)) error {
		opts := RunWaveOpts{
			IMPLPath:  implPath,
			WaveModel: waveModel,
		}
		_, err := runWaveAgentStructured(ctx, opts, agentSpec, wtPath, onChunk)
		return err
	})

	// Inject the DAG-based agent scheduler into the orchestrator package.
	// PrioritizeAgents (in scheduler.go) orders agents by dependency graph
	// so independent agents can run in parallel.
	orchestrator.SetPrioritizeAgentsFunc(PrioritizeAgents)
}

// ObsEmitter is the observability emitter interface accepted by engine opts.
// *observability.Emitter satisfies this interface; a nil *observability.Emitter
// is also safe (all methods are nil-receiver-safe on the concrete type).
type ObsEmitter interface {
	Emit(ctx context.Context, event observability.Event)
	EmitSync(ctx context.Context, event observability.Event) result.Result[observability.EmitData]
}

// Event is emitted during wave execution (mirrors orchestrator.OrchestratorEvent).
type Event struct {
	Event string // e.g. "agent_started", "agent_complete", "run_complete"
	// Data is a typed payload struct; see runner_data_types.go for the full list.
	Data any
}

// RunScoutOpts configures a Scout agent run.
type RunScoutOpts struct {
	Feature              string                  // human feature description (required)
	RepoPath             string                  // absolute path to the repository being scouted (required)
	PolywaveRepoPath          string                  // path to scout-and-wave protocol repo (optional; falls back to $POLYWAVE_REPO then ~/code/scout-and-wave)
	IMPLOutPath          string                  // where to write the IMPL doc (required)
	ScoutModel           string                  // optional: model override for the Scout agent (e.g. "claude-opus-4-6")
	ProgramManifestPath  string                  // optional: path to PROGRAM manifest; Scout receives frozen contracts as input
	UseStructuredOutput  bool                    // if true, invoke Scout via API backend with output_config.format
	OutputSchemaOverride map[string]any          // optional: overrides GenerateScoutSchema(); useful in tests
	ObsEmitter           ObsEmitter   // optional: non-blocking observability emitter
	Logger               *slog.Logger // optional: nil falls back to slog.Default()
}

// RunPlannerOpts configures a Planner agent run.
type RunPlannerOpts struct {
	Description    string // human project description (required)
	RepoPath       string // absolute path to the repository being planned (required)
	PolywaveRepoPath    string // path to scout-and-wave protocol repo (optional; falls back to $POLYWAVE_REPO then ~/code/scout-and-wave)
	ProgramOutPath string // where to write the PROGRAM manifest (required)
	PlannerModel   string // optional: model override for the Planner agent
}

// RunWaveOpts configures a wave execution run.
type RunWaveOpts struct {
	IMPLPath             string // absolute path to IMPL doc (required)
	RepoPath             string // absolute path to the target repository (required)
	Slug                 string // IMPL slug for event routing (required)
	WaveModel            string // optional: default model for wave agents; per-agent model: field overrides this
	ScaffoldModel        string // optional: model for scaffold agent; falls back to WaveModel if empty
	IntegrationModel     string // optional: model for integration agent (E26); falls back to WaveModel if empty
	UseStructuredOutput  bool         // if true, use structured output for wave agent completion reports
	Logger               *slog.Logger // optional: nil falls back to slog.Default()
}

// RunMergeOpts configures a merge operation.
type RunMergeOpts struct {
	IMPLPath string
	RepoPath string
	WaveNum  int
	Logger   *slog.Logger // optional: nil falls back to slog.Default()
}

// ResolveConflictsOpts configures the engine-level conflict resolution function.
type ResolveConflictsOpts struct {
	IMPLPath   string                          // path to IMPL YAML manifest
	RepoPath   string                          // repo root (where git merge is in progress)
	WaveNum    int                             // which wave's merge is conflicted
	ChatModel  string                          // optional model override
	OnProgress func(file string, status string) // per-file progress callback
	OnOutput   func(chunk string)              // streaming output callback (model text chunks)
}

// FixBuildOpts configures the AI-powered build failure fixer.
type FixBuildOpts struct {
	IMPLPath   string                       // path to IMPL YAML manifest
	RepoPath   string                       // repo root
	WaveNum    int                          // which wave's build failed
	ErrorLog   string                       // captured test/lint/gate output
	GateType   string                       // "test", "typecheck", "lint", "build", or "custom"
	ChatModel  string                       // optional model override (same format as conflict resolver)
	OnOutput   func(chunk string)           // streaming text callback
	OnToolCall func(ev backend.ToolCallEvent) // optional tool call observability
	Logger     *slog.Logger                  // optional: nil falls back to slog.Default()
}

// RunVerificationOpts configures post-merge verification.
type RunVerificationOpts struct {
	RepoPath    string
	TestCommand string // falls back to "go test ./..." if empty
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

// RunChatOpts configures a chat agent run with conversation history.
type RunChatOpts struct {
	IMPLPath    string        // path to IMPL doc for context (required)
	RepoPath    string        // absolute path to the repository (required)
	PolywaveRepoPath string        // path to scout-and-wave protocol repo (optional)
	History     []ChatMessage // previous conversation turns (optional)
	Message     string        // current user message (required)
	ChatModel   string        // model override (e.g. "ollama:qwen2.5-coder:32b"); empty = backend default
}

