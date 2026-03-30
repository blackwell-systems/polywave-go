package protocol

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/errparse"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/format"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// loggerFrom returns the provided logger if non-nil, otherwise slog.Default().
// This helper allows optional logger injection while maintaining backward compatibility.
func loggerFrom(l *slog.Logger) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	return l
}

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

// ValidateQualityGate checks that a quality gate configuration is valid.
// Returns error if gate has invalid field combinations.
// BLOCKER 2 FIX: Format gates with fix=true MUST be in PRE_VALIDATION phase
// to prevent cache invalidation during parallel VALIDATION phase execution.
func ValidateQualityGate(gate QualityGate) error {
	// Format gates with fix=true MUST be in PRE_VALIDATION phase
	// (or empty phase, which defaults to VALIDATION — that's an error too)
	if gate.Fix && gate.Phase != GatePhasePre && gate.Phase != "" {
		return fmt.Errorf("format gates with fix=true must be in PRE_VALIDATION phase (got %s)", gate.Phase)
	}
	// Empty phase + fix=true is also invalid (would default to VALIDATION)
	if gate.Fix && gate.Phase == "" {
		return fmt.Errorf("format gates with fix=true must be in PRE_VALIDATION phase (got empty, which defaults to VALIDATION)")
	}
	return nil
}

// gateGroup represents a set of gates that execute together (sequentially or concurrently).
type gateGroup struct {
	parallel bool            // true = execute concurrently, false = sequential
	gates    []QualityGate
}

// groupGatesByPhase partitions gates into phase buckets.
// Gates with empty Phase are placed in GatePhaseMain (VALIDATION).
func groupGatesByPhase(gates []QualityGate) map[GatePhase][]QualityGate {
	grouped := make(map[GatePhase][]QualityGate)
	for _, gate := range gates {
		phase := gate.Phase
		if phase == "" {
			phase = GatePhaseMain // backward compat default
		}
		grouped[phase] = append(grouped[phase], gate)
	}
	return grouped
}

// groupGatesByParallelGroup partitions gates within a phase into execution groups.
// Gates with empty ParallelGroup run sequentially (one group per gate).
// Gates with the same non-empty ParallelGroup run concurrently (one group).
func groupGatesByParallelGroup(gates []QualityGate) []gateGroup {
	var sequential []QualityGate
	parallelMap := make(map[string][]QualityGate)

	for _, gate := range gates {
		if gate.ParallelGroup == "" {
			sequential = append(sequential, gate)
		} else {
			parallelMap[gate.ParallelGroup] = append(parallelMap[gate.ParallelGroup], gate)
		}
	}

	var groups []gateGroup
	// Sequential gates first (maintain order)
	for _, gate := range sequential {
		groups = append(groups, gateGroup{parallel: false, gates: []QualityGate{gate}})
	}
	// Parallel groups next
	for _, groupGates := range parallelMap {
		groups = append(groups, gateGroup{parallel: true, gates: groupGates})
	}
	return groups
}

