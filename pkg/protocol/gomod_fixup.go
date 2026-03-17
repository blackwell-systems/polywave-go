package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// deepRelativePathRe matches replace directive paths with 3+ levels of "../"
// which indicates a worktree-depth artifact (e.g. ../../../../sibling-module).
var deepRelativePathRe = regexp.MustCompile(`=>\s+((?:\.\./){3,}\S+)`)

// FixGoModReplacePaths scans go.mod in repoDir for replace directives with
// deep relative paths (3+ levels of ../) — a common artifact when wave agents
// run in worktrees and incorrectly adjust paths relative to their worktree
// depth instead of the repo root.
//
// For each deep path, it strips the excess ../ levels, keeping only the
// correct repo-root-relative path (one ../ for sibling directories).
//
// Returns true if any fixes were applied (and auto-committed).
func FixGoModReplacePaths(repoDir string) (bool, error) {
	gomodPath := filepath.Join(repoDir, "go.mod")
	content, err := os.ReadFile(gomodPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // not a Go project
		}
		return false, fmt.Errorf("read go.mod: %w", err)
	}

	original := string(content)
	fixed := fixDeepReplacePaths(original)

	if fixed == original {
		return false, nil
	}

	if err := os.WriteFile(gomodPath, []byte(fixed), 0644); err != nil {
		return false, fmt.Errorf("write go.mod: %w", err)
	}

	// Auto-commit the fix
	if err := git.AddAll(repoDir); err != nil {
		return true, fmt.Errorf("git add: %w", err)
	}
	if _, err := git.Commit(repoDir, "fix: auto-correct go.mod replace paths from worktree depth"); err != nil {
		return true, fmt.Errorf("git commit: %w", err)
	}

	return true, nil
}

// fixDeepReplacePaths rewrites lines like:
//
//	github.com/foo/bar => ../../../../bar
//
// to:
//
//	github.com/foo/bar => ../bar
//
// It extracts the final directory name from the deep path and prefixes it
// with a single "../".
func fixDeepReplacePaths(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if !strings.Contains(line, "=>") {
			continue
		}
		loc := deepRelativePathRe.FindStringSubmatchIndex(line)
		if loc == nil {
			continue
		}
		// loc[2]:loc[3] is the capture group (the deep path)
		deepPath := line[loc[2]:loc[3]]
		// Extract the final component (e.g. "../../../../bar" → "bar")
		base := filepath.Base(deepPath)
		fixed := "../" + base
		lines[i] = line[:loc[2]] + fixed + line[loc[3]:]
	}
	return strings.Join(lines, "\n")
}
