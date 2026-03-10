package protocol

import (
	"bytes"
	"fmt"
	"os/exec"
)

// GateResult represents the outcome of executing a single quality gate.
// It captures all execution details including stdout/stderr and pass/fail status.
type GateResult struct {
	Type     string `json:"type"`
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Required bool   `json:"required"`
	Passed   bool   `json:"passed"`
}

// RunGates executes quality gates for a specific wave from the manifest.
// It runs each gate command and collects results.
// The repoDir parameter is the working directory for command execution.
//
// Returns an empty slice if manifest has no QualityGates or Gates is empty.
// Returns an error only for system-level failures (e.g., cannot create command).
// Gate failures are recorded in GateResult.Passed; the caller decides how to handle them.
func RunGates(manifest *IMPLManifest, waveNumber int, repoDir string) ([]GateResult, error) {
	// Return empty results if no quality gates defined
	if manifest.QualityGates == nil || len(manifest.QualityGates.Gates) == 0 {
		return []GateResult{}, nil
	}

	var results []GateResult
	for _, gate := range manifest.QualityGates.Gates {
		result := GateResult{
			Type:     gate.Type,
			Command:  gate.Command,
			Required: gate.Required,
		}

		// Create command and set working directory
		cmd := exec.Command("sh", "-c", gate.Command)
		cmd.Dir = repoDir

		// Capture stdout and stderr
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Execute the command
		err := cmd.Run()

		// Capture output
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()

		// Determine exit code and pass/fail status
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				// Command could not be started or other system error
				// Record as a failed gate with exit code -1
				result.ExitCode = -1
				if result.Stderr == "" {
					result.Stderr = fmt.Sprintf("command failed to execute: %v", err)
				}
			}
			result.Passed = false
		} else {
			result.ExitCode = 0
			result.Passed = true
		}

		results = append(results, result)
	}

	return results, nil
}
