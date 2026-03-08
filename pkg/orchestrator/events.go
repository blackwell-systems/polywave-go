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

// SetEventPublisher injects a publisher function that will receive all
// OrchestratorEvents emitted during wave execution.
func (o *Orchestrator) SetEventPublisher(pub EventPublisher) {
	o.eventPublisher = pub
}
