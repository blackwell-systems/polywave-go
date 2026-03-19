package protocol

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// amendTestManifest returns a minimal valid IMPLManifest for amend tests.
func amendTestManifest(slug string) *IMPLManifest {
	return &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: slug,
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1, Action: "new"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement foo", Files: []string{"pkg/foo.go"}},
				},
			},
		},
		CompletionReports: make(map[string]CompletionReport),
	}
}

// saveAmendTestManifest saves a manifest to a temp dir and returns the path.
func saveAmendTestManifest(t *testing.T, m *IMPLManifest) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "IMPL-test.yaml")
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal test manifest: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}
	return path
}

// TestAmendImpl_RejectsComplete verifies that AmendImpl blocks when completion_date is set.
func TestAmendImpl_RejectsComplete(t *testing.T) {
	m := amendTestManifest("test-feature")
	m.CompletionDate = "2026-01-01"
	path := saveAmendTestManifest(t, m)

	_, err := AmendImpl(AmendImplOpts{
		ManifestPath: path,
		AddWave:      true,
	})
	if err == nil {
		t.Fatal("expected error for complete manifest, got nil")
	}
	if !errors.Is(err, ErrAmendBlocked) {
		t.Errorf("expected ErrAmendBlocked, got: %v", err)
	}
}

// TestAmendImpl_RejectsComplete_SAWMarker verifies that AmendImpl blocks when SAW:COMPLETE marker is present.
func TestAmendImpl_RejectsComplete_SAWMarker(t *testing.T) {
	m := amendTestManifest("test-feature")
	dir := t.TempDir()
	path := filepath.Join(dir, "IMPL-test.yaml")

	// Write manifest with SAW:COMPLETE marker in raw bytes
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	raw := string(data) + "\n# SAW:COMPLETE\n"
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	_, err = AmendImpl(AmendImplOpts{
		ManifestPath: path,
		AddWave:      true,
	})
	if err == nil {
		t.Fatal("expected error for SAW:COMPLETE manifest, got nil")
	}
	if !errors.Is(err, ErrAmendBlocked) {
		t.Errorf("expected ErrAmendBlocked, got: %v", err)
	}
}

// TestAmendImpl_AddWave verifies that AmendImpl appends a new wave with correct number.
func TestAmendImpl_AddWave(t *testing.T) {
	m := amendTestManifest("test-feature")
	path := saveAmendTestManifest(t, m)

	result, err := AmendImpl(AmendImplOpts{
		ManifestPath: path,
		AddWave:      true,
	})
	if err != nil {
		t.Fatalf("AmendImpl AddWave failed: %v", err)
	}

	if result.Operation != "add-wave" {
		t.Errorf("Operation = %q, want %q", result.Operation, "add-wave")
	}
	if result.NewWaveNumber != 2 {
		t.Errorf("NewWaveNumber = %d, want 2", result.NewWaveNumber)
	}

	// Verify manifest on disk was updated
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load updated manifest: %v", err)
	}
	if len(loaded.Waves) != 2 {
		t.Errorf("waves count = %d, want 2", len(loaded.Waves))
	}
	if loaded.Waves[1].Number != 2 {
		t.Errorf("new wave number = %d, want 2", loaded.Waves[1].Number)
	}
}

// TestAmendImpl_AddWave_EmptyWaves verifies that AddWave on a manifest with no waves adds wave 1.
func TestAmendImpl_AddWave_EmptyWaves(t *testing.T) {
	m := &IMPLManifest{
		Title:             "Test Feature",
		FeatureSlug:       "test-feature",
		Verdict:           "SUITABLE",
		TestCommand:       "go test ./...",
		LintCommand:       "go vet ./...",
		Waves:             []Wave{},
		CompletionReports: make(map[string]CompletionReport),
	}
	path := saveAmendTestManifest(t, m)

	result, err := AmendImpl(AmendImplOpts{
		ManifestPath: path,
		AddWave:      true,
	})
	if err != nil {
		t.Fatalf("AmendImpl AddWave on empty waves failed: %v", err)
	}

	if result.NewWaveNumber != 1 {
		t.Errorf("NewWaveNumber = %d, want 1", result.NewWaveNumber)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load updated manifest: %v", err)
	}
	if len(loaded.Waves) != 1 {
		t.Errorf("waves count = %d, want 1", len(loaded.Waves))
	}
	if loaded.Waves[0].Number != 1 {
		t.Errorf("new wave number = %d, want 1", loaded.Waves[0].Number)
	}
}

