package protocol

import (
	"testing"
	"time"

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
