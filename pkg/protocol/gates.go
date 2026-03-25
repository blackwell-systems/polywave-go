package protocol

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/errparse"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/format"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// GateResult represents the outcome of executing a single quality gate.
// It captures all execution details including stdout/stderr and pass/fail status.
type GateResult struct {
	Type         string                     `json:"type"`
	Command      string                     `json:"command"`
	ExitCode     int                        `json:"exit_code"`
	Stdout       string                     `json:"stdout"`
	Stderr       string                     `json:"stderr"`
	Required     bool                       `json:"required"`
	Passed       bool                       `json:"passed"`
	Skipped      bool                       `json:"skipped,omitempty"`
	SkipReason   string                     `json:"skip_reason,omitempty"`
	FromCache    bool                       `json:"from_cache,omitempty"`
	ParsedErrors []result.SAWError `json:"parsed_errors,omitempty"`
}

// GatesData wraps gate execution results for use with result.Result[T].
type GatesData struct {
	Gates []GateResult `json:"gates"`
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
	case "build", "test", "tests", "lint", "format":
		return true
	}
	return false
}

// runFormatGate executes a format gate for the given project root.
// If gate.Command is non-empty, it is used as-is (bypasses auto-detection).
// If gate.Fix is true, runs the fix command (rewrites files) and invalidates
// the cache (if non-nil) because file content has changed.
// If gate.Fix is false (default), runs the check command (report only).
func runFormatGate(gate QualityGate, repoDir string, cache *gatecache.Cache) GateResult {
	result := GateResult{
		Type:     gate.Type,
		Command:  gate.Command,
		Required: gate.Required,
	}

	// Detect formatter if no explicit command provided.
	var cmd string
	if gate.Command != "" {
		cmd = gate.Command
	} else {
		cfg := format.DetectFormatter(repoDir)
		if cfg.Tool == "" {
			// No formatter detected — treat as skipped (not an error).
			result.Passed = true
			result.Skipped = true
			result.SkipReason = "no formatter detected for project type"
			return result
		}
		if gate.Fix {
			cmd = cfg.FixCmd
		} else {
			cmd = cfg.CheckCmd
		}
		result.Command = cmd
	}

	// Execute the formatter command.
	execCmd := exec.Command("sh", "-c", cmd)
	execCmd.Dir = repoDir
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err := execCmd.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			if result.Stderr == "" {
				result.Stderr = fmt.Sprintf("format gate failed to execute: %v", err)
			}
		}
		result.Passed = false
	} else {
		result.ExitCode = 0
		result.Passed = true
	}

	// In fix mode: invalidate the cache so subsequent gates
	// see the freshly-formatted files.
	if gate.Fix && cache != nil {
		_ = cache.Invalidate()
	}

	// Parse structured errors from formatter output.
	if pr := errparse.ParseOutput(gate.Type, result.Command, result.Stdout, result.Stderr); pr != nil {
		result.ParsedErrors = pr.Errors
	}

	return result
}

