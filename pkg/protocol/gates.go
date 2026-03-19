package protocol

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/errparse"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
)

// GateResult represents the outcome of executing a single quality gate.
// It captures all execution details including stdout/stderr and pass/fail status.
type GateResult struct {
	Type         string                   `json:"type"`
	Command      string                   `json:"command"`
	ExitCode     int                      `json:"exit_code"`
	Stdout       string                   `json:"stdout"`
	Stderr       string                   `json:"stderr"`
	Required     bool                     `json:"required"`
	Passed       bool                     `json:"passed"`
	Skipped      bool                     `json:"skipped,omitempty"`
	SkipReason   string                   `json:"skip_reason,omitempty"`
	FromCache    bool                     `json:"from_cache,omitempty"`
	ParsedErrors []errparse.StructuredError `json:"parsed_errors,omitempty"`
}

// isDocsOnlyWave returns true when every file owned by the given wave number
// is a documentation or configuration file with no compilable source code.
// Returns false if the wave has no files (conservative — run gates anyway).
func isDocsOnlyWave(manifest *IMPLManifest, waveNumber int) bool {
	docsExts := map[string]bool{
		".md": true, ".yaml": true, ".yml": true, ".txt": true, ".rst": true,
	}
	total := 0
	for _, wave := range manifest.Waves {
		if wave.Number != waveNumber {
			continue
		}
		for _, agent := range wave.Agents {
			for _, f := range agent.Files {
				total++
				if !docsExts[filepath.Ext(f)] {
					return false
				}
			}
		}
	}
	return total > 0
}

// isSourceGateType returns true for gate types that require compiled or
// tested source code and are not meaningful for docs-only changes.
func isSourceGateType(t string) bool {
	switch t {
	case "build", "test", "tests", "lint":
		return true
	}
	return false
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

	// Derive the repo name from the directory path for per-repo gate filtering.
	repoName := filepath.Base(repoDir)

	// Skip source-code gates when the wave only touches documentation files.
	docsOnly := isDocsOnlyWave(manifest, waveNumber)

	var results []GateResult
	for _, gate := range manifest.QualityGates.Gates {
		// Skip gates scoped to a different repo.
		if gate.Repo != "" && gate.Repo != repoName {
			continue
		}

		// Auto-skip build/test/lint gates for docs-only waves.
		if docsOnly && isSourceGateType(gate.Type) {
			results = append(results, GateResult{
				Type:       gate.Type,
				Command:    gate.Command,
				Required:   gate.Required,
				Passed:     true,
				Skipped:    true,
				SkipReason: "docs-only wave: no compilable source files changed",
			})
			continue
		}

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

		// Parse structured errors from output
		if pr := errparse.ParseOutput(gate.Type, gate.Command, result.Stdout, result.Stderr); pr != nil {
			result.ParsedErrors = pr.Errors
		}

		results = append(results, result)
	}

	return results, nil
}

// RunGatesWithCache executes quality gates with optional result caching.
// If cache is nil, it behaves identically to RunGates.
// For each gate, if a valid (non-expired) cached result exists, it is returned
// directly without re-executing the gate command. Otherwise the gate is run
// normally and the result is stored in the cache.
func RunGatesWithCache(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache) ([]GateResult, error) {
	if cache == nil {
		return RunGates(manifest, waveNumber, repoDir)
	}

	// Return empty results if no quality gates defined
	if manifest.QualityGates == nil || len(manifest.QualityGates.Gates) == 0 {
		return []GateResult{}, nil
	}

	// Build a cache key from the current repository state
	cacheKey, err := cache.BuildKey(repoDir)
	if err != nil {
		// If we can't build a key (e.g. not a git repo), fall back to no-cache
		return RunGates(manifest, waveNumber, repoDir)
	}

	repoName := filepath.Base(repoDir)
	docsOnly := isDocsOnlyWave(manifest, waveNumber)

	var results []GateResult
	for _, gate := range manifest.QualityGates.Gates {
		// Skip gates scoped to a different repo.
		if gate.Repo != "" && gate.Repo != repoName {
			continue
		}

		// Auto-skip build/test/lint gates for docs-only waves.
		if docsOnly && isSourceGateType(gate.Type) {
			results = append(results, GateResult{
				Type:       gate.Type,
				Command:    gate.Command,
				Required:   gate.Required,
				Passed:     true,
				Skipped:    true,
				SkipReason: "docs-only wave: no compilable source files changed",
			})
			continue
		}

		// Check the cache first
		if cached, ok := cache.Get(cacheKey, gate.Type); ok {
			results = append(results, GateResult{
				Type:      gate.Type,
				Command:   gate.Command,
				Required:  gate.Required,
				Passed:    cached.Passed,
				ExitCode:  cached.ExitCode,
				Stdout:    cached.Stdout,
				Stderr:    cached.Stderr,
				FromCache: true,
			})
			continue
		}

		// Cache miss — run the gate
		result := GateResult{
			Type:     gate.Type,
			Command:  gate.Command,
			Required: gate.Required,
		}

		cmd := exec.Command("sh", "-c", gate.Command)
		cmd.Dir = repoDir

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		runErr := cmd.Run()
		result.Stdout = stdout.String()
		result.Stderr = stderr.String()

		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = -1
				if result.Stderr == "" {
					result.Stderr = fmt.Sprintf("command failed to execute: %v", runErr)
				}
			}
			result.Passed = false
		} else {
			result.ExitCode = 0
			result.Passed = true
		}

		// Parse structured errors from output
		if pr := errparse.ParseOutput(gate.Type, gate.Command, result.Stdout, result.Stderr); pr != nil {
			result.ParsedErrors = pr.Errors
		}

		// Store in cache (ignore errors — cache is best-effort)
		_ = cache.Put(cacheKey, gate.Type, gatecache.CachedResult{
			GateType: gate.Type,
			Command:  gate.Command,
			Passed:   result.Passed,
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		})

		results = append(results, result)
	}

	return results, nil
}
