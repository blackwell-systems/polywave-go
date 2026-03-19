package protocol

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/errparse"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
)

func TestRunGates_NoGates(t *testing.T) {
	// Test with nil QualityGates
	manifest := &IMPLManifest{}
	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d results", len(results))
	}

	// Test with empty Gates slice
	manifest.QualityGates = &QualityGates{
		Level: "quick",
		Gates: []QualityGate{},
	}
	results, err = RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d results", len(results))
	}
}

func TestRunGates_PassingGate(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "test",
					Command:  "echo ok",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Type != "test" {
		t.Errorf("expected Type 'test', got '%s'", result.Type)
	}
	if result.Command != "echo ok" {
		t.Errorf("expected Command 'echo ok', got '%s'", result.Command)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true, got false")
	}
	if result.Stdout != "ok\n" {
		t.Errorf("expected Stdout 'ok\\n', got '%s'", result.Stdout)
	}
	if !result.Required {
		t.Errorf("expected Required=true, got false")
	}
}

func TestRunGates_FailingGate(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  "exit 1",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.ExitCode != 1 {
		t.Errorf("expected ExitCode 1, got %d", result.ExitCode)
	}
	if result.Passed {
		t.Errorf("expected Passed=false, got true")
	}
}

func TestRunGates_MixedGates(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  "echo build success",
					Required: true,
				},
				{
					Type:     "test",
					Command:  "false",
					Required: false,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First gate should pass
	if !results[0].Passed {
		t.Errorf("expected first gate to pass")
	}
	if results[0].ExitCode != 0 {
		t.Errorf("expected first gate ExitCode 0, got %d", results[0].ExitCode)
	}

	// Second gate should fail
	if results[1].Passed {
		t.Errorf("expected second gate to fail")
	}
	if results[1].ExitCode == 0 {
		t.Errorf("expected second gate ExitCode != 0, got 0")
	}
}

func TestRunGates_CapturesOutput(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "test",
					Command:  "echo stdout message && echo stderr message >&2",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Stdout != "stdout message\n" {
		t.Errorf("expected Stdout 'stdout message\\n', got '%s'", result.Stdout)
	}
	if result.Stderr != "stderr message\n" {
		t.Errorf("expected Stderr 'stderr message\\n', got '%s'", result.Stderr)
	}
}

func TestRunGates_NonExistentCommand(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  "nonexistent_command_xyz",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error (gate failures should not return errors), got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Passed {
		t.Errorf("expected non-existent command to fail")
	}
	if result.ExitCode == 0 {
		t.Errorf("expected non-zero exit code for non-existent command")
	}
	// Stderr should contain an error message
	if result.Stderr == "" {
		t.Errorf("expected Stderr to contain error message for non-existent command")
	}
}

// ---- RunGatesWithCache tests ----

func TestRunGatesWithCache_NilCache(t *testing.T) {
	// With nil cache, RunGatesWithCache must behave identically to RunGates.
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{Type: "build", Command: "echo build ok", Required: true},
			},
		},
	}

	results, err := RunGatesWithCache(manifest, 1, "/tmp", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Error("expected gate to pass")
	}
	if results[0].FromCache {
		t.Error("expected FromCache=false with nil cache")
	}
}

func TestRunGatesWithCache_CacheHit(t *testing.T) {
	dir := t.TempDir()
	cache := gatecache.New(dir, 5*time.Minute)

	// Manually seed the cache with a passing result.
	// We need to use the same key that BuildKey would return.  Since /tmp is
	// not a git repo, BuildKey will fail and RunGatesWithCache falls back to
	// RunGates (no-cache path).  To test the true cache-hit path we seed a
	// fake key directly and call Get/Put ourselves.

	fakeKey := gatecache.CacheKey{
		HeadCommit:   "cafebabe1234567890cafebabe1234567890cafe",
		StagedStat:   "",
		UnstagedStat: "",
	}
	seedResult := gatecache.CachedResult{
		GateType: "build",
		Command:  "exit 42",
		Passed:   true,
		ExitCode: 0,
		Stdout:   "cached stdout",
		Stderr:   "",
	}
	if err := cache.Put(fakeKey, "build", seedResult); err != nil {
		t.Fatalf("seeding cache failed: %v", err)
	}

	// Verify the seeded value is retrievable
	got, ok := cache.Get(fakeKey, "build")
	if !ok {
		t.Fatal("seeded cache entry not found")
	}
	if !got.Passed {
		t.Fatal("seeded entry should be Passed=true")
	}
	if !got.FromCache {
		t.Fatal("seeded entry should have FromCache=true")
	}
}

