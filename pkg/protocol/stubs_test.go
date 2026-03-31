package protocol

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanStubs_NoStubs(t *testing.T) {
	// Create temp file with clean code
	tmpDir := t.TempDir()
	cleanFile := filepath.Join(tmpDir, "clean.go")
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello, world!")
}
`
	if err := os.WriteFile(cleanFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := ScanStubs([]string{cleanFile})
	if !result.IsSuccess() {
		t.Fatalf("ScanStubs failed: %v", result.Errors)
	}

	data := result.GetData()
	if len(data.Hits) != 0 {
		t.Errorf("expected 0 hits, got %d", len(data.Hits))
	}
}

func TestScanStubs_HasStubs(t *testing.T) {
	// Create temp file with various stub patterns
	tmpDir := t.TempDir()
	stubFile := filepath.Join(tmpDir, "stubs.go")
	content := `package main

// TODO: implement this function
func DoSomething() {
	panic("not implemented")
}

// FIXME: bug in logic
func DoAnother() {
	// stub implementation
	return
}

// XXX: refactor needed
// HACK: temporary workaround
func DoThird() {
	// placeholder
}
`
	if err := os.WriteFile(stubFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := ScanStubs([]string{stubFile})
	if !result.IsSuccess() {
		t.Fatalf("ScanStubs failed: %v", result.Errors)
	}

	data := result.GetData()
	// We expect multiple hits
	expectedPatterns := []string{"TODO", "panic(\"not implemented\")", "FIXME", "// stub", "XXX", "HACK", "// placeholder"}
	if len(data.Hits) < len(expectedPatterns) {
		t.Errorf("expected at least %d hits, got %d", len(expectedPatterns), len(data.Hits))
	}

	// Verify that line numbers are correct (should be positive)
	for _, hit := range data.Hits {
		if hit.Line <= 0 {
			t.Errorf("invalid line number: %d", hit.Line)
		}
		if hit.File != stubFile {
			t.Errorf("expected file %s, got %s", stubFile, hit.File)
		}
		if hit.Pattern == "" {
			t.Errorf("pattern should not be empty")
		}
		if hit.Context == "" {
			t.Errorf("context should not be empty")
		}
	}

	// Verify TODO is found
	found := false
	for _, hit := range data.Hits {
		if hit.Pattern == "TODO" && hit.Line == 3 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find TODO at line 3")
	}
}

func TestScanStubs_FileNotFound(t *testing.T) {
	// Non-existent file should be skipped without error
	result := ScanStubs([]string{"/nonexistent/file.go"})
	if !result.IsSuccess() {
		t.Fatalf("ScanStubs should not error on missing file: %v", result.Errors)
	}

	data := result.GetData()
	if len(data.Hits) != 0 {
		t.Errorf("expected 0 hits for missing file, got %d", len(data.Hits))
	}
}

func TestScanStubs_EmptyFiles(t *testing.T) {
	// Empty file list should return empty hits
	result := ScanStubs([]string{})
	if !result.IsSuccess() {
		t.Fatalf("ScanStubs failed: %v", result.Errors)
	}

	data := result.GetData()
	if len(data.Hits) != 0 {
		t.Errorf("expected 0 hits for empty file list, got %d", len(data.Hits))
	}
}

func TestScanStubs_CaseInsensitive(t *testing.T) {
	// Test case-insensitive matching for TODO/FIXME/etc
	tmpDir := t.TempDir()
	caseFile := filepath.Join(tmpDir, "case.go")
	content := `package main

// todo: lowercase marker
// Todo: mixed case marker
// tOdO: weird case marker
`
	if err := os.WriteFile(caseFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := ScanStubs([]string{caseFile})
	if !result.IsSuccess() {
		t.Fatalf("ScanStubs failed: %v", result.Errors)
	}

	data := result.GetData()
	// Should find all 3 todo variants
	if len(data.Hits) != 3 {
		t.Errorf("expected 3 hits (case-insensitive), got %d", len(data.Hits))
	}
}

func TestScanStubs_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// File 1: has TODO
	file1 := filepath.Join(tmpDir, "file1.go")
	if err := os.WriteFile(file1, []byte("// TODO: fix this\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// File 2: clean
	file2 := filepath.Join(tmpDir, "file2.go")
	if err := os.WriteFile(file2, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// File 3: has FIXME
	file3 := filepath.Join(tmpDir, "file3.go")
	if err := os.WriteFile(file3, []byte("// FIXME: refactor\n"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := ScanStubs([]string{file1, file2, file3})
	if !result.IsSuccess() {
		t.Fatalf("ScanStubs failed: %v", result.Errors)
	}

	data := result.GetData()
	// Should find 2 hits (file1 TODO, file3 FIXME)
	if len(data.Hits) != 2 {
		t.Errorf("expected 2 hits, got %d", len(data.Hits))
	}

	// Verify file paths are correct
	foundFile1 := false
	foundFile3 := false
	for _, hit := range data.Hits {
		if hit.File == file1 {
			foundFile1 = true
		}
		if hit.File == file3 {
			foundFile3 = true
		}
	}
	if !foundFile1 || !foundFile3 {
		t.Errorf("expected hits from file1 and file3")
	}
}

func TestScanStubs_RustPatterns(t *testing.T) {
	// Test Rust-specific stub patterns
	tmpDir := t.TempDir()
	rustFile := filepath.Join(tmpDir, "test.rs")
	content := `fn main() {
    unimplemented!()
    todo!()
}
`
	if err := os.WriteFile(rustFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result := ScanStubs([]string{rustFile})
	if !result.IsSuccess() {
		t.Fatalf("ScanStubs failed: %v", result.Errors)
	}

	data := result.GetData()
	// Should find both Rust patterns
	if len(data.Hits) != 2 {
		t.Errorf("expected 2 hits for Rust patterns, got %d", len(data.Hits))
	}
}

func TestAppendStubReport_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a minimal manifest
	manifest := &IMPLManifest{
		Title:       "Test",
		FeatureSlug: "test-stub",
		Waves:       []Wave{{Number: 1, Agents: []Agent{{ID: "A"}}}},
	}
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if saveRes := Save(context.Background(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("failed to write manifest: %v", saveRes.Errors)
	}

	scanResult := ScanStubs([]string{})
	res := AppendStubReport(manifestPath, "wave1", scanResult)
	if res.IsFatal() {
		t.Fatalf("AppendStubReport failed: %v", res.Errors)
	}
	appendData := res.GetData()
	if !appendData.Appended {
		t.Error("expected Appended=true on success")
	}
	if appendData.WaveKey != "wave1" {
		t.Errorf("expected WaveKey=wave1, got %s", appendData.WaveKey)
	}
	if appendData.ManifestPath != manifestPath {
		t.Errorf("expected ManifestPath=%s, got %s", manifestPath, appendData.ManifestPath)
	}
}

func TestAppendStubReport_FatalOnMissingManifest(t *testing.T) {
	scanResult := ScanStubs([]string{})
	res := AppendStubReport("/nonexistent/path/impl.yaml", "wave1", scanResult)
	if !res.IsFatal() {
		t.Error("expected FATAL result when manifest does not exist")
	}
	if len(res.Errors) == 0 {
		t.Error("expected at least one error in FATAL result")
	}
	if res.Errors[0].Code != "STUB_APPEND_FAILED" {
		t.Errorf("expected error code STUB_APPEND_FAILED, got %s", res.Errors[0].Code)
	}
}
