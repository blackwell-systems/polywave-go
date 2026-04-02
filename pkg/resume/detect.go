// Package resume provides detection of interrupted SAW sessions.
// It scans IMPL docs and git state to determine what was in progress
// and recommends how to resume.
package resume

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// SessionState describes the state of an interrupted SAW session.
type SessionState struct {
	IMPLSlug          string                  `json:"impl_slug"`
	IMPLPath          string                  `json:"impl_path"`
	CurrentWave       int                     `json:"current_wave"`
	TotalWaves        int                     `json:"total_waves"`
	CompletedAgents   []string                `json:"completed_agents"`
	FailedAgents      []string                `json:"failed_agents"`
	PendingAgents     []string                `json:"pending_agents"`
	OrphanedWorktrees []string                `json:"orphaned_worktrees"`
	SuggestedAction   string                  `json:"suggested_action"`
	ProgressPct       float64                 `json:"progress_pct"`
	CanAutoResume     bool                    `json:"can_auto_resume"`
	ResumeCommand     string                  `json:"resume_command"`
	DirtyWorktrees    []DirtyWorktree         `json:"dirty_worktrees,omitempty"`
	AgentSessions     map[string]AgentSession `json:"agent_sessions,omitempty"`
	RecoverySteps     []protocol.RecoveryStep `json:"recovery_steps,omitempty"`
}

// AgentSession holds session tracking information for a launched agent.
type AgentSession struct {
	AgentID      string `json:"agent_id"`
	SessionID    string `json:"session_id"`     // Claude Code session ID
	WaveNum      int    `json:"wave_num"`
	WorktreePath string `json:"worktree_path"`
	LastActive   string `json:"last_active"` // RFC3339 timestamp
}

// worktreeEntry holds data parsed from `git worktree list --porcelain`.
type worktreeEntry struct {
	path   string
	branch string
}

// Detect scans a repository for interrupted SAW sessions.
// It reads IMPL YAML files from docs/IMPL/ (excluding the complete/ subdirectory),
// skips manifests with state COMPLETE or NOT_SUITABLE, and returns a SessionState
// for each manifest that appears to be in progress.
// Detect is backward-compatible: it delegates to DetectWithConfig with a single repo.
func Detect(ctx context.Context, repoPath string) result.Result[[]SessionState] {
	return DetectWithConfig(ctx, []string{repoPath})
}

// DetectWithConfig scans for interrupted SAW sessions across multiple repositories.
// It reads IMPL YAML files from docs/IMPL/ in each repo (excluding complete/ subdirectory),
// skips manifests with state COMPLETE or NOT_SUITABLE, and returns a SessionState
// for each manifest that appears to be in progress.
// Worktrees are scanned across ALL repos so that cross-repo worktrees are found.
func DetectWithConfig(ctx context.Context, repoPaths []string) result.Result[[]SessionState] {
	if len(repoPaths) == 0 {
		return result.NewSuccess([]SessionState{})
	}

	var sessions []SessionState

	for _, repoPath := range repoPaths {
		implDir := protocol.IMPLDir(repoPath)

		entries, err := os.ReadDir(implDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return result.NewFailure[[]SessionState]([]result.SAWError{
				result.NewFatal(result.CodeSessionSaveFailed,
					fmt.Sprintf("resume.DetectWithConfig: reading %s: %v", implDir, err)).
					WithContext("implDir", implDir).WithCause(err),
			})
		}

		for _, entry := range entries {
			// Skip the complete/ subdirectory entirely.
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}

			implPath := filepath.Join(implDir, name)

			manifest, err := protocol.Load(ctx, implPath)
			if err != nil {
				// Skip unreadable / malformed manifests.
				continue
			}

			// Skip finished or unsuitable manifests.
			if manifest.State == protocol.StateComplete || manifest.State == protocol.StateNotSuitable {
				continue
			}

			state, err := buildSessionState(repoPaths, implPath, manifest)
			if err != nil {
				continue
			}

			// Skip manifests where no work has started (no completion reports,
			// no orphaned worktrees). These are freshly scouted IMPLs awaiting
			// their first wave — not interrupted sessions.
			if len(state.CompletedAgents) == 0 && len(state.FailedAgents) == 0 && len(state.OrphanedWorktrees) == 0 {
				continue
			}

			sessions = append(sessions, state)
		}
	}

	if sessions == nil {
		sessions = []SessionState{}
	}

	return result.NewSuccess(sessions)
}