// executeSingleGate runs a single gate and returns its result.
// Handles cache lookup, format gate special case, and result caching.
// This function extracts the per-gate execution logic from RunGatesWithCache's loop.
func executeSingleGate(gate QualityGate, repoDir string, cache *gatecache.Cache, logger *slog.Logger) GateResult {
	log := loggerFrom(logger)
	// Build a per-gate cache key that includes the gate command string.
	// This ensures that changing the command (e.g. adding a flag) causes
	// a cache miss rather than returning a stale result.
	var cacheKey gatecache.CacheKey
	if cache != nil {
		var err error
		cacheKey, err = cache.BuildKeyForGate(repoDir, gate.Command)
		if err != nil {
			// Cannot build key (e.g. not a git repo) — fall back to no-cache for
			// this gate by running it below without checking or populating cache.
			// We continue through the rest of the function.
			cacheKey = gatecache.CacheKey{}
		}
	}

	// Check the cache first (only when we successfully built a key)
	if cache != nil && cacheKey.HeadCommit != "" {
		if cached, ok := cache.Get(cacheKey, gate.Type); ok {
			skipReason := fmt.Sprintf("cached at SHA %s", cacheKey.HeadCommit)
			log.Debug("protocol: gate skipped (cached)", "gate", gate.Type, "sha", cacheKey.HeadCommit)
			return GateResult{
				Type:       gate.Type,
				Command:    gate.Command,
				Required:   gate.Required,
				Passed:     cached.Passed,
				ExitCode:   cached.ExitCode,
				Stdout:     cached.Stdout,
				Stderr:     cached.Stderr,
				FromCache:  true,
				SkipReason: skipReason,
			}
		}
	}

	// Format gates use the dedicated runner for auto-detection + fix mode.
	// Place after cache-hit check so cached format results are still served,
	// but before the generic exec block. Fix mode always bypasses cache anyway
	// since we invalidate after running.
	if gate.Type == "format" {
		return runFormatGate(gate, repoDir, cache, logger)
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
	if cache != nil && cacheKey.HeadCommit != "" {
		_ = cache.Put(cacheKey, gate.Type, gatecache.CachedResult{
			GateType: gate.Type,
			Command:  gate.Command,
			Passed:   result.Passed,
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		})
	}

	return result
}

// executeGatesConcurrently runs gates in parallel using goroutines.
// Returns when all gates complete. Collects results in order.
func executeGatesConcurrently(gates []QualityGate, repoDir string, cache *gatecache.Cache, logger *slog.Logger) []GateResult {
	var wg sync.WaitGroup
	results := make([]GateResult, len(gates))

	for i, gate := range gates {
		wg.Add(1)
		go func(idx int, g QualityGate) {
			defer wg.Done()
			results[idx] = executeSingleGate(g, repoDir, cache, logger)
		}(i, gate)
	}

	wg.Wait()
	return results
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
func runFormatGate(gate QualityGate, repoDir string, cache *gatecache.Cache, logger *slog.Logger) GateResult {
	_ = loggerFrom(logger) // Reserved for future logging
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

// runGates executes quality gates for a specific wave from the manifest.
// It runs each gate command and collects results.
// The repoDir parameter is the working directory for command execution.
//
// This is an internal implementation detail. Callers outside this package must
// use RunPreMergeGates or RunPostMergeGates. RunGatesWithCache is the correct
// public entry point when optional caching is desired.
//
// Returns a successful Result with empty Gates slice if manifest has no QualityGates or Gates is empty.
// Gate failures are recorded in GateResult.Passed; the caller decides how to handle them.
func runGates(manifest *IMPLManifest, waveNumber int, repoDir string, logger *slog.Logger) result.Result[GatesData] {
	_ = loggerFrom(logger) // Reserved for future logging
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
			gates = append(gates, runFormatGate(gate, repoDir, nil, logger))
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
// This is the preferred public entry point for pre-merge gate execution at finalize-wave step 3.
// It delegates to RunGatesWithCache with a filtered manifest; cache param may be nil.
// Signature mirrors RunGatesWithCache exactly.
func RunPreMergeGates(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache, logger *slog.Logger) result.Result[GatesData] {
	_ = loggerFrom(logger) // Reserved for future logging
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
	return RunGatesWithCache(&tmp, waveNumber, repoDir, cache, logger)
}

// RunPostMergeGates executes only gates with timing="post-merge".
// Called at finalize-wave step 5, after MergeAgents completes successfully.
// Returns empty GatesData (not error) when no post-merge gates are defined.
// Runs without cache (post-merge state is always fresh, so caching is intentionally omitted).
// This is the public entry point for post-merge gate execution; it delegates to the
// internal runGates function.
func RunPostMergeGates(manifest *IMPLManifest, waveNumber int, repoDir string, logger *slog.Logger) result.Result[GatesData] {
	_ = loggerFrom(logger) // Reserved for future logging
	filtered := filterGatesByTiming(manifest, "post-merge")
	if len(filtered) == 0 {
		return result.NewSuccess(GatesData{Gates: []GateResult{}})
	}
	tmp := *manifest
	tmp.QualityGates = &QualityGates{
		Level: manifest.QualityGates.Level,
		Gates: filtered,
	}
	return runGates(&tmp, waveNumber, repoDir, logger)
}

// RunGatesWithCache executes quality gates with optional result caching.
// This is the public entry point for pre-merge gate execution with optional caching.
// If cache is nil, it delegates to the internal runGates function (no caching).
// Gates are grouped by phase (PRE → VALIDATION → POST) and executed in order.
// Within each phase, gates can be grouped by parallel_group for concurrent execution.
// Prefer RunPreMergeGates over calling this directly for pre-merge gate execution.
func RunGatesWithCache(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache, logger *slog.Logger) result.Result[GatesData] {
	_ = loggerFrom(logger) // Reserved for future logging

	if cache == nil {
		return runGates(manifest, waveNumber, repoDir, logger)
	}

	// Return empty results if no quality gates defined
	if manifest.QualityGates == nil || len(manifest.QualityGates.Gates) == 0 {
		return result.NewSuccess(GatesData{Gates: []GateResult{}})
	}

	repoName := filepath.Base(repoDir)
	docsOnly := isDocsOnlyWave(manifest, waveNumber)

	// Filter gates by repo + docs-only logic
	var filteredGates []QualityGate
	for _, gate := range manifest.QualityGates.Gates {
		// Validate gate before execution (BLOCKER 2 fix)
		if err := ValidateQualityGate(gate); err != nil {
			return result.NewFailure[GatesData]([]result.SAWError{
				result.NewFatal("GATE_INVALID", fmt.Sprintf("invalid gate %s: %v", gate.Type, err)),
			})
		}

		// Skip gates scoped to a different repo
		if gate.Repo != "" && gate.Repo != repoName {
			continue
		}

		// Auto-skip build/test/lint gates for docs-only waves
		if docsOnly && isSourceGateType(gate.Type) {
			// Add skipped result to maintain output consistency
			filteredGates = append(filteredGates, gate)
			continue
		}

		filteredGates = append(filteredGates, gate)
	}

	// Group gates by phase
	phaseGroups := groupGatesByPhase(filteredGates)

	var allResults []GateResult

	// Execute phases in order: PRE → VALIDATION → POST
	for _, phase := range []GatePhase{GatePhasePre, GatePhaseMain, GatePhasePost} {
		phaseGates := phaseGroups[phase]
		if len(phaseGates) == 0 {
			continue
		}

		// Group gates within phase by parallel_group
		groups := groupGatesByParallelGroup(phaseGates)

		for _, group := range groups {
			// Check for docs-only skip within group execution
			var groupResults []GateResult
			var activeGates []QualityGate

			for _, gate := range group.gates {
				if docsOnly && isSourceGateType(gate.Type) {
					// Skip with result
					groupResults = append(groupResults, GateResult{
						Type:       gate.Type,
						Command:    gate.Command,
						Required:   gate.Required,
						Passed:     true,
						Skipped:    true,
						SkipReason: "docs-only wave: no compilable source files changed",
					})
				} else {
					activeGates = append(activeGates, gate)
				}
			}

			if len(activeGates) > 0 {
				if group.parallel {
					// Execute concurrently
					results := executeGatesConcurrently(activeGates, repoDir, cache, logger)
					groupResults = append(groupResults, results...)
				} else {
					// Execute sequentially
					for _, gate := range activeGates {
						gateResult := executeSingleGate(gate, repoDir, cache, logger)
						groupResults = append(groupResults, gateResult)
					}
				}
			}

			allResults = append(allResults, groupResults...)
		}
	}

	return result.NewSuccess(GatesData{Gates: allResults})
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