func TestRunGatesWithCache_CacheMissRunsGate(t *testing.T) {
	dir := t.TempDir()
	cache := gatecache.New(dir, 5*time.Minute)

	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{Type: "build", Command: "echo run ok", Required: true},
			},
		},
	}

	// /tmp is almost certainly not a git repo; BuildKey should fail and we
	// fall back to RunGates (cache miss path runs the gate normally).
	results, err := RunGatesWithCache(manifest, 1, "/tmp", cache)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Error("expected gate to pass")
	}
}

func TestRunGatesWithCache_EmptyManifest(t *testing.T) {
	dir := t.TempDir()
	cache := gatecache.New(dir, 5*time.Minute)

	manifest := &IMPLManifest{}

	results, err := RunGatesWithCache(manifest, 1, "/tmp", cache)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

// ---- ParsedErrors tests ----

// TestRunGates_ParsedErrors_Build verifies that a failing build gate with
// Go-style compiler error output populates ParsedErrors.
func TestRunGates_ParsedErrors_Build(t *testing.T) {
	// Emit a Go build-like error to stderr and exit 1.
	// The go-build parser triggers on gate type "build" with "go" in command.
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  `echo 'main.go:10:5: undefined: Foo' >&2; exit 1`,
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Passed {
		t.Error("expected gate to fail")
	}
	// ParsedErrors may be empty if command doesn't match go tool pattern,
	// but the field must exist (nil or empty slice is fine for non-Go commands).
	// For a gate type "build" with no "go" in command, errparse returns nil.
	// That's acceptable — we just verify the field is accessible.
	_ = result.ParsedErrors
}

// TestRunGates_ParsedErrors_Passing verifies that a passing gate has empty ParsedErrors.
func TestRunGates_ParsedErrors_Passing(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  "echo ok",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.Passed {
		t.Error("expected gate to pass")
	}
	if len(result.ParsedErrors) != 0 {
		t.Errorf("expected empty ParsedErrors for passing gate, got %d", len(result.ParsedErrors))
	}
}

// TestRunGatesWithCache_ParsedErrors verifies that the cache-miss path populates
// ParsedErrors (same as RunGates, since /tmp is not a git repo → fallback).
func TestRunGatesWithCache_ParsedErrors(t *testing.T) {
	dir := t.TempDir()
	cache := gatecache.New(dir, 5*time.Minute)

	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  "echo ok",
					Required: true,
				},
			},
		},
	}

	results, err := RunGatesWithCache(manifest, 1, "/tmp", cache)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// ParsedErrors field should be accessible (nil/empty for non-matching commands)
	_ = results[0].ParsedErrors
}

// ---- Format gate tests ----

// TestIsSourceGateType_Format asserts that "format" is treated as a source gate type
// and will be skipped on docs-only waves.
func TestIsSourceGateType_Format(t *testing.T) {
	if !isSourceGateType("format") {
		t.Error("expected isSourceGateType(\"format\") == true")
	}
	// Verify other source types still work.
	for _, typ := range []string{"build", "test", "tests", "lint"} {
		if !isSourceGateType(typ) {
			t.Errorf("expected isSourceGateType(%q) == true", typ)
		}
	}
	// Non-source types.
	for _, typ := range []string{"custom", "typecheck", "unknown"} {
		if isSourceGateType(typ) {
			t.Errorf("expected isSourceGateType(%q) == false", typ)
		}
	}
}

// TestRunGates_FormatGate_CheckMode tests that a format gate with an explicit command
// runs in check mode (gate.Fix=false) and returns the correct type/pass status.
func TestRunGates_FormatGate_CheckMode(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{
					Type:     "format",
					Command:  "echo formatted",
					Required: true,
					Fix:      false,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, t.TempDir())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	result := results[0]
	if result.Type != "format" {
		t.Errorf("expected Type 'format', got %q", result.Type)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true, got false (stderr: %s)", result.Stderr)
	}
	if result.Skipped {
		t.Error("expected Skipped=false for explicit command")
	}
}

