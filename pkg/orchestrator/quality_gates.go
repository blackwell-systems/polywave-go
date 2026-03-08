package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

// QualityGateResult is one gate's execution outcome.
type QualityGateResult struct {
	Type     string
	Command  string
	Required bool
	Passed   bool
	Output   string // combined stdout+stderr, truncated to 2000 chars
}

const (
	gateTimeout     = 5 * time.Minute
	maxOutputLength = 2000
)

// RunQualityGates executes the configured gates in repoPath (E21).
// If gates is nil or gates.Level == "quick", returns empty slice and nil error.
// For each gate: runs command; if Required and exit != 0, collects a blocking error.
// Returns all gate results plus the first blocking error (if any).
func RunQualityGates(repoPath string, gates *types.QualityGates) ([]QualityGateResult, error) {
	if gates == nil || strings.EqualFold(gates.Level, "quick") {
		return nil, nil
	}

	var results []QualityGateResult
	var firstBlockingErr error

	for _, gate := range gates.Gates {
		result := runGate(repoPath, gate)
		results = append(results, result)

		if gate.Required && !result.Passed && firstBlockingErr == nil {
			firstBlockingErr = fmt.Errorf(
				"required quality gate %q failed (command: %q): %s",
				gate.Type, gate.Command, truncate(result.Output, maxOutputLength),
			)
		}
	}

	return results, firstBlockingErr
}

// runGate executes a single gate and returns its result.
func runGate(repoPath string, gate types.QualityGate) QualityGateResult {
	result := QualityGateResult{
		Type:     gate.Type,
		Command:  gate.Command,
		Required: gate.Required,
	}

	args := strings.Fields(gate.Command)
	if len(args) == 0 {
		result.Output = "empty command"
		return result
	}

	ctx, cancel := context.WithTimeout(context.Background(), gateTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = repoPath

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	result.Output = truncate(buf.String(), maxOutputLength)
	result.Passed = err == nil

	return result
}

// truncate shortens s to at most maxLen bytes, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
