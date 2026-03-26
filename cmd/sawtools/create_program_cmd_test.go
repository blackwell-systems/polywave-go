package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestIMPL creates a minimal IMPL YAML file in the temp repo structure.
func writeTestIMPLForConflicts(t *testing.T, repoDir, slug string, files []string) {
	t.Helper()
	implDir := filepath.Join(repoDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatal(err)
	}

	var foEntries []string
	for _, f := range files {
		foEntries = append(foEntries, "  - file: "+f+"\n    agent: A")
	}

	content := "feature_slug: " + slug + "\n" +
		"title: Test " + slug + "\n" +
		"state: reviewed\n" +
		"file_ownership:\n" + strings.Join(foEntries, "\n") + "\n" +
		"waves:\n" +
		"  - wave_number: 1\n" +
		"    agents:\n" +
		"      - id: A\n" +
		"        task: do stuff\n"

	path := filepath.Join(implDir, "IMPL-"+slug+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCreateProgramCmd_MissingFromImpls(t *testing.T) {
	cmd := newRootCmd()
	cmd.AddCommand(newCreateProgramCmd())

	cmd.SetArgs([]string{"create-program"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --from-impls not provided")
	}
	if !strings.Contains(err.Error(), "--from-impls is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateProgramCmd_SingleImpl(t *testing.T) {
	cmd := newRootCmd()
	cmd.AddCommand(newCreateProgramCmd())

	cmd.SetArgs([]string{"create-program", "--from-impls", "only-one"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when only 1 slug provided")
	}
	if !strings.Contains(err.Error(), "need at least 2") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckIMPLConflictsCmd_NoConflicts(t *testing.T) {
	repoDir := t.TempDir()
	writeTestIMPLForConflicts(t, repoDir, "alpha", []string{"pkg/alpha/a.go"})
	writeTestIMPLForConflicts(t, repoDir, "beta", []string{"pkg/beta/b.go"})

	cmd := newRootCmd()
	cmd.AddCommand(newCheckIMPLConflictsCmd())

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{}) // suppress stderr

	cmd.SetArgs([]string{"check-impl-conflicts", "--impls", "alpha", "--impls", "beta", "--repo-dir", repoDir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("expected no error for disjoint IMPLs, got: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, `"conflicts"`) {
		t.Fatalf("expected JSON output with conflicts field, got: %s", output)
	}
	// No conflicts means the conflicts array should be empty (no file entries)
	if strings.Contains(output, `"file":`) {
		t.Fatalf("expected no file conflicts, got: %s", output)
	}
}

func TestCheckIMPLConflictsCmd_WithConflicts(t *testing.T) {
	repoDir := t.TempDir()
	// Both IMPLs own the same file -> conflict
	writeTestIMPLForConflicts(t, repoDir, "gamma", []string{"pkg/shared/common.go"})
	writeTestIMPLForConflicts(t, repoDir, "delta", []string{"pkg/shared/common.go"})

	cmd := newRootCmd()
	cmd.AddCommand(newCheckIMPLConflictsCmd())

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{}) // suppress stderr

	cmd.SetArgs([]string{"check-impl-conflicts", "--impls", "gamma", "--impls", "delta", "--repo-dir", repoDir})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error (exit 1) when conflicts are found")
	}
	if !strings.Contains(err.Error(), "conflicts detected") {
		t.Fatalf("unexpected error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "pkg/shared/common.go") {
		t.Fatalf("expected conflict report to contain overlapping file, got: %s", output)
	}
	if !strings.Contains(output, "gamma") || !strings.Contains(output, "delta") {
		t.Fatalf("expected conflict report to contain both IMPL slugs, got: %s", output)
	}
}

// writeAbsIMPL writes a minimal IMPL YAML directly to an absolute path (cross-repo scenario).
func writeAbsIMPL(t *testing.T, absPath, slug string, files []string) {
	t.Helper()

	var foEntries []string
	for _, f := range files {
		foEntries = append(foEntries, "  - file: "+f+"\n    agent: A")
	}

	content := "feature_slug: " + slug + "\n" +
		"title: " + slug + "\n" +
		"state: reviewed\n" +
		"file_ownership:\n" + strings.Join(foEntries, "\n") + "\n" +
		"waves:\n" +
		"  - number: 1\n" +
		"    agents:\n" +
		"      - id: A\n" +
		"        task: do it\n"

	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCreateProgramCmd_AbsolutePaths(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	absPath1 := filepath.Join(dir1, "IMPL-cross-a.yaml")
	absPath2 := filepath.Join(dir2, "IMPL-cross-b.yaml")

	writeAbsIMPL(t, absPath1, "cross-a", []string{"pkg/cross-a/main.go"})
	writeAbsIMPL(t, absPath2, "cross-b", []string{"pkg/cross-b/main.go"})

	repoDir := t.TempDir()

	cmd := newRootCmd()
	cmd.AddCommand(newCreateProgramCmd())

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{}) // suppress stderr

	cmd.SetArgs([]string{
		"create-program",
		"--from-impls", absPath1,
		"--from-impls", absPath2,
		"--repo-dir", repoDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for absolute paths, got: %v", err)
	}

	var result struct {
		ManifestPath string `json:"manifest_path"`
		Manifest     struct {
			Impls []struct {
				Slug    string `json:"slug"`
				AbsPath string `json:"abs_path"`
			} `json:"impls"`
		} `json:"manifest"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	if result.ManifestPath == "" {
		t.Fatal("expected non-empty manifest_path in output")
	}
	if len(result.Manifest.Impls) != 2 {
		t.Fatalf("expected 2 impls in manifest, got %d", len(result.Manifest.Impls))
	}
	// Verify AbsPath is recorded for cross-repo entries
	for _, impl := range result.Manifest.Impls {
		if impl.AbsPath == "" {
			t.Errorf("expected AbsPath to be set for cross-repo impl %q, got empty", impl.Slug)
		}
	}
}

func TestCreateProgramCmd_MixedSlugAndPath(t *testing.T) {
	repoDir := t.TempDir()
	// Slug-based IMPL in the normal docs/IMPL structure
	writeTestIMPLForConflicts(t, repoDir, "slug-a", []string{"pkg/slug-a/main.go"})

	// Path-based IMPL at a standalone absolute path (simulates cross-repo)
	dir2 := t.TempDir()
	absPath2 := filepath.Join(dir2, "IMPL-path-b.yaml")
	writeAbsIMPL(t, absPath2, "path-b", []string{"pkg/path-b/main.go"})

	cmd := newRootCmd()
	cmd.AddCommand(newCreateProgramCmd())

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{}) // suppress stderr

	cmd.SetArgs([]string{
		"create-program",
		"--from-impls", "slug-a",
		"--from-impls", absPath2,
		"--repo-dir", repoDir,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected no error for mixed slug+path, got: %v", err)
	}

	var result struct {
		Manifest struct {
			Impls []interface{} `json:"impls"`
		} `json:"manifest"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	if len(result.Manifest.Impls) != 2 {
		t.Fatalf("expected 2 impls in manifest, got %d", len(result.Manifest.Impls))
	}
}
