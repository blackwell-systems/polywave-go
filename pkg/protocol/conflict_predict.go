package protocol

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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

// PredictConflictsFromReports cross-references completion reports for all agents
// in the given wave to detect files that appear in more than one agent's report.
//
// This implements E11: conflict prediction before merge. Any non-IMPL file
// (files outside .saw-state/ or docs/IMPL/) that appears in multiple agents'
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
	warnings := make([]result.SAWError, 0, len(conflictPredictions))
	for _, cp := range conflictPredictions {
		warnings = append(warnings, result.NewWarning(
			"CONFLICT_PREDICT_FAILED",
			fmt.Sprintf("E11 conflict prediction: %s has differing edits (agents: %v)", cp.File, cp.Agents),
		))
	}
	// Also add a summary warning
	warnings = append([]result.SAWError{
		result.NewWarning(
			"CONFLICT_PREDICT_FAILED",
			fmt.Sprintf("E11 conflict prediction: %d file(s) appear in multiple agent reports (merge conflict risk):\n%s",
				len(conflictPredictions), joinLines(conflictMessages)),
		),
	}, warnings...)

	return result.NewPartial(data, warnings)
}

// isIMPLStateFile returns true for IMPL doc paths and .saw-state/ files, which
// are expected to be modified by multiple agents and do not cause merge conflicts.
func isIMPLStateFile(path string) bool {
	// Allow multiple agents to touch IMPL docs and state directories.
	return hasPathPrefix(path, "docs/IMPL/") ||
		hasPathPrefix(path, ".saw-state/") ||
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
		return fmt.Sprintf("saw/%s/wave%d-agent-%s", manifest.FeatureSlug, waveNum, id)
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
		branchName := fmt.Sprintf("saw/%s/wave%d-agent-%s", manifest.FeatureSlug, waveNum, agentID)
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
