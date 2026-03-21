package orchestrator

// OrchestratorEvent is emitted during wave execution.
// The API layer maps these to api.SSEEvent without the orchestrator importing pkg/api.
type OrchestratorEvent struct {
	Event string
	Data  interface{}
}

// EventPublisher is a function that receives orchestrator events.
// The API layer injects a concrete implementation via SetEventPublisher.
type EventPublisher func(ev OrchestratorEvent)

// AgentStartedPayload is the Data payload for the "agent_started" event.
type AgentStartedPayload struct {
	Agent string   `json:"agent"`
	Wave  int      `json:"wave"`
	Files []string `json:"files"`
}

// AgentCompletePayload is the Data payload for the "agent_complete" event.
type AgentCompletePayload struct {
	Agent  string `json:"agent"`
	Wave   int    `json:"wave"`
	Status string `json:"status"`
	Branch string `json:"branch"`
}

// AgentFailedPayload is the Data payload for the "agent_failed" event.
type AgentFailedPayload struct {
	Agent       string `json:"agent"`
	Wave        int    `json:"wave"`
	Status      string `json:"status"`
	FailureType string `json:"failure_type"`
	Notes       string `json:"notes,omitempty"`
	Message     string `json:"message"`
}

// WaveCompletePayload is the Data payload for the "wave_complete" event.
type WaveCompletePayload struct {
	Wave        int    `json:"wave"`
	MergeStatus string `json:"merge_status"`
}

// RunCompletePayload is the Data payload for the "run_complete" event.
type RunCompletePayload struct {
	Status string `json:"status"`
	Waves  int    `json:"waves"`
	Agents int    `json:"agents"`
}

// AgentOutputPayload is the Data payload for the "agent_output" SSE event.
// It is emitted once per text chunk while the agent is running.
type AgentOutputPayload struct {
	Agent string `json:"agent"`
	Wave  int    `json:"wave"`
	Chunk string `json:"chunk"`
}

// AgentToolCallPayload is the Data payload for the "agent_tool_call" SSE event.
// Emitted once per tool invocation (IsResult=false) and once per tool result (IsResult=true).
type AgentToolCallPayload struct {
	Agent      string `json:"agent"`
	Wave       int    `json:"wave"`
	ToolID     string `json:"tool_id"`
	ToolName   string `json:"tool_name"`
	Input      string `json:"input"`
	IsResult   bool   `json:"is_result"`
	IsError    bool   `json:"is_error"`
	DurationMs int64  `json:"duration_ms"`
}

// AgentPrioritizedPayload is the Data payload for the "agent_prioritized" event.
// It shows when agents in a wave are reordered for optimal scheduling based on
// dependency graph critical path analysis.
type AgentPrioritizedPayload struct {
	Wave             int      `json:"wave"`
	OriginalOrder    []string `json:"original_order"`
	PrioritizedOrder []string `json:"prioritized_order"`
	Reordered        bool     `json:"reordered"`
	Reason           string   `json:"reason"`
}

// AutoRetryStartedPayload is published when the orchestrator auto-retries an agent (E19).
type AutoRetryStartedPayload struct {
	Agent       string `json:"agent"`
	Wave        int    `json:"wave"`
	FailureType string `json:"failure_type"`
	Attempt     int    `json:"attempt"`
	MaxAttempts int    `json:"max_attempts"`
}

// AutoRetryExhaustedPayload is published when auto-retries are exhausted (E19).
type AutoRetryExhaustedPayload struct {
	Agent       string `json:"agent"`
	Wave        int    `json:"wave"`
	FailureType string `json:"failure_type"`
	Attempts    int    `json:"attempts"`
}

// --- Program Lifecycle Events (E40) ---
// These event types cover all state transitions in the program tier loop.
// They are emitted by RunTierLoop via the OnEvent callback.

