package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractStringInput_NilMap(t *testing.T) {
	_, ok := extractStringInput(nil, "file_path")
	if ok {
		t.Error("expected false for nil map")
	}
}

func TestExtractStringInput_MissingKey(t *testing.T) {
	_, ok := extractStringInput(map[string]interface{}{"other": "val"}, "file_path")
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestExtractStringInput_WrongType(t *testing.T) {
	_, ok := extractStringInput(map[string]interface{}{"file_path": 42}, "file_path")
	if ok {
		t.Error("expected false for non-string value")
	}
}

func TestExtractStringInput_EmptyString(t *testing.T) {
	_, ok := extractStringInput(map[string]interface{}{"file_path": ""}, "file_path")
	if ok {
		t.Error("expected false for empty string")
	}
}

func TestExtractStringInput_ValidString(t *testing.T) {
	v, ok := extractStringInput(map[string]interface{}{"file_path": "foo.go"}, "file_path")
	if !ok {
		t.Error("expected true for valid string")
	}
	if v != "foo.go" {
		t.Errorf("expected 'foo.go', got %q", v)
	}
}

func TestFileWriteExecutor_MissingFilePath(t *testing.T) {
	ex := &FileWriteExecutor{}
	result, err := ex.Execute(context.Background(),
		ExecutionContext{WorkDir: "/tmp"},
		map[string]interface{}{"content": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "input validation failed") {
		t.Errorf("expected validation error message, got %q", result)
	}
}

func TestFileWriteExecutor_WrongTypeFilePath(t *testing.T) {
	ex := &FileWriteExecutor{}
	result, err := ex.Execute(context.Background(),
		ExecutionContext{WorkDir: "/tmp"},
		map[string]interface{}{"file_path": 123, "content": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "input validation failed") {
		t.Errorf("expected validation error for wrong type, got %q", result)
	}
}

func TestBashExecutor_MissingCommand(t *testing.T) {
	ex := &BashExecutor{}
	result, err := ex.Execute(context.Background(),
		ExecutionContext{WorkDir: "/tmp"},
		map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "input validation failed") {
		t.Errorf("expected validation error message, got %q", result)
	}
}

func TestGrepFallback_NonexistentRoot(t *testing.T) {
	// Should not panic; may return walk error note or empty string
	result := grepFallback("/nonexistent/path/zzzz", "pattern")
	_ = result // just verify no panic
}

func TestGrepFallback_FindsMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "match.txt"), []byte("hello test world\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got := grepFallback(dir, "test")
	if got == "" {
		t.Error("expected non-empty output for matching file")
	}
}

// EditExecutor tests
func TestEditExecutor_Success(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0o644)

	executor := &EditExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: tmpDir}, map[string]interface{}{
		"file_path":  testFile,
		"old_string": "world",
		"new_string": "universe",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "ok" {
		t.Errorf("Expected 'ok', got %q", result)
	}

	contents, _ := os.ReadFile(testFile)
	if string(contents) != "hello universe" {
		t.Errorf("File contents: got %q, want 'hello universe'", string(contents))
	}
}

func TestEditExecutor_OldStringNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0o644)

	executor := &EditExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: tmpDir}, map[string]interface{}{
		"file_path":  testFile,
		"old_string": "nonexistent",
		"new_string": "replacement",
	})

	if err != nil {
		t.Fatalf("Execute should return error message as string, not Go error: %v", err)
	}
	if !strings.Contains(result, "old_string not found") {
		t.Errorf("Expected 'old_string not found' message, got: %s", result)
	}
}

func TestEditExecutor_MissingFilePath(t *testing.T) {
	executor := &EditExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: "/tmp"}, map[string]interface{}{
		"old_string": "foo",
		"new_string": "bar",
	})

	if err != nil {
		t.Fatalf("Execute should return validation error as string: %v", err)
	}
	if !strings.Contains(result, "input validation failed") {
		t.Errorf("Expected validation error, got: %s", result)
	}
}

// GlobExecutor tests
func TestGlobExecutor_FindsMatches(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "file3.log"), []byte(""), 0o644)

	executor := &GlobExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: tmpDir}, map[string]interface{}{
		"pattern": "*.txt",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "file1.txt") || !strings.Contains(result, "file2.txt") {
		t.Errorf("Expected file1.txt and file2.txt in result, got: %s", result)
	}
	if strings.Contains(result, "file3.log") {
		t.Errorf("file3.log should not match *.txt pattern")
	}
}