// TestAmendImpl_RedirectAgent_RejectsComplete verifies that redirect is blocked for a complete agent.
func TestAmendImpl_RedirectAgent_RejectsComplete(t *testing.T) {
	m := amendTestManifest("test-feature")
	m.CompletionReports["A"] = CompletionReport{
		Status: "complete",
		Commit: "abc123",
	}
	path := saveAmendTestManifest(t, m)

	_, err := AmendImpl(AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "A",
		WaveNum:       1,
		NewTask:       "New task",
	})
	if err == nil {
		t.Fatal("expected error for complete agent, got nil")
	}
	if !errors.Is(err, ErrAmendBlocked) {
		t.Errorf("expected ErrAmendBlocked, got: %v", err)
	}
}

// TestAmendImpl_RedirectAgent_NotFound verifies that redirect is blocked when agent doesn't exist.
func TestAmendImpl_RedirectAgent_NotFound(t *testing.T) {
	m := amendTestManifest("test-feature")
	path := saveAmendTestManifest(t, m)

	_, err := AmendImpl(AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "Z",
		WaveNum:       1,
		NewTask:       "New task",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent agent, got nil")
	}
	if !errors.Is(err, ErrAmendBlocked) {
		t.Errorf("expected ErrAmendBlocked, got: %v", err)
	}
}

// TestAmendImpl_RedirectAgent_Success verifies successful redirect for an agent with no report.
func TestAmendImpl_RedirectAgent_Success(t *testing.T) {
	m := amendTestManifest("test-feature")
	// No completion report for agent A
	path := saveAmendTestManifest(t, m)

	newTask := "Updated task description"
	result, err := AmendImpl(AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "A",
		WaveNum:       1,
		NewTask:       newTask,
	})
	if err != nil {
		t.Fatalf("AmendImpl RedirectAgent failed: %v", err)
	}

	if result.Operation != "redirect-agent" {
		t.Errorf("Operation = %q, want %q", result.Operation, "redirect-agent")
	}
	if result.AgentID != "A" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "A")
	}

	// Verify task was updated on disk
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load updated manifest: %v", err)
	}
	if loaded.Waves[0].Agents[0].Task != newTask {
		t.Errorf("agent task = %q, want %q", loaded.Waves[0].Agents[0].Task, newTask)
	}
}

// TestAmendImpl_RedirectAgent_ClearsPartialReport verifies that a partial report is cleared on redirect.
func TestAmendImpl_RedirectAgent_ClearsPartialReport(t *testing.T) {
	m := amendTestManifest("test-feature")
	m.CompletionReports["A"] = CompletionReport{
		Status: "partial",
		Commit: "abc123",
	}
	path := saveAmendTestManifest(t, m)

	_, err := AmendImpl(AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "A",
		WaveNum:       1,
		NewTask:       "New task",
	})
	if err != nil {
		t.Fatalf("AmendImpl RedirectAgent failed: %v", err)
	}

	// Verify partial report was cleared
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("failed to load updated manifest: %v", err)
	}
	if _, ok := loaded.CompletionReports["A"]; ok {
		t.Error("expected partial completion report to be cleared after redirect")
	}
}

// TestAmendImpl_RedirectAgent_NoBaseCommit verifies that redirect proceeds with a warning when base_commit is empty.
func TestAmendImpl_RedirectAgent_NoBaseCommit(t *testing.T) {
	m := amendTestManifest("test-feature")
	// Wave has no base_commit (default)
	path := saveAmendTestManifest(t, m)

	result, err := AmendImpl(AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "A",
		WaveNum:       1,
		NewTask:       "Updated task",
	})
	if err != nil {
		t.Fatalf("AmendImpl RedirectAgent (no base_commit) failed: %v", err)
	}

	// Should contain a warning about skipped commit check
	if len(result.Warnings) == 0 {
		t.Error("expected warning about skipped worktree commit check, got none")
	}
	found := false
	for _, w := range result.Warnings {
		if contains(w, "base_commit not set") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning containing 'base_commit not set', got: %v", result.Warnings)
	}
}
