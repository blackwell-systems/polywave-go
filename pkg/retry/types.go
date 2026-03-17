package retry

// RetryConfig configures the E24 verification loop behavior.
type RetryConfig struct {
	MaxRetries int    // Maximum retry attempts (default: 2, 3rd failure -> blocked)
	IMPLPath   string // Path to the parent IMPL manifest
	RepoPath   string // Repository root path
}

// RetryResult captures the outcome of a retry attempt.
type RetryResult struct {
	Attempt    int    `json:"attempt"`
	AgentID    string `json:"agent_id"`
	GatePassed bool   `json:"gate_passed"`
	GateOutput string `json:"gate_output"`
	RetryIMPL  string `json:"retry_impl,omitempty"`
	FinalState string `json:"final_state"` // "passed" | "retrying" | "blocked"
}

// QualityGateFailure describes a quality gate that failed after wave merge.
// This is a local type to avoid importing pkg/engine or pkg/orchestrator.
type QualityGateFailure struct {
	GateType    string   // "build", "test", "lint"
	Command     string   // the command that failed
	Output      string   // stderr/stdout from the failed command
	FailedFiles []string // files implicated in the failure
}

// Event is a retry loop lifecycle event for progress reporting.
// Mirrors engine.Event shape without importing it.
type Event struct {
	Event string      `json:"event"` // "retry_started", "retry_gate_pass", "retry_gate_fail", "retry_blocked"
	Data  interface{} `json:"data,omitempty"`
}
