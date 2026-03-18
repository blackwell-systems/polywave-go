package protocol

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// RunTierGate verifies that all IMPLs in a tier are complete and runs the tier-level
// quality gates defined in the PROGRAM manifest. Returns structured result with
// per-gate and per-IMPL status.
func RunTierGate(manifest *PROGRAMManifest, tierNumber int, repoPath string) (*TierGateResult, error) {
	// Find the tier in the manifest
	var tier *ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == tierNumber {
			tier = &manifest.Tiers[i]
			break
		}
	}

	if tier == nil {
		return nil, fmt.Errorf("tier %d not found in manifest", tierNumber)
	}

	result := &TierGateResult{
		TierNumber:   tierNumber,
		ImplStatuses: make([]ImplTierStatus, 0, len(tier.Impls)),
		GateResults:  make([]GateResult, 0),
		AllImplsDone: true,
		Passed:       true,
	}

	// Check all IMPLs in the tier
	for _, implSlug := range tier.Impls {
		// Find the matching IMPL in the manifest
		var implStatus string
		found := false
		for _, impl := range manifest.Impls {
			if impl.Slug == implSlug {
				implStatus = impl.Status
				found = true
				break
			}
		}

		if !found {
			// IMPL referenced in tier but not defined in impls list
			implStatus = "not_found"
			result.AllImplsDone = false
		}

		status := ImplTierStatus{
			Slug:   implSlug,
			Status: implStatus,
		}
		result.ImplStatuses = append(result.ImplStatuses, status)

		// Check if this IMPL is complete
		if implStatus != "complete" {
			result.AllImplsDone = false
		}
	}

	// If not all IMPLs are done, the tier cannot pass
	if !result.AllImplsDone {
		result.Passed = false
		return result, nil
	}

	// All IMPLs are done, now run the tier gates
	for _, gate := range manifest.TierGates {
		gateResult := runTierGateCommand(gate, repoPath)
		result.GateResults = append(result.GateResults, gateResult)

		// If a required gate fails, the tier fails
		if gate.Required && !gateResult.Passed {
			result.Passed = false
		}
	}

	return result, nil
}

// runTierGateCommand executes a single tier gate command with a 5-minute timeout.
func runTierGateCommand(gate QualityGate, repoPath string) GateResult {
	result := GateResult{
		Type:     gate.Type,
		Command:  gate.Command,
		Required: gate.Required,
		Passed:   false,
	}

	// Create context with 5-minute timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create command with context
	cmd := exec.CommandContext(ctx, "sh", "-c", gate.Command)
	cmd.Dir = repoPath

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err := cmd.Run()

	// Capture stdout and stderr separately
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	// Determine pass/fail status
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// Timeout
			if result.Stderr == "" {
				result.Stderr = "command timed out after 5 minutes"
			} else {
				result.Stderr += "\n[command timed out after 5 minutes]"
			}
		}
		result.Passed = false
	} else {
		result.Passed = true
	}

	return result
}