// buildSessionState constructs a SessionState from a loaded manifest.
func buildSessionState(repoPaths []string, implPath string, manifest *protocol.IMPLManifest) (SessionState, error) {
	totalAgents := 0
	for _, wave := range manifest.Waves {
		totalAgents += len(wave.Agents)
	}

	var completed, failed, pending []string

	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			report, exists := manifest.CompletionReports[agent.ID]
			if !exists {
				pending = append(pending, agent.ID)
				continue
			}
			switch report.Status {
			case protocol.StatusComplete:
				completed = append(completed, agent.ID)
			case protocol.StatusPartial, protocol.StatusBlocked:
				failed = append(failed, agent.ID)
			default:
				pending = append(pending, agent.ID)
			}
		}
	}

	// Step 1: Collect orphaned worktrees from all repos.
	orphaned, _ := detectOrphanedWorktrees(repoPaths, manifest)

	// Current wave: highest wave number with any completion report,
	// or inferred from orphaned worktrees if no reports exist.
	currentWave := determineCurrentWave(manifest, orphaned)

	// Step 2: Classify worktrees as dirty/clean (Agent B provides ClassifyWorktrees).
	dirty := ClassifyWorktrees(orphaned, manifest, nil)

	// Step 3: Build suggested action and resume command using dirty classification.
	suggestedAction, resumeCommand := buildActionAndCommandInternal(implPath, manifest, orphaned, dirty, completed, failed, currentWave)

	// Progress percentage.
	var progressPct float64
	if totalAgents > 0 {
		progressPct = float64(len(completed)) / float64(totalAgents) * 100.0
	}

	// CanAutoResume: true only if no failed agents and current wave is fully complete.
	canAutoResume := len(failed) == 0 && isCurrentWaveFullyComplete(manifest, currentWave, completed)

	// Step 4: Load agent sessions if available (Agent C provides LoadAgentSessions).
	// Use the first repo path as the state dir base (where .saw-state/ lives).
	var agentSessions map[string]AgentSession
	if len(repoPaths) > 0 {
		stateDir := protocol.SAWStateDir(repoPaths[0])
		if loadRes := LoadAgentSessions(stateDir, manifest.FeatureSlug); loadRes.IsSuccess() {
			agentSessions = loadRes.GetData()
		}
	}

	ss := SessionState{
		IMPLSlug:          manifest.FeatureSlug,
		IMPLPath:          implPath,
		CurrentWave:       currentWave,
		TotalWaves:        len(manifest.Waves),
		CompletedAgents:   noNilSlice(completed),
		FailedAgents:      noNilSlice(failed),
		PendingAgents:     noNilSlice(pending),
		OrphanedWorktrees: noNilSlice(orphaned),
		SuggestedAction:   suggestedAction,
		ProgressPct:       progressPct,
		CanAutoResume:     canAutoResume,
		ResumeCommand:     resumeCommand,
		DirtyWorktrees:    dirty,
		AgentSessions:     agentSessions,
	}
	ss.RecoverySteps = BuildRecoverySteps(ss, manifest)

	return ss, nil
}

// determineCurrentWave returns the highest wave number that has at least one
// completion report. If no completion reports exist but orphaned worktrees are
// present, it falls back to the lowest wave number inferred from worktree branch
// names (agents were launched but none completed yet). Returns 0 if nothing found.
func determineCurrentWave(manifest *protocol.IMPLManifest, orphaned []string) int {
	highest := 0
	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			if _, exists := manifest.CompletionReports[agent.ID]; exists {
				if wave.Number > highest {
					highest = wave.Number
				}
			}
		}
	}
	if highest > 0 {
		return highest
	}

	// Fallback: infer from orphaned worktree paths using the lowest wave number found.
	// If wave 1 and wave 2 both have orphaned worktrees, wave 1 is incomplete — return 1.
	lowest := 0
	for _, worktreePath := range orphaned {
		m := worktreePattern.FindStringSubmatch(worktreePath)
		if m == nil {
			continue
		}
		waveNum, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if waveNum > 0 && (lowest == 0 || waveNum < lowest) {
			lowest = waveNum
		}
	}
	return lowest
}

