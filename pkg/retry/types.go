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
