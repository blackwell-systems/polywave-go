package protocol

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// ConflictData holds the result of conflict prediction.
type ConflictData struct {
	ConflictsDetected int
	Conflicts         []ConflictPrediction
}

// ConflictPrediction describes a single predicted conflict.
type ConflictPrediction struct {
	File   string
	Agents []string
}

// DiffPattern classifies the type of changes in a git diff
type DiffPattern string

const (
	DiffPatternAppendOnly DiffPattern = "append_only"
	DiffPatternLineEdits  DiffPattern = "line_edits"
	DiffPatternDeletions  DiffPattern = "deletions"
	DiffPatternMixed      DiffPattern = "mixed"
)

// DiffAnalysis holds the result of analyzing a single agent's diff for one file
type DiffAnalysis struct {
	AgentID string
	File    string
	Pattern DiffPattern
	Hunks   []HunkRange
	Hash    string
}

// MergeStrategy suggests how to resolve a detected conflict
type MergeStrategy string

const (
	MergeStrategyAutomatic  MergeStrategy = "automatic"
	MergeStrategyManual     MergeStrategy = "manual"
	MergeStrategySequential MergeStrategy = "sequential"
)

// ConflictPredictionEnhanced extends ConflictPrediction with strategy metadata
type ConflictPredictionEnhanced struct {
	File         string
	Agents       []string
	Strategy     MergeStrategy
	Reason       string
	MergeOrder   []string
	DiffAnalyses []*DiffAnalysis
}

// ConflictDataEnhanced extends ConflictData with strategy recommendations
type ConflictDataEnhanced struct {
	ConflictsDetected int
	Conflicts         []ConflictPredictionEnhanced
	AutoMergeable     int
}

// AnalyzeDiffPattern inspects the diff between baseRef and agentBranch for file
// and returns the pattern classification.
func AnalyzeDiffPattern(repoPath, baseRef, agentBranch, file string) (*DiffAnalysis, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("repoPath required")
	}

	// Get diff with zero context to extract hunks
	diff, err := git.DiffUnifiedZero(repoPath, baseRef, agentBranch, file)
	if err != nil {
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	hunks := parseDiffHunks(diff)

	// Get diff stats to determine pattern
	stats, err := git.GetDiffStats(repoPath, baseRef, agentBranch, file)
	if err != nil {
		// Fallback to conservative pattern on stats error
		return &DiffAnalysis{
			File:    file,
			Pattern: DiffPatternMixed,
			Hunks:   hunks,
		}, nil
	}

	// Classify pattern based on stats
	pattern := DiffPatternMixed
	if stats.LinesRemoved == 0 && stats.LinesAdded > 0 {
		// Pure additions → append-only
		pattern = DiffPatternAppendOnly
	} else if stats.LinesRemoved > 0 && stats.LinesAdded == 0 {
		pattern = DiffPatternDeletions
	} else if stats.LinesRemoved > 0 && stats.LinesAdded > 0 {
		pattern = DiffPatternLineEdits
	}

	// Get file hash from agent branch
	hash, err := computeFileHashInBranch(repoPath, agentBranch, file)
	if err != nil {
		hash = "" // Non-fatal, leave empty
	}

	return &DiffAnalysis{
		File:    file,
		Pattern: pattern,
		Hunks:   hunks,
		Hash:    hash,
	}, nil
}

