package analyzer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RenameInfo describes a type rename operation.
type RenameInfo struct {
	Old   string `json:"old"`   // Old type name (e.g., "AuthToken")
	New   string `json:"new"`   // New type name (e.g., "SessionToken")
	Scope string `json:"scope"` // Package scope (e.g., "pkg/auth")
}

// CascadeResult contains all detected cascade candidates from type renames.
type CascadeResult struct {
	CascadeCandidates []CascadeCandidate `json:"cascade_candidates"`
}

// CascadeCandidate represents a file location that references an old type name.
// It classifies the reference as syntax (affects compilation) or semantic (affects documentation).
type CascadeCandidate struct {
	File        string `json:"file"`         // Absolute path to file containing reference
	Line        int    `json:"line"`         // Line number of reference
	Match       string `json:"match"`        // The matched text
	CascadeType string `json:"cascade_type"` // "syntax" | "semantic"
	Severity    string `json:"severity"`     // "high" | "medium" | "low"
	Reason      string `json:"reason"`       // Human-readable explanation
}

// DetectCascades scans repoRoot for files referencing old type names.
// It returns a list of cascade candidates with severity classification.
// Algorithm:
// 1. Walk all .go files in repoRoot (skip vendor/, .git/, test files)
// 2. For each file, parse AST and search for old type name references
// 3. Classify each match as syntax or semantic based on AST context
// 4. Assign severity: high (import/type decl), medium (var decl), low (comment/string)
// 5. Sort results by file path, then line number for determinism
func DetectCascades(repoRoot string, renames []RenameInfo) (CascadeResult, error) {
	var candidates []CascadeCandidate

	// Build map of old type names to rename info for quick lookup
	renameMap := make(map[string]RenameInfo)
	for _, r := range renames {
		renameMap[r.Old] = r
	}

	// Walk all .go files in repo
	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor, .git, and hidden directories
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files, skip test files
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Scan this file for rename references
		fileCandidates, err := scanFileForRenames(path, renameMap)
		if err != nil {
			// Non-fatal: skip unparseable files
			return nil
		}

		candidates = append(candidates, fileCandidates...)
		return nil
	})

	if err != nil {
		return CascadeResult{}, fmt.Errorf("walk repo: %w", err)
	}

	// Sort for determinism: by file path, then line number
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].File != candidates[j].File {
			return candidates[i].File < candidates[j].File
		}
		return candidates[i].Line < candidates[j].Line
	})

	return CascadeResult{CascadeCandidates: candidates}, nil
}

// scanFileForRenames parses a single .go file and searches for references to renamed types.
// Returns a list of cascade candidates found in this file.
func scanFileForRenames(path string, renameMap map[string]RenameInfo) ([]CascadeCandidate, error) {
	var candidates []CascadeCandidate

	// Parse file with full AST (not just imports)
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	// Read file content for string literal searching
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	lines := strings.Split(string(content), "\n")

	// Track found matches to avoid duplicates
	seen := make(map[string]bool)

	// 1. Check import statements for old type names
	for _, imp := range astFile.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		line := fset.Position(imp.Pos()).Line

		for oldName, rename := range renameMap {
			// Check if import path contains the old package scope
			if strings.Contains(importPath, rename.Scope) {
				key := fmt.Sprintf("%s:%d:%s", path, line, oldName)
				if !seen[key] {
					seen[key] = true
					candidates = append(candidates, CascadeCandidate{
						File:        path,
						Line:        line,
						Match:       importPath,
						CascadeType: "syntax",
						Severity:    "high",
						Reason:      fmt.Sprintf("import statement references package containing renamed type %s", oldName),
					})
				}
			}
		}
	}

	// 2. Walk AST for type declarations and variable declarations
	ast.Inspect(astFile, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeSpec:
			// Type declaration: type OldName struct { ... }
			typeName := node.Name.Name
			if rename, exists := renameMap[typeName]; exists {
				line := fset.Position(node.Pos()).Line
				key := fmt.Sprintf("%s:%d:%s", path, line, typeName)
				if !seen[key] {
					seen[key] = true
					candidates = append(candidates, CascadeCandidate{
						File:        path,
						Line:        line,
						Match:       typeName,
						CascadeType: "syntax",
						Severity:    "high",
						Reason:      fmt.Sprintf("type declaration uses renamed type %s (now %s)", rename.Old, rename.New),
					})
				}
			}

		case *ast.Field:
			// Field in struct or function parameter: field SomeType
			if ident, ok := node.Type.(*ast.Ident); ok {
				if rename, exists := renameMap[ident.Name]; exists {
					line := fset.Position(node.Pos()).Line
					key := fmt.Sprintf("%s:%d:%s", path, line, ident.Name)
					if !seen[key] {
						seen[key] = true
						candidates = append(candidates, CascadeCandidate{
							File:        path,
							Line:        line,
							Match:       ident.Name,
							CascadeType: "syntax",
							Severity:    "medium",
							Reason:      fmt.Sprintf("field declaration uses renamed type %s (now %s)", rename.Old, rename.New),
						})
					}
				}
			}

		case *ast.ValueSpec:
			// Variable declaration: var x OldName
			if node.Type != nil {
				if ident, ok := node.Type.(*ast.Ident); ok {
					if rename, exists := renameMap[ident.Name]; exists {
						line := fset.Position(node.Pos()).Line
						key := fmt.Sprintf("%s:%d:%s", path, line, ident.Name)
						if !seen[key] {
							seen[key] = true
							candidates = append(candidates, CascadeCandidate{
								File:        path,
								Line:        line,
								Match:       ident.Name,
								CascadeType: "syntax",
								Severity:    "medium",
								Reason:      fmt.Sprintf("variable declaration uses renamed type %s (now %s)", rename.Old, rename.New),
							})
						}
					}
				}
			}
		}

		return true
	})

	// 3. Check comments for old type names (semantic cascade)
	if astFile.Comments != nil {
		for _, commentGroup := range astFile.Comments {
			for _, comment := range commentGroup.List {
				line := fset.Position(comment.Pos()).Line
				commentText := comment.Text

				for oldName, rename := range renameMap {
					if strings.Contains(commentText, oldName) {
						key := fmt.Sprintf("%s:%d:comment:%s", path, line, oldName)
						if !seen[key] {
							seen[key] = true
							candidates = append(candidates, CascadeCandidate{
								File:        path,
								Line:        line,
								Match:       commentText,
								CascadeType: "semantic",
								Severity:    "low",
								Reason:      fmt.Sprintf("comment mentions renamed type %s (now %s)", rename.Old, rename.New),
							})
						}
					}
				}
			}
		}
	}

	// 4. Check string literals for old type names (semantic cascade)
	// This requires scanning the file content line by line
	ast.Inspect(astFile, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			line := fset.Position(lit.Pos()).Line
			value := lit.Value

			for oldName, rename := range renameMap {
				if strings.Contains(value, oldName) {
					key := fmt.Sprintf("%s:%d:string:%s", path, line, oldName)
					if !seen[key] {
						seen[key] = true
						// Get the actual line content for display
						lineContent := ""
						if line > 0 && line <= len(lines) {
							lineContent = strings.TrimSpace(lines[line-1])
						}
						candidates = append(candidates, CascadeCandidate{
							File:        path,
							Line:        line,
							Match:       lineContent,
							CascadeType: "semantic",
							Severity:    "low",
							Reason:      fmt.Sprintf("string literal mentions renamed type %s (now %s)", rename.Old, rename.New),
						})
					}
				}
			}
		}
		return true
	})

	return candidates, nil
}
