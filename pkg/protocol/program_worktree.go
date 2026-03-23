package protocol

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ProgramBranchName returns the slug-scoped branch name for a program-tier IMPL.
// Format: saw/program/{program-slug}/tier{N}-impl-{impl-slug}
func ProgramBranchName(programSlug string, tierNumber int, implSlug string) string {
	return fmt.Sprintf("saw/program/%s/tier%d-impl-%s", programSlug, tierNumber, implSlug)
}

// ProgramWorktreeDir returns the worktree directory path for a program-tier IMPL.
// Format: {repoDir}/.claude/worktrees/saw/program/{program-slug}/tier{N}-impl-{impl-slug}
func ProgramWorktreeDir(repoDir, programSlug string, tierNumber int, implSlug string) string {
	return filepath.Join(repoDir, ".claude", "worktrees", "saw", "program", programSlug,
		fmt.Sprintf("tier%d-impl-%s", tierNumber, implSlug))
}

// ProgramWorktreeInfo contains the details of a created worktree for a single IMPL.
type ProgramWorktreeInfo struct {
	ImplSlug string `json:"impl_slug"`
	Path     string `json:"path"`
	Branch   string `json:"branch"`
}

// CreateProgramWorktreesData contains the list of worktrees created for a program tier.
type CreateProgramWorktreesData struct {
	TierNumber int                   `json:"tier_number"`
	Worktrees  []ProgramWorktreeInfo `json:"worktrees"`
}

// CreateProgramWorktrees creates git worktrees for all IMPLs in a program tier.
// It parses the PROGRAM manifest from programManifestPath, finds the tier by tierNumber,
// and creates a worktree for each IMPL slug in that tier.
//
// Branch names follow: saw/program/{program-slug}/tier{N}-impl-{impl-slug}
// Worktree paths follow: {repoDir}/.claude/worktrees/saw/program/{program-slug}/tier{N}-impl-{impl-slug}
//
// If any worktree creation fails, returns a failure result immediately (no partial state).
func CreateProgramWorktrees(programManifestPath string, tierNumber int, repoDir string) result.Result[*CreateProgramWorktreesData] {
	manifest, err := ParseProgramManifest(programManifestPath)
	if err != nil {
		return result.NewFailure[*CreateProgramWorktreesData]([]result.StructuredError{{
			Code: "E_PROGRAM_WORKTREE", Message: fmt.Sprintf("failed to parse program manifest: %v", err), Severity: "fatal",
		}})
	}

	// Find the tier by number
	var targetTier *ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == tierNumber {
			targetTier = &manifest.Tiers[i]
			break
		}
	}

	if targetTier == nil {
		return result.NewFailure[*CreateProgramWorktreesData]([]result.StructuredError{{
			Code: "E_PROGRAM_WORKTREE", Message: fmt.Sprintf("tier %d not found in program manifest", tierNumber), Severity: "fatal",
		}})
	}

	// Resolve absolute path for repoDir
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return result.NewFailure[*CreateProgramWorktreesData]([]result.StructuredError{{
			Code: "E_PROGRAM_WORKTREE", Message: fmt.Sprintf("failed to resolve repo path: %v", err), Severity: "fatal",
		}})
	}

	var worktrees []ProgramWorktreeInfo

	for _, implSlug := range targetTier.Impls {
		branchName := ProgramBranchName(manifest.ProgramSlug, tierNumber, implSlug)
		worktreePath := ProgramWorktreeDir(absRepoDir, manifest.ProgramSlug, tierNumber, implSlug)

		// Auto-clean stale merged branches
		if git.BranchExists(absRepoDir, branchName) {
			if git.IsAncestor(absRepoDir, branchName, "HEAD") {
				_ = git.WorktreeRemove(absRepoDir, worktreePath)
				_ = git.DeleteBranch(absRepoDir, branchName)
				fmt.Fprintf(os.Stderr, "Cleaned up stale merged branch %q\n", branchName)
			} else {
				return result.NewFailure[*CreateProgramWorktreesData]([]result.StructuredError{{
					Code: "E_PROGRAM_WORKTREE", Message: fmt.Sprintf("branch %q already exists and is not merged into HEAD; delete it manually or merge first", branchName), Severity: "fatal",
				}})
			}
		}

		// Create the worktree
		if err := git.WorktreeAdd(absRepoDir, worktreePath, branchName); err != nil {
			return result.NewFailure[*CreateProgramWorktreesData]([]result.StructuredError{{
				Code: "E_PROGRAM_WORKTREE", Message: fmt.Sprintf("failed to create worktree for impl %s: %v", implSlug, err), Severity: "fatal",
			}})
		}

		// Install pre-commit hook (log warning on error, don't fail)
		if err := git.InstallHooks(absRepoDir, worktreePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to install hooks for impl %s: %v\n", implSlug, err)
		}

		worktrees = append(worktrees, ProgramWorktreeInfo{
			ImplSlug: implSlug,
			Path:     worktreePath,
			Branch:   branchName,
		})
	}

	return result.NewSuccess(&CreateProgramWorktreesData{
		TierNumber: tierNumber,
		Worktrees:  worktrees,
	})
}
