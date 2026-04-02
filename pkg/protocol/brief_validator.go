package protocol

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// BriefValidationData is the structured result of ValidateBriefs.
type BriefValidationData struct {
	Valid        bool                       `json:"valid"`
	AgentResults map[string]AgentBriefResult `json:"agent_results"`
	Summary      string                     `json:"summary"`
}

// AgentBriefResult holds validation results for a single agent.
type AgentBriefResult struct {
	AgentID string       `json:"agent_id"`
	Passed  bool         `json:"passed"`
	Issues  []BriefIssue `json:"issues"`
}

// BriefIssue describes a single validation failure.
type BriefIssue struct {
	Check       string   `json:"check"`                  // "symbol_missing" | "line_invalid"
	Severity    string   `json:"severity"`               // "error" | "warning"
	Description string   `json:"description"`
	File        string   `json:"file,omitempty"`
	Symbol      string   `json:"symbol,omitempty"`
	LineNumber  int      `json:"line_number,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// ValidateBriefs validates agent briefs in the IMPL doc at implPath.
// For each agent, extracts symbols mentioned in the brief and checks:
//  1. Symbol exists in owned files (grep-based, language-agnostic)
//  2. Line number references are valid (file has that many lines)
//  3. Optional: line content matches brief context if provided
//
// Returns BriefValidationData with per-agent results and suggestions.
func ValidateBriefs(ctx context.Context, implPath string) (BriefValidationData, error) {
	manifest, err := Load(ctx, implPath)
	if err != nil {
		return BriefValidationData{}, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Determine repo root for resolving file paths.
	repoRoot := manifest.Repository
	if repoRoot == "" && len(manifest.Repositories) > 0 {
		repoRoot = manifest.Repositories[0]
	}

	// Build a set of files owned by each agent for quick lookup.
	// Also track action:new files so we can skip existence checks.
	agentFiles := make(map[string][]string)     // agentID -> []filePath
	agentNewFiles := make(map[string]map[string]bool) // agentID -> set of new-action files
	for _, fo := range manifest.FileOwnership {
		filePath := briefResolveFilePath(fo, repoRoot)
		agentFiles[fo.Agent] = append(agentFiles[fo.Agent], filePath)
		if fo.Action == "new" {
			if agentNewFiles[fo.Agent] == nil {
				agentNewFiles[fo.Agent] = make(map[string]bool)
			}
			agentNewFiles[fo.Agent][filePath] = true
		}
	}

	agentResults := make(map[string]AgentBriefResult)
	totalIssues := 0

	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			if len(agentFiles[agent.ID]) == 0 {
				// Warn in summary but skip validation
				result := AgentBriefResult{
					AgentID: agent.ID,
					Passed:  true,
					Issues:  nil,
				}
				agentResults[agent.ID] = result
				continue
			}

			var issues []BriefIssue
			newFiles := agentNewFiles[agent.ID]
			ownedFiles := agentFiles[agent.ID]

			// Check symbol references.
			symbols := extractSymbols(agent.Task)
			for _, sym := range symbols {
				// Try each owned file.
				found := false
				var allSuggestions []string
				var checkedFile string

				for _, filePath := range ownedFiles {
					// Skip existence check for action:new files.
					if newFiles[filePath] {
						found = true
						break
					}
					checkedFile = filePath
					ok, suggestions := checkSymbolExists(filePath, sym)
					if ok {
						found = true
						break
					}
					allSuggestions = append(allSuggestions, suggestions...)
				}

				if !found && checkedFile != "" {
					// Deduplicate and limit suggestions to 3.
					seen := make(map[string]bool)
					var dedupSuggestions []string
					for _, s := range allSuggestions {
						if !seen[s] {
							seen[s] = true
							dedupSuggestions = append(dedupSuggestions, s)
						}
					}
					if len(dedupSuggestions) > 3 {
						dedupSuggestions = dedupSuggestions[:3]
					}
					issues = append(issues, BriefIssue{
						Check:       "symbol_missing",
						Severity:    "error",
						Description: fmt.Sprintf("symbol %q not found in owned files", sym),
						File:        checkedFile,
						Symbol:      sym,
						Suggestions: dedupSuggestions,
					})
				}
			}

			// Check line number references.
			lineRefs := extractLineReferences(agent.Task)
			for filePath, lineNums := range lineRefs {
				// Resolve relative file path against repo root if needed.
				resolvedPath := filePath
				if repoRoot != "" && !strings.HasPrefix(filePath, "/") {
					resolvedPath = repoRoot + "/" + filePath
				}

				for _, lineNum := range lineNums {
					if !checkLineValid(resolvedPath, lineNum) {
						issues = append(issues, BriefIssue{
							Check:       "line_invalid",
							Severity:    "error",
							Description: fmt.Sprintf("line %d referenced but file %q may not have that many lines", lineNum, filePath),
							File:        resolvedPath,
							LineNumber:  lineNum,
						})
					}
				}
			}

			passed := true
			for _, issue := range issues {
				if issue.Severity == "error" {
					passed = false
					break
				}
			}

			agentResults[agent.ID] = AgentBriefResult{
				AgentID: agent.ID,
				Passed:  passed,
				Issues:  issues,
			}
			totalIssues += len(issues)
		}
	}

	// Determine overall validity: no error-severity issues.
	allValid := true
	for _, ar := range agentResults {
		if !ar.Passed {
			allValid = false
			break
		}
	}

	summary := fmt.Sprintf("%d agents validated, %d issues found", len(agentResults), totalIssues)

	return BriefValidationData{
		Valid:        allValid,
		AgentResults: agentResults,
		Summary:      summary,
	}, nil
}

// briefResolveFilePath builds the absolute file path from a FileOwnership entry,
// using the repo root when available.
func briefResolveFilePath(fo FileOwnership, defaultRepoRoot string) string {
	repoRoot := defaultRepoRoot
	if fo.Repo != "" {
		repoRoot = fo.Repo
	}
	if repoRoot != "" && !strings.HasPrefix(fo.File, "/") {
		return repoRoot + "/" + fo.File
	}
	return fo.File
}

// extractSymbols parses task text and returns all symbol references.
// Patterns matched:
//   - Backtick-wrapped identifiers: `FuncName`, `TypeName`, `method()`
//   - Code block declarations: type X struct, func Y(), const Z
//
// Returns unique symbol names (deduplicated).
func extractSymbols(taskText string) []string {
	seen := make(map[string]bool)
	var symbols []string

	addSymbol := func(s string) {
		s = strings.TrimSuffix(s, "()")
		s = strings.TrimSuffix(s, "(")
		if s != "" && !seen[s] {
			seen[s] = true
			symbols = append(symbols, s)
		}
	}

	// Backtick-wrapped type names (uppercase first letter).
	reBacktickType := regexp.MustCompile("`([A-Z][a-zA-Z0-9_]*)`")
	for _, m := range reBacktickType.FindAllStringSubmatch(taskText, -1) {
		addSymbol(m[1])
	}

	// Backtick-wrapped function/variable names (lowercase first letter).
	reBacktickFunc := regexp.MustCompile("`([a-z][a-zA-Z0-9_]*(?:\\(\\))?)`")
	for _, m := range reBacktickFunc.FindAllStringSubmatch(taskText, -1) {
		addSymbol(m[1])
	}

	// type X struct declarations.
	reTypeStruct := regexp.MustCompile(`\btype\s+([A-Z][a-zA-Z0-9_]*)\s+struct`)
	for _, m := range reTypeStruct.FindAllStringSubmatch(taskText, -1) {
		addSymbol(m[1])
	}

	// func Y() declarations.
	reFuncDecl := regexp.MustCompile(`\bfunc\s+([a-z][a-zA-Z0-9_]*)`)
	for _, m := range reFuncDecl.FindAllStringSubmatch(taskText, -1) {
		addSymbol(m[1])
	}

	sort.Strings(symbols)
	return symbols
}

// extractLineReferences parses task text for line number mentions.
// Patterns matched:
//   - "around line N"
//   - "at line N"
//   - "line N"
//   - "lines N-M"
//
// Returns map[file_path][]line_number.
func extractLineReferences(taskText string) map[string][]int {
	result := make(map[string][]int)

	// Match file paths (look for *.go, *.ts, *.py, etc. or paths with /).
	reFilePath := regexp.MustCompile(`([a-zA-Z0-9_./-]+\.[a-zA-Z]{1,5})`)
	// Match line references.
	reLineRef := regexp.MustCompile(`(?i)(?:around line|at line|line)\s+(\d+)`)
	reLineRange := regexp.MustCompile(`(?i)lines\s+(\d+)-(\d+)`)

	// Split into paragraphs to associate file paths with line references.
	paragraphs := strings.Split(taskText, "\n\n")
	for _, para := range paragraphs {
		// Find file paths in this paragraph.
		fileMatches := reFilePath.FindAllString(para, -1)
		var paraFiles []string
		for _, f := range fileMatches {
			// Only treat as file path if it looks like a path (has extension or slash).
			if strings.Contains(f, "/") || strings.Contains(f, ".") {
				paraFiles = append(paraFiles, f)
			}
		}

		// Find line ranges first (lines N-M).
		for _, m := range reLineRange.FindAllStringSubmatch(para, -1) {
			start, _ := strconv.Atoi(m[1])
			end, _ := strconv.Atoi(m[2])
			for lineNum := start; lineNum <= end; lineNum++ {
				fileKey := pickFileKey(paraFiles, lineNum)
				result[fileKey] = briefAppendUniqueInt(result[fileKey], lineNum)
			}
		}

		// Find single line references.
		// Remove range matches first to avoid double-counting.
		cleanPara := reLineRange.ReplaceAllString(para, "")
		for _, m := range reLineRef.FindAllStringSubmatch(cleanPara, -1) {
			lineNum, _ := strconv.Atoi(m[1])
			if lineNum > 0 {
				fileKey := pickFileKey(paraFiles, lineNum)
				result[fileKey] = briefAppendUniqueInt(result[fileKey], lineNum)
			}
		}
	}

	return result
}

// pickFileKey returns the best file key for a line number from available files in a paragraph.
// Prefers .go files, then the first available file, then falls back to empty string.
func pickFileKey(files []string, _ int) string {
	for _, f := range files {
		if strings.HasSuffix(f, ".go") {
			return f
		}
	}
	if len(files) > 0 {
		return files[0]
	}
	return ""
}

// briefAppendUniqueInt appends lineNum to slice only if not already present.
func briefAppendUniqueInt(slice []int, lineNum int) []int {
	for _, v := range slice {
		if v == lineNum {
			return slice
		}
	}
	return append(slice, lineNum)
}

// checkSymbolExists greps file for symbol. Returns (found, suggestions).
// If not found, runs fuzzy match against file content to suggest alternatives.
// suggestions is empty if symbol found, non-empty with alternatives if not found.
func checkSymbolExists(filePath, symbol string) (bool, []string) {
	cmd := exec.Command("grep", "-n", symbol, filePath)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}

	// Symbol not found — attempt fuzzy match to provide suggestions.
	suggestions := fuzzyMatchSymbols(filePath, symbol)
	return false, suggestions
}

// fuzzyMatchSymbols reads filePath and returns up to 3 identifiers that are
// similar to the target symbol using a simple Levenshtein-based ranking.
func fuzzyMatchSymbols(filePath, target string) []string {
	content, err := readFileContent(filePath)
	if err != nil {
		return nil
	}

	reIdent := regexp.MustCompile(`\b([A-Za-z][a-zA-Z0-9_]{2,})\b`)
	matches := reIdent.FindAllString(content, -1)

	// Deduplicate.
	seen := make(map[string]bool)
	var candidates []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			candidates = append(candidates, m)
		}
	}

	// Score by Levenshtein distance.
	type candidateScore struct {
		name string
		dist int
	}
	var scoredCandidates []candidateScore
	for _, c := range candidates {
		d := levenshtein(strings.ToLower(target), strings.ToLower(c))
		scoredCandidates = append(scoredCandidates, candidateScore{c, d})
	}

	sort.Slice(scoredCandidates, func(i, j int) bool {
		return scoredCandidates[i].dist < scoredCandidates[j].dist
	})

	var suggestions []string
	for i, s := range scoredCandidates {
		if i >= 3 {
			break
		}
		if s.dist <= len(target)/2+2 { // Only suggest if reasonably close.
			suggestions = append(suggestions, s.name)
		}
	}
	return suggestions
}

// readFileContent reads and returns the entire content of a file.
func readFileContent(filePath string) (string, error) {
	cmd := exec.Command("cat", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

// checkLineValid returns true if filePath has at least lineNum lines.
// Uses exec.Command("wc", "-l", filePath) for language-agnostic line counting.
func checkLineValid(filePath string, lineNum int) bool {
	cmd := exec.Command("wc", "-l", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false
	}
	// wc -l output is like "     42 filename" or "42 filename".
	fields := strings.Fields(out.String())
	if len(fields) == 0 {
		return false
	}
	count, err := strconv.Atoi(fields[0])
	if err != nil {
		return false
	}
	return count >= lineNum
}

// levenshtein computes the edit distance between two strings.
func levenshtein(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	la, lb := len(ra), len(rb)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			curr[j] = min3(del, ins, sub)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
