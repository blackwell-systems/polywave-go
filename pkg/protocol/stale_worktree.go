package protocol

import (
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// StaleWorktree represents a single stale worktree detected during scanning.
type StaleWorktree struct {
	WorktreePath string `json:"worktree_path"`
	BranchName   string `json:"branch_name"`
	Slug         string `json:"slug"`
	Reason       string `json:"reason"` // "completed_impl" | "orphaned" | "merged_but_not_cleaned"
	RepoPath     string `json:"repo_path"`
}

// StaleCleanupResult represents the overall cleanup result for stale worktrees.
type StaleCleanupResult struct {
	Cleaned []StaleWorktree `json:"cleaned"`
	Skipped []StaleWorktree `json:"skipped"`
	Errors  []struct {
		Worktree StaleWorktree `json:"worktree"`
		Error    string        `json:"error"`
	} `json:"errors"`
}

// DetectStaleWorktrees scans git worktrees in repoPath, parses branch names to
// extract IMPL slugs, and checks whether each worktree is stale. A worktree is
// stale if its IMPL doc has been moved to complete/, if no IMPL doc exists at
// all (orphaned), or if its branch HEAD is already an ancestor of main.
func DetectStaleWorktrees(repoPath string) ([]StaleWorktree, error) {
	worktrees, err := git.WorktreeList(repoPath)
	if err != nil {
		return nil, err
	}

	var stale []StaleWorktree

	for _, wt := range worktrees {
		wtPath := wt[0]
		branch := wt[1]

		// Extract slug; skip legacy branches with no slug.
		slug := ExtractSlug(branch)
		if slug == "" {
			continue
		}

		// Verify this is a SAW branch (has wave/agent info).
		if _, _, ok := ParseBranch(branch); !ok {
			continue
		}

		// Check for completed IMPL.
		completePath := filepath.Join(repoPath, "docs", "IMPL", "complete", "IMPL-"+slug+".yaml")
		activePath := filepath.Join(repoPath, "docs", "IMPL", "IMPL-"+slug+".yaml")

		if fileExists(completePath) {
			stale = append(stale, StaleWorktree{
				WorktreePath: wtPath,
				BranchName:   branch,
				Slug:         slug,
				Reason:       "completed_impl",
				RepoPath:     repoPath,
			})
			continue
		}

		if !fileExists(activePath) {
			stale = append(stale, StaleWorktree{
				WorktreePath: wtPath,
				BranchName:   branch,
				Slug:         slug,
				Reason:       "orphaned",
				RepoPath:     repoPath,
			})
			continue
		}

		// Check if branch HEAD is ancestor of main (already merged).
		branchSHA, err := git.RevParse(repoPath, "refs/heads/"+branch)
		if err != nil {
			// Branch may not exist as a ref; skip.
			continue
		}
		if git.IsAncestor(repoPath, branchSHA, "main") {
			stale = append(stale, StaleWorktree{
				WorktreePath: wtPath,
				BranchName:   branch,
				Slug:         slug,
				Reason:       "merged_but_not_cleaned",
				RepoPath:     repoPath,
			})
		}
	}

	return stale, nil
}

// CleanStaleWorktrees removes the given stale worktrees. It skips worktrees
// with uncommitted changes unless force is true. Returns structured results.
func CleanStaleWorktrees(stale []StaleWorktree, force bool) (*StaleCleanupResult, error) {
	result := &StaleCleanupResult{
		Cleaned: []StaleWorktree{},
		Skipped: []StaleWorktree{},
		Errors: []struct {
			Worktree StaleWorktree `json:"worktree"`
			Error    string        `json:"error"`
		}{},
	}

	for _, sw := range stale {
		// Check for uncommitted changes.
		status, err := git.StatusPorcelain(sw.WorktreePath)
		if err == nil && status != "" && !force {
			result.Skipped = append(result.Skipped, sw)
			continue
		}

		// Remove worktree.
		if err := git.WorktreeRemove(sw.RepoPath, sw.WorktreePath); err != nil {
			result.Errors = append(result.Errors, struct {
				Worktree StaleWorktree `json:"worktree"`
				Error    string        `json:"error"`
			}{Worktree: sw, Error: err.Error()})
			continue
		}

		// Delete branch.
		if err := git.DeleteBranch(sw.RepoPath, sw.BranchName); err != nil {
			// Worktree removed but branch delete failed — still report as error.
			result.Errors = append(result.Errors, struct {
				Worktree StaleWorktree `json:"worktree"`
				Error    string        `json:"error"`
			}{Worktree: sw, Error: "worktree removed but branch delete failed: " + err.Error()})
			continue
		}

		// Prune stale worktree metadata.
		_ = git.WorktreePrune(sw.RepoPath)

		result.Cleaned = append(result.Cleaned, sw)
	}

	return result, nil
}

// fileExists returns true if the path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
