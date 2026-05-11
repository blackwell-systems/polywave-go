package protocol

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// TestCascadeError represents an orphaned test file found during cascade check.
type TestCascadeError struct {
	Symbol   string   `json:"symbol"`    // function/method name that changed
	TestFile string   `json:"test_file"` // repo-relative path to *_test.go
	Line     int      `json:"line"`      // line in test file
	Context  string   `json:"context"`   // surrounding line content
	Agents   []string `json:"agents"`    // agents that own the symbol definition
}

// changeKeywords are the task description keywords that indicate a symbol's
// signature is changing in a way that test files might need to be updated.
var changeKeywords = []string{
	"change signature",
	"migrate",
	"update signature",
	"change return type",
	"rename",
	"changed signature",
}

// CheckTestCascade detects orphaned test call sites for changed symbols.
// For each file in m.FileOwnership, it identifies function/method names
// that the owning agent is changing (based on agent task keywords:
// "change signature", "migrate", "update signature", "change return type",
// "rename"). For each such symbol it runs a whole-repo grep restricted to
// *_test.go files. Any test file calling the symbol that is NOT in
// m.FileOwnership is reported as a TestCascadeError.
//
// Returns Result[[]TestCascadeError].
// Returns SUCCESS with empty slice when no orphaned test callers found.
// Returns PARTIAL with warnings when symbol extraction was ambiguous.
// Returns FATAL when repoDir is unreadable or manifest is nil.
func CheckTestCascade(ctx context.Context, m *IMPLManifest, repoDir string) result.Result[[]TestCascadeError] {
	if m == nil {
		return result.NewFailure[[]TestCascadeError]([]result.PolywaveError{
			result.NewFatal(result.CodeCheckCallerInvalidInput, "manifest is nil"),
		})
	}

	// Verify repoDir is readable
	if _, err := os.ReadDir(repoDir); err != nil {
		return result.NewFailure[[]TestCascadeError]([]result.PolywaveError{
			result.NewFatal(result.CodeCheckCallerFileRead, fmt.Sprintf("cannot read repoDir %q: %v", repoDir, err)),
		})
	}

	// Step 1: Build set of owned files (all files in FileOwnership)
	ownedFiles := make(map[string]bool)
	for _, fo := range m.FileOwnership {
		ownedFiles[fo.File] = true
	}

	// Step 2: Build map of changed symbols -> owning agent IDs
	// For each agent task in Waves, find files owned by that agent and check task for change keywords
	changedSymbols := make(map[string][]string) // symbol -> []agentID

	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			task := strings.ToLower(agent.Task)
			hasChangeKeyword := false
			for _, kw := range changeKeywords {
				if strings.Contains(task, kw) {
					hasChangeKeyword = true
					break
				}
			}
			if !hasChangeKeyword {
				continue
			}

			// Extract backtick-quoted names from the task
			symbols := extractBacktickNames(agent.Task)
			for _, sym := range symbols {
				changedSymbols[sym] = appendUniqueAgent(changedSymbols[sym], agent.ID)
			}
		}
	}

	// If no changed symbols detected, return success immediately
	if len(changedSymbols) == 0 {
		empty := []TestCascadeError{}
		return result.NewSuccess(empty)
	}

	// Step 3: For each changed symbol, walk repoDir for *_test.go files
	var cascadeErrors []TestCascadeError

	for symbol, agents := range changedSymbols {
		err := filepath.WalkDir(repoDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if d.IsDir() {
				// Skip hidden directories and vendor
				name := d.Name()
				if name == "vendor" || (len(name) > 0 && name[0] == '.') {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, "_test.go") {
				return nil
			}

			// Compute repo-relative path
			relPath, relErr := filepath.Rel(repoDir, path)
			if relErr != nil {
				relPath = path
			}

			// Check if this test file is already owned
			if ownedFiles[relPath] {
				return nil
			}

			// Scan the test file for the symbol
			matches, scanErr := scanFileForSymbol(path, symbol)
			if scanErr != nil {
				return nil // skip unreadable files
			}

			for _, match := range matches {
				cascadeErrors = append(cascadeErrors, TestCascadeError{
					Symbol:   symbol,
					TestFile: relPath,
					Line:     match.line,
					Context:  match.context,
					Agents:   agents,
				})
			}
			return nil
		})
		if err != nil {
			// Walk error is non-fatal; continue with other symbols
			continue
		}
	}

	return result.NewSuccess(cascadeErrors)
}

// lineMatch holds a line number and its content from a file scan.
type lineMatch struct {
	line    int
	context string
}

// scanFileForSymbol opens path and returns all lines containing symbol as a substring.
func scanFileForSymbol(path, symbol string) ([]lineMatch, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var matches []lineMatch
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		text := scanner.Text()
		if strings.Contains(text, symbol) {
			matches = append(matches, lineMatch{
				line:    lineNum,
				context: strings.TrimSpace(text),
			})
		}
	}
	return matches, scanner.Err()
}

// extractBacktickNames extracts all backtick-quoted names from s.
// E.g. "change signature of `Foo` and `Bar`" -> ["Foo", "Bar"]
func extractBacktickNames(s string) []string {
	var names []string
	for {
		start := strings.Index(s, "`")
		if start == -1 {
			break
		}
		rest := s[start+1:]
		end := strings.Index(rest, "`")
		if end == -1 {
			break
		}
		name := rest[:end]
		if name != "" {
			names = append(names, name)
		}
		s = rest[end+1:]
	}
	return names
}

// appendUniqueAgent appends val to slice only if not already present.
func appendUniqueAgent(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}