// isCurrentWaveFullyComplete returns true when every agent in the given wave
// appears in the completed list.
func isCurrentWaveFullyComplete(manifest *protocol.IMPLManifest, waveNum int, completed []string) bool {
	if waveNum == 0 {
		return false
	}
	completedSet := make(map[string]bool, len(completed))
	for _, id := range completed {
		completedSet[id] = true
	}
	for _, wave := range manifest.Waves {
		if wave.Number != waveNum {
			continue
		}
		for _, agent := range wave.Agents {
			if !completedSet[agent.ID] {
				return false
			}
		}
		return true
	}
	return false
}

// buildActionAndCommandInternal is the full implementation used by buildSessionState.
// It accepts implPath explicitly so resume commands include the correct IMPL doc path.
func buildActionAndCommandInternal(
	implPath string,
	manifest *protocol.IMPLManifest,
	orphaned []string,
	dirtyWorktrees []DirtyWorktree,
	completed []string,
	failed []string,
	currentWave int,
) (string, string) {
	switch {
	case len(failed) > 0:
		ids := strings.Join(failed, ", ")
		action := fmt.Sprintf("Rerun failed agent(s): %s", ids)
		cmd := fmt.Sprintf("sawtools retry %s --wave %d", implPath, currentWave)
		return action, cmd

	case len(orphaned) > 0:
		// Distinguish dirty from clean worktrees.
		anyDirty := false
		for _, dw := range dirtyWorktrees {
			if dw.HasChanges {
				anyDirty = true
				break
			}
		}
		if anyDirty {
			action := fmt.Sprintf("Resume wave %d (agents have uncommitted work)", currentWave)
			cmd := fmt.Sprintf("sawtools run-wave %s --wave %d", implPath, currentWave)
			return action, cmd
		}
		action := fmt.Sprintf("Clean up orphaned worktrees, then resume wave %d", currentWave)
		cmd := fmt.Sprintf("sawtools cleanup %s --wave %d", implPath, currentWave)
		return action, cmd

	case currentWave == 0:
		action := "Start wave 1"
		cmd := fmt.Sprintf("sawtools run-wave %s --wave 1", implPath)
		return action, cmd

	case isCurrentWaveFullyComplete(manifest, currentWave, completed):
		nextWave := currentWave + 1
		action := fmt.Sprintf("Start wave %d", nextWave)
		cmd := fmt.Sprintf("sawtools run-wave %s --wave %d", implPath, nextWave)
		return action, cmd

	default:
		action := fmt.Sprintf("Resume wave %d", currentWave)
		cmd := fmt.Sprintf("sawtools run-wave %s --wave %d", implPath, currentWave)
		return action, cmd
	}
}

// worktreePattern matches SAW worktree branch names in both legacy and slug-scoped formats:
//   - Legacy: wave1-agent-A, wave2-agent-B3
//   - New: saw/my-slug/wave1-agent-A, saw/my-slug/wave2-agent-B3
//
// Note: This is a search pattern (no anchors) because it's used to find branch names
// within git worktree list output (which includes refs/heads/ prefixes and paths).
// It intentionally allows lowercase for backward compat with existing worktrees.
var worktreePattern = regexp.MustCompile(`(?:saw/[a-z0-9][-a-z0-9]*/)?wave(\d+)-agent-([A-Za-z0-9]+)`)

// slugPattern extracts the IMPL slug from a slug-scoped SAW branch name.
// Input should already have refs/heads/ stripped. Captures the slug portion.
var slugPattern = regexp.MustCompile(`^saw/([a-z0-9][-a-z0-9]*)/wave\d+-agent-`)

// loadCompletedSlugs scans the docs/IMPL/complete/ directory for IMPL-*.yaml files
// and returns a set of slugs extracted from filenames (strip "IMPL-" prefix and ".yaml" suffix).
// Worktrees belonging to completed IMPLs are stale artifacts, not interrupted sessions.
func loadCompletedSlugs(repoPath string) map[string]bool {
	completeDir := protocol.IMPLCompleteDir(repoPath)
	entries, err := os.ReadDir(completeDir)
	if err != nil {
		return nil
	}
	slugs := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "IMPL-") && strings.HasSuffix(name, ".yaml") {
			// Invariant: IMPL filenames follow the pattern "IMPL-{slug}.yaml" where {slug}
			// matches the FeatureSlug field inside the manifest. Scout agents are responsible
			// for ensuring this invariant holds when creating IMPL docs. If a filename and
			// FeatureSlug diverge, completed IMPLs may not be excluded from orphan detection.
			slug := strings.TrimSuffix(strings.TrimPrefix(name, "IMPL-"), ".yaml")
			if slug != "" {
				slugs[slug] = true
			}
		}
	}
	return slugs
}

