package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractSymbols(t *testing.T) {
	taskText := `
Create ` + "`" + `BriefValidationData` + "`" + ` and ` + "`" + `AgentBriefResult` + "`" + ` structs.

Also implement ` + "`" + `validateBriefs` + "`" + ` and ` + "`" + `checkSymbolExists` + "`" + ` helpers.

The implementation should follow:

` + "```" + `
type IMPLManifest struct {}
func loadManifest() {}
` + "```" + `
`

	symbols := extractSymbols(taskText)

	symbolSet := make(map[string]bool)
	for _, s := range symbols {
		symbolSet[s] = true
	}

	// Should find type names from backticks.
	if !symbolSet["BriefValidationData"] {
		t.Error("expected BriefValidationData to be extracted")
	}
	if !symbolSet["AgentBriefResult"] {
		t.Error("expected AgentBriefResult to be extracted")
	}

	// Should find function names from backticks.
	if !symbolSet["validateBriefs"] {
		t.Error("expected validateBriefs to be extracted")
	}
	if !symbolSet["checkSymbolExists"] {
		t.Error("expected checkSymbolExists to be extracted")
	}

	// Should find type from code block.
	if !symbolSet["IMPLManifest"] {
		t.Error("expected IMPLManifest to be extracted from code block")
	}

	// Should find func from code block.
	if !symbolSet["loadManifest"] {
		t.Error("expected loadManifest to be extracted from code block")
	}
}

func TestExtractSymbols_Deduplication(t *testing.T) {
	taskText := "`ValidateBriefs` is the main function. Call `ValidateBriefs` with context."

	symbols := extractSymbols(taskText)

	count := 0
	for _, s := range symbols {
		if s == "ValidateBriefs" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected ValidateBriefs to appear exactly once, got %d", count)
	}
}

func TestExtractSymbols_MethodCallStrip(t *testing.T) {
	taskText := "`manifest.Load()` should be called first."

	symbols := extractSymbols(taskText)
	// manifest.Load() — after splitting, manifest is lowercase and Load is after dot
	// The regex won't match across dots, but let's verify we don't crash.
	_ = symbols
}

func TestExtractLineReferences(t *testing.T) {
	taskText := `Read pkg/protocol/critic.go around line 135 for context.

Also check at line 184 in pkg/protocol/types.go.

See line 50 for more.

Look at lines 10-12 for the pattern.
`

	refs := extractLineReferences(taskText)

	// Should have found line 135 associated with a file.
	found135 := false
	for _, lines := range refs {
		for _, l := range lines {
			if l == 135 {
				found135 = true
			}
		}
	}
	if !found135 {
		t.Error("expected line 135 to be extracted")
	}

	// Should have found line 184.
	found184 := false
	for _, lines := range refs {
		for _, l := range lines {
			if l == 184 {
				found184 = true
			}
		}
	}
	if !found184 {
		t.Error("expected line 184 to be extracted")
	}

	// lines 10-12 should produce 10, 11, 12.
	found10, found11, found12 := false, false, false
	for _, lines := range refs {
		for _, l := range lines {
			switch l {
			case 10:
				found10 = true
			case 11:
				found11 = true
			case 12:
				found12 = true
			}
		}
	}
	if !found10 || !found11 || !found12 {
		t.Errorf("expected lines 10, 11, 12 from range; got 10=%v 11=%v 12=%v", found10, found11, found12)
	}
}

func TestCheckSymbolExists_Found(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "sample.go")
	content := `package sample

type MyType struct{}

func myFunc() {}
`
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	found, suggestions := checkSymbolExists(f, "MyType")
	if !found {
		t.Error("expected MyType to be found")
	}
	if len(suggestions) != 0 {
		t.Errorf("expected no suggestions when found, got %v", suggestions)
	}
}

func TestCheckSymbolExists_NotFound(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "sample.go")
	content := `package sample

type MyType struct{}

func myFunc() {}
`
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	found, suggestions := checkSymbolExists(f, "MyTypo")
	if found {
		t.Error("expected MyTypo to not be found")
	}
	// Should suggest MyType as a close match.
	if len(suggestions) == 0 {
		t.Error("expected suggestions when symbol not found")
	}
	hasSuggestion := false
	for _, s := range suggestions {
		if strings.Contains(s, "MyType") {
			hasSuggestion = true
		}
	}
	if !hasSuggestion {
		t.Errorf("expected MyType to appear in suggestions for MyTypo, got %v", suggestions)
	}
}

func TestCheckLineValid_Valid(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "lines.txt")
	lines := strings.Repeat("line\n", 20)
	if err := os.WriteFile(f, []byte(lines), 0644); err != nil {
		t.Fatal(err)
	}

	if !checkLineValid(f, 1) {
		t.Error("expected line 1 to be valid in 20-line file")
	}
	if !checkLineValid(f, 20) {
		t.Error("expected line 20 to be valid in 20-line file")
	}
	if checkLineValid(f, 21) {
		t.Error("expected line 21 to be invalid in 20-line file")
	}
}

