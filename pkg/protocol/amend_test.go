package protocol

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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

	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath: path,
		AddWave:      true,
	})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for complete manifest")
	}
	if len(res.Errors) == 0 || res.Errors[0].Code != result.CodeAmendBlocked {
		t.Errorf("expected AMEND_BLOCKED error code, got: %v", res.Errors)
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

	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath: path,
		AddWave:      true,
	})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for SAW:COMPLETE manifest")
	}
	if len(res.Errors) == 0 || res.Errors[0].Code != result.CodeAmendBlocked {
		t.Errorf("expected AMEND_BLOCKED error code, got: %v", res.Errors)
	}
}

// TestAmendImpl_AddWave verifies that AmendImpl appends a new wave with correct number.
func TestAmendImpl_AddWave(t *testing.T) {
	m := amendTestManifest("test-feature")
	path := saveAmendTestManifest(t, m)

	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath: path,
		AddWave:      true,
	})
	if !res.IsSuccess() {
		t.Fatalf("AmendImpl AddWave failed: %v", res.Errors)
	}

	data := res.GetData()
	if data.Operation != "add-wave" {
		t.Errorf("Operation = %q, want %q", data.Operation, "add-wave")
	}
	if data.NewWaveNumber != 2 {
		t.Errorf("NewWaveNumber = %d, want 2", data.NewWaveNumber)
	}

	// Verify manifest on disk was updated
	loaded, err := Load(context.Background(), path)
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

	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath: path,
		AddWave:      true,
	})
	if !res.IsSuccess() {
		t.Fatalf("AmendImpl AddWave on empty waves failed: %v", res.Errors)
	}

	data := res.GetData()
	if data.NewWaveNumber != 1 {
		t.Errorf("NewWaveNumber = %d, want 1", data.NewWaveNumber)
	}

	loaded, err := Load(context.Background(), path)
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

	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "A",
		WaveNum:       1,
		NewTask:       "New task",
	})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for complete agent")
	}
	if len(res.Errors) == 0 || res.Errors[0].Code != result.CodeAmendBlocked {
		t.Errorf("expected AMEND_BLOCKED error code, got: %v", res.Errors)
	}
}

// TestAmendImpl_RedirectAgent_NotFound verifies that redirect is blocked when agent doesn't exist.
func TestAmendImpl_RedirectAgent_NotFound(t *testing.T) {
	m := amendTestManifest("test-feature")
	path := saveAmendTestManifest(t, m)

	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "Z",
		WaveNum:       1,
		NewTask:       "New task",
	})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for nonexistent agent")
	}
	if len(res.Errors) == 0 || res.Errors[0].Code != result.CodeAmendBlocked {
		t.Errorf("expected AMEND_BLOCKED error code, got: %v", res.Errors)
	}
}

// TestAmendImpl_RedirectAgent_Success verifies successful redirect for an agent with no report.
func TestAmendImpl_RedirectAgent_Success(t *testing.T) {
	m := amendTestManifest("test-feature")
	// No completion report for agent A
	path := saveAmendTestManifest(t, m)

	newTask := "Updated task description"
	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "A",
		WaveNum:       1,
		NewTask:       newTask,
	})
	if !res.IsSuccess() {
		t.Fatalf("AmendImpl RedirectAgent failed: %v", res.Errors)
	}

	data := res.GetData()
	if data.Operation != "redirect-agent" {
		t.Errorf("Operation = %q, want %q", data.Operation, "redirect-agent")
	}
	if data.AgentID != "A" {
		t.Errorf("AgentID = %q, want %q", data.AgentID, "A")
	}

	// Verify task was updated on disk
	loaded, err := Load(context.Background(), path)
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

	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "A",
		WaveNum:       1,
		NewTask:       "New task",
	})
	if !res.IsSuccess() {
		t.Fatalf("AmendImpl RedirectAgent failed: %v", res.Errors)
	}

	// Verify partial report was cleared
	loaded, err := Load(context.Background(), path)
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

	res := AmendImpl(context.Background(), AmendImplOpts{
		ManifestPath:  path,
		RedirectAgent: true,
		AgentID:       "A",
		WaveNum:       1,
		NewTask:       "Updated task",
	})
	if !res.IsSuccess() {
		t.Fatalf("AmendImpl RedirectAgent (no base_commit) failed: %v", res.Errors)
	}

	data := res.GetData()
	// Should contain a warning about skipped commit check
	if len(data.Warnings) == 0 {
		t.Error("expected warning about skipped worktree commit check, got none")
	}
	found := false
	for _, w := range data.Warnings {
		if strings.Contains(w, "base_commit not set") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning containing 'base_commit not set', got: %v", data.Warnings)
	}
}
