package protocol

import (
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// validateCrossFieldConsistency checks relationships between fields that
// single-field validators cannot catch. It uses SV01_CROSS_FIELD error codes
// for all issues found.
//
// Checks performed:
//  1. FileOwnership agent IDs must appear in some Wave.Agents
//  2. FileOwnership wave numbers must match existing Wave.Number values
//  3. Agent.Files entries must appear in FileOwnership with matching agent+wave
//  4. NOT_SUITABLE verdict: waves/file_ownership/interface_contracts should be empty
//  5. Completion report agent IDs must match Wave agent IDs
func validateCrossFieldConsistency(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Build lookup structures from waves.
	waveNumbers := make(map[int]bool)
	// agentToWave maps agent ID -> wave number for all agents across all waves.
	agentToWave := make(map[string]int)
	for _, w := range m.Waves {
		waveNumbers[w.Number] = true
		for _, a := range w.Agents {
			agentToWave[a.ID] = w.Number
		}
	}

	// Check 1: FileOwnership agents exist in waves.
	// Check 2: FileOwnership wave numbers are valid.
	for i, fo := range m.FileOwnership {
		// Allow "Scaffold" agent at wave 0 (scaffold files).
		if fo.Agent == "Scaffold" && fo.Wave == 0 {
			continue
		}

		if _, ok := agentToWave[fo.Agent]; !ok {
			errs = append(errs, result.SAWError{
				Code:     SV01CrossFieldError,
				Severity: "error",
				Message:  fmt.Sprintf("file_ownership[%d]: agent %q does not appear in any wave's agents list", i, fo.Agent),
				Field:    fmt.Sprintf("file_ownership[%d].agent", i),
				Context:  map[string]string{"agent_id": fo.Agent, "wave": fmt.Sprintf("%d", fo.Wave)},
			})
		}

		if fo.Wave > 0 && !waveNumbers[fo.Wave] {
			errs = append(errs, result.SAWError{
				Code:     SV01CrossFieldError,
				Severity: "error",
				Message:  fmt.Sprintf("file_ownership[%d]: wave %d does not match any existing wave number", i, fo.Wave),
				Field:    fmt.Sprintf("file_ownership[%d].wave", i),
				Context:  map[string]string{"agent_id": fo.Agent, "wave": fmt.Sprintf("%d", fo.Wave)},
			})
		}
	}

	// Check 3: Agent files in ownership table (with matching agent+wave).
	// Build a set of (file, agent, wave) tuples from file_ownership.
	type ownerKey struct {
		file  string
		agent string
		wave  int
	}
	ownerSet := make(map[ownerKey]bool)
	for _, fo := range m.FileOwnership {
		ownerSet[ownerKey{file: fo.File, agent: fo.Agent, wave: fo.Wave}] = true
	}

	for _, w := range m.Waves {
		for _, a := range w.Agents {
			for _, f := range a.Files {
				key := ownerKey{file: f, agent: a.ID, wave: w.Number}
				if !ownerSet[key] {
					errs = append(errs, result.SAWError{
						Code:     SV01CrossFieldError,
						Severity: "error",
						Message:  fmt.Sprintf("agent %s (wave %d) lists file %q but no matching file_ownership entry exists for agent=%s wave=%d", a.ID, w.Number, f, a.ID, w.Number),
						Field:    fmt.Sprintf("waves[%d].agents[%s].files", w.Number-1, a.ID),
						Context:  map[string]string{"agent_id": a.ID, "wave": fmt.Sprintf("%d", w.Number)},
					})
				}
			}
		}
	}

	// Check 4: NOT_SUITABLE verdict consistency (warnings, not errors).
	if m.Verdict == "NOT_SUITABLE" {
		if len(m.Waves) > 0 {
			errs = append(errs, result.SAWError{
				Code:     SV01CrossFieldError,
				Severity: "error",
				Message:  fmt.Sprintf("verdict is NOT_SUITABLE but %d wave(s) are defined — expected empty waves", len(m.Waves)),
				Field:    "waves",
			})
		}
		if len(m.FileOwnership) > 0 {
			errs = append(errs, result.SAWError{
				Code:     SV01CrossFieldError,
				Severity: "error",
				Message:  fmt.Sprintf("verdict is NOT_SUITABLE but %d file_ownership entries exist — expected empty", len(m.FileOwnership)),
				Field:    "file_ownership",
			})
		}
		if len(m.InterfaceContracts) > 0 {
			errs = append(errs, result.SAWError{
				Code:     SV01CrossFieldError,
				Severity: "error",
				Message:  fmt.Sprintf("verdict is NOT_SUITABLE but %d interface_contracts exist — expected empty", len(m.InterfaceContracts)),
				Field:    "interface_contracts",
			})
		}
	}

	// Check 5: Completion report agent validity.
	for agentID := range m.CompletionReports {
		if _, ok := agentToWave[agentID]; !ok {
			errs = append(errs, result.SAWError{
				Code:     SV01CrossFieldError,
				Severity: "error",
				Message:  fmt.Sprintf("completion_reports[%s]: agent %q does not appear in any wave's agents list", agentID, agentID),
				Field:    fmt.Sprintf("completion_reports[%s]", agentID),
				Context:  map[string]string{"agent_id": agentID},
			})
		}
	}

	return errs
}