// ProgramTierStartedPayload is the Data payload for the "program_tier_started" event.
// Emitted when a tier begins execution.
type ProgramTierStartedPayload struct {
	Tier       int      `json:"tier"`
	TotalTiers int      `json:"total_tiers"`
	IMPLs      []string `json:"impls"`
}

// ProgramScoutLaunchedPayload is the Data payload for the "program_scout_launched" event.
// Emitted when a Scout agent is launched within a tier.
type ProgramScoutLaunchedPayload struct {
	Tier     int    `json:"tier"`
	IMPLSlug string `json:"impl_slug"`
}

// ProgramScoutCompletePayload is the Data payload for the "program_scout_complete" event.
// Emitted when a Scout agent finishes producing an IMPL doc.
type ProgramScoutCompletePayload struct {
	Tier     int    `json:"tier"`
	IMPLSlug string `json:"impl_slug"`
	Status   string `json:"status"` // "complete", "failed"
	Error    string `json:"error,omitempty"`
}

// ProgramIMPLCompletePayload is the Data payload for the "program_impl_complete" event.
// Emitted when an IMPL completes all its waves within a tier.
type ProgramIMPLCompletePayload struct {
	Tier       int    `json:"tier"`
	IMPLSlug   string `json:"impl_slug"`
	WaveCount  int    `json:"wave_count"`
	AgentCount int    `json:"agent_count"`
}

// ProgramTierGateStartedPayload is the Data payload for the "program_tier_gate_started" event.
// Emitted when tier gate verification begins (E29).
type ProgramTierGateStartedPayload struct {
	Tier int `json:"tier"`
}

// ProgramTierGateResultPayload is the Data payload for the "program_tier_gate_result" event.
// Emitted when a tier gate passes or fails (E29).
type ProgramTierGateResultPayload struct {
	Tier   int    `json:"tier"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

// ProgramContractsFrozenPayload is the Data payload for the "program_contracts_frozen" event.
// Emitted when contracts are frozen at a tier boundary (E30).
type ProgramContractsFrozenPayload struct {
	Tier      int      `json:"tier"`
	Contracts []string `json:"contracts"` // names/paths of frozen contracts
}

// ProgramTierAdvancedPayload is the Data payload for the "program_tier_advanced" event.
// Emitted when the program advances from one tier to the next.
type ProgramTierAdvancedPayload struct {
	FromTier int `json:"from_tier"`
	ToTier   int `json:"to_tier"`
}

// ProgramReplanTriggeredPayload is the Data payload for the "program_replan_triggered" event.
// Emitted when Planner re-engagement starts (E34).
type ProgramReplanTriggeredPayload struct {
	Tier   int    `json:"tier"`
	Reason string `json:"reason"`
}

// ProgramCompletePayload is the Data payload for the "program_complete" event.
// Emitted when all tiers in the program have completed successfully.
type ProgramCompletePayload struct {
	TotalTiers int    `json:"total_tiers"`
	TotalIMPLs int    `json:"total_impls"`
	FinalState string `json:"final_state"`
}

// Program event type constants for use with OrchestratorEvent.Event field.
const (
	EventProgramTierStarted      = "program_tier_started"
	EventProgramScoutLaunched    = "program_scout_launched"
	EventProgramScoutComplete    = "program_scout_complete"
	EventProgramIMPLComplete     = "program_impl_complete"
	EventProgramTierGateStarted  = "program_tier_gate_started"
	EventProgramTierGateResult   = "program_tier_gate_result"
	EventProgramContractsFrozen  = "program_contracts_frozen"
	EventProgramTierAdvanced     = "program_tier_advanced"
	EventProgramReplanTriggered  = "program_replan_triggered"
	EventProgramComplete         = "program_complete"
)

// SetEventPublisher injects a publisher function that will receive all
// OrchestratorEvents emitted during wave execution.
func (o *Orchestrator) SetEventPublisher(pub EventPublisher) {
	o.eventPublisher = pub
}