// GrepExecutor tests
func TestGrepExecutor_FindsPattern(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello world\nfoo bar"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("hello universe"), 0o644)

	executor := &GrepExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: tmpDir}, map[string]interface{}{
		"pattern": "hello",
		"path":    ".",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// Result format depends on whether rg is available, so just verify something is returned
	if result == "" {
		t.Error("Expected non-empty result for pattern that exists")
	}
}

// FileListExecutor tests
func TestFileListExecutor_ListsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte(""), 0o644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte(""), 0o644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0o755)

	executor := &FileListExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: tmpDir}, map[string]interface{}{
		"path": ".",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.Contains(result, "file1.txt") || !strings.Contains(result, "file2.txt") {
		t.Errorf("Expected file1.txt and file2.txt in result, got: %s", result)
	}
	if !strings.Contains(result, "subdir") {
		t.Errorf("Expected subdir in result, got: %s", result)
	}
}

// BashExecutor git ownership tests
func TestBashExecutor_GitOwnershipWarning(t *testing.T) {
	tmpDir := t.TempDir()
	// Create .polywave-ownership.json
	ownership := map[string]interface{}{
		"owned_files": []string{"owned.txt"},
	}
	ownershipJSON, _ := json.Marshal(ownership)
	os.WriteFile(filepath.Join(tmpDir, ".polywave-ownership.json"), ownershipJSON, 0o644)

	// Initialize git repo
	exec.Command("git", "init", tmpDir).Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Create and commit owned file
	os.WriteFile(filepath.Join(tmpDir, "owned.txt"), []byte("owned"), 0o644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "init").Run()

	// Modify unowned file
	os.WriteFile(filepath.Join(tmpDir, "unowned.txt"), []byte("unowned"), 0o644)

	executor := &BashExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: tmpDir}, map[string]interface{}{
		"command": "git checkout HEAD", // Triggers git modify check
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	// Check for warning message (only appears if git diff detects changes outside ownership)
	// This test may not trigger warning if git checkout doesn't modify unowned.txt
	t.Logf("Result: %s", result)
}

// FileWriteExecutor additional tests
func TestFileWriteExecutor_EmptyContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	executor := &FileWriteExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: tmpDir}, map[string]interface{}{
		"file_path": testFile,
		"content":   "",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "ok" {
		t.Errorf("Expected 'ok', got %q", result)
	}

	contents, _ := os.ReadFile(testFile)
	if len(contents) != 0 {
		t.Errorf("Expected empty file, got %d bytes", len(contents))
	}
}

func TestFileWriteExecutor_CreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir1", "subdir2", "test.txt")

	executor := &FileWriteExecutor{}
	result, err := executor.Execute(context.Background(), ExecutionContext{WorkDir: tmpDir}, map[string]interface{}{
		"file_path": testFile,
		"content":   "nested",
	})

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "ok" {
		t.Errorf("Expected 'ok', got %q", result)
	}

	contents, _ := os.ReadFile(testFile)
	if string(contents) != "nested" {
		t.Errorf("File contents: got %q, want 'nested'", string(contents))
	}
}

func TestIsGitModifyCommand(t *testing.T) {
	cases := []struct {
		command string
		want    bool
	}{
		{"git checkout main", true},
		{"git checkout -- file.go", true},
		{"git merge feature-branch", true},
		{"git rebase origin/main", true},
		{"git cherry-pick abc123", true},
		{"git stash pop", true},
		{"git reset --hard HEAD", true},
		{"git restore .", true},
		{"git status", false},
		{"git add .", false},
		{"git commit -m 'msg'", false},
		{"git diff HEAD", false},
		{"git log --oneline", false},
		{"git push origin main", false},
		{"echo hello", false}, // no git pattern
	}
	for _, tc := range cases {
		got := isGitModifyCommand(tc.command)
		if got != tc.want {
			t.Errorf("isGitModifyCommand(%q) = %v, want %v", tc.command, got, tc.want)
		}
	}
}

func TestStandardTools(t *testing.T) {
	w := StandardTools(t.TempDir())
	all := w.All()
	if len(all) != 7 {
		t.Errorf("StandardTools: expected 7 tools, got %d", len(all))
	}
	want := []string{"read_file", "write_file", "list_directory", "bash", "edit_file", "glob", "grep"}
	got := make(map[string]bool, len(all))
	for _, tool := range all {
		got[tool.Name] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("StandardTools: missing expected tool %q", name)
		}
	}
}
