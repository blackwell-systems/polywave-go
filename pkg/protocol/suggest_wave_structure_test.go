package protocol

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestSuggestWaveStructure_CleanManifest tests that a well-structured manifest
// where callers are in a downstream wave with correct depends_on returns no problems.
func TestSuggestWaveStructure_CleanManifest(t *testing.T) {
	t.Parallel()

	// Create a temp dir to simulate the repo.
	repoDir := t.TempDir()

	// Create files that reference the symbol.
	defFile := filepath.Join(repoDir, "pkg/lib/def.go")
	callerFile := filepath.Join(repoDir, "pkg/app/caller.go")

	if err := os.MkdirAll(filepath.Dir(defFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(callerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defFile, []byte("package lib\n\nfunc MyFunc() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(callerFile, []byte("package app\n\nfunc use() { MyFunc() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		Title:       "Test",
		FeatureSlug: "test",
		FileOwnership: []FileOwnership{
			{File: "pkg/lib/def.go", Agent: "A", Wave: 1},
			{File: "pkg/app/caller.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []InterfaceContract{
			{
				Name:     "MyFunc",
				Location: "pkg/lib/def.go",
			},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "rename `MyFunc` to update signature"},
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "B", Task: "update callers", Dependencies: []string{"A"}},
				},
			},
		},
	}

	res := SuggestWaveStructure(context.Background(), m, repoDir)
	if res.IsFatal() {
		t.Fatalf("expected success, got FATAL: %v", res.Errors)
	}

	problems := res.GetData()
	if len(problems) != 0 {
		t.Errorf("expected no problems for clean manifest, got %d: %+v", len(problems), problems)
	}
}

// TestSuggestWaveStructure_EarlierWaveCaller tests that a caller in an earlier
// wave than the definition is reported as a WaveStructureProblem.
func TestSuggestWaveStructure_EarlierWaveCaller(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()

	defFile := filepath.Join(repoDir, "pkg/lib/def.go")
	callerFile := filepath.Join(repoDir, "pkg/app/early_caller.go")

	if err := os.MkdirAll(filepath.Dir(defFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(callerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defFile, []byte("package lib\n\nfunc ChangedAPI() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Caller is in wave 1 but definition is in wave 2 — earlier wave caller.
	if err := os.WriteFile(callerFile, []byte("package app\n\nfunc use() { ChangedAPI() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		Title:       "Test",
		FeatureSlug: "test",
		FileOwnership: []FileOwnership{
			{File: "pkg/app/early_caller.go", Agent: "A", Wave: 1},
			{File: "pkg/lib/def.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []InterfaceContract{
			{
				Name:     "ChangedAPI",
				Location: "pkg/lib/def.go",
			},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "implement early caller"},
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "B", Task: "rename `ChangedAPI` with new signature"},
				},
			},
		},
	}

	res := SuggestWaveStructure(context.Background(), m, repoDir)
	if res.IsFatal() {
		t.Fatalf("expected success result, got FATAL: %v", res.Errors)
	}

	problems := res.GetData()
	if len(problems) == 0 {
		t.Error("expected at least one WaveStructureProblem for earlier-wave caller, got none")
	}

	// Verify the problem describes the earlier-wave scenario.
	found := false
	for _, p := range problems {
		if p.Symbol == "ChangedAPI" && p.CallerWave < p.ChangedInWave {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WaveStructureProblem with CallerWave < ChangedInWave for 'ChangedAPI', got: %+v", problems)
	}
}

// TestSuggestWaveStructure_NilManifest tests that a nil manifest returns a FATAL result.
func TestSuggestWaveStructure_NilManifest(t *testing.T) {
	t.Parallel()

	res := SuggestWaveStructure(context.Background(), nil, "/tmp")
	if !res.IsFatal() {
		t.Error("expected FATAL result for nil manifest, got non-FATAL")
	}
	if len(res.Errors) == 0 {
		t.Error("expected errors in FATAL result")
	}
}

// TestSuggestWaveStructure_LaterWaveMissingDependsOn tests that callers in a later
// wave without the defining agent in depends_on are flagged as errors.
func TestSuggestWaveStructure_LaterWaveMissingDependsOn(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()

	defFile := filepath.Join(repoDir, "pkg/lib/api.go")
	callerFile := filepath.Join(repoDir, "pkg/app/user.go")

	if err := os.MkdirAll(filepath.Dir(defFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(callerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defFile, []byte("package lib\n\nfunc SharedAPI() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(callerFile, []byte("package app\n\nfunc use() { SharedAPI() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		Title:       "Test",
		FeatureSlug: "test",
		FileOwnership: []FileOwnership{
			{File: "pkg/lib/api.go", Agent: "A", Wave: 1},
			// Agent B in wave 2 calls SharedAPI but does NOT declare depends_on: [A].
			{File: "pkg/app/user.go", Agent: "B", Wave: 2},
		},
		InterfaceContracts: []InterfaceContract{
			{
				Name:     "SharedAPI",
				Location: "pkg/lib/api.go",
			},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "rename `SharedAPI` signature change"},
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					// No Dependencies listed — missing depends_on: A.
					{ID: "B", Task: "use shared api"},
				},
			},
		},
	}

	res := SuggestWaveStructure(context.Background(), m, repoDir)
	if res.IsFatal() {
		t.Fatalf("expected success result, got FATAL: %v", res.Errors)
	}

	problems := res.GetData()
	if len(problems) == 0 {
		t.Error("expected at least one WaveStructureProblem for missing depends_on, got none")
	}

	// Verify the problem is about the later wave.
	found := false
	for _, p := range problems {
		if p.Symbol == "SharedAPI" && p.CallerWave > p.ChangedInWave {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected WaveStructureProblem with CallerWave > ChangedInWave for 'SharedAPI', got: %+v", problems)
	}
}
