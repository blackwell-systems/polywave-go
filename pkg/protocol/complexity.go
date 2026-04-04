package protocol

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

const codeAgentLOCBudget = "V048_AGENT_LOC_BUDGET"

// CheckAgentComplexity returns complexity errors and warnings for agent scope.
//
// Hard errors (block execution):
//   - V047_TRIVIAL_SCOPE: IMPL is SUITABLE but has only 1 agent owning 1 file.
//     SAW adds no parallelization value; the change should be made directly.
//
// Warnings (advisory):
//   - W001_AGENT_SCOPE_LARGE: agent owns >8 files or creates >5 new files.
func CheckAgentComplexity(ctx context.Context, m *IMPLManifest) []result.SAWError {
	_ = ctx
	var warnings []result.SAWError

	// V047: Reject trivial single-agent, single-file IMPLs declared SUITABLE.
	// The suitability gate is LLM-driven; this catch ensures small-scope work
	// doesn't incur full SAW orchestration overhead for zero parallelism benefit.
	// Exempt retry IMPLs (slug contains "-retry-") — those are machine-generated
	// and are intentionally targeted single-agent documents.
	isRetry := strings.Contains(m.FeatureSlug, "-retry-")
	if !isRetry && (m.Verdict == "SUITABLE" || m.Verdict == "SUITABLE_WITH_CAVEATS") {
		totalAgents := 0
		for _, wave := range m.Waves {
			totalAgents += len(wave.Agents)
		}
		if totalAgents == 1 && len(m.FileOwnership) == 1 {
			warnings = append(warnings, result.SAWError{
				Code:     result.CodeTrivialScope,
				Message:  "IMPL has 1 agent owning 1 file — SAW adds no parallelization value at this scope; make the change directly instead",
				Severity: "error",
				Field:    "file_ownership",
			})
		}
	}

	// Build per-agent file count maps from file_ownership
	totalFiles := make(map[string]int)
	newFiles := make(map[string]int)

	for _, fo := range m.FileOwnership {
		totalFiles[fo.Agent]++
		if fo.Action == "new" {
			newFiles[fo.Agent]++
		}
	}

	// Warn for any agent with total files > 8
	for agentID, count := range totalFiles {
		if count > 8 {
			warnings = append(warnings, result.SAWError{
				Code:     result.CodeAgentScopeLarge,
				Message:  fmt.Sprintf("agent %s owns %d files (threshold: 8) — consider splitting into two agents", agentID, count),
				Severity: "warning",
				Field:    "file_ownership",
			})
		}
	}

	// Warn for any agent with new files > 5
	for agentID, newCount := range newFiles {
		if newCount > 5 {
			warnings = append(warnings, result.SAWError{
				Code:     result.CodeAgentScopeLarge,
				Message:  fmt.Sprintf("agent %s creates %d new files (threshold: 5) — consider splitting into two agents", agentID, newCount),
				Severity: "warning",
				Field:    "file_ownership",
			})
		}
	}

	return warnings
}

// CheckAgentLOCBudget checks that no agent owns files totalling more than maxLOC
// lines. Only action=modify files are counted (action=new files do not yet exist).
// Returns blocking errors for agents over budget.
// If repoPath is empty, all checks are skipped (allows offline validation).
func CheckAgentLOCBudget(ctx context.Context, m *IMPLManifest, repoPath string, maxLOC int) []result.SAWError {
	_ = ctx
	if repoPath == "" {
		return nil
	}

	// Build per-agent LOC and file count maps (action=modify only)
	totalLOC := make(map[string]int)
	fileCount := make(map[string]int)
	type fileEntry struct {
		file  string
		lines int
	}
	perFileLOC := make(map[string][]fileEntry)

	for _, fo := range m.FileOwnership {
		if fo.Action != "modify" {
			continue
		}
		if fo.V048Exempt {
			continue
		}
		absPath := filepath.Join(repoPath, fo.File)
		f, err := os.Open(absPath)
		if err != nil {
			// File doesn't exist or can't be opened — skip silently
			continue
		}
		lines := 0
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lines++
		}
		f.Close()
		totalLOC[fo.Agent] += lines
		fileCount[fo.Agent]++
		perFileLOC[fo.Agent] = append(perFileLOC[fo.Agent], fileEntry{fo.File, lines})
	}

	var errs []result.SAWError
	for agentID, loc := range totalLOC {
		if loc > maxLOC {
			entries := perFileLOC[agentID]
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].lines > entries[j].lines
			})
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("agent %s owns %d lines across %d files (limit: %d) — split the agent or reduce scope", agentID, loc, fileCount[agentID], maxLOC))
			for _, e := range entries {
				sb.WriteString(fmt.Sprintf("\n  %s: %d lines", e.file, e.lines))
			}
			errs = append(errs, result.SAWError{
				Code:     codeAgentLOCBudget,
				Message:  sb.String(),
				Severity: "error",
				Field:    "file_ownership",
			})
		}
	}
	return errs
}
