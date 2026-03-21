package protocol

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// CompletionValidationResult captures the outcome of cross-validating a
// completion report's claims against actual git state and the file system.
type CompletionValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateCompletionReportClaims cross-validates a completion report's fields
// against actual git state and disk state.
//
// It performs the following checks:
//   - Commit SHA exists in the repository (git rev-parse --verify).
//   - Commit is reachable from the agent's expected branch.
//   - FilesChanged is a subset of the agent's owned files (union frozen scaffold paths).
//   - FilesCreated actually exist on disk in repoDir.
//   - Worktree path exists on disk (if non-empty).
//
// repoDir must be the absolute path to the repository root. An error is
// returned only for system-level failures (e.g. cannot load manifest); field
// violations are recorded in result.Errors with Valid=false.
func ValidateCompletionReportClaims(
	manifest *IMPLManifest,
	agentID string,
	report CompletionReport,
	repoDir string,
) (*CompletionValidationResult, error) {
	result := &CompletionValidationResult{Valid: true}

	// --- Commit SHA verification ---
	if report.Commit != "" {
		// Use cat-file -e to check object existence. git rev-parse --verify
		// exits 0 even for syntactically valid SHAs that don't exist in the repo.
		_, err := git.Run(repoDir, "cat-file", "-e", report.Commit)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("commit %s does not exist in repository: %v", report.Commit, err))
		} else {
			// Verify the commit is on the agent's branch.
			// git branch --contains <sha> lists all branches reachable from the commit.
			out, branchErr := git.Run(repoDir, "branch", "--contains", report.Commit)
			if branchErr != nil {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("could not verify commit branch membership: %v", branchErr))
			} else {
				// Derive the expected branch name for this agent from the manifest.
				expectedBranch := ""
				if manifest.FeatureSlug != "" {
					// Find the wave number for this agent from the file ownership table.
					waveNum := 0
					for _, fo := range manifest.FileOwnership {
						if fo.Agent == agentID {
							waveNum = fo.Wave
							break
						}
					}
					if waveNum > 0 {
						expectedBranch = BranchName(manifest.FeatureSlug, waveNum, agentID)
					}
				}
				// Also check legacy / report-supplied branch.
				branchToFind := report.Branch
				if branchToFind == "" {
					branchToFind = expectedBranch
				}
				if branchToFind != "" && !branchContains(out, branchToFind) {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("commit %s is not reachable from branch %s", report.Commit, branchToFind))
				}
			}
		}
	}

	// --- Build owned-files set for this agent ---
	ownedFiles := make(map[string]bool)
	for _, fo := range manifest.FileOwnership {
		if fo.Agent == agentID {
			ownedFiles[fo.File] = true
		}
	}

	// Scaffold files are "frozen paths" — any agent may touch them without
	// violating I1 because they were pre-committed before wave execution.
	frozenPaths := make(map[string]bool)
	for _, sc := range manifest.Scaffolds {
		if sc.FilePath != "" {
			frozenPaths[sc.FilePath] = true
		}
	}

	// --- FilesChanged ownership check ---
	for _, f := range report.FilesChanged {
		if !ownedFiles[f] && !frozenPaths[f] {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("file %s in files_changed is not in agent %s owned files or frozen scaffold paths", f, agentID))
		}
	}

	// --- FilesCreated existence check ---
	for _, f := range report.FilesCreated {
		fullPath := repoDir + "/" + f
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			result.Valid = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("file %s in files_created does not exist on disk at %s", f, fullPath))
		}
	}

	// --- Worktree path existence check ---
	if report.Worktree != "" {
		if _, err := os.Stat(report.Worktree); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("worktree path %s does not exist on disk (may have been cleaned up)", report.Worktree))
		}
	}

	return result, nil
}

// branchContains checks whether branchName appears in the output of
// "git branch --contains <sha>". Each line is of the form "  branch-name"
// or "* branch-name" (current branch).
func branchContains(output, branchName string) bool {
	for _, line := range strings.Split(output, "\n") {
		candidate := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "* "))
		if candidate == branchName {
			return true
		}
	}
	return false
}
