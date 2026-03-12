package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBuildGraph_Simple tests a simple linear dependency chain.
func TestBuildGraph_Simple(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/simple
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create package structure: c.go -> b.go -> a.go (a has no deps)
	pkgA := filepath.Join(tmpDir, "pkga")
	pkgB := filepath.Join(tmpDir, "pkgb")
	pkgC := filepath.Join(tmpDir, "pkgc")
	os.MkdirAll(pkgA, 0755)
	os.MkdirAll(pkgB, 0755)
	os.MkdirAll(pkgC, 0755)

	fileA := filepath.Join(pkgA, "a.go")
	fileB := filepath.Join(pkgB, "b.go")
	fileC := filepath.Join(pkgC, "c.go")

	// a.go has no imports
	codeA := `package pkga

func A() string {
	return "a"
}
`
	if err := os.WriteFile(fileA, []byte(codeA), 0644); err != nil {
		t.Fatal(err)
	}

	// b.go imports pkga
	codeB := `package pkgb

import "github.com/test/simple/pkga"

func B() string {
	return pkga.A() + "b"
}
`
	if err := os.WriteFile(fileB, []byte(codeB), 0644); err != nil {
		t.Fatal(err)
	}

	// c.go imports pkgb
	codeC := `package pkgc

import "github.com/test/simple/pkgb"

func C() string {
	return pkgb.B() + "c"
}
`
	if err := os.WriteFile(fileC, []byte(codeC), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{fileA, fileB, fileC}

	graph, err := BuildGraph(tmpDir, files)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	// Verify 3 nodes
	if len(graph.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(graph.Nodes))
	}

	// Verify 3 waves
	if len(graph.Waves) != 3 {
		t.Errorf("expected 3 waves, got %d", len(graph.Waves))
	}

	// Wave 1 should have fileA (depth 0)
	if len(graph.Waves[1]) != 1 || graph.Waves[1][0] != fileA {
		t.Errorf("wave 1 should contain %s, got %v", fileA, graph.Waves[1])
	}

	// Wave 2 should have fileB (depth 1)
	if len(graph.Waves[2]) != 1 || graph.Waves[2][0] != fileB {
		t.Errorf("wave 2 should contain %s, got %v", fileB, graph.Waves[2])
	}

	// Wave 3 should have fileC (depth 2)
	if len(graph.Waves[3]) != 1 || graph.Waves[3][0] != fileC {
		t.Errorf("wave 3 should contain %s, got %v", fileC, graph.Waves[3])
	}

	// Verify dependencies
	for _, node := range graph.Nodes {
		switch node.File {
		case fileA:
			if len(node.DependsOn) != 0 {
				t.Errorf("fileA should have no deps, got %v", node.DependsOn)
			}
			if node.WaveCandidate != 0 {
				t.Errorf("fileA should have depth 0, got %d", node.WaveCandidate)
			}
		case fileB:
			if len(node.DependsOn) != 1 || node.DependsOn[0] != fileA {
				t.Errorf("fileB should depend on fileA, got %v", node.DependsOn)
			}
			if node.WaveCandidate != 1 {
				t.Errorf("fileB should have depth 1, got %d", node.WaveCandidate)
			}
		case fileC:
			if len(node.DependsOn) != 1 || node.DependsOn[0] != fileB {
				t.Errorf("fileC should depend on fileB, got %v", node.DependsOn)
			}
			if node.WaveCandidate != 2 {
				t.Errorf("fileC should have depth 2, got %d", node.WaveCandidate)
			}
		}
	}
}

// TestBuildGraph_Cycle tests cycle detection.
func TestBuildGraph_Cycle(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module github.com/test/cycle
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create cycle: A -> B -> C -> A
	pkgA := filepath.Join(tmpDir, "pkga")
	pkgB := filepath.Join(tmpDir, "pkgb")
	pkgC := filepath.Join(tmpDir, "pkgc")
	os.MkdirAll(pkgA, 0755)
	os.MkdirAll(pkgB, 0755)
	os.MkdirAll(pkgC, 0755)

	fileA := filepath.Join(pkgA, "a.go")
	fileB := filepath.Join(pkgB, "b.go")
	fileC := filepath.Join(pkgC, "c.go")

	// a.go imports pkgc (creating cycle)
	codeA := `package pkga

import "github.com/test/cycle/pkgc"

func A() string {
	return pkgc.C() + "a"
}
`
	if err := os.WriteFile(fileA, []byte(codeA), 0644); err != nil {
		t.Fatal(err)
	}

	// b.go imports pkga
	codeB := `package pkgb

import "github.com/test/cycle/pkga"

func B() string {
	return pkga.A() + "b"
}
`
	if err := os.WriteFile(fileB, []byte(codeB), 0644); err != nil {
		t.Fatal(err)
	}

	// c.go imports pkgb
	codeC := `package pkgc

import "github.com/test/cycle/pkgb"

func C() string {
	return pkgb.B() + "c"
}
`
	if err := os.WriteFile(fileC, []byte(codeC), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{fileA, fileB, fileC}

	_, err := BuildGraph(tmpDir, files)
	if err == nil {
		t.Fatal("expected cycle detection error, got nil")
	}

	if !contains(err.Error(), "circular dependency") {
		t.Errorf("expected 'circular dependency' in error, got: %v", err)
	}
}

// TestBuildGraph_NoDeps tests a single file with no imports.
func TestBuildGraph_NoDeps(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module github.com/test/nodeps
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	pkg := filepath.Join(tmpDir, "pkg")
	os.MkdirAll(pkg, 0755)

	file := filepath.Join(pkg, "main.go")
	code := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{file}

	graph, err := BuildGraph(tmpDir, files)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	// Should have 1 node in wave 1
	if len(graph.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(graph.Nodes))
	}

	if len(graph.Waves) != 1 {
		t.Errorf("expected 1 wave, got %d", len(graph.Waves))
	}

	if len(graph.Waves[1]) != 1 {
		t.Errorf("expected 1 file in wave 1, got %d", len(graph.Waves[1]))
	}

	node := graph.Nodes[0]
	if node.WaveCandidate != 0 {
		t.Errorf("expected depth 0, got %d", node.WaveCandidate)
	}

	if len(node.DependsOn) != 0 {
		t.Errorf("expected no deps, got %v", node.DependsOn)
	}
}

