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

// SetEventPublisher injects a publisher function that will receive all
// OrchestratorEvents emitted during wave execution.
func (o *Orchestrator) SetEventPublisher(pub EventPublisher) {
	o.eventPublisher = pub
}
