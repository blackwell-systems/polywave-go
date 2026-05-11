package protocol

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// WaveStructureProblem describes an E21A-inducing IMPL pattern.
type WaveStructureProblem struct {
	Symbol        string   `json:"symbol"`          // changed function/type name
	ChangedInWave int      `json:"changed_in_wave"` // wave where signature change occurs
	CallerWave    int      `json:"caller_wave"`     // wave where callers live (problematic)
	CallerFiles   []string `json:"caller_files"`    // files with unprotected callers
	Message       string   `json:"message"`         // description of the problem
}

// SuggestWaveStructure validates wave structure safety for interface changes.
// For each interface contract or file in m, it identifies exported symbols
// whose signatures change relative to the current codebase. For each such
// symbol it finds all callers in m.FileOwnership and checks:
//  1. If callers are in an earlier wave than the definition: error.
//  2. If callers are in the same wave as the definition but a different
//     agent with no bridge pattern: warning.
//  3. If callers are in a later wave but depends_on does not include the
//     defining agent: error.
//
// Returns Result[[]WaveStructureProblem].
func SuggestWaveStructure(ctx context.Context, m *IMPLManifest, repoDir string) result.Result[[]WaveStructureProblem] {
	if m == nil {
		return result.NewFailure[[]WaveStructureProblem]([]result.PolywaveError{
			result.NewFatal("W001_MANIFEST_NIL", "manifest is nil"),
		})
	}

	// Step 1: Build file->wave and file->agent maps from FileOwnership.
	fileWave := make(map[string]int)    // file -> wave number
	fileAgent := make(map[string]string) // file -> agent ID
	for _, fo := range m.FileOwnership {
		fileWave[fo.File] = fo.Wave
		fileAgent[fo.File] = fo.Agent
	}

	// Build wave -> agents map for depends_on checking.
	// waveAgents[waveNum][agentID] = Agent
	waveAgents := make(map[int]map[string]Agent)
	for _, w := range m.Waves {
		waveAgents[w.Number] = make(map[string]Agent)
		for _, ag := range w.Agents {
			waveAgents[w.Number][ag.ID] = ag
		}
	}

	// Step 2: Build symbolDefinedInWave from InterfaceContracts.
	// The contract's Location is the defining file; look up its wave.
	symbolDefinedInWave := make(map[string]int)    // symbol -> wave
	symbolOwnerAgent := make(map[string]string)    // symbol -> agent ID
	for _, contract := range m.InterfaceContracts {
		if contract.Location == "" || contract.Name == "" {
			continue
		}
		w, ok := fileWave[contract.Location]
		if !ok {
			continue
		}
		symbolDefinedInWave[contract.Name] = w
		symbolOwnerAgent[contract.Name] = fileAgent[contract.Location]
	}

	// Step 3: Also extract symbols from task text in FileOwnership entries.
	// For each FileOwnership entry where the agent's task mentions change keywords,
	// extract backtick-quoted names or names in parentheses.
	changeKeywords := []string{
		"migrate", "update signature", "change return type", "rename", "change signature",
	}
	backtickRe := regexp.MustCompile("`([A-Za-z][A-Za-z0-9_]*)`")
	parenRe := regexp.MustCompile(`\(([A-Z][A-Za-z0-9_]*)\)`)

	// Build agent task text per agent ID.
	agentTask := make(map[string]string)
	for _, w := range m.Waves {
		for _, ag := range w.Agents {
			agentTask[ag.ID] = ag.Task
		}
	}

	for _, fo := range m.FileOwnership {
		task := strings.ToLower(agentTask[fo.Agent])
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

		// Extract symbols from the original (non-lowercased) task text.
		originalTask := agentTask[fo.Agent]
		for _, m2 := range backtickRe.FindAllStringSubmatch(originalTask, -1) {
			sym := m2[1]
			if _, exists := symbolDefinedInWave[sym]; !exists {
				symbolDefinedInWave[sym] = fo.Wave
				symbolOwnerAgent[sym] = fo.Agent
			}
		}
		for _, m2 := range parenRe.FindAllStringSubmatch(originalTask, -1) {
			sym := m2[1]
			if _, exists := symbolDefinedInWave[sym]; !exists {
				symbolDefinedInWave[sym] = fo.Wave
				symbolOwnerAgent[sym] = fo.Agent
			}
		}
	}

	// Step 4: For each symbol, grep for callers in FileOwnership files.
	var problems []WaveStructureProblem

	for sym, definedWave := range symbolDefinedInWave {
		defAgent := symbolOwnerAgent[sym]

		// Group caller files by wave.
		// callersByWave[callerWave] = []callerFile
		callersByWave := make(map[int][]string)

		for _, fo := range m.FileOwnership {
			// Skip the defining file itself.
			if fo.Agent == defAgent && fo.Wave == definedWave {
				// Could still be a different file owned by the same agent in the same wave.
				// Only skip if the symbol is definitely defined in this exact file.
				// Since we don't have exact file-to-symbol mapping, we skip all files
				// of the same agent in the same wave to avoid false positives.
				continue
			}

			absPath := filepath.Join(repoDir, fo.File)
			if !fileContainsSymbol(absPath, sym) {
				continue
			}

			callersByWave[fo.Wave] = append(callersByWave[fo.Wave], fo.File)
		}

		// Step 5: Check each caller wave against the defined wave.
		for callerWave, callerFiles := range callersByWave {
			switch {
			case callerWave < definedWave:
				// Definite error: caller is in an earlier wave than the definition.
				problems = append(problems, WaveStructureProblem{
					Symbol:        sym,
					ChangedInWave: definedWave,
					CallerWave:    callerWave,
					CallerFiles:   callerFiles,
					Message: fmt.Sprintf(
						"symbol %q is changed in wave %d but called from wave %d (earlier wave); "+
							"move callers to wave %d or later",
						sym, definedWave, callerWave, definedWave,
					),
				})

			case callerWave == definedWave:
				// Warning: same-wave caller without bridge pattern (no depends_on).
				// Find any caller's agent in the same wave.
				missingDeps := collectMissingDepsForSameWave(
					m, callerWave, callerFiles, defAgent, waveAgents,
				)
				if len(missingDeps) > 0 {
					problems = append(problems, WaveStructureProblem{
						Symbol:        sym,
						ChangedInWave: definedWave,
						CallerWave:    callerWave,
						CallerFiles:   callerFiles,
						Message: fmt.Sprintf(
							"symbol %q is changed in wave %d and called from the same wave by agents %v, "+
								"but those agents do not list %q in their depends_on",
							sym, definedWave, missingDeps, defAgent,
						),
					})
				}

			case callerWave > definedWave:
				// Check that callers in later waves have depends_on referencing the defining agent.
				missingDeps := collectMissingDepsForLaterWave(
					m, callerWave, callerFiles, defAgent, waveAgents,
				)
				if len(missingDeps) > 0 {
					problems = append(problems, WaveStructureProblem{
						Symbol:        sym,
						ChangedInWave: definedWave,
						CallerWave:    callerWave,
						CallerFiles:   callerFiles,
						Message: fmt.Sprintf(
							"symbol %q is changed in wave %d but callers in wave %d (agents %v) "+
								"do not declare depends_on referencing the defining agent %q",
							sym, definedWave, callerWave, missingDeps, defAgent,
						),
					})
				}
			}
		}
	}

	return result.NewSuccess(problems)
}