// PredictConflictsFromReports cross-references completion reports for all agents
// in the given wave to detect files that appear in more than one agent's report.
//
// This implements E11: conflict prediction before merge. Any non-IMPL file
// (files outside .polywave-state/ or docs/IMPL/) that appears in multiple agents'
// files_changed or files_created lists is flagged as a conflict risk.
//
// Returns Partial if conflicts are detected (warnings), Success if none.
// Returns Fatal if an unexpected failure occurs.
func PredictConflictsFromReports(ctx context.Context, manifest *IMPLManifest, waveNum int) result.Result[ConflictData] {
	_ = ctx
	if manifest == nil {
		return result.NewSuccess(ConflictData{})
	}

	// Find the target wave.
	var targetWave *Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			targetWave = &manifest.Waves[i]
			break
		}
	}
	if targetWave == nil {
		return result.NewSuccess(ConflictData{})
	}

	// Build map: file -> list of agent IDs that reported touching it.
	fileAgents := make(map[string][]string)
	for _, agent := range targetWave.Agents {
		report, ok := manifest.CompletionReports[agent.ID]
		if !ok {
			continue
		}
		seen := make(map[string]bool)
		allFiles := append([]string{}, report.FilesChanged...)
		allFiles = append(allFiles, report.FilesCreated...)
		for _, f := range allFiles {
			if f == "" || isIMPLStateFile(f) {
				continue
			}
			if !seen[f] {
				seen[f] = true
				fileAgents[f] = append(fileAgents[f], agent.ID)
			}
		}
	}

	// Collect conflicts: any file touched by 2+ agents.
	// Two passes to avoid false positives on cascade patches:
	//   1. Identical edits (SHA hash) — git auto-resolves convergent changes.
	//   2. Non-overlapping hunks — git 3-way merge handles disjoint line ranges.
	var conflictPredictions []ConflictPrediction
	var conflictMessages []string
	for file, agents := range fileAgents {
		if len(agents) > 1 {
			// Pass 1: identical content — safe regardless of position.
			if manifest.FeatureSlug != "" && allAgentsHaveSameContent(manifest, file, agents, waveNum) {
				continue
			}
			// Pass 2: non-overlapping hunks — cascade patches are safe to merge.
			if manifest.FeatureSlug != "" && !agentsHaveOverlappingHunks(manifest, file, agents, waveNum) {
				continue
			}
			conflictPredictions = append(conflictPredictions, ConflictPrediction{
				File:   file,
				Agents: agents,
			})
			conflictMessages = append(conflictMessages, fmt.Sprintf("  %s has overlapping edits (agents: %v)", file, agents))
		}
	}

	data := ConflictData{
		ConflictsDetected: len(conflictPredictions),
		Conflicts:         conflictPredictions,
	}

	if len(conflictPredictions) == 0 {
		return result.NewSuccess(data)
	}

	// Return Partial with warnings for each conflict (merge conflict risk).
	warnings := make([]result.PolywaveError, 0, len(conflictPredictions))
	for _, cp := range conflictPredictions {
		warnings = append(warnings, result.NewWarning(
			"CONFLICT_PREDICT_FAILED",
			fmt.Sprintf("E11 conflict prediction: %s has differing edits (agents: %v)", cp.File, cp.Agents),
		))
	}
	// Also add a summary warning
	warnings = append([]result.PolywaveError{
		result.NewWarning(
			"CONFLICT_PREDICT_FAILED",
			fmt.Sprintf("E11 conflict prediction: %d file(s) appear in multiple agent reports (merge conflict risk):\n%s",
				len(conflictPredictions), joinLines(conflictMessages)),
		),
	}, warnings...)

	return result.NewPartial(data, warnings)
}

