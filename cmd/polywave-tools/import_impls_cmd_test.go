package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// writeIMPLDoc writes a minimal IMPL YAML document for testing.
func writeIMPLDoc(t *testing.T, dir, slug, title string, state protocol.ProtocolState, files []string) string {
	t.Helper()
	content := "title: " + title + "\n"
	content += "feature_slug: " + slug + "\n"
	content += "state: " + string(state) + "\n"
	content += "verdict: SUITABLE\n"
	content += "test_command: go test ./...\n"
	content += "lint_command: go vet ./...\n"
	content += "file_ownership:\n"
	for _, f := range files {
		content += "  - file: " + f + "\n    agent: A\n"
	}
	content += "interface_contracts: []\n"
	content += "waves:\n"
	content += "  - number: 1\n    agents:\n      - id: A\n        task: do stuff\n"

	path := filepath.Join(dir, "IMPL-"+slug+".yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeIMPLDoc: %v", err)
	}
	return path
}

func TestImportImplsDiscover(t *testing.T) {
	// Create temp repo with docs/IMPL/ containing 2 IMPL docs
	tmpDir := t.TempDir()
	implDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeIMPLDoc(t, implDir, "feature-a", "Feature A", protocol.StateReviewed, []string{"pkg/a/a.go"})
	writeIMPLDoc(t, implDir, "feature-b", "Feature B", protocol.StateComplete, []string{"pkg/b/b.go"})

	programPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")

	// Run the command
	cmd := newImportImplsCmd()
	cmd.SetArgs([]string{
		"--program", programPath,
		"--discover",
		"--repo-dir", tmpDir,
	})

	// Capture output
	outBuf := &strings.Builder{}
	cmd.SetOut(outBuf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("import-impls --discover failed: %v", err)
	}

	// Parse output
	var result protocol.ImportIMPLsData
	if err := json.Unmarshal([]byte(outBuf.String()), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\nOutput: %s", err, outBuf.String())
	}

	// Verify results
	if len(result.ImplsImported) != 2 {
		t.Errorf("expected 2 imported IMPLs, got %d", len(result.ImplsImported))
	}
	if len(result.ImplsDiscovered) != 2 {
		t.Errorf("expected 2 discovered paths, got %d", len(result.ImplsDiscovered))
	}
	if !result.Created {
		t.Error("expected created=true for new manifest")
	}

	// Verify manifest was written
	if _, err := os.Stat(programPath); err != nil {
		t.Errorf("program manifest not written: %v", err)
	}

	// Verify tier assignments exist for both
	if len(result.TierAssignments) != 2 {
		t.Errorf("expected 2 tier assignments, got %d", len(result.TierAssignments))
	}
}

func TestImportImplsFromFiles(t *testing.T) {
	tmpDir := t.TempDir()
	implDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatal(err)
	}

	pathA := writeIMPLDoc(t, implDir, "explicit-a", "Explicit A", protocol.StateReviewed, []string{"pkg/x/x.go"})
	pathB := writeIMPLDoc(t, implDir, "explicit-b", "Explicit B", protocol.StateComplete, []string{"pkg/y/y.go"})

	programPath := filepath.Join(tmpDir, "PROGRAM-explicit.yaml")

	cmd := newImportImplsCmd()
	cmd.SetArgs([]string{
		"--program", programPath,
		"--from-impls", pathA + "," + pathB,
		"--repo-dir", tmpDir,
	})

	outBuf := &strings.Builder{}
	cmd.SetOut(outBuf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("import-impls --from-impls failed: %v", err)
	}

	var result protocol.ImportIMPLsData
	if err := json.Unmarshal([]byte(outBuf.String()), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	if len(result.ImplsImported) != 2 {
		t.Errorf("expected 2 imported IMPLs, got %d", len(result.ImplsImported))
	}

	// --from-impls should NOT populate ImplsDiscovered
	if len(result.ImplsDiscovered) != 0 {
		t.Errorf("expected 0 discovered paths for --from-impls, got %d", len(result.ImplsDiscovered))
	}

	// Verify statuses mapped correctly using canonical IMPLStateToStatus.
	// StateReviewed maps to "pending"; StateComplete maps to "complete".
	statusMap := make(map[string]string)
	for _, imp := range result.ImplsImported {
		statusMap[imp.Slug] = imp.Status
	}
	if statusMap["explicit-a"] != "pending" {
		t.Errorf("expected explicit-a status=pending, got %s", statusMap["explicit-a"])
	}
	if statusMap["explicit-b"] != "complete" {
		t.Errorf("expected explicit-b status=complete, got %s", statusMap["explicit-b"])
	}
}

func TestImportImplsP1Conflict(t *testing.T) {
	// Two IMPL docs with overlapping file ownership
	tmpDir := t.TempDir()
	implDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Both IMPLs own the same file
	overlappingFile := "pkg/shared/shared.go"
	writeIMPLDoc(t, implDir, "overlap-a", "Overlap A", protocol.StateReviewed, []string{overlappingFile, "pkg/a/a.go"})
	writeIMPLDoc(t, implDir, "overlap-b", "Overlap B", protocol.StateReviewed, []string{overlappingFile, "pkg/b/b.go"})

	programPath := filepath.Join(tmpDir, "PROGRAM-overlap.yaml")

	cmd := newImportImplsCmd()
	cmd.SetArgs([]string{
		"--program", programPath,
		"--discover",
		"--repo-dir", tmpDir,
	})

	outBuf := &strings.Builder{}
	cmd.SetOut(outBuf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("import-impls failed: %v", err)
	}

	var result protocol.ImportIMPLsData
	if err := json.Unmarshal([]byte(outBuf.String()), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v", err)
	}

	// The overlapping IMPLs should be in different tiers (tier assignment logic)
	tierA := result.TierAssignments["overlap-a"]
	tierB := result.TierAssignments["overlap-b"]
	if tierA == tierB {
		t.Errorf("overlapping IMPLs should be in different tiers, both in tier %d", tierA)
	}

	// If they end up in the same tier, we'd also see P1 conflicts from ValidateProgramImportMode.
	// Since our tier assignment separates them, P1 conflicts may or may not appear
	// depending on whether ValidateProgramImportMode still flags cross-tier overlap.
	// The key assertion is the tier separation above.
}
