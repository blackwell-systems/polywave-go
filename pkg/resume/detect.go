// Package resume provides detection of interrupted SAW sessions.
// It scans IMPL docs and git state to determine what was in progress
// and recommends how to resume.
package resume

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// SessionState describes the state of an interrupted SAW session.
type SessionState struct {
	IMPLSlug          string   `json:"impl_slug"`
	IMPLPath          string   `json:"impl_path"`
	CurrentWave       int      `json:"current_wave"`
	TotalWaves        int      `json:"total_waves"`
	CompletedAgents   []string `json:"completed_agents"`
	FailedAgents      []string `json:"failed_agents"`
	PendingAgents     []string `json:"pending_agents"`
	OrphanedWorktrees []string `json:"orphaned_worktrees"`
	SuggestedAction   string   `json:"suggested_action"`
	ProgressPct       float64  `json:"progress_pct"`
	CanAutoResume     bool     `json:"can_auto_resume"`
	ResumeCommand     string   `json:"resume_command"`
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
func Detect(repoPath string) ([]SessionState, error) {
	implDir := filepath.Join(repoPath, "docs", "IMPL")

	entries, err := os.ReadDir(implDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SessionState{}, nil
		}
		return nil, fmt.Errorf("resume.Detect: reading %s: %w", implDir, err)
	}

	var sessions []SessionState

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

		manifest, err := protocol.Load(implPath)
		if err != nil {
			// Skip unreadable / malformed manifests.
			continue
		}

		// Skip finished or unsuitable manifests.
		if manifest.State == protocol.StateComplete || manifest.State == protocol.StateNotSuitable {
			continue
		}

		state, err := buildSessionState(repoPath, implPath, manifest)
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

	if sessions == nil {
		sessions = []SessionState{}
	}

	return sessions, nil
}

// buildSessionState constructs a SessionState from a loaded manifest.
func buildSessionState(repoPath, implPath string, manifest *protocol.IMPLManifest) (SessionState, error) {
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
			case "complete":
				completed = append(completed, agent.ID)
			case "partial", "blocked":
				failed = append(failed, agent.ID)
			default:
				pending = append(pending, agent.ID)
			}
		}
	}

	// Current wave: highest wave number with any completion report.
	currentWave := determineCurrentWave(manifest)

	// Orphaned worktrees.
	orphaned, _ := detectOrphanedWorktrees(repoPath, manifest)

	// Progress percentage.
	var progressPct float64
	if totalAgents > 0 {
		progressPct = float64(len(completed)) / float64(totalAgents) * 100.0
	}

	// Determine suggested action and resume command.
	suggestedAction, resumeCommand := buildActionAndCommand(implPath, manifest, currentWave, completed, failed, pending, orphaned)

	// CanAutoResume: true only if no failed agents and current wave is fully complete.
	canAutoResume := len(failed) == 0 && isCurrentWaveFullyComplete(manifest, currentWave, completed)

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
	}

	return ss, nil
}

// determineCurrentWave returns the highest wave number that has at least one
// completion report. If no completion reports exist, returns 0 (none started).
func determineCurrentWave(manifest *protocol.IMPLManifest) int {
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
	return highest
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

// buildActionAndCommand returns the human-readable SuggestedAction and the
// exact sawtools ResumeCommand for the given session state.
func buildActionAndCommand(
	implPath string,
	manifest *protocol.IMPLManifest,
	currentWave int,
	completed, failed, pending []string,
	orphaned []string,
) (string, string) {
	switch {
	case len(failed) > 0:
		ids := strings.Join(failed, ", ")
		action := fmt.Sprintf("Rerun failed agent(s): %s", ids)
		cmd := fmt.Sprintf("sawtools retry %s --wave %d", implPath, currentWave)
		return action, cmd

	case len(orphaned) > 0:
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

// worktreePattern matches SAW worktree branch names like wave1-agent-A or wave2-agent-INT.
var worktreePattern = regexp.MustCompile(`wave(\d+)-agent-([A-Za-z0-9]+)`)

// detectOrphanedWorktrees runs `git worktree list --porcelain` and returns paths of
// worktrees whose wave is not fully merged (i.e., not all agents in that wave have a
// "complete" completion report).
func detectOrphanedWorktrees(repoPath string, manifest *protocol.IMPLManifest) ([]string, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath

	out, err := cmd.Output()
	if err != nil {
		// Git not available or no worktrees — treat as empty.
		return nil, nil
	}

	entries := parseWorktreePorcelain(out)

	// Build a set of agents with "complete" status.
	completedSet := make(map[string]bool)
	for id, report := range manifest.CompletionReports {
		if report.Status == "complete" {
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

	var orphaned []string
	for _, entry := range entries {
		// Try to match on the path first, then on the branch.
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

		// Check if the wave is fully merged.
		waveNum := 0
		fmt.Sscanf(m[1], "%d", &waveNum)
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