// PredictConflictsWithStrategy is the enhanced E11 function that analyzes patterns
func PredictConflictsWithStrategy(ctx context.Context, manifest *IMPLManifest, waveNum int) result.Result[ConflictDataEnhanced] {
	_ = ctx
	if manifest == nil {
		return result.NewSuccess(ConflictDataEnhanced{})
	}

	// Find the target wave
	var targetWave *Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			targetWave = &manifest.Waves[i]
			break
		}
	}
	if targetWave == nil {
		return result.NewSuccess(ConflictDataEnhanced{})
	}

	// Build map: file -> list of agent IDs that reported touching it
	fileAgents := make(map[string][]string)
	for _, agent := range targetWave.Agents {
		report, ok := manifest.CompletionReports[agent.ID]
		if !ok {
			continue
		}
		seen := make(map[string]bool)
		allFiles := append([]string{}, report.FilesChanged...)
		allFiles = append(allFiles, report.FilesCreated...)
		for _, f := range allFiles {
			if f == "" || isIMPLStateFile(f) {
				continue
			}
			if !seen[f] {
				seen[f] = true
				fileAgents[f] = append(fileAgents[f], agent.ID)
			}
		}
	}

	// Find merge base for this wave (common ancestor of all agent branches)
	var mergeBase string
	if manifest.Repository != "" && manifest.FeatureSlug != "" && len(fileAgents) > 0 {
		// Get any two agents to find merge base
		var firstAgent, secondAgent string
		for _, agents := range fileAgents {
			if len(agents) >= 2 {
				firstAgent = agents[0]
				secondAgent = agents[1]
				break
			}
		}
		if firstAgent != "" && secondAgent != "" {
			branchFmt := func(id string) string {
				return fmt.Sprintf("polywave/%s/wave%d-agent-%s", manifest.FeatureSlug, waveNum, id)
			}
			base, err := git.MergeBase(manifest.Repository, branchFmt(firstAgent), branchFmt(secondAgent))
			if err == nil {
				mergeBase = base
			}
		}
	}

	// Analyze conflicts with pattern detection
	var conflictPredictions []ConflictPredictionEnhanced
	var conflictMessages []string
	autoMergeableCount := 0

	for file, agents := range fileAgents {
		if len(agents) <= 1 {
			continue
		}

		// Pass 1: identical content check (existing E11 logic)
		if manifest.FeatureSlug != "" && allAgentsHaveSameContent(manifest, file, agents, waveNum) {
			continue
		}

		// Pass 2: analyze diff patterns for each agent
		var analyses []*DiffAnalysis
		if mergeBase != "" && manifest.FeatureSlug != "" {
			for _, agentID := range agents {
				branchName := fmt.Sprintf("polywave/%s/wave%d-agent-%s", manifest.FeatureSlug, waveNum, agentID)
				analysis, err := AnalyzeDiffPattern(manifest.Repository, mergeBase, branchName, file)
				if err == nil && analysis != nil {
					analysis.AgentID = agentID
					analyses = append(analyses, analysis)
				}
			}
		}

		// Determine merge strategy based on patterns
		strategy, reason := determineMergeStrategy(analyses, agents)

		// Determine merge order (for sequential merges)
		mergeOrder := agents
		if strategy == MergeStrategySequential {
			// Sort by pattern safety: append-only first, then line edits, then mixed
			mergeOrder = sortAgentsByPattern(analyses, agents)
		}

		conflictPredictions = append(conflictPredictions, ConflictPredictionEnhanced{
			File:         file,
			Agents:       agents,
			Strategy:     strategy,
			Reason:       reason,
			MergeOrder:   mergeOrder,
			DiffAnalyses: analyses,
		})

		if strategy == MergeStrategyAutomatic {
			autoMergeableCount++
			conflictMessages = append(conflictMessages,
				fmt.Sprintf("  %s: auto-mergeable (%s) (agents: %v)", file, reason, agents))
		} else {
			conflictMessages = append(conflictMessages,
				fmt.Sprintf("  %s: %s (%s) (agents: %v)", file, strategy, reason, agents))
		}
	}

	data := ConflictDataEnhanced{
		ConflictsDetected: len(conflictPredictions),
		Conflicts:         conflictPredictions,
		AutoMergeable:     autoMergeableCount,
	}

	if len(conflictPredictions) == 0 {
		return result.NewSuccess(data)
	}

	// Return Partial with warnings for each conflict
	warnings := make([]result.PolywaveError, 0, len(conflictPredictions)+1)
	warnings = append(warnings, result.NewWarning(
		"CONFLICT_PREDICT_ENHANCED",
		fmt.Sprintf("E11 enhanced conflict prediction: %d file(s) with conflicts detected (%d auto-mergeable):\n%s",
			len(conflictPredictions), autoMergeableCount, joinLines(conflictMessages)),
	))
	for _, cp := range conflictPredictions {
		if cp.Strategy != MergeStrategyAutomatic {
			warnings = append(warnings, result.NewWarning(
				"CONFLICT_PREDICT_ENHANCED",
				fmt.Sprintf("%s requires %s merge: %s (agents: %v)",
					cp.File, cp.Strategy, cp.Reason, cp.Agents),
			))
		}
	}

	return result.NewPartial(data, warnings)
}