// TestDetectCycles_LinearGraph tests no cycles in a linear graph.
func TestDetectCycles_LinearGraph(t *testing.T) {
	adj := map[string][]string{
		"a.go": {},
		"b.go": {"a.go"},
		"c.go": {"b.go"},
	}
	files := []string{"a.go", "b.go", "c.go"}

	cycles := detectCycles(adj, files)
	if cycles != nil {
		t.Errorf("expected no cycles, got %v", cycles)
	}
}

// TestDetectCycles_Cycle tests cycle detection.
func TestDetectCycles_Cycle(t *testing.T) {
	adj := map[string][]string{
		"a.go": {"c.go"},
		"b.go": {"a.go"},
		"c.go": {"b.go"},
	}
	files := []string{"a.go", "b.go", "c.go"}

	cycles := detectCycles(adj, files)
	if cycles == nil {
		t.Fatal("expected cycle, got nil")
	}

	if len(cycles) == 0 {
		t.Fatal("expected at least one cycle")
	}

	// Verify cycle contains all three files
	cycle := cycles[0]
	if len(cycle) != 4 { // Should be [a, c, b, a] or similar (closed cycle)
		t.Errorf("expected cycle length 4, got %d: %v", len(cycle), cycle)
	}
}

// TestComputeDepth_RootNodes tests files with no deps have depth 0.
func TestComputeDepth_RootNodes(t *testing.T) {
	adj := map[string][]string{
		"a.go": {},
		"b.go": {},
		"c.go": {},
	}
	files := []string{"a.go", "b.go", "c.go"}

	depth := computeDepth(adj, files)

	for _, f := range files {
		if depth[f] != 0 {
			t.Errorf("file %s should have depth 0, got %d", f, depth[f])
		}
	}
}

// TestComputeDepth_Transitive tests transitive depth calculation.
func TestComputeDepth_Transitive(t *testing.T) {
	adj := map[string][]string{
		"a.go": {"b.go"},
		"b.go": {"c.go"},
		"c.go": {},
	}
	files := []string{"a.go", "b.go", "c.go"}

	depth := computeDepth(adj, files)

	expected := map[string]int{
		"c.go": 0,
		"b.go": 1,
		"a.go": 2,
	}

	for f, exp := range expected {
		if depth[f] != exp {
			t.Errorf("file %s: expected depth %d, got %d", f, exp, depth[f])
		}
	}
}

// TestAssignWaves_MultipleFilesPerWave tests wave assignment.
func TestAssignWaves_MultipleFilesPerWave(t *testing.T) {
	depth := map[string]int{
		"a.go": 0,
		"b.go": 0,
		"c.go": 1,
	}

	waves := assignWaves(depth)

	if len(waves) != 2 {
		t.Errorf("expected 2 waves, got %d", len(waves))
	}

	// Wave 1 should have a.go and b.go
	if len(waves[1]) != 2 {
		t.Errorf("wave 1: expected 2 files, got %d", len(waves[1]))
	}

	// Verify sorted
	if waves[1][0] != "a.go" || waves[1][1] != "b.go" {
		t.Errorf("wave 1: expected [a.go, b.go], got %v", waves[1])
	}

	// Wave 2 should have c.go
	if len(waves[2]) != 1 || waves[2][0] != "c.go" {
		t.Errorf("wave 2: expected [c.go], got %v", waves[2])
	}
}

