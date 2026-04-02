package protocol

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// makeTestCascadeManifest creates a minimal IMPLManifest for testing.
func makeTestCascadeManifest(agentID, task string, ownedFiles []string) *IMPLManifest {
	fo := make([]FileOwnership, len(ownedFiles))
	for i, f := range ownedFiles {
		fo[i] = FileOwnership{File: f, Agent: agentID, Wave: 1}
	}
	return &IMPLManifest{
		Title:       "test",
		FeatureSlug: "test-feature",
		FileOwnership: fo,
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: agentID, Task: task, Files: ownedFiles},
				},
			},
		},
	}
}

// TestCheckTestCascade_OrphanTestFile verifies that a *_test.go calling a changed
// symbol but NOT in FileOwnership is reported as a TestCascadeError.
func TestCheckTestCascade_OrphanTestFile(t *testing.T) {
	// Create a temp dir to act as the repo root
	dir := t.TempDir()

	// Write a test file that calls the changed symbol but is NOT owned
	orphanTestPath := filepath.Join(dir, "pkg", "other", "foo_test.go")
	if err := os.MkdirAll(filepath.Dir(orphanTestPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	orphanContent := `package other

import "testing"

func TestCallsFoo(t *testing.T) {
	result := MyFunc()
	_ = result
}
`
	if err := os.WriteFile(orphanTestPath, []byte(orphanContent), 0644); err != nil {
		t.Fatalf("write orphan test: %v", err)
	}

	// The agent task mentions changing `MyFunc` signature
	task := "change signature to `MyFunc` to return error"
	m := makeTestCascadeManifest("A", task, []string{"pkg/foo/foo.go"})

	res := CheckTestCascade(context.Background(), m, dir)
	if !res.IsSuccess() {
		t.Fatalf("expected SUCCESS, got %s: %v", res.Code, res.Errors)
	}

	errs := res.GetData()
	if len(errs) == 0 {
		t.Fatal("expected at least one TestCascadeError, got none")
	}

	found := false
	for _, e := range errs {
		if e.Symbol == "MyFunc" && e.TestFile == filepath.Join("pkg", "other", "foo_test.go") {
			found = true
			if len(e.Agents) == 0 || e.Agents[0] != "A" {
				t.Errorf("expected agent A in Agents, got %v", e.Agents)
			}
			if e.Line <= 0 {
				t.Errorf("expected positive line number, got %d", e.Line)
			}
		}
	}
	if !found {
		t.Errorf("did not find expected TestCascadeError for MyFunc in pkg/other/foo_test.go; got: %+v", errs)
	}
}

// TestCheckTestCascade_OwnedTestFile verifies that when the test file IS in FileOwnership,
// no TestCascadeError is reported.
func TestCheckTestCascade_OwnedTestFile(t *testing.T) {
	dir := t.TempDir()

	// Write a test file that calls the changed symbol
	ownedTestPath := filepath.Join(dir, "pkg", "foo", "foo_test.go")
	if err := os.MkdirAll(filepath.Dir(ownedTestPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ownedContent := `package foo

import "testing"

func TestMyFunc(t *testing.T) {
	result := MyFunc()
	_ = result
}
`
	if err := os.WriteFile(ownedTestPath, []byte(ownedContent), 0644); err != nil {
		t.Fatalf("write owned test: %v", err)
	}

	// The agent owns both the implementation file and the test file
	task := "change signature to `MyFunc` to return error"
	m := makeTestCascadeManifest("A", task, []string{
		"pkg/foo/foo.go",
		"pkg/foo/foo_test.go",
	})

	res := CheckTestCascade(context.Background(), m, dir)
	if !res.IsSuccess() {
		t.Fatalf("expected SUCCESS, got %s: %v", res.Code, res.Errors)
	}

	errs := res.GetData()
	if len(errs) != 0 {
		t.Errorf("expected no TestCascadeErrors, got %d: %+v", len(errs), errs)
	}
}

// TestCheckTestCascade_NilManifest verifies that a nil manifest returns FATAL.
func TestCheckTestCascade_NilManifest(t *testing.T) {
	dir := t.TempDir()

	res := CheckTestCascade(context.Background(), nil, dir)
	if !res.IsFatal() {
		t.Errorf("expected FATAL result for nil manifest, got %s", res.Code)
	}
}

// TestCheckTestCascade_NoKeywords verifies that a task with no change keywords
// produces SUCCESS with an empty slice (no symbols detected).
func TestCheckTestCascade_NoKeywords(t *testing.T) {
	dir := t.TempDir()

	// Write a test file
	testPath := filepath.Join(dir, "pkg", "foo", "foo_test.go")
	if err := os.MkdirAll(filepath.Dir(testPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(testPath, []byte(`package foo

func TestSomething(t *testing.T) {}
`), 0644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	// Task has no change keywords
	task := "add new helper `DoSomething` in pkg/foo/foo.go"
	m := makeTestCascadeManifest("A", task, []string{"pkg/foo/foo.go"})

	res := CheckTestCascade(context.Background(), m, dir)
	if !res.IsSuccess() {
		t.Fatalf("expected SUCCESS, got %s: %v", res.Code, res.Errors)
	}

	errs := res.GetData()
	if len(errs) != 0 {
		t.Errorf("expected empty slice, got %d errors: %+v", len(errs), errs)
	}
}