// determineMergeStrategy analyzes diff patterns to suggest merge approach
func determineMergeStrategy(analyses []*DiffAnalysis, agents []string) (MergeStrategy, string) {
	if len(analyses) == 0 {
		// No pattern data - be conservative
		return MergeStrategyManual, "no diff analysis available"
	}

	// Count pattern types
	appendOnlyCount := 0
	lineEditsCount := 0
	deletionsCount := 0
	mixedCount := 0
	overlappingHunks := false

	for _, a := range analyses {
		switch a.Pattern {
		case DiffPatternAppendOnly:
			appendOnlyCount++
		case DiffPatternLineEdits:
			lineEditsCount++
		case DiffPatternDeletions:
			deletionsCount++
		case DiffPatternMixed:
			mixedCount++
		}
	}

	// Check for overlapping hunks between agents
	if len(analyses) >= 2 {
		for i := 0; i < len(analyses); i++ {
			for j := i + 1; j < len(analyses); j++ {
				if hunksOverlap(analyses[i].Hunks, analyses[j].Hunks) {
					overlappingHunks = true
					break
				}
			}
			if overlappingHunks {
				break
			}
		}
	}

	// Decision logic (conservative approach)
	if overlappingHunks {
		return MergeStrategyManual, "overlapping line ranges detected"
	}

	// All agents doing pure append-only changes
	if appendOnlyCount == len(analyses) && appendOnlyCount > 0 {
		return MergeStrategyAutomatic, "all agents append-only (new functions/tests)"
	}

	// Mix of append-only and no overlaps
	if appendOnlyCount > 0 && lineEditsCount == 0 && deletionsCount == 0 && mixedCount == 0 {
		return MergeStrategyAutomatic, "all append-only changes"
	}

	// Non-overlapping hunks with safe patterns
	if !overlappingHunks && mixedCount == 0 && deletionsCount == 0 {
		return MergeStrategySequential, "non-overlapping edits (cascade pattern)"
	}

	// Default to manual for safety
	reason := "mixed patterns detected"
	if mixedCount > 0 {
		reason = "complex mixed changes detected"
	} else if deletionsCount > 0 {
		reason = "deletions require review"
	} else if lineEditsCount > 0 {
		reason = "line edits require review"
	}
	return MergeStrategyManual, reason
}

// sortAgentsByPattern returns agents sorted by merge safety (append-only first)
func sortAgentsByPattern(analyses []*DiffAnalysis, agents []string) []string {
	// Build pattern map
	patternMap := make(map[string]DiffPattern)
	for _, a := range analyses {
		patternMap[a.AgentID] = a.Pattern
	}

	// Sort: append-only, then line edits, then deletions, then mixed
	sorted := make([]string, len(agents))
	copy(sorted, agents)

	// Simple priority sort
	type agentPriority struct {
		id       string
		priority int
	}
	priorities := make([]agentPriority, len(agents))
	for i, id := range agents {
		pattern := patternMap[id]
		priority := 4 // default (unknown)
		switch pattern {
		case DiffPatternAppendOnly:
			priority = 1
		case DiffPatternLineEdits:
			priority = 2
		case DiffPatternDeletions:
			priority = 3
		case DiffPatternMixed:
			priority = 4
		}
		priorities[i] = agentPriority{id: id, priority: priority}
	}

	// Sort by priority
	for i := 0; i < len(priorities); i++ {
		for j := i + 1; j < len(priorities); j++ {
			if priorities[j].priority < priorities[i].priority {
				priorities[i], priorities[j] = priorities[j], priorities[i]
			}
		}
	}

	result := make([]string, len(priorities))
	for i, p := range priorities {
		result[i] = p.id
	}
	return result
}

// isIMPLStateFile returns true for IMPL doc paths and .polywave-state/ files, which
// are expected to be modified by multiple agents and do not cause merge conflicts.
func isIMPLStateFile(path string) bool {
	// Allow multiple agents to touch IMPL docs and state directories.
	return hasPathPrefix(path, "docs/IMPL/") ||
		hasPathPrefix(path, ".polywave-state/") ||
		hasPathPrefix(path, "docs/IMPL")
}

// hasPathPrefix returns true if path starts with prefix (allowing for leading slash variation).
func hasPathPrefix(path, prefix string) bool {
	if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
		return true
	}
	// Handle leading slash
	if len(path) > 0 && path[0] == '/' {
		return hasPathPrefix(path[1:], prefix)
	}
	return false
}

// joinLines joins a slice of strings into a newline-separated string.
func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}

// HunkRange represents a contiguous range of lines in the base file that an
// agent's diff modifies. Used for hunk-level conflict prediction (E11).
type HunkRange struct {
	Start int // first modified line (1-indexed, inclusive)
	End   int // last modified line (1-indexed, inclusive)
}

