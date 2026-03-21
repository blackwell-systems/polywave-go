package protocol

// SCAFFOLD: This file is a compilation stub created by Agent B (Wave 1).
// Agent A (Wave 1) owns and implements the real version of this file.
// This stub will be replaced by Agent A's implementation at merge time.

// StaleWorktree represents a single stale worktree detected during scanning.
type StaleWorktree struct {
	WorktreePath string `json:"worktree_path"`
	BranchName   string `json:"branch_name"`
	Slug         string `json:"slug"`
	Reason       string `json:"reason"` // "completed_impl" | "orphaned" | "merged_but_not_cleaned"
	RepoPath     string `json:"repo_path"`
}

// StaleCleanupResult holds the results of cleaning stale worktrees.
type StaleCleanupResult struct {
	Cleaned []StaleWorktree `json:"cleaned"`
	Skipped []StaleWorktree `json:"skipped"`
	Errors  []struct {
		Worktree StaleWorktree `json:"worktree"`
		Error    string        `json:"error"`
	} `json:"errors"`
}

// DetectStaleWorktrees scans git worktrees and returns stale ones.
// STUB: Real implementation by Agent A.
func DetectStaleWorktrees(repoPath string) ([]StaleWorktree, error) {
	return nil, nil
}

// CleanStaleWorktrees removes stale worktrees.
// STUB: Real implementation by Agent A.
func CleanStaleWorktrees(stale []StaleWorktree, force bool) (*StaleCleanupResult, error) {
	return &StaleCleanupResult{}, nil
}
