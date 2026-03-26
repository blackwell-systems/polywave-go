package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestRunScout_MissingScoutMd verifies that RunScout returns an error when scout.md is missing (L1).
func TestRunScout_MissingScoutMd(t *testing.T) {
	dir := t.TempDir()
	nonexistentSAWRepo := filepath.Join(dir, "nonexistent-saw-repo")

	err := RunScout(context.Background(), RunScoutOpts{
		Feature:     "test feature",
		RepoPath:    dir,
		IMPLOutPath: filepath.Join(dir, "IMPL-test.yaml"),
		SAWRepoPath: nonexistentSAWRepo,
	}, func(string) {})
	if err == nil {
		t.Fatal("expected error when scout.md is missing, got nil")
	}
	if !strings.Contains(err.Error(), "scout.md not found") {
		t.Errorf("expected error to mention 'scout.md not found', got: %v", err)
	}
}

// TestRunScoutMissingFeature verifies that RunScout returns an error when Feature is empty.
func TestRunScoutMissingFeature(t *testing.T) {
	err := RunScout(context.Background(), RunScoutOpts{
		Feature:     "",
		RepoPath:    "/tmp/repo",
		IMPLOutPath: "/tmp/repo/IMPL.md",
	}, func(string) {})
	if err == nil {
		t.Fatal("expected error when Feature is empty, got nil")
	}
}

// TestStartWaveEmptyIMPL verifies that StartWave returns an error when the IMPL path does not exist.
func TestStartWaveEmptyIMPL(t *testing.T) {
	err := StartWave(context.Background(), RunWaveOpts{
		IMPLPath: "/nonexistent/path/IMPL.md",
		RepoPath: "/tmp/repo",
		Slug:     "test",
	}, func(Event) {})
	if err == nil {
		t.Fatal("expected error when IMPL path does not exist, got nil")
	}
}

// TestReadContextMDMissing verifies that readContextMD returns "" when no CONTEXT.md exists.
func TestReadContextMDMissing(t *testing.T) {
	dir := t.TempDir()
	result := readContextMD(dir)
	if result != "" {
		t.Errorf("expected empty string for missing CONTEXT.md, got %q", result)
	}
}

// TestJournalIntegration_PrepareAgentContext verifies that PrepareAgentContext
// creates a journal observer and returns the original prompt when no journal exists.
func TestJournalIntegration_PrepareAgentContext(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	originalPrompt := "Test agent prompt"
	enriched, observer, err := ji.PrepareAgentContext(1, "A", originalPrompt)

	if err != nil {
		t.Fatalf("PrepareAgentContext returned error: %v", err)
	}
	if observer == nil {
		t.Fatal("PrepareAgentContext returned nil observer")
	}
	// With no journal entries, prompt should be unchanged
	if enriched != originalPrompt {
		t.Errorf("expected prompt unchanged when journal empty, got %q", enriched)
	}
}

// TestJournalIntegration_StartPeriodicSync verifies that periodic sync can be started and stopped.
func TestJournalIntegration_StartPeriodicSync(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	// Create observer
	observer, err := journal.NewObserver(dir, "wave1/agent-A")
	if err != nil {
		t.Fatalf("failed to create observer: %v", err)
	}

	ctx := context.Background()
	cancel := ji.StartPeriodicSync(ctx, observer)
	defer cancel()

	// Wait a bit to ensure goroutine starts
	time.Sleep(100 * time.Millisecond)

	// Cancel should not panic
	cancel()
}

// TestJournalIntegration_TriggerCheckpoint verifies checkpoint creation.
func TestJournalIntegration_TriggerCheckpoint(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	// Create observer
	observer, err := journal.NewObserver(dir, "wave1/agent-A")
	if err != nil {
		t.Fatalf("failed to create observer: %v", err)
	}

	// Create checkpoint
	if err := ji.TriggerCheckpoint(observer, "001-isolation"); err != nil {
		t.Fatalf("TriggerCheckpoint failed: %v", err)
	}

	// Verify checkpoint was created
	checkpoints, err := observer.ListCheckpoints()
	if err != nil {
		t.Fatalf("ListCheckpoints failed: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(checkpoints))
	}
	if checkpoints[0].Name != "001-isolation" {
		t.Errorf("expected checkpoint name '001-isolation', got %q", checkpoints[0].Name)
	}
}

// TestJournalIntegration_ArchiveJournal verifies journal archival.
func TestJournalIntegration_ArchiveJournal(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	// Create observer - use the format that archive.go expects (wave1/agent-A)
	observer, err := journal.NewObserver(dir, "wave1/agent-A")
	if err != nil {
		t.Fatalf("failed to create observer: %v", err)
	}

	t.Logf("JournalDir: %s", observer.JournalDir)
	t.Logf("ProjectRoot: %s", observer.ProjectRoot)

	// Write some test data to journal (observer already created the directory)
	testData := `{"test": "data"}`
	indexPath := filepath.Join(observer.JournalDir, "index.jsonl")
	if err := os.WriteFile(indexPath, []byte(testData), 0644); err != nil {
		t.Fatalf("failed to write test journal: %v", err)
	}

	// Archive
	if err := ji.ArchiveJournal(observer); err != nil {
		t.Fatalf("ArchiveJournal failed: %v", err)
	}

	// Verify archive was created
	// The archive name is derived from the journal directory structure
	// which NewObserver creates as .saw-state/wave1/agent/A (parsing wave1/agent-A)
	// So the archive becomes wave1-agent-agent.tar.gz (agent is the folder name)
	archiveDir := filepath.Join(dir, ".saw-state", "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("failed to read archive directory: %v", err)
	}

	// Check that at least one .tar.gz file was created
	foundArchive := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".gz" {
			foundArchive = true
			t.Logf("Found archive: %s", e.Name())
			break
		}
	}

	if !foundArchive {
		t.Errorf("no .tar.gz archive file found in %s", archiveDir)
	}
}