// extractSlugFromBranch extracts the IMPL slug from a slug-scoped SAW branch name.
// Returns the slug and true if found, or empty string and false for legacy branches.
// The candidate should already have refs/heads/ stripped.
func extractSlugFromBranch(candidate string) (string, bool) {
	m := slugPattern.FindStringSubmatch(candidate)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// detectOrphanedWorktrees runs `git worktree list --porcelain` in each of the given
// repos and returns paths of worktrees whose wave is not fully merged (i.e., not all
// agents in that wave have a "complete" completion report).
// Worktrees whose slug matches a completed IMPL (in docs/IMPL/complete/) are excluded.
func detectOrphanedWorktrees(repoPaths []string, manifest *protocol.IMPLManifest) ([]string, error) {
	// Build a set of agents with "complete" status.
	completedSet := make(map[string]bool)
	for id, report := range manifest.CompletionReports {
		if report.Status == protocol.StatusComplete {
			completedSet[id] = true
		}
	}

	// Build a map of waveNum → all agent IDs in that wave.
	waveAgents := make(map[int][]string)
	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			waveAgents[wave.Number] = append(waveAgents[wave.Number], agent.ID)
		}
	}

	// Build a unified set of completed IMPL slugs across all repos.
	completedSlugs := make(map[string]bool)
	for _, repoPath := range repoPaths {
		for slug := range loadCompletedSlugs(repoPath) {
			completedSlugs[slug] = true
		}
	}

	var orphaned []string

	for _, repoPath := range repoPaths {
		out, err := git.WorktreeListRaw(repoPath)
		if err != nil {
			// Git not available or no worktrees — skip this repo.
			continue
		}

		entries := parseWorktreePorcelain(out)

		for _, entry := range entries {
			// Try to match on the branch first, then on the path.
			candidate := entry.branch
			if candidate == "" {
				candidate = entry.path
			}
			// Strip refs/heads/ prefix if present.
			candidate = strings.TrimPrefix(candidate, "refs/heads/")

			m := worktreePattern.FindStringSubmatch(candidate)
			if m == nil {
				continue
			}

			// Skip worktrees belonging to completed IMPLs — these are stale
			// artifacts from finished work, not interrupted sessions.
			if slug, ok := extractSlugFromBranch(candidate); ok && completedSlugs[slug] {
				continue
			}

			// Filter by IMPL slug: only consider worktrees that belong to this manifest.
			// Slug-scoped branches contain "saw/{slug}/" — if present, must match.
			// Legacy branches (no slug prefix) are attributed to any manifest (backward compat).
			if manifest.FeatureSlug != "" && strings.Contains(candidate, "saw/") {
				expectedPrefix := "saw/" + manifest.FeatureSlug + "/"
				if !strings.Contains(candidate, expectedPrefix) {
					continue // belongs to a different IMPL
				}
			}

			// Check if the wave is fully merged.
			waveNum, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			agents, known := waveAgents[waveNum]
			if !known {
				// Wave not in manifest — orphaned.
				orphaned = append(orphaned, entry.path)
				continue
			}

			allDone := true
			for _, agentID := range agents {
				if !completedSet[agentID] {
					allDone = false
					break
				}
			}
			if !allDone {
				orphaned = append(orphaned, entry.path)
			}
		}
	}

	return orphaned, nil
}

// parseWorktreePorcelain parses the output of `git worktree list --porcelain`.
// Each worktree block is separated by a blank line and starts with "worktree <path>".
func parseWorktreePorcelain(data []byte) []worktreeEntry {
	var entries []worktreeEntry
	var current worktreeEntry
	started := false

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if started {
				entries = append(entries, current)
				current = worktreeEntry{}
				started = false
			}
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			current.path = strings.TrimPrefix(line, "worktree ")
			started = true
			continue
		}
		if strings.HasPrefix(line, "branch ") {
			current.branch = strings.TrimPrefix(line, "branch ")
			continue
		}
	}
	// Flush the last block if there was no trailing newline.
	if started {
		entries = append(entries, current)
	}

	return entries
}

// noNilSlice converts a nil slice to an empty slice so JSON output is [] not null.
func noNilSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