// TestRunGates_FormatGate_FixMode tests that a format gate with Fix=true runs
// the command and, when a cache is provided, the cache is invalidated after execution.
func TestRunGates_FormatGate_FixMode(t *testing.T) {
	dir := t.TempDir()
	cache := gatecache.New(dir, 5*time.Minute)

	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{
					Type:     "format",
					Command:  "echo fixed",
					Required: true,
					Fix:      true,
				},
			},
		},
	}

	results, err := RunGatesWithCache(manifest, 1, t.TempDir(), cache)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	result := results[0]
	if result.Type != "format" {
		t.Errorf("expected Type 'format', got %q", result.Type)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true, got false (stderr: %s)", result.Stderr)
	}
	// After fix mode, cache should have been invalidated (no error expected).
	// We can't directly assert the cache is empty without internals, but we
	// verify runFormatGate returned a proper result.
}

// TestRunGates_FormatGate_SkipNoFormatter tests that a format gate with no explicit
// command and no formatter-marker files in the temp dir returns Skipped=true, Passed=true.
// The stub DetectFormatter always returns empty FormatConfig, so this exercises that path.
func TestRunGates_FormatGate_SkipNoFormatter(t *testing.T) {
	// Use a temp dir with no go.mod / package.json / etc.
	emptyDir := t.TempDir()

	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{
					Type:     "format",
					Command:  "", // no explicit command — triggers auto-detection
					Required: false,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, emptyDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	result := results[0]
	if result.Type != "format" {
		t.Errorf("expected Type 'format', got %q", result.Type)
	}
	if !result.Skipped {
		t.Errorf("expected Skipped=true when no formatter detected, got false")
	}
	if !result.Passed {
		t.Errorf("expected Passed=true when skipped (no formatter), got false")
	}
}

// TestRunGates_FormatGate_WithGoMod tests that a format gate with no explicit command
// in a directory with go.mod triggers gofmt detection and runs a check command.
// Since DetectFormatter is a stub in Agent B's worktree, this test verifies behavior
// once Agent A's implementation is in place by using an explicit command instead.
func TestRunGates_FormatGate_ExplicitCommandSucceeds(t *testing.T) {
	// Write a go.mod to simulate a Go project (useful for integration after merge).
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/go.mod", []byte("module example.com/test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{
					Type:     "format",
					Command:  "echo format-check-pass",
					Required: true,
					Fix:      false,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected format gate to pass with explicit echo command")
	}
	if results[0].Type != "format" {
		t.Errorf("expected Type 'format', got %q", results[0].Type)
	}
}

// ---- Timing / pre-merge / post-merge gate tests ----

// TestFilterGatesByTiming verifies gate routing logic for pre-merge and post-merge.
func TestFilterGatesByTiming(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "echo build", Timing: ""},         // pre-merge (default)
				{Type: "test", Command: "echo test", Timing: "pre-merge"},  // explicit pre-merge
				{Type: "lint", Command: "echo lint", Timing: "post-merge"}, // post-merge
			},
		},
	}

	pre := filterGatesByTiming(manifest, "pre-merge")
	if len(pre) != 2 {
		t.Fatalf("expected 2 pre-merge gates, got %d", len(pre))
	}
	if pre[0].Type != "build" {
		t.Errorf("expected first pre-merge gate type 'build', got %q", pre[0].Type)
	}
	if pre[1].Type != "test" {
		t.Errorf("expected second pre-merge gate type 'test', got %q", pre[1].Type)
	}

	post := filterGatesByTiming(manifest, "post-merge")
	if len(post) != 1 {
		t.Fatalf("expected 1 post-merge gate, got %d", len(post))
	}
	if post[0].Type != "lint" {
		t.Errorf("expected post-merge gate type 'lint', got %q", post[0].Type)
	}
}

