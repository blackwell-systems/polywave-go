package solver

// DepNode represents one agent in the dependency graph.
// AgentID is the protocol agent ID (e.g. "A", "B2").
// DependsOn lists agent IDs this agent depends on.
type DepNode struct {
	AgentID   string
	DependsOn []string
	Files     []string
}

// Assignment is the solver output: an agent ID mapped to its computed wave number.
type Assignment struct {
	AgentID string
	Wave    int
}

// SolveResult holds the full solver output.
type SolveResult struct {
	Assignments []Assignment
	WaveCount   int
	Valid       bool
	Errors      []string
}