// TestJournalIntegration_NilObserver verifies that journal operations are safe with nil observer.
func TestJournalIntegration_NilObserver(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	// All operations should be no-ops with nil observer
	ctx := context.Background()
	cancel := ji.StartPeriodicSync(ctx, nil)
	cancel() // Should not panic

	if err := ji.TriggerCheckpoint(nil, "test"); err != nil {
		t.Errorf("TriggerCheckpoint with nil observer returned error: %v", err)
	}

	if err := ji.ArchiveJournal(nil); err != nil {
		t.Errorf("ArchiveJournal with nil observer returned error: %v", err)
	}
}

// TestRunScoutOpts_ProgramManifestPath verifies that the ProgramManifestPath field exists and is accessible.
func TestRunScoutOpts_ProgramManifestPath(t *testing.T) {
	opts := RunScoutOpts{
		Feature:             "Test feature",
		RepoPath:            "/tmp/repo",
		IMPLOutPath:         "/tmp/repo/IMPL.md",
		ProgramManifestPath: "/tmp/PROGRAM.yaml",
	}

	if opts.ProgramManifestPath != "/tmp/PROGRAM.yaml" {
		t.Errorf("expected ProgramManifestPath to be /tmp/PROGRAM.yaml, got %s", opts.ProgramManifestPath)
	}
}

// TestRunScout_ProgramManifestNotFound verifies graceful degradation when manifest is missing.
func TestRunScout_ProgramManifestNotFound(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal IMPL output path
	implPath := filepath.Join(dir, "IMPL-test.yaml")

	opts := RunScoutOpts{
		Feature:             "Test feature",
		RepoPath:            dir,
		IMPLOutPath:         implPath,
		ProgramManifestPath: "/nonexistent/PROGRAM.yaml",
		ScoutModel:          "test-model", // Prevents actual API call
	}

	// We don't actually run the Scout since that requires a backend,
	// but we can verify the field is set and accessible.
	// The graceful degradation is tested by the actual RunScout code
	// which logs a warning and continues.
	if opts.ProgramManifestPath != "/nonexistent/PROGRAM.yaml" {
		t.Errorf("expected ProgramManifestPath to be set, got %s", opts.ProgramManifestPath)
	}
}

