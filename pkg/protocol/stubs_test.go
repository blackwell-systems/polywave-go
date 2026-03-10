package protocol

import (
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

	result, err := ScanStubs([]string{cleanFile})
	if err != nil {
		t.Fatalf("ScanStubs failed: %v", err)
	}

	if len(result.Hits) != 0 {
		t.Errorf("expected 0 hits, got %d", len(result.Hits))
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

	result, err := ScanStubs([]string{stubFile})
	if err != nil {
		t.Fatalf("ScanStubs failed: %v", err)
	}

	// We expect multiple hits
	expectedPatterns := []string{"TODO", "panic(\"not implemented\")", "FIXME", "// stub", "XXX", "HACK", "// placeholder"}
	if len(result.Hits) < len(expectedPatterns) {
		t.Errorf("expected at least %d hits, got %d", len(expectedPatterns), len(result.Hits))
	}

	// Verify that line numbers are correct (should be positive)
	for _, hit := range result.Hits {
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
	for _, hit := range result.Hits {
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
	result, err := ScanStubs([]string{"/nonexistent/file.go"})
	if err != nil {
		t.Fatalf("ScanStubs should not error on missing file: %v", err)
	}

	if len(result.Hits) != 0 {
		t.Errorf("expected 0 hits for missing file, got %d", len(result.Hits))
	}
}

func TestScanStubs_EmptyFiles(t *testing.T) {
	// Empty file list should return empty hits
	result, err := ScanStubs([]string{})
	if err != nil {
		t.Fatalf("ScanStubs failed: %v", err)
	}

	if len(result.Hits) != 0 {
		t.Errorf("expected 0 hits for empty file list, got %d", len(result.Hits))
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

	result, err := ScanStubs([]string{caseFile})
	if err != nil {
		t.Fatalf("ScanStubs failed: %v", err)
	}

	// Should find all 3 todo variants
	if len(result.Hits) != 3 {
		t.Errorf("expected 3 hits (case-insensitive), got %d", len(result.Hits))
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

	result, err := ScanStubs([]string{file1, file2, file3})
	if err != nil {
		t.Fatalf("ScanStubs failed: %v", err)
	}

	// Should find 2 hits (file1 TODO, file3 FIXME)
	if len(result.Hits) != 2 {
		t.Errorf("expected 2 hits, got %d", len(result.Hits))
	}

	// Verify file paths are correct
	foundFile1 := false
	foundFile3 := false
	for _, hit := range result.Hits {
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

	result, err := ScanStubs([]string{rustFile})
	if err != nil {
		t.Fatalf("ScanStubs failed: %v", err)
	}

	// Should find both Rust patterns
	if len(result.Hits) != 2 {
		t.Errorf("expected 2 hits for Rust patterns, got %d", len(result.Hits))
	}
}