func TestCheckLineValid_NonExistentFile(t *testing.T) {
	if checkLineValid("/nonexistent/path/file.go", 1) {
		t.Error("expected false for non-existent file")
	}
}

func TestValidateBriefs(t *testing.T) {
	// Create a temp dir with a real IMPL doc and file to validate.
	tmp := t.TempDir()

	// Create the owned file with known symbols.
	pkgDir := filepath.Join(tmp, "pkg", "mymodule")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(pkgDir, "handler.go")
	goContent := `package mymodule

type Handler struct{}

func newHandler() *Handler {
	return &Handler{}
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a minimal IMPL manifest YAML.
	implContent := `title: Test IMPL
feature_slug: test-validate-briefs
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
repository: ` + tmp + `
file_ownership:
  - file: pkg/mymodule/handler.go
    agent: A
    wave: 1
    action: modify
waves:
  - number: 1
    agents:
      - id: A
        task: |
          Create ` + "`" + `Handler` + "`" + ` struct and ` + "`" + `newHandler` + "`" + ` function in pkg/mymodule/handler.go.
        files:
          - pkg/mymodule/handler.go
`

	implPath := filepath.Join(tmp, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	result, err := ValidateBriefs(ctx, implPath)
	if err != nil {
		t.Fatalf("ValidateBriefs returned error: %v", err)
	}

	if result.AgentResults == nil {
		t.Fatal("expected non-nil AgentResults")
	}

	agentA, ok := result.AgentResults["A"]
	if !ok {
		t.Fatal("expected result for agent A")
	}
	if agentA.AgentID != "A" {
		t.Errorf("expected AgentID A, got %s", agentA.AgentID)
	}

	// Handler and newHandler are both present in the file, so no errors expected.
	for _, issue := range agentA.Issues {
		if issue.Severity == "error" {
			t.Errorf("unexpected error issue: %+v", issue)
		}
	}

	if result.Summary == "" {
		t.Error("expected non-empty Summary")
	}
	t.Logf("Summary: %s", result.Summary)
	t.Logf("Issues for agent A: %d", len(agentA.Issues))
}

func TestValidateBriefs_NewFileSkipped(t *testing.T) {
	tmp := t.TempDir()

	// File does NOT exist — action:new should skip symbol checks.
	implContent := `title: Test IMPL New File
feature_slug: test-new-file
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
repository: ` + tmp + `
file_ownership:
  - file: pkg/newmodule/service.go
    agent: B
    wave: 1
    action: new
waves:
  - number: 1
    agents:
      - id: B
        task: |
          Create ` + "`" + `Service` + "`" + ` struct in pkg/newmodule/service.go.
        files:
          - pkg/newmodule/service.go
`

	implPath := filepath.Join(tmp, "IMPL-new-file.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	result, err := ValidateBriefs(ctx, implPath)
	if err != nil {
		t.Fatalf("ValidateBriefs returned error: %v", err)
	}

	agentB, ok := result.AgentResults["B"]
	if !ok {
		t.Fatal("expected result for agent B")
	}

	// action:new files should not generate symbol_missing errors.
	for _, issue := range agentB.Issues {
		if issue.Check == "symbol_missing" {
			t.Errorf("unexpected symbol_missing issue for action:new file: %+v", issue)
		}
	}
}

func TestValidateBriefs_MissingSymbol(t *testing.T) {
	tmp := t.TempDir()

	// Create file WITHOUT the symbol mentioned in the brief.
	pkgDir := filepath.Join(tmp, "pkg", "mod")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(pkgDir, "mod.go")
	goContent := `package mod

type ActualName struct{}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	implContent := `title: Test Missing Symbol
feature_slug: test-missing-symbol
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
repository: ` + tmp + `
file_ownership:
  - file: pkg/mod/mod.go
    agent: C
    wave: 1
    action: modify
waves:
  - number: 1
    agents:
      - id: C
        task: |
          Implement ` + "`" + `NonExistentSymbol` + "`" + ` in pkg/mod/mod.go.
        files:
          - pkg/mod/mod.go
`

	implPath := filepath.Join(tmp, "IMPL-missing.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	result, err := ValidateBriefs(ctx, implPath)
	if err != nil {
		t.Fatalf("ValidateBriefs returned error: %v", err)
	}

	agentC := result.AgentResults["C"]
	found := false
	for _, issue := range agentC.Issues {
		if issue.Check == "symbol_missing" && issue.Symbol == "NonExistentSymbol" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected symbol_missing issue for NonExistentSymbol, got issues: %+v", agentC.Issues)
	}
}