// fileContainsSymbol reports whether the file at absPath contains a reference
// to sym as a call expression or type reference. It uses simple line scanning
// with a regex to avoid parsing overhead for files that don't exist.
func fileContainsSymbol(absPath, sym string) bool {
	f, err := os.Open(absPath)
	if err != nil {
		return false
	}
	defer f.Close()

	// Match sym( for function calls or sym{ for composite literals.
	// Also match sym. for method selectors and sym as standalone identifier.
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(sym) + `\b`)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comment lines.
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// collectMissingDepsForSameWave returns agent IDs in callerWave that own callerFiles
// but do not list defAgent in their depends_on (or Dependencies).
func collectMissingDepsForSameWave(
	m *IMPLManifest,
	callerWave int,
	callerFiles []string,
	defAgent string,
	waveAgents map[int]map[string]Agent,
) []string {
	// Build set of caller files for quick lookup.
	callerFileSet := make(map[string]bool)
	for _, f := range callerFiles {
		callerFileSet[f] = true
	}

	// Find agents that own these caller files in callerWave.
	callerAgentIDs := make(map[string]bool)
	for _, fo := range m.FileOwnership {
		if fo.Wave != callerWave {
			continue
		}
		if callerFileSet[fo.File] && fo.Agent != defAgent {
			callerAgentIDs[fo.Agent] = true
		}
	}

	// Check each caller agent's depends_on.
	var missing []string
	agents := waveAgents[callerWave]
	for agentID := range callerAgentIDs {
		ag, ok := agents[agentID]
		if !ok {
			continue
		}
		if !sliceContains(ag.Dependencies, defAgent) {
			missing = append(missing, agentID)
		}
	}
	return missing
}

// collectMissingDepsForLaterWave returns agent IDs in callerWave that own callerFiles
// but do not list defAgent in their depends_on.
func collectMissingDepsForLaterWave(
	m *IMPLManifest,
	callerWave int,
	callerFiles []string,
	defAgent string,
	waveAgents map[int]map[string]Agent,
) []string {
	return collectMissingDepsForSameWave(m, callerWave, callerFiles, defAgent, waveAgents)
}

// sliceContains returns true if s is present in slice.
func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