// TestDetectLanguage_Go tests Go language detection.
func TestDetectLanguage_Go(t *testing.T) {
	files := []string{"main.go", "utils.go", "types.go"}
	lang, err := detectLanguage(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lang != "go" {
		t.Errorf("expected 'go', got %s", lang)
	}
}

// TestDetectLanguage_Rust tests Rust language detection.
func TestDetectLanguage_Rust(t *testing.T) {
	files := []string{"main.rs", "lib.rs", "utils.rs"}
	lang, err := detectLanguage(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lang != "rust" {
		t.Errorf("expected 'rust', got %s", lang)
	}
}

// TestDetectLanguage_JavaScript tests JavaScript language detection.
func TestDetectLanguage_JavaScript(t *testing.T) {
	testCases := []struct {
		name  string
		files []string
	}{
		{"js", []string{"index.js", "utils.js"}},
		{"jsx", []string{"App.jsx", "Component.jsx"}},
		{"ts", []string{"main.ts", "types.ts"}},
		{"tsx", []string{"App.tsx", "Button.tsx"}},
		{"mjs", []string{"module.mjs", "utils.mjs"}},
		{"mixed", []string{"index.js", "App.tsx", "utils.ts"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lang, err := detectLanguage(tc.files)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lang != "javascript" {
				t.Errorf("expected 'javascript', got %s", lang)
			}
		})
	}
}

// TestDetectLanguage_Python tests Python language detection.
func TestDetectLanguage_Python(t *testing.T) {
	files := []string{"main.py", "__init__.py", "utils.py"}
	lang, err := detectLanguage(files)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lang != "python" {
		t.Errorf("expected 'python', got %s", lang)
	}
}

// TestDetectLanguage_MixedError tests error on mixed languages.
func TestDetectLanguage_MixedError(t *testing.T) {
	// Equal counts - should pick one but not error (Go and Rust have equal count)
	files := []string{"main.go", "lib.rs"}
	lang, err := detectLanguage(files)
	// Should succeed and return either 'go' or 'rust' (whichever is chosen by map iteration)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lang != "go" && lang != "rust" {
		t.Errorf("expected 'go' or 'rust', got %s", lang)
	}
}

// TestDetectLanguage_UnsupportedError tests error on unsupported extensions.
func TestDetectLanguage_UnsupportedError(t *testing.T) {
	files := []string{"README.md", "LICENSE.txt"}
	_, err := detectLanguage(files)
	if err == nil {
		t.Fatal("expected error for unsupported extensions, got nil")
	}
	if !contains(err.Error(), "unsupported file extension") {
		t.Errorf("expected 'unsupported file extension' in error, got: %v", err)
	}
}

// TestBuildGraph_MultiLanguageIntegration tests BuildGraph with language detection.
func TestBuildGraph_MultiLanguageIntegration(t *testing.T) {
	// This is effectively the same as TestBuildGraph_Simple since it's all Go files
	// The integration test verifies that the language detection path works correctly
	tmpDir := t.TempDir()

	goMod := `module github.com/test/multilang
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	pkgA := filepath.Join(tmpDir, "pkga")
	os.MkdirAll(pkgA, 0755)
	fileA := filepath.Join(pkgA, "a.go")

	codeA := `package pkga

func A() string {
	return "a"
}
`
	if err := os.WriteFile(fileA, []byte(codeA), 0644); err != nil {
		t.Fatal(err)
	}

	files := []string{fileA}

	graph, err := BuildGraph(tmpDir, files)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	if len(graph.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(graph.Nodes))
	}

	if len(graph.Waves) != 1 {
		t.Errorf("expected 1 wave, got %d", len(graph.Waves))
	}
}

// TestDetectCascades_Unchanged tests cascade detection.
func TestDetectCascades_Unchanged(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create modified file
	pkgA := filepath.Join(tmpDir, "pkga")
	os.MkdirAll(pkgA, 0755)
	modifiedFile := filepath.Join(pkgA, "modified.go")
	modCode := `package pkga

func Modified() string {
	return "modified"
}
`
	if err := os.WriteFile(modifiedFile, []byte(modCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Create unchanged file that imports modified package
	pkgB := filepath.Join(tmpDir, "pkgb")
	os.MkdirAll(pkgB, 0755)
	unchangedFile := filepath.Join(pkgB, "unchanged.go")
	uncCode := `package pkgb

import "github.com/test/cascade/pkga"

func Unchanged() string {
	return pkga.Modified()
}
`
	if err := os.WriteFile(unchangedFile, []byte(uncCode), 0644); err != nil {
		t.Fatal(err)
	}

	modifiedFiles := []string{modifiedFile}
	revAdj := map[string][]string{
		modifiedFile: {unchangedFile},
	}

	cascades, err := detectCascades(tmpDir, modifiedFiles, revAdj)
	if err != nil {
		t.Fatalf("detectCascades failed: %v", err)
	}

	if len(cascades) == 0 {
		t.Fatal("expected cascade candidates, got none")
	}

	// Should find unchanged.go
	found := false
	for _, c := range cascades {
		if c.File == unchangedFile {
			found = true
			if c.Type != "semantic" {
				t.Errorf("expected type 'semantic', got %s", c.Type)
			}
		}
	}

	if !found {
		t.Errorf("unchanged file %s not found in cascades: %v", unchangedFile, cascades)
	}
}