// parseDiffHunks extracts old-file line ranges from a unified diff produced
// with --unified=0. Only modification hunks (count > 0) are returned;
// pure insertions (-a,0) do not modify existing lines and are skipped.
func parseDiffHunks(diff string) []HunkRange {
	var hunks []HunkRange
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "@@") {
			continue
		}
		// @@ -old[,count] +new[,count] @@ [context]
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		oldPart := parts[1] // e.g. "-10,5" or "-10"
		if !strings.HasPrefix(oldPart, "-") {
			continue
		}
		rangeStr := oldPart[1:] // strip leading "-"
		var start, count int
		if comma := strings.Index(rangeStr, ","); comma >= 0 {
			start, _ = strconv.Atoi(rangeStr[:comma])
			count, _ = strconv.Atoi(rangeStr[comma+1:])
		} else {
			start, _ = strconv.Atoi(rangeStr)
			count = 1
		}
		if count == 0 {
			// Pure insertion after line 'start' — track as a zero-width anchor.
			// Two agents inserting at the same anchor position conflict in 3-way merge.
			hunks = append(hunks, HunkRange{Start: start, End: start})
			continue
		}
		hunks = append(hunks, HunkRange{Start: start, End: start + count - 1})
	}
	return hunks
}

// hunksOverlap returns true if any hunk in a overlaps with any hunk in b.
// Two ranges overlap when they share at least one line.
func hunksOverlap(a, b []HunkRange) bool {
	for _, ha := range a {
		for _, hb := range b {
			if ha.Start <= hb.End && hb.Start <= ha.End {
				return true
			}
		}
	}
	return false
}

// agentsHaveOverlappingHunks returns true when at least one pair of agents'
// diffs for file have overlapping line ranges (meaning a 3-way merge conflict
// is likely). Returns true (conservative/safe) when git calls fail.
func agentsHaveOverlappingHunks(manifest *IMPLManifest, file string, agents []string, waveNum int) bool {
	if manifest.Repository == "" || len(agents) < 2 {
		return true // can't check — assume conflict (safe default)
	}

	// Find common ancestor of the first two agent branches (all branches share
	// the same base commit, so merge-base of any pair gives the base ref).
	branchFmt := func(id string) string {
		return fmt.Sprintf("polywave/%s/wave%d-agent-%s", manifest.FeatureSlug, waveNum, id)
	}
	mergeBase, err := git.MergeBase(manifest.Repository, branchFmt(agents[0]), branchFmt(agents[1]))
	if err != nil {
		return true // can't determine base — assume conflict
	}

	// Collect hunk ranges for each agent that has a non-trivial diff.
	agentHunks := make(map[string][]HunkRange, len(agents))
	for _, id := range agents {
		diff, err := git.DiffUnifiedZero(manifest.Repository, mergeBase, branchFmt(id), file)
		if err != nil || diff == "" {
			continue
		}
		if h := parseDiffHunks(diff); len(h) > 0 {
			agentHunks[id] = h
		}
	}

	// Check all pairs for overlapping hunks.
	ids := make([]string, 0, len(agentHunks))
	for id := range agentHunks {
		ids = append(ids, id)
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if hunksOverlap(agentHunks[ids[i]], agentHunks[ids[j]]) {
				return true
			}
		}
	}
	return false
}

// allAgentsHaveSameContent checks if all agents produced identical file content.
// Returns true if all hashes match (safe to merge), false if any differ or on error.
func allAgentsHaveSameContent(manifest *IMPLManifest, file string, agents []string, waveNum int) bool {
	if len(agents) <= 1 {
		return true // No conflict with single agent
	}

	var hashes []string
	for _, agentID := range agents {
		branchName := fmt.Sprintf("polywave/%s/wave%d-agent-%s", manifest.FeatureSlug, waveNum, agentID)
		hash, err := computeFileHashInBranch(manifest.Repository, branchName, file)
		if err != nil {
			// Hash computation failed - fall back to blocking (safe default)
			return false
		}
		hashes = append(hashes, hash)
	}

	// Check if all hashes are identical
	firstHash := hashes[0]
	for _, h := range hashes[1:] {
		if h != firstHash {
			return false // Differing content
		}
	}

	return true // All hashes match - identical edits
}

// computeFileHashInBranch reads file content from a git branch and returns SHA256 hash.
// Uses "git show branch:file" to read content without checking out the branch.
func computeFileHashInBranch(repoPath, branchName, relFile string) (string, error) {
	// Use git show to read file content from branch
	content, err := git.Run(repoPath, "show", fmt.Sprintf("%s:%s", branchName, relFile))
	if err != nil {
		return "", fmt.Errorf("failed to read %s from branch %s: %w", relFile, branchName, err)
	}

	// Compute SHA256 hash
	hash := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", hash), nil
}
