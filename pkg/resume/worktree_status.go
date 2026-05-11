package resume

import (
	"bufio"
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// DirtyWorktree describes a Polywave agent worktree and whether it has uncommitted changes.
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
// Only worktrees whose branch name matches the Polywave pattern and belongs to the given
// manifest's FeatureSlug are included. Worktrees with non-matching branches are
// skipped. Paths that do not exist are silently skipped. Git failures are treated
// as clean (not an error). Locked worktrees are conservatively treated as dirty.
func ClassifyWorktrees(worktreePaths []string, manifest *protocol.IMPLManifest, logger *slog.Logger) []DirtyWorktree {
	log := logger
	if log == nil {
		log = slog.Default()
	}
	// Build the required slug prefix for branch filtering.
	slugPrefix := ""
	if manifest != nil && manifest.FeatureSlug != "" {
		slugPrefix = "polywave/" + manifest.FeatureSlug + "/"
	}

	// Collect locked worktree paths using any path as the repo anchor.
	lockedPaths := map[string]bool{}
	if len(worktreePaths) > 0 {
		lockedPaths = detectLockedWorktreePaths(worktreePaths[0])
	}

	var classified []DirtyWorktree

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

		// Match against the Polywave worktree pattern (defined in detect.go, same package).
		m := worktreePattern.FindStringSubmatch(branch)
		if m == nil {
			continue
		}

		// Slug filtering: if a slug is configured and the branch has a polywave/ prefix,
		// it must match the expected slug. Legacy branches without "polywave/" pass through
		// for backward compatibility.
		if slugPrefix != "" && strings.Contains(branch, "polywave/") {
			if !strings.HasPrefix(branch, slugPrefix) {
				continue
			}
		}

		// Extract wave number and agent ID from the regex match groups.
		waveNum, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		agentID := m[2]

		hasChanges := isWorktreeDirty(wt, lockedPaths, log)

		classified = append(classified, DirtyWorktree{
			Path:       wt,
			Branch:     branch,
			AgentID:    agentID,
			WaveNum:    waveNum,
			HasChanges: hasChanges,
		})
	}

	if classified == nil {
		classified = []DirtyWorktree{}
	}

	return classified
}

// isWorktreeDirty returns true if the worktree at path has uncommitted changes or
// is locked (locked = conservatively assume work in progress).
func isWorktreeDirty(path string, lockedPaths map[string]bool, logger *slog.Logger) bool {
	log := logger
	if log == nil {
		log = slog.Default()
	}

	// Resolve symlinks so the lookup matches git's canonical path output.
	resolvedPath := path
	if rp, err := filepath.EvalSymlinks(path); err == nil {
		resolvedPath = rp
	}
	if lockedPaths[resolvedPath] {
		return true
	}

	out, err := git.StatusPorcelain(path)
	if err != nil {
		// Git command failed — treat as clean and log a warning.
		log.Warn("resume: ClassifyWorktrees: git status failed", "path", path, "err", err)
		return false
	}

	return out != ""
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
		branch, err := git.SymbolicRef(path)
		if err != nil {
			return ""
		}
		return branch
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
	branch, err := git.SymbolicRef(path)
	if err != nil {
		return ""
	}
	return branch
}

// detectLockedWorktreePaths runs `git worktree list --porcelain` using anyRepoPath
// as the working directory and returns a set of paths marked as locked.
func detectLockedWorktreePaths(anyRepoPath string) map[string]bool {
	locked := map[string]bool{}

	out, err := git.WorktreeListRaw(anyRepoPath)
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
			raw := strings.TrimPrefix(line, "worktree ")
			// Normalize: git may output real paths (e.g. /private/var on macOS);
			// store the canonical form so lookups against EvalSymlinks-resolved paths match.
			if rp, err := filepath.EvalSymlinks(raw); err == nil {
				currentPath = rp
			} else {
				currentPath = raw
			}
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
