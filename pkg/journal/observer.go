package journal

// Path is a type alias for filesystem paths
type Path string

// JournalObserver tracks tool execution history for a Wave agent.
// Full implementation in Agent A's worktree.
// This stub definition allows Agent C's checkpoint code to compile independently.
type JournalObserver struct {
	ProjectRoot Path
	JournalDir  Path
	AgentID     string
	cursorPath  Path
	indexPath   Path
	recentPath  Path
	resultsDir  Path
}
