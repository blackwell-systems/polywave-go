package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildGraph_Success verifies that BuildGraph succeeds on a real Go source
// directory and returns a graph with at least one node.
func TestBuildGraph_Success(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := "module github.com/test/success\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	src := `package main

func main() {}
`
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	res := BuildGraph(context.Background(), tmpDir, []string{mainFile})
	if !res.IsSuccess() {
		t.Fatalf("BuildGraph() expected success, got code=%s errors=%v", res.Code, res.Errors)
	}
	graph := res.GetData()
	if graph == nil {
		t.Fatal("BuildGraph() returned nil graph on success")
	}
	if len(graph.Nodes) < 1 {
		t.Errorf("BuildGraph() expected at least 1 node, got %d", len(graph.Nodes))
	}
}

// TestBuildGraph_NonexistentRepoRoot verifies that BuildGraph returns a FATAL
// result when given a nonexistent repoRoot. The walk/mod read fails, triggering
// a Z013_WALK_FAILED or equivalent fatal code.
func TestBuildGraph_NonexistentRepoRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real Go file so file parsing succeeds, but pass a fake repoRoot
	// so go.mod resolution and any repoRoot-relative walk fails.
	src := `package pkg

func F() {}
`
	goFile := filepath.Join(tmpDir, "f.go")
	if err := os.WriteFile(goFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	fakeRoot := "/nonexistent/repo/that/does/not/exist/xyz"
	res := BuildGraph(context.Background(), fakeRoot, []string{goFile})
	if res.IsSuccess() {
		t.Fatal("BuildGraph() expected failure with nonexistent repoRoot, got success")
	}
	if !res.IsFatal() {
		t.Errorf("BuildGraph() expected FATAL result for nonexistent repoRoot, got code=%s", res.Code)
	}
}

// TestBuildGraph_CancelledContext verifies that BuildGraph returns a FATAL
// result immediately when given a pre-cancelled context.
func TestBuildGraph_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := "module github.com/test/cancelled\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	src := `package main

func main() {}
`
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling BuildGraph

	res := BuildGraph(ctx, tmpDir, []string{mainFile})
	if res.IsSuccess() {
		t.Fatal("BuildGraph() expected failure for cancelled context, got success")
	}
	if !res.IsFatal() {
		t.Errorf("BuildGraph() expected FATAL result for cancelled context, got code=%s", res.Code)
	}
}

// TestToOutput_Serialization verifies that ToOutput followed by FormatYAML and
// FormatJSON both return IsSuccess() with non-empty bytes.
func TestToOutput_Serialization(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := "module github.com/test/serial\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	src := `package main

func main() {}
`
	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	graphRes := BuildGraph(context.Background(), tmpDir, []string{mainFile})
	if !graphRes.IsSuccess() {
		t.Fatalf("BuildGraph() expected success, got code=%s errors=%v", graphRes.Code, graphRes.Errors)
	}

	output := ToOutput(graphRes.GetData())
	if output == nil {
		t.Fatal("ToOutput() returned nil")
	}

	yamlRes := FormatYAML(output)
	if !yamlRes.IsSuccess() {
		t.Errorf("FormatYAML() expected success, got code=%s errors=%v", yamlRes.Code, yamlRes.Errors)
	}
	yamlBytes := *yamlRes.Data
	if len(yamlBytes) == 0 {
		t.Error("FormatYAML() returned empty bytes")
	}

	jsonRes := FormatJSON(output)
	if !jsonRes.IsSuccess() {
		t.Errorf("FormatJSON() expected success, got code=%s errors=%v", jsonRes.Code, jsonRes.Errors)
	}
	jsonBytes := *jsonRes.Data
	if len(jsonBytes) == 0 {
		t.Error("FormatJSON() returned empty bytes")
	}

}