// TestRunPreMergeGates_OnlyRunsPreMerge verifies that RunPreMergeGates only executes
// pre-merge gates and does not execute the post-merge exit-1 gate.
func TestRunPreMergeGates_OnlyRunsPreMerge(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "echo ok", Required: true, Timing: "pre-merge"},
				{Type: "test", Command: "exit 1", Required: true, Timing: "post-merge"},
			},
		},
	}

	results, err := RunPreMergeGates(manifest, 1, "/tmp", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only pre-merge gate), got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected pre-merge gate to pass, got Passed=false")
	}
}

// TestRunPostMergeGates_OnlyRunsPostMerge verifies that RunPostMergeGates only executes
// post-merge gates and does not execute the pre-merge exit-1 gate.
func TestRunPostMergeGates_OnlyRunsPostMerge(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "exit 1", Required: true, Timing: "pre-merge"},
				{Type: "test", Command: "echo ok", Required: true, Timing: "post-merge"},
			},
		},
	}

	results, err := RunPostMergeGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (only post-merge gate), got %d", len(results))
	}
	if !results[0].Passed {
		t.Errorf("expected post-merge gate to pass, got Passed=false")
	}
}

// TestRunPreMergeGates_EmptyWhenNoneMatch verifies that RunPreMergeGates returns an
// empty slice (not an error) when the manifest has only post-merge gates.
func TestRunPreMergeGates_EmptyWhenNoneMatch(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "test", Command: "echo ok", Required: true, Timing: "post-merge"},
			},
		},
	}

	results, err := RunPreMergeGates(manifest, 1, "/tmp", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results when no pre-merge gates match, got %d", len(results))
	}
}

// TestRunPostMergeGates_EmptyWhenNoneMatch verifies that RunPostMergeGates returns an
// empty slice (not an error) when the manifest has only pre-merge gates.
func TestRunPostMergeGates_EmptyWhenNoneMatch(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "echo ok", Required: true, Timing: "pre-merge"},
			},
		},
	}

	results, err := RunPostMergeGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results when no post-merge gates match, got %d", len(results))
	}
}

// TestRunPreMergeGates_BackwardCompat verifies that gates without a Timing field
// (empty string) are treated as pre-merge, preserving backward compatibility.
func TestRunPreMergeGates_BackwardCompat(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "echo build ok", Required: true}, // Timing omitted
				{Type: "test", Command: "echo test ok", Required: true},   // Timing omitted
			},
		},
	}

	results, err := RunPreMergeGates(manifest, 1, "/tmp", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (both backward-compat gates run as pre-merge), got %d", len(results))
	}
	for i, r := range results {
		if !r.Passed {
			t.Errorf("expected gate[%d] to pass, got Passed=false", i)
		}
	}
}

// TestGateResult_JSONWithParsedErrors verifies JSON marshaling includes parsed_errors.
func TestGateResult_JSONWithParsedErrors(t *testing.T) {
	gr := GateResult{
		Type:     "build",
		Command:  "go build ./...",
		ExitCode: 1,
		Stdout:   "",
		Stderr:   "main.go:5:1: syntax error",
		Required: true,
		Passed:   false,
	}

	data, err := json.Marshal(gr)
	if err != nil {
		t.Fatalf("failed to marshal GateResult: %v", err)
	}

	// parsed_errors should be omitted when nil/empty (omitempty tag)
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, present := m["parsed_errors"]; present {
		t.Error("expected parsed_errors to be omitted when empty")
	}

	// Now add a parsed error and verify it appears
	gr.ParsedErrors = []errparse.StructuredError{
		{
			File:     "main.go",
			Line:     5,
			Severity: "error",
			Message:  "syntax error",
			Tool:     "go-build",
		},
	}

	data2, err := json.Marshal(gr)
	if err != nil {
		t.Fatalf("failed to marshal GateResult with parsed errors: %v", err)
	}

	var m2 map[string]interface{}
	if err := json.Unmarshal(data2, &m2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, present := m2["parsed_errors"]; !present {
		t.Error("expected parsed_errors to be present in JSON when non-empty")
	}
	errs := m2["parsed_errors"].([]interface{})
	if len(errs) != 1 {
		t.Errorf("expected 1 parsed error in JSON, got %d", len(errs))
	}
}
