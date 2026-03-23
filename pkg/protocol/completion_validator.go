package protocol

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// CompletionValidationData captures the outcome of cross-validating a
// completion report's claims against actual git state and the file system.
type CompletionValidationData struct {
	Valid    bool     `json:"valid"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateCompletionReportClaims cross-validates a completion report's fields
// against actual git state and disk state.
//
// It performs the following checks:
//   - Commit SHA exists in the repository (git cat-file -e).
//   - Commit is reachable from the agent's expected branch.
//   - FilesChanged is a subset of the agent's owned files (union frozen scaffold paths).
//   - FilesCreated actually exist on disk in repoDir.
//   - Worktree path exists on disk (if non-empty).
//
// repoDir must be the absolute path to the repository root. A FATAL result is
// returned only for field violations; system-level failures produce a FATAL
// result with the error message included.
func ValidateCompletionReportClaims(
	manifest *IMPLManifest,
	agentID string,
	report CompletionReport,
	repoDir string,
) result.Result[CompletionValidationData] {
	data := CompletionValidationData{Valid: true}
	var errs []result.StructuredError
	var warnings []string

	// --- Commit SHA verification ---
	if report.Commit != "" {
		// Use cat-file -e to check object existence. git rev-parse --verify
		// exits 0 even for syntactically valid SHAs that don't exist in the repo.
		_, err := git.Run(repoDir, "cat-file", "-e", report.Commit)
		if err != nil {
			data.Valid = false
			errs = append(errs, result.StructuredError{
				Code:     "E001",
				Message:  fmt.Sprintf("commit %s does not exist in repository: %v", report.Commit, err),
				Severity: "fatal",
				Field:    "commit",
			})
		} else {
			// Verify the commit is on the agent's branch.
			// git branch --contains <sha> lists all branches reachable from the commit.
			out, branchErr := git.Run(repoDir, "branch", "--contains", report.Commit)
			if branchErr != nil {
				warnings = append(warnings,
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
					warnings = append(warnings,
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
			data.Valid = false
			errs = append(errs, result.StructuredError{
				Code:     "E002",
				Message:  fmt.Sprintf("file %s in files_changed is not in agent %s owned files or frozen scaffold paths", f, agentID),
				Severity: "fatal",
				Field:    "files_changed",
				File:     f,
			})
		}
	}

	// --- FilesCreated existence check ---
	for _, f := range report.FilesCreated {
		fullPath := repoDir + "/" + f
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			data.Valid = false
			errs = append(errs, result.StructuredError{
				Code:     "E003",
				Message:  fmt.Sprintf("file %s in files_created does not exist on disk at %s", f, fullPath),
				Severity: "fatal",
				Field:    "files_created",
				File:     f,
			})
		}
	}

	// --- Worktree path existence check ---
	if report.Worktree != "" {
		if _, err := os.Stat(report.Worktree); os.IsNotExist(err) {
			warnings = append(warnings,
				fmt.Sprintf("worktree path %s does not exist on disk (may have been cleaned up)", report.Worktree))
		}
	}

	data.Warnings = warnings

	if len(errs) > 0 {
		return result.NewFailure[CompletionValidationData](errs)
	}
	if len(warnings) > 0 {
		return result.NewPartial(data, nil)
	}
	return result.NewSuccess(data)
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
