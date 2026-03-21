package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// writeGeneratorTestIMPL writes a minimal IMPL YAML doc to the given directory.
func writeGeneratorTestIMPL(t *testing.T, repoPath, slug, title string, state ProtocolState, files []string, contracts []string) {
	t.Helper()
	dir := filepath.Join(repoPath, "docs", "IMPL")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	var fo []FileOwnership
	for _, f := range files {
		fo = append(fo, FileOwnership{File: f, Agent: "A"})
	}

	var ic []InterfaceContract
	for _, c := range contracts {
		ic = append(ic, InterfaceContract{Name: c, Definition: "stub", Location: "stub"})
	}

	manifest := &IMPLManifest{
		Title:              title,
		FeatureSlug:        slug,
		State:              state,
		Verdict:            "SUITABLE",
		FileOwnership:      fo,
		InterfaceContracts: ic,
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}, {ID: "B"}}},
		},
		CompletionReports: make(map[string]CompletionReport),
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "IMPL-"+slug+".yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateProgramFromIMPLs_Basic(t *testing.T) {
	repoPath := t.TempDir()

	writeGeneratorTestIMPL(t, repoPath, "feature-a", "Feature A", StateReviewed,
		[]string{"pkg/a/file1.go", "pkg/a/file2.go"}, []string{"InterfaceA"})
	writeGeneratorTestIMPL(t, repoPath, "feature-b", "Feature B", StateReviewed,
		[]string{"pkg/b/file1.go"}, []string{"InterfaceB"})

	result, err := GenerateProgramFromIMPLs(GenerateProgramOpts{
		ImplSlugs:   []string{"feature-a", "feature-b"},
		RepoPath:    repoPath,
		ProgramSlug: "test-program",
		Title:       "Test Program",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Manifest.State != ProgramStateReviewed {
		t.Errorf("expected state REVIEWED, got %s", result.Manifest.State)
	}

	if len(result.Manifest.Impls) != 2 {
		t.Fatalf("expected 2 IMPLs, got %d", len(result.Manifest.Impls))
	}

	// No overlapping files -> all tier 1 -> 1 tier
	if len(result.Manifest.Tiers) != 1 {
		t.Errorf("expected 1 tier (no overlap), got %d", len(result.Manifest.Tiers))
	}

	if result.Manifest.Completion.ImplsTotal != 2 {
		t.Errorf("expected ImplsTotal=2, got %d", result.Manifest.Completion.ImplsTotal)
	}
	if result.Manifest.Completion.TiersTotal != 1 {
		t.Errorf("expected TiersTotal=1, got %d", result.Manifest.Completion.TiersTotal)
	}
	if result.Manifest.Completion.TotalAgents != 4 {
		t.Errorf("expected TotalAgents=4 (2 per IMPL), got %d", result.Manifest.Completion.TotalAgents)
	}
	if result.Manifest.Completion.TotalWaves != 2 {
		t.Errorf("expected TotalWaves=2 (1 per IMPL), got %d", result.Manifest.Completion.TotalWaves)
	}
}

func TestGenerateProgramFromIMPLs_WithOverlap(t *testing.T) {
	repoPath := t.TempDir()

	// Both IMPLs share "pkg/shared/file.go"
	writeGeneratorTestIMPL(t, repoPath, "impl-x", "IMPL X", StateReviewed,
		[]string{"pkg/shared/file.go", "pkg/x/own.go"}, nil)
	writeGeneratorTestIMPL(t, repoPath, "impl-y", "IMPL Y", StateReviewed,
		[]string{"pkg/shared/file.go", "pkg/y/own.go"}, nil)

	result, err := GenerateProgramFromIMPLs(GenerateProgramOpts{
		ImplSlugs:   []string{"impl-x", "impl-y"},
		RepoPath:    repoPath,
		ProgramSlug: "overlap-test",
		Title:       "Overlap Test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Overlapping files should result in 2 tiers
	if len(result.Manifest.Tiers) != 2 {
		t.Errorf("expected 2 tiers (overlap), got %d", len(result.Manifest.Tiers))
	}

	// The second-tier IMPL should have DependsOn set
	for _, impl := range result.Manifest.Impls {
		if impl.Tier == 2 {
			if len(impl.DependsOn) == 0 {
				t.Errorf("tier-2 IMPL %q should have DependsOn set", impl.Slug)
			}
		}
	}

	// Conflict report should contain the shared file
	if len(result.ConflictReport.Conflicts) == 0 {
		t.Error("expected at least one conflict for shared file")
	}
}

func TestGenerateProgramFromIMPLs_AutoSlug(t *testing.T) {
	repoPath := t.TempDir()

	writeGeneratorTestIMPL(t, repoPath, "alpha", "Alpha", StateReviewed, []string{"a.go"}, nil)
	writeGeneratorTestIMPL(t, repoPath, "beta", "Beta", StateReviewed, []string{"b.go"}, nil)

	result, err := GenerateProgramFromIMPLs(GenerateProgramOpts{
		ImplSlugs: []string{"alpha", "beta"},
		RepoPath:  repoPath,
		// ProgramSlug and Title intentionally empty
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSlug := "alpha-and-beta"
	if result.Manifest.ProgramSlug != expectedSlug {
		t.Errorf("expected auto-derived slug %q, got %q", expectedSlug, result.Manifest.ProgramSlug)
	}

	if !strings.HasPrefix(result.Manifest.Title, "Auto-generated PROGRAM: ") {
		t.Errorf("expected auto-derived title prefix, got %q", result.Manifest.Title)
	}

	// Test with >3 slugs
	writeGeneratorTestIMPL(t, repoPath, "gamma", "Gamma", StateReviewed, []string{"c.go"}, nil)
	writeGeneratorTestIMPL(t, repoPath, "delta", "Delta", StateReviewed, []string{"d.go"}, nil)

	result2, err := GenerateProgramFromIMPLs(GenerateProgramOpts{
		ImplSlugs: []string{"alpha", "beta", "gamma", "delta"},
		RepoPath:  repoPath,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSlug2 := "auto-program-alpha"
	if result2.Manifest.ProgramSlug != expectedSlug2 {
		t.Errorf("expected auto-derived slug %q for >3 slugs, got %q", expectedSlug2, result2.Manifest.ProgramSlug)
	}
}

func TestGenerateProgramFromIMPLs_WritesToDisk(t *testing.T) {
	repoPath := t.TempDir()

	writeGeneratorTestIMPL(t, repoPath, "disk-test", "Disk Test", StateReviewed, []string{"x.go"}, nil)

	result, err := GenerateProgramFromIMPLs(GenerateProgramOpts{
		ImplSlugs:   []string{"disk-test"},
		RepoPath:    repoPath,
		ProgramSlug: "disk-test-program",
		Title:       "Disk Test Program",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := filepath.Join(repoPath, "docs", "PROGRAM-disk-test-program.yaml")
	if result.ManifestPath != expectedPath {
		t.Errorf("expected manifest path %q, got %q", expectedPath, result.ManifestPath)
	}

	// Verify file exists and is valid YAML
	data, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatalf("failed to read written manifest: %v", err)
	}

	var loaded PROGRAMManifest
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("written manifest is invalid YAML: %v", err)
	}

	if loaded.ProgramSlug != "disk-test-program" {
		t.Errorf("loaded slug mismatch: expected %q, got %q", "disk-test-program", loaded.ProgramSlug)
	}
	if loaded.State != ProgramStateReviewed {
		t.Errorf("loaded state mismatch: expected REVIEWED, got %s", loaded.State)
	}
}

func TestGenerateProgramFromIMPLs_MissingIMPL(t *testing.T) {
	repoPath := t.TempDir()

	// Create docs/IMPL dir but no IMPL files
	if err := os.MkdirAll(filepath.Join(repoPath, "docs", "IMPL"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := GenerateProgramFromIMPLs(GenerateProgramOpts{
		ImplSlugs:   []string{"nonexistent-slug"},
		RepoPath:    repoPath,
		ProgramSlug: "test",
	})
	if err == nil {
		t.Fatal("expected error for missing IMPL, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-slug") {
		t.Errorf("error should mention the missing slug, got: %v", err)
	}
}
