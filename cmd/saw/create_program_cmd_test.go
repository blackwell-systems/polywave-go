package main

import (
	"bytes"
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
