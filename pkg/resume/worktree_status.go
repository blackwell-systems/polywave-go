package resume

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// DirtyWorktree describes a SAW agent worktree and whether it has uncommitted changes.
// NOTE: Agent A (detect.go) also declares this struct as part of the SessionState
// extension. The Integration Agent will deduplicate at merge time — only one
// declaration should remain in the final package.
type DirtyWorktree struct {
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	AgentID    string `json:"agent_id"`
	WaveNum    int    `json:"wave_num"`
	HasChanges bool   `json:"has_changes"` // uncommitted modifications
}

// ClassifyWorktrees inspects each worktree path to determine if it has uncommitted
// changes (dirty) or is clean. It uses `git status --porcelain` on each path.
//
// Only worktrees whose branch name matches the SAW pattern and belongs to the given
// manifest's FeatureSlug are included. Worktrees with non-matching branches are
// skipped. Paths that do not exist are silently skipped. Git failures are treated
// as clean (not an error). Locked worktrees are conservatively treated as dirty.
func ClassifyWorktrees(worktreePaths []string, manifest *protocol.IMPLManifest) ([]DirtyWorktree, error) {
	// Build the required slug prefix for branch filtering.
	slugPrefix := ""
	if manifest != nil && manifest.FeatureSlug != "" {
		slugPrefix = "saw/" + manifest.FeatureSlug + "/"
	}

	// Collect locked worktree paths using any path as the repo anchor.
	lockedPaths := map[string]bool{}
	if len(worktreePaths) > 0 {
		lockedPaths = detectLockedWorktreePaths(worktreePaths[0])
	}

	var result []DirtyWorktree

	for _, wt := range worktreePaths {
		// Skip non-existent paths silently.
		if _, err := os.Stat(wt); os.IsNotExist(err) {
			continue
		}

		// Resolve the current branch name for this worktree.
		branch := resolveWorktreeBranch(wt)
		if branch == "" {
			continue
		}

		// Strip refs/heads/ prefix for pattern matching.
		branch = strings.TrimPrefix(branch, "refs/heads/")

		// Match against the SAW worktree pattern (defined in detect.go, same package).
		m := worktreePattern.FindStringSubmatch(branch)
		if m == nil {
			continue
		}

		// Slug filtering: if a slug is configured and the branch has a saw/ prefix,
		// it must match the expected slug. Legacy branches without "saw/" pass through
		// for backward compatibility.
		if slugPrefix != "" && strings.Contains(branch, "saw/") {
			if !strings.HasPrefix(branch, slugPrefix) {
				continue
			}
		}

		// Extract wave number and agent ID from the regex match groups.
		waveNum := 0
		fmt.Sscanf(m[1], "%d", &waveNum)
		agentID := m[2]

		hasChanges := isWorktreeDirty(wt, lockedPaths)

		result = append(result, DirtyWorktree{
			Path:       wt,
			Branch:     branch,
			AgentID:    agentID,
			WaveNum:    waveNum,
			HasChanges: hasChanges,
		})
	}

	if result == nil {
		result = []DirtyWorktree{}
	}

	return result, nil
}

// isWorktreeDirty returns true if the worktree at path has uncommitted changes or
// is locked (locked = conservatively assume work in progress).
func isWorktreeDirty(path string, lockedPaths map[string]bool) bool {
	if lockedPaths[path] {
		return true
	}

	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		// Git command failed — treat as clean and log a warning.
		fmt.Fprintf(os.Stderr, "resume: ClassifyWorktrees: git status failed for %s: %v\n", path, err)
		return false
	}

	return strings.TrimSpace(string(out)) != ""
}

// resolveWorktreeBranch returns the current branch name (refs/heads/...) for the
// worktree at path. Returns empty string on any failure or for detached HEAD.
//
// Linked worktrees have a .git file (not directory), so we use `git symbolic-ref`
// for them rather than reading HEAD directly.
func resolveWorktreeBranch(path string) string {
	headPath := path + "/.git"
	info, err := os.Stat(headPath)
	if err != nil {
		// No .git at all — try symbolic-ref as fallback.
		return branchViaSymbolicRef(path)
	}

	if info.IsDir() {
		// Main worktree: read HEAD file directly.
		data, err := os.ReadFile(headPath + "/HEAD")
		if err != nil {
			return ""
		}
		line := strings.TrimSpace(string(data))
		if strings.HasPrefix(line, "ref: ") {
			return strings.TrimPrefix(line, "ref: ")
		}
		return "" // detached HEAD — no branch name
	}

	// Linked worktree: .git is a file pointing to the gitdir.
	return branchViaSymbolicRef(path)
}

// branchViaSymbolicRef uses `git symbolic-ref HEAD` to return the current branch.
func branchViaSymbolicRef(path string) string {
	cmd := exec.Command("git", "-C", path, "symbolic-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectLockedWorktreePaths runs `git worktree list --porcelain` using anyRepoPath
// as the working directory and returns a set of paths marked as locked.
func detectLockedWorktreePaths(anyRepoPath string) map[string]bool {
	locked := map[string]bool{}

	cmd := exec.Command("git", "-C", anyRepoPath, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return locked
	}

	// Parse blocks manually to detect the "locked" attribute.
	scanner := bufio.NewScanner(bytes.NewReader(out))
	var currentPath string
	var isLocked bool

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// End of block — flush.
			if currentPath != "" && isLocked {
				locked[currentPath] = true
			}
			currentPath = ""
			isLocked = false
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
			continue
		}
		if strings.HasPrefix(line, "locked") {
			isLocked = true
			continue
		}
	}
	// Flush final block (no trailing blank line in some git versions).
	if currentPath != "" && isLocked {
		locked[currentPath] = true
	}

	return locked
}
