package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ContextMDEntry is one completed feature record for docs/CONTEXT.md (E18).
type ContextMDEntry struct {
	Slug    string
	ImplDoc string
	Waves   int
	Agents  int
	Date    string // YYYY-MM-DD; set by caller or auto-filled if empty
}

// UpdateContextMD creates or updates docs/CONTEXT.md in repoPath (E18).
// If the file does not exist, creates it with the canonical schema.
// Appends entry to the features_completed list.
// Commits: git commit -m "chore: update docs/CONTEXT.md for {entry.Slug}"
func UpdateContextMD(repoPath string, entry ContextMDEntry) error {
	// 1. Auto-fill date if empty.
	if entry.Date == "" {
		entry.Date = time.Now().Format("2006-01-02")
	}

	// 2. Build context file path.
	contextPath := filepath.Join(repoPath, "docs", "CONTEXT.md")

	// 3. If file does not exist, create it with the canonical schema.
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		// Ensure docs/ directory exists.
		if err := os.MkdirAll(filepath.Dir(contextPath), 0755); err != nil {
			return fmt.Errorf("UpdateContextMD: create docs dir: %w", err)
		}
		canonical := fmt.Sprintf(`# docs/CONTEXT.md — Project memory for Scout-and-Wave (E17/E18)
created: %s
protocol_version: "0.14.0"

architecture: ""
decisions: []
conventions: []
established_interfaces: []

features_completed: []
`, entry.Date)
		if err := os.WriteFile(contextPath, []byte(canonical), 0644); err != nil {
			return fmt.Errorf("UpdateContextMD: create CONTEXT.md: %w", err)
		}
	}

	// 4. Append the entry to features_completed.
	entryLines := fmt.Sprintf("  - slug: %s\n    impl_doc: %s\n    waves: %d\n    agents: %d\n    date: %s\n",
		entry.Slug, entry.ImplDoc, entry.Waves, entry.Agents, entry.Date)

	f, err := os.OpenFile(contextPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("UpdateContextMD: open CONTEXT.md for append: %w", err)
	}
	if _, err := f.WriteString(entryLines); err != nil {
		f.Close()
		return fmt.Errorf("UpdateContextMD: write entry: %w", err)
	}
	f.Close()

	// 5. Git add and commit.
	addCmd := exec.Command("git", "-C", repoPath, "add", "docs/CONTEXT.md")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("UpdateContextMD: git add: %w\n%s", err, out)
	}

	commitMsg := fmt.Sprintf("chore: update docs/CONTEXT.md for %s", entry.Slug)
	commitCmd := exec.Command("git", "-C", repoPath, "commit", "-m", commitMsg)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("UpdateContextMD: git commit: %w\n%s", err, out)
	}

	return nil
}