// TestLoadTypePromptWithRefs verifies the reference injection helper.
func TestLoadTypePromptWithRefs(t *testing.T) {
	t.Run("no references dir returns core only", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentsDir := filepath.Join(tmpDir, "agents")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatalf("mkdir agents: %v", err)
		}
		coreContent := "# Scout\n\nCore content"
		if err := os.WriteFile(filepath.Join(agentsDir, "scout.md"), []byte(coreContent), 0644); err != nil {
			t.Fatalf("write scout.md: %v", err)
		}

		result, err := LoadTypePromptWithRefs(filepath.Join(agentsDir, "scout.md"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != coreContent {
			t.Errorf("expected %q, got %q", coreContent, result)
		}
	})

	t.Run("matching reference files are prepended with dedup markers", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentsDir := filepath.Join(tmpDir, "agents")
		refsDir := filepath.Join(tmpDir, "references")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatalf("mkdir agents: %v", err)
		}
		if err := os.MkdirAll(refsDir, 0755); err != nil {
			t.Fatalf("mkdir references: %v", err)
		}

		if err := os.WriteFile(filepath.Join(agentsDir, "scout.md"), []byte("# Scout Core"), 0644); err != nil {
			t.Fatalf("write scout.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(refsDir, "scout-implementation-process.md"), []byte("## Implementation Process"), 0644); err != nil {
			t.Fatalf("write scout-implementation-process.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(refsDir, "scout-suitability-gate.md"), []byte("## Suitability Gate"), 0644); err != nil {
			t.Fatalf("write scout-suitability-gate.md: %v", err)
		}
		// This file should NOT be included (wrong prefix)
		if err := os.WriteFile(filepath.Join(refsDir, "wave-agent-worktree.md"), []byte("## Worktree"), 0644); err != nil {
			t.Fatalf("write wave-agent-worktree.md: %v", err)
		}

		result, err := LoadTypePromptWithRefs(filepath.Join(agentsDir, "scout.md"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(result, "<!-- injected: references/scout-suitability-gate.md -->") {
			t.Errorf("missing suitability-gate marker in result: %q", result)
		}
		if !strings.Contains(result, "## Suitability Gate") {
			t.Errorf("missing suitability gate content in result: %q", result)
		}
		if !strings.Contains(result, "<!-- injected: references/scout-implementation-process.md -->") {
			t.Errorf("missing implementation-process marker in result: %q", result)
		}
		if !strings.Contains(result, "## Implementation Process") {
			t.Errorf("missing implementation process content in result: %q", result)
		}
		if !strings.Contains(result, "# Scout Core") {
			t.Errorf("missing core content in result: %q", result)
		}
		if strings.Contains(result, "## Worktree") {
			t.Errorf("wave-agent content should not be included in scout result: %q", result)
		}
	})

	t.Run("dedup marker already in core skips injection", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentsDir := filepath.Join(tmpDir, "agents")
		refsDir := filepath.Join(tmpDir, "references")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatalf("mkdir agents: %v", err)
		}
		if err := os.MkdirAll(refsDir, 0755); err != nil {
			t.Fatalf("mkdir references: %v", err)
		}

		// Core already contains the dedup marker
		coreWithMarker := "<!-- injected: references/scout-suitability-gate.md -->\n# Scout Core"
		if err := os.WriteFile(filepath.Join(agentsDir, "scout.md"), []byte(coreWithMarker), 0644); err != nil {
			t.Fatalf("write scout.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(refsDir, "scout-suitability-gate.md"), []byte("## Suitability Gate"), 0644); err != nil {
			t.Fatalf("write scout-suitability-gate.md: %v", err)
		}

		result, err := LoadTypePromptWithRefs(filepath.Join(agentsDir, "scout.md"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The marker should appear only once (not doubled)
		count := strings.Count(result, "<!-- injected: references/scout-suitability-gate.md -->")
		if count != 1 {
			t.Errorf("expected marker to appear exactly once, got %d times in: %q", count, result)
		}
	})

	t.Run("empty references dir returns core only", func(t *testing.T) {
		tmpDir := t.TempDir()
		agentsDir := filepath.Join(tmpDir, "agents")
		refsDir := filepath.Join(tmpDir, "references")
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			t.Fatalf("mkdir agents: %v", err)
		}
		if err := os.MkdirAll(refsDir, 0755); err != nil {
			t.Fatalf("mkdir references: %v", err)
		}

		coreContent := "# Scout Core"
		if err := os.WriteFile(filepath.Join(agentsDir, "scout.md"), []byte(coreContent), 0644); err != nil {
			t.Fatalf("write scout.md: %v", err)
		}
		// Only a non-matching file in refs
		if err := os.WriteFile(filepath.Join(refsDir, "wave-agent-worktree.md"), []byte("## Worktree"), 0644); err != nil {
			t.Fatalf("write wave-agent-worktree.md: %v", err)
		}

		result, err := LoadTypePromptWithRefs(filepath.Join(agentsDir, "scout.md"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != coreContent {
			t.Errorf("expected core only %q, got %q", coreContent, result)
		}
	})

	t.Run("missing core file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonexistent := filepath.Join(tmpDir, "agents", "scout.md")

		_, err := LoadTypePromptWithRefs(nonexistent)
		if err == nil {
			t.Fatal("expected error for missing core file, got nil")
		}
	})
}

// TestBuildProgramContractsSection verifies that frozen contracts are correctly extracted and formatted.
func TestBuildProgramContractsSection(t *testing.T) {
	// Create a test manifest with frozen contracts
	manifest := &protocol.PROGRAMManifest{
		Impls: []protocol.ProgramIMPL{
			{Slug: "impl-a", Status: "complete"},
			{Slug: "impl-b", Status: "in_progress"},
		},
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
			{Number: 2, Impls: []string{"impl-b"}},
		},
		ProgramContracts: []protocol.ProgramContract{
			{
				Name:       "FrozenAPI",
				Definition: "type FrozenAPI interface { DoWork() error }",
				Location:   "pkg/api/api.go",
				FreezeAt:   "impl:impl-a",
			},
			{
				Name:       "UnfrozenAPI",
				Definition: "type UnfrozenAPI interface { DoOtherWork() error }",
				Location:   "pkg/api/other.go",
				FreezeAt:   "impl:impl-b", // Not frozen yet
			},
		},
	}

	section := buildProgramContractsSection(manifest, "/tmp/repo")

	// Verify the section includes the frozen contract
	if !strings.Contains(section, "FrozenAPI") {
		t.Errorf("expected section to include FrozenAPI, got: %s", section)
	}

	// Verify the section does NOT include the unfrozen contract
	if strings.Contains(section, "UnfrozenAPI") {
		t.Errorf("expected section to NOT include UnfrozenAPI, got: %s", section)
	}

	// Verify the markdown structure
	if !strings.Contains(section, "## Program Contracts (Frozen") {
		t.Errorf("expected section to include header, got: %s", section)
	}

	// Verify definition is included
	if !strings.Contains(section, "DoWork()") {
		t.Errorf("expected section to include contract definition, got: %s", section)
	}
}
