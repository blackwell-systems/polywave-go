package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
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
func UpdateContextMD(_ context.Context, repoPath string, entry ContextMDEntry) result.Result[UpdateContextData] {
	// ctx not yet threaded to git operations (internal/git functions don't accept context)
	// 1. Auto-fill date if empty.
	if entry.Date == "" {
		entry.Date = time.Now().Format("2006-01-02")
	}

	// 2. Build context file path.
	contextPath := protocol.ContextMDPath(repoPath)

	// 3. If file does not exist, create it with the canonical schema.
	if _, err := os.Stat(contextPath); os.IsNotExist(err) {
		// Ensure docs/ directory exists.
		if err := os.MkdirAll(filepath.Dir(contextPath), 0755); err != nil {
			return result.NewFailure[UpdateContextData]([]result.PolywaveError{
				result.NewFatal(result.CodeContextError,
					fmt.Sprintf("UpdateContextMD: create docs dir: %s", err.Error())),
			})
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
			return result.NewFailure[UpdateContextData]([]result.PolywaveError{
				result.NewFatal(result.CodeContextError,
					fmt.Sprintf("UpdateContextMD: create CONTEXT.md: %s", err.Error())),
			})
		}
	}

	// 4. Append the entry to features_completed.
	entryLines := fmt.Sprintf("  - slug: %s\n    impl_doc: %s\n    waves: %d\n    agents: %d\n    date: %s\n",
		entry.Slug, entry.ImplDoc, entry.Waves, entry.Agents, entry.Date)

	f, err := os.OpenFile(contextPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return result.NewFailure[UpdateContextData]([]result.PolywaveError{
			result.NewFatal(result.CodeContextError,
				fmt.Sprintf("UpdateContextMD: open CONTEXT.md for append: %s", err.Error())),
		})
	}
	if _, err := f.WriteString(entryLines); err != nil {
		f.Close()
		return result.NewFailure[UpdateContextData]([]result.PolywaveError{
			result.NewFatal(result.CodeContextError,
				fmt.Sprintf("UpdateContextMD: write entry: %s", err.Error())),
		})
	}
	f.Close()

	// 5. Git add and commit.
	if err := git.Add(repoPath, "docs/CONTEXT.md"); err != nil {
		return result.NewFailure[UpdateContextData]([]result.PolywaveError{
			result.NewFatal(result.CodeContextError,
				fmt.Sprintf("UpdateContextMD: git add: %s", err.Error())),
		})
	}

	commitMsg := fmt.Sprintf("chore: update docs/CONTEXT.md for %s", entry.Slug)
	if _, err := git.CommitWithMessage(repoPath, commitMsg); err != nil {
		return result.NewFailure[UpdateContextData]([]result.PolywaveError{
			result.NewFatal(result.CodeContextError,
				fmt.Sprintf("UpdateContextMD: git commit: %s", err.Error())),
		})
	}

	return result.NewSuccess(UpdateContextData{
		Slug:        entry.Slug,
		ContextPath: contextPath,
	})
}
