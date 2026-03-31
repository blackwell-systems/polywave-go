package protocol

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// AmendImplOpts controls which amend operation to perform.
// Exactly one of AddWave, RedirectAgent, or ExtendScope must be true.
type AmendImplOpts struct {
	ManifestPath  string // absolute path to the IMPL YAML
	AddWave       bool   // append a new empty wave skeleton to the manifest
	RedirectAgent bool   // re-queue an agent (update task + clear completion report)
	AgentID       string // required when RedirectAgent=true
	WaveNum       int    // required when RedirectAgent=true; wave the agent belongs to
	NewTask       string // required when RedirectAgent=true; replacement task text
	ExtendScope   bool   // re-engage Scout with current IMPL as context (handled by CLI layer)
}

// AmendImplData is the outcome of a successful AmendImpl call.
type AmendImplData struct {
	ManifestPath  string   // absolute path (same as input)
	Operation     string   // "add-wave" | "redirect-agent" | "extend-scope"
	NewWaveNumber int      // only set when Operation=="add-wave"
	AgentID       string   // only set when Operation=="redirect-agent"
	Warnings      []string // non-fatal issues detected during amend
}

// amendBlocked is an internal error code used in SAWError when a hard
// protocol invariant prevents the amend. Callers can check for this code via
// result.Errors[0].Code when result.IsFatal() is true.
const amendBlocked = "AMEND_BLOCKED"

// AmendImpl performs the amend operation described by opts on the manifest at
// opts.ManifestPath, saves the updated manifest, and returns the result.
// Returns a fatal Result with Code "AMEND_BLOCKED" if a hard invariant would
// be violated.
func AmendImpl(opts AmendImplOpts) result.Result[AmendImplData] {
	// Step 1: Load manifest
	m, err := Load(context.TODO(), opts.ManifestPath)
	if err != nil {
		return result.NewFailure[AmendImplData]([]result.SAWError{
			{
				Code:     result.CodeManifestInvalid,
				Message:  fmt.Sprintf("failed to load manifest: %v", err),
				Severity: "fatal",
			},
		})
	}

	// Step 2: Check completion_date field
	if m.CompletionDate != "" {
		return result.NewFailure[AmendImplData]([]result.SAWError{
			{
				Code:     amendBlocked,
				Message:  fmt.Sprintf("IMPL is complete (completion_date=%s); cannot amend", m.CompletionDate),
				Severity: "fatal",
			},
		})
	}

	// Step 3: Check raw file for SAW:COMPLETE marker
	rawBytes, err := os.ReadFile(opts.ManifestPath)
	if err != nil {
		return result.NewFailure[AmendImplData]([]result.SAWError{
			{
				Code:     result.CodeManifestInvalid,
				Message:  fmt.Sprintf("failed to read manifest file: %v", err),
				Severity: "fatal",
			},
		})
	}
	if strings.Contains(string(rawBytes), "SAW:COMPLETE") {
		return result.NewFailure[AmendImplData]([]result.SAWError{
			{
				Code:     amendBlocked,
				Message:  "IMPL is complete (SAW:COMPLETE marker present); cannot amend",
				Severity: "fatal",
			},
		})
	}

	switch {
	case opts.AddWave:
		return amendAddWave(opts, m)
	case opts.RedirectAgent:
		return amendRedirectAgent(opts, m)
	default:
		// ExtendScope is handled by the CLI layer; nothing to do in the engine
		return result.NewSuccess(AmendImplData{
			ManifestPath: opts.ManifestPath,
			Operation:    "extend-scope",
		})
	}
}