// RunGates executes quality gates for a specific wave from the manifest.
// It runs each gate command and collects results.
// The repoDir parameter is the working directory for command execution.
//
// Returns a successful Result with empty Gates slice if manifest has no QualityGates or Gates is empty.
// Gate failures are recorded in GateResult.Passed; the caller decides how to handle them.
func RunGates(manifest *IMPLManifest, waveNumber int, repoDir string) result.Result[GatesData] {
	// Return empty results if no quality gates defined
	if manifest.QualityGates == nil || len(manifest.QualityGates.Gates) == 0 {
		return result.NewSuccess(GatesData{Gates: []GateResult{}})
	}

	// Derive the repo name from the directory path for per-repo gate filtering.
	repoName := filepath.Base(repoDir)

	// Skip source-code gates when the wave only touches documentation files.
	docsOnly := isDocsOnlyWave(manifest, waveNumber)

	var gates []GateResult
	for _, gate := range manifest.QualityGates.Gates {
		// Skip gates scoped to a different repo.
		if gate.Repo != "" && gate.Repo != repoName {
			continue
		}

		// Auto-skip build/test/lint gates for docs-only waves.
		if docsOnly && isSourceGateType(gate.Type) {
			gates = append(gates, GateResult{
				Type:       gate.Type,
				Command:    gate.Command,
				Required:   gate.Required,
				Passed:     true,
				Skipped:    true,
				SkipReason: "docs-only wave: no compilable source files changed",
			})
			continue
		}

		// Skip gates whose build system is absent in repoDir (e.g. go gates in a
		// docs-only repo that has no go.mod). Skipped gates count as passed so they
		// never block execution in repos that simply don't use that toolchain.
		if bs := gateBuildSystem(gate); bs != "" {
			if !detectBuildSystems(repoDir)[bs] {
				gates = append(gates, GateResult{
					Type:       gate.Type,
					Command:    gate.Command,
					Required:   gate.Required,
					Passed:     true,
					Skipped:    true,
					SkipReason: fmt.Sprintf("no %s project detected in repo (missing marker file)", bs),
				})
				continue
			}
		}

		// Format gates use the dedicated runner for auto-detection + fix mode.
		if gate.Type == "format" {
			gates = append(gates, runFormatGate(gate, repoDir, nil))
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

		gates = append(gates, result)
	}

	return result.NewSuccess(GatesData{Gates: gates})
}

// filterGatesByTiming returns gates matching the given timing category.
// timing="pre-merge" returns gates with Timing=="" or Timing=="pre-merge".
// timing="post-merge" returns gates with Timing=="post-merge".
func filterGatesByTiming(manifest *IMPLManifest, timing string) []QualityGate {
	if manifest.QualityGates == nil {
		return nil
	}
	var out []QualityGate
	for _, g := range manifest.QualityGates.Gates {
		switch timing {
		case "pre-merge":
			if g.Timing == "" || g.Timing == "pre-merge" {
				out = append(out, g)
			}
		case "post-merge":
			if g.Timing == "post-merge" {
				out = append(out, g)
			}
		}
	}
	return out
}

// RunPreMergeGates executes only gates with timing="pre-merge" or timing="" (default).
// It replaces the direct RunGatesWithCache call at finalize-wave step 3.
// Signature mirrors RunGatesWithCache exactly; cache param may be nil.
func RunPreMergeGates(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache) result.Result[GatesData] {
	filtered := filterGatesByTiming(manifest, "pre-merge")
	if len(filtered) == 0 {
		return result.NewSuccess(GatesData{Gates: []GateResult{}})
	}
	// Build a temporary manifest with only pre-merge gates
	tmp := *manifest
	tmp.QualityGates = &QualityGates{
		Level: manifest.QualityGates.Level,
		Gates: filtered,
	}
	return RunGatesWithCache(&tmp, waveNumber, repoDir, cache)
}

// RunPostMergeGates executes only gates with timing="post-merge".
// Called at finalize-wave step 5, after MergeAgents completes successfully.
// Returns empty GatesData (not error) when no post-merge gates are defined.
// Runs without cache (post-merge state is always fresh).
func RunPostMergeGates(manifest *IMPLManifest, waveNumber int, repoDir string) result.Result[GatesData] {
	filtered := filterGatesByTiming(manifest, "post-merge")
	if len(filtered) == 0 {
		return result.NewSuccess(GatesData{Gates: []GateResult{}})
	}
	tmp := *manifest
	tmp.QualityGates = &QualityGates{
		Level: manifest.QualityGates.Level,
		Gates: filtered,
	}
	return RunGates(&tmp, waveNumber, repoDir)
}

// RunGatesWithCache executes quality gates with optional result caching.
// If cache is nil, it behaves identically to RunGates.
// For each gate, if a valid (non-expired) cached result exists, it is returned
// directly without re-executing the gate command. Otherwise the gate is run
// normally and the result is stored in the cache.
func RunGatesWithCache(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache) result.Result[GatesData] {
	if cache == nil {
		return RunGates(manifest, waveNumber, repoDir)
	}

	// Return empty results if no quality gates defined
	if manifest.QualityGates == nil || len(manifest.QualityGates.Gates) == 0 {
		return result.NewSuccess(GatesData{Gates: []GateResult{}})
	}

	repoName := filepath.Base(repoDir)
	docsOnly := isDocsOnlyWave(manifest, waveNumber)

	var gates []GateResult
	for _, gate := range manifest.QualityGates.Gates {
		// Skip gates scoped to a different repo.
		if gate.Repo != "" && gate.Repo != repoName {
			continue
		}

		// Auto-skip build/test/lint gates for docs-only waves.
		if docsOnly && isSourceGateType(gate.Type) {
			gates = append(gates, GateResult{
				Type:       gate.Type,
				Command:    gate.Command,
				Required:   gate.Required,
				Passed:     true,
				Skipped:    true,
				SkipReason: "docs-only wave: no compilable source files changed",
			})
			continue
		}

		// Build a per-gate cache key that includes the gate command string.
		// This ensures that changing the command (e.g. adding a flag) causes
		// a cache miss rather than returning a stale result.
		cacheKey, err := cache.BuildKeyForGate(repoDir, gate.Command)
		if err != nil {
			// Cannot build key (e.g. not a git repo) — fall back to no-cache for
			// this gate by running it below without checking or populating cache.
			// We continue through the rest of the loop body.
			cacheKey = gatecache.CacheKey{}
		}

		// Check the cache first (only when we successfully built a key)
		if cacheKey.HeadCommit != "" {
			if cached, ok := cache.Get(cacheKey, gate.Type); ok {
				skipReason := fmt.Sprintf("cached at SHA %s", cacheKey.HeadCommit)
				fmt.Fprintf(os.Stderr, "gate [%s]: skipped (cached at SHA %s)\n",
					gate.Type, cacheKey.HeadCommit)
				gates = append(gates, GateResult{
					Type:       gate.Type,
					Command:    gate.Command,
					Required:   gate.Required,
					Passed:     cached.Passed,
					ExitCode:   cached.ExitCode,
					Stdout:     cached.Stdout,
					Stderr:     cached.Stderr,
					FromCache:  true,
					SkipReason: skipReason,
				})
				continue
			}
		}

		// Format gates use the dedicated runner for auto-detection + fix mode.
		// Place after cache-hit check so cached format results are still served,
		// but before the generic exec block. Fix mode always bypasses cache anyway
		// since we invalidate after running.
		if gate.Type == "format" {
			gates = append(gates, runFormatGate(gate, repoDir, cache))
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

		// Store in cache (ignore errors — cache is best-effort).
		// Only store when we have a valid key (HeadCommit non-empty).
		if cacheKey.HeadCommit != "" {
			_ = cache.Put(cacheKey, gate.Type, gatecache.CachedResult{
				GateType: gate.Type,
				Command:  gate.Command,
				Passed:   result.Passed,
				ExitCode: result.ExitCode,
				Stdout:   result.Stdout,
				Stderr:   result.Stderr,
			})
		}

		gates = append(gates, result)
	}

	return result.NewSuccess(GatesData{Gates: gates})
}

// detectBuildSystems returns the set of build systems present in repoDir by
// checking for well-known marker files. Only systems with a detected marker
// are included; the map value is always true.
func detectBuildSystems(repoDir string) map[string]bool {
	markers := map[string]string{
		"go.mod":           "go",
		"package.json":     "node",
		"Cargo.toml":       "rust",
		"pyproject.toml":   "python",
		"setup.py":         "python",
		"pom.xml":          "maven",
		"build.gradle":     "gradle",
		"build.gradle.kts": "gradle",
	}
	found := map[string]bool{}
	for file, system := range markers {
		if _, err := os.Stat(filepath.Join(repoDir, file)); err == nil {
			found[system] = true
		}
	}
	return found
}

// gateBuildSystem returns the build system required to run the gate, inferred
// from the first token of the gate command. Returns "" for custom or unknown
// gates so they are never skipped by the build-system check.
func gateBuildSystem(gate QualityGate) string {
	if gate.Type == "custom" || gate.Command == "" {
		return ""
	}
	fields := strings.Fields(gate.Command)
	if len(fields) == 0 {
		return ""
	}
	switch fields[0] {
	case "go", "golangci-lint", "staticcheck":
		return "go"
	case "npm", "yarn", "pnpm", "npx", "node":
		return "node"
	case "cargo":
		return "rust"
	case "python", "python3", "pip", "pip3", "pytest", "ruff", "mypy", "uv":
		return "python"
	case "mvn":
		return "maven"
	case "gradle", "gradlew", "./gradlew":
		return "gradle"
	}
	return ""
}
