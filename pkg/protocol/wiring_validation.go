package protocol

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// ValidateWiringDeclarations checks each WiringDeclaration in the manifest:
// for each entry, it opens entry.MustBeCalledFrom (resolved against repoPath),
// attempts Go AST parsing to find the symbol as a call expression, and falls
// back to a bufio line scan for non-Go files or parse failures.
// Returns a WiringValidationResult with any gaps (severity: "error").
func ValidateWiringDeclarations(manifest *IMPLManifest, repoPath string) (*WiringValidationResult, error) {
	result := &WiringValidationResult{
		Gaps: []WiringGap{},
	}

	for _, entry := range manifest.Wiring {
		absPath := filepath.Join(repoPath, entry.MustBeCalledFrom)

		found, err := symbolFoundInFile(absPath, entry.Symbol)
		if err != nil {
			// File not found or unreadable — report as a gap
			result.Gaps = append(result.Gaps, WiringGap{
				Declaration: entry,
				Reason:      fmt.Sprintf("could not read file %s: %v", entry.MustBeCalledFrom, err),
				Severity:    "error",
			})
			continue
		}

		if !found {
			result.Gaps = append(result.Gaps, WiringGap{
				Declaration: entry,
				Reason:      fmt.Sprintf("symbol %q not found as a call in %s", entry.Symbol, entry.MustBeCalledFrom),
				Severity:    "error",
			})
		}
	}

	result.Valid = len(result.Gaps) == 0
	if result.Valid {
		result.Summary = fmt.Sprintf("all %d wiring declarations satisfied", len(manifest.Wiring))
	} else {
		result.Summary = fmt.Sprintf("%d wiring gap(s) found in %d declarations", len(result.Gaps), len(manifest.Wiring))
	}

	return result, nil
}

// symbolFoundInFile returns true if symbol appears as a call expression in
// the file at absPath. For .go files it uses Go AST; for everything else (or
// if AST parsing fails) it falls back to a bufio line scan.
func symbolFoundInFile(absPath, symbol string) (bool, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return false, err
	}
	f.Close()

	// Try Go AST first for .go files
	if strings.HasSuffix(absPath, ".go") {
		found, parseErr := symbolFoundViaAST(absPath, symbol)
		if parseErr == nil {
			return found, nil
		}
		// AST parse failed — fall through to grep
	}

	return symbolFoundViaGrep(absPath, symbol)
}

// symbolFoundViaAST parses absPath as Go source and walks the AST looking for
// a call expression whose function name matches symbol. Handles both plain
// identifiers (Symbol(args)) and selector expressions (pkg.Symbol(args)).
func symbolFoundViaAST(absPath, symbol string) (bool, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		return false, err
	}

	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			if fn.Name == symbol {
				found = true
			}
		case *ast.SelectorExpr:
			if fn.Sel.Name == symbol {
				found = true
			}
		}
		return !found
	})

	return found, nil
}

// symbolFoundViaGrep opens absPath and scans line by line for symbol as a substring.
func symbolFoundViaGrep(absPath, symbol string) (bool, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), symbol) {
			return true, nil
		}
	}
	return false, scanner.Err()
}

// CheckWiringOwnership verifies that, for each WiringDeclaration in waveNum,
// the entry.MustBeCalledFrom file is owned by the named agent in that wave.
// Returns a slice of violation strings; empty means all declarations are valid.
func CheckWiringOwnership(manifest *IMPLManifest, waveNum int) []string {
	// Build per-agent file sets for this wave
	agentFiles := map[string]map[string]bool{}
	for _, fo := range manifest.FileOwnership {
		if fo.Wave != waveNum {
			continue
		}
		if agentFiles[fo.Agent] == nil {
			agentFiles[fo.Agent] = map[string]bool{}
		}
		agentFiles[fo.Agent][fo.File] = true
	}

	var violations []string
	for _, entry := range manifest.Wiring {
		if entry.Wave != waveNum {
			continue
		}
		owned := agentFiles[entry.Agent]
		if !owned[entry.MustBeCalledFrom] {
			violations = append(violations, fmt.Sprintf(
				"wiring entry for %q: must_be_called_from %q is not in agent %s file_ownership (wave %d)",
				entry.Symbol, entry.MustBeCalledFrom, entry.Agent, waveNum,
			))
		}
	}

	return violations
}

// FormatWiringBriefSection produces a markdown section listing the wiring
// obligations for agentID. Returns "" if the agent has no obligations.
func FormatWiringBriefSection(manifest *IMPLManifest, agentID string) string {
	var entries []WiringDeclaration
	for _, w := range manifest.Wiring {
		if w.Agent == agentID {
			entries = append(entries, w)
		}
	}
	if len(entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Wiring Obligations\n\n")
	sb.WriteString("The following symbols MUST be wired into their caller files. This is\n")
	sb.WriteString("declared in the IMPL doc wiring: block and will be checked post-merge\n")
	sb.WriteString("by validate-integration (severity: error if missing).\n\n")
	sb.WriteString("| Symbol | Defined In | Must Be Called From | Pattern |\n")
	sb.WriteString("|--------|------------|---------------------|--------|\n")
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			e.Symbol, e.DefinedIn, e.MustBeCalledFrom, e.IntegrationPattern))
	}

	return sb.String()
}