// amendAddWave appends a new empty wave to the manifest.
func amendAddWave(opts AmendImplOpts, m *IMPLManifest) result.Result[AmendImplData] {
	// Find highest wave number
	maxWave := 0
	for _, w := range m.Waves {
		if w.Number > maxWave {
			maxWave = w.Number
		}
	}

	newWaveNum := maxWave + 1
	newWave := Wave{Number: newWaveNum, Agents: []Agent{}}

	// Validate-first using only ordering/invariant checks (not schema checks that reject
	// empty agent lists, since an empty wave skeleton is the expected initial state).
	// We check wave ordering specifically: the new wave must be maxWave+1 which is sequential.
	mCopy := *m
	wavesCopy := make([]Wave, len(m.Waves))
	copy(wavesCopy, m.Waves)
	wavesCopy = append(wavesCopy, newWave)
	mCopy.Waves = wavesCopy

	// Run only wave-ordering validation (I3) to catch structural issues.
	// Full Validate() rejects empty agent lists by design, which is expected here.
	if errs := validateI3WaveOrdering(&mCopy); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Message
		}
		return result.NewFailure[AmendImplData]([]result.SAWError{
			{
				Code:     amendBlocked,
				Message:  fmt.Sprintf("validation failed after adding wave: %s", strings.Join(msgs, "; ")),
				Severity: "fatal",
			},
		})
	}

	// Validation passed — mutate real manifest and save
	m.Waves = append(m.Waves, newWave)
	if saveRes := Save(context.TODO(), m, opts.ManifestPath); saveRes.IsFatal() {
		return result.NewFailure[AmendImplData](saveRes.Errors)
	}

	return result.NewSuccess(AmendImplData{
		ManifestPath:  opts.ManifestPath,
		Operation:     "add-wave",
		NewWaveNumber: newWaveNum,
	})
}

// amendRedirectAgent updates an agent's task and clears any non-complete completion report.
func amendRedirectAgent(opts AmendImplOpts, m *IMPLManifest) result.Result[AmendImplData] {
	var warnings []string

	// Step 1: Find the agent in the specified wave
	waveIdx := -1
	agentIdx := -1
	for wi, w := range m.Waves {
		if w.Number == opts.WaveNum {
			waveIdx = wi
			for ai, a := range w.Agents {
				if a.ID == opts.AgentID {
					agentIdx = ai
					break
				}
			}
			break
		}
	}

	if waveIdx == -1 || agentIdx == -1 {
		return result.NewFailure[AmendImplData]([]result.SAWError{
			{
				Code:     amendBlocked,
				Message:  fmt.Sprintf("agent %s not found in wave %d", opts.AgentID, opts.WaveNum),
				Severity: "fatal",
			},
		})
	}

	// Step 2: Reject if agent has a complete completion report
	if cr, ok := m.CompletionReports[opts.AgentID]; ok && cr.Status == StatusComplete {
		return result.NewFailure[AmendImplData]([]result.SAWError{
			{
				Code:     amendBlocked,
				Message:  fmt.Sprintf("agent %s has a complete completion report; cannot redirect a completed agent", opts.AgentID),
				Severity: "fatal",
			},
		})
	}

	// Step 3: Worktree commit check
	branchName := BranchName(m.FeatureSlug, opts.WaveNum, opts.AgentID)
	wave := m.Waves[waveIdx]

	if wave.BaseCommit == "" {
		warnings = append(warnings, "base_commit not set; skipping worktree commit check")
	} else {
		// Find repo root via git rev-parse
		manifestDir := filepath.Dir(opts.ManifestPath)
		repoRoot, err := git.Run(manifestDir, "rev-parse", "--show-toplevel")
		if err != nil {
			warnings = append(warnings, "could not find repo root; skipping worktree commit check")
		} else {
			repoRoot = strings.TrimSpace(repoRoot)
			// Check for commits on the agent's branch since base commit
			lines, _ := git.LogOneline(repoRoot, wave.BaseCommit+".."+branchName)
			if len(lines) > 0 {
				return result.NewFailure[AmendImplData]([]result.SAWError{
					{
						Code:     amendBlocked,
						Message:  fmt.Sprintf("agent %s has commits on branch %s; cannot redirect a committed agent", opts.AgentID, branchName),
						Severity: "fatal",
					},
				})
			}
		}
	}

	// Step 4: Update agent task
	m.Waves[waveIdx].Agents[agentIdx].Task = opts.NewTask

	// Step 5: Delete partial completion report if present
	if cr, ok := m.CompletionReports[opts.AgentID]; ok && cr.Status != StatusComplete {
		delete(m.CompletionReports, opts.AgentID)
	}

	// Step 6: Save
	if saveRes := Save(context.TODO(), m, opts.ManifestPath); saveRes.IsFatal() {
		return result.NewFailure[AmendImplData](saveRes.Errors)
	}

	return result.NewSuccess(AmendImplData{
		ManifestPath: opts.ManifestPath,
		Operation:    "redirect-agent",
		AgentID:      opts.AgentID,
		Warnings:     warnings,
	})
}
