package suitability

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

func TestScanPreImplementation_AllDone(t *testing.T) {
	// Setup temporary directory with fully implemented files
	tmpDir := t.TempDir()

	mainFile := filepath.Join(tmpDir, "feature.go")
	testFile := filepath.Join(tmpDir, "feature_test.go")

	// Create a complete implementation
	mainContent := `package feature

import "fmt"

// FeatureFunc implements the core feature
func FeatureFunc() string {
	return "implemented"
}

// HelperFunc is a helper function
func HelperFunc(x int) int {
	return x * 2
}

// AnotherFunc does something else
func AnotherFunc() {
	fmt.Println("working")
}
`

	// Create a comprehensive test file (>100 lines for high coverage)
	testContent := `package feature

import "testing"

func TestFeatureFunc(t *testing.T) {
	result := FeatureFunc()
	if result != "implemented" {
		t.Errorf("expected 'implemented', got '%s'", result)
	}
}

func TestHelperFunc(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{1, 2},
		{2, 4},
		{3, 6},
		{0, 0},
		{-1, -2},
	}

	for _, tt := range tests {
		result := HelperFunc(tt.input)
		if result != tt.expected {
			t.Errorf("HelperFunc(%d) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestAnotherFunc(t *testing.T) {
	// Test that it doesn't panic
	AnotherFunc()
}
` + generateTestPadding(70)

	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	requirements := []Requirement{
		{
			ID:          "F1",
			Description: "Core feature implementation",
			Files:       []string{"feature.go"},
		},
	}

	scanResult := ScanPreImplementation(tmpDir, requirements)
	if scanResult.IsFatal() {
		t.Fatalf("ScanPreImplementation failed: %v", scanResult.Errors)
	}

	data := scanResult.GetData()

	if data.PreImplementation.TotalItems != 1 {
		t.Errorf("expected TotalItems=1, got %d", data.PreImplementation.TotalItems)
	}

	if data.PreImplementation.Done != 1 {
		t.Errorf("expected Done=1, got %d", data.PreImplementation.Done)
	}

	if data.PreImplementation.Partial != 0 {
		t.Errorf("expected Partial=0, got %d", data.PreImplementation.Partial)
	}

	if data.PreImplementation.Todo != 0 {
		t.Errorf("expected Todo=0, got %d", data.PreImplementation.Todo)
	}

	if data.PreImplementation.TimeSavedMinutes != 7 {
		t.Errorf("expected TimeSavedMinutes=7, got %d", data.PreImplementation.TimeSavedMinutes)
	}

	if len(data.PreImplementation.ItemStatus) != 1 {
		t.Fatalf("expected 1 item status, got %d", len(data.PreImplementation.ItemStatus))
	}

	item := data.PreImplementation.ItemStatus[0]
	if item.Status != "DONE" {
		t.Errorf("expected status=DONE, got %s", item.Status)
	}

	if item.Completeness != 1.0 {
		t.Errorf("expected completeness=1.0, got %f", item.Completeness)
	}

	if item.TestCoverage != "high" {
		t.Errorf("expected test coverage=high, got %s", item.TestCoverage)
	}
}

func TestScanPreImplementation_Mixed(t *testing.T) {
	tmpDir := t.TempDir()

	// DONE: Complete implementation with tests
	doneFile := filepath.Join(tmpDir, "done.go")
	doneTest := filepath.Join(tmpDir, "done_test.go")
	doneContent := `package main

func DoneFunc() string {
	return "complete"
}

// Ten lines of implementation
func Helper1() {}
func Helper2() {}
func Helper3() {}
func Helper4() {}
func Helper5() {}
func Helper6() {}
func Helper7() {}
func Helper8() {}
func Helper9() {}
`
	doneTestContent := `package main
import "testing"

func TestDoneFunc(t *testing.T) {
	if DoneFunc() != "complete" {
		t.Error("failed")
	}
}

// Padding to reach >100 lines
` + generateTestPadding(95)

	// PARTIAL: Has TODO comments
	partialFile := filepath.Join(tmpDir, "partial.go")
	partialTest := filepath.Join(tmpDir, "partial_test.go")
	partialContent := `package main

func PartialFunc() string {
	// TODO: implement error handling
	return "partial"
}

func AnotherPartial() {
	// FIXME: this is a stub
}

// More lines for implementation detection
func Extra1() {}
func Extra2() {}
func Extra3() {}
func Extra4() {}
func Extra5() {}
func Extra6() {}
func Extra7() {}
`
	partialTestContent := `package main
import "testing"

func TestPartial(t *testing.T) {
	// 60 lines = medium coverage
}
` + generateTestPadding(55)

	// TODO: File doesn't exist (referenced in requirements but not created)
	// todoFile would be filepath.Join(tmpDir, "todo.go") but we don't create it

	// Write files
	if err := os.WriteFile(doneFile, []byte(doneContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doneTest, []byte(doneTestContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partialFile, []byte(partialContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partialTest, []byte(partialTestContent), 0644); err != nil {
		t.Fatal(err)
	}

	requirements := []Requirement{
		{ID: "F1", Description: "Done feature", Files: []string{"done.go"}},
		{ID: "F2", Description: "Partial feature", Files: []string{"partial.go"}},
		{ID: "F3", Description: "Todo feature", Files: []string{"todo.go"}},
	}

	scanResult := ScanPreImplementation(tmpDir, requirements)
	if scanResult.IsFatal() {
		t.Fatalf("ScanPreImplementation failed: %v", scanResult.Errors)
	}

	data := scanResult.GetData()

	if data.PreImplementation.TotalItems != 3 {
		t.Errorf("expected TotalItems=3, got %d", data.PreImplementation.TotalItems)
	}

	if data.PreImplementation.Done != 1 {
		t.Errorf("expected Done=1, got %d", data.PreImplementation.Done)
	}

	if data.PreImplementation.Partial != 1 {
		t.Errorf("expected Partial=1, got %d", data.PreImplementation.Partial)
	}

	if data.PreImplementation.Todo != 1 {
		t.Errorf("expected Todo=1, got %d", data.PreImplementation.Todo)
	}

	// Time saved: 1 DONE * 7 + 1 PARTIAL * 3 = 10 minutes
	expectedTime := 10
	if data.PreImplementation.TimeSavedMinutes != expectedTime {
		t.Errorf("expected TimeSavedMinutes=%d, got %d", expectedTime, data.PreImplementation.TimeSavedMinutes)
	}
}

func TestScanPreImplementation_AllTodo(t *testing.T) {
	tmpDir := t.TempDir()

	requirements := []Requirement{
		{ID: "F1", Description: "Missing feature 1", Files: []string{"missing1.go"}},
		{ID: "F2", Description: "Missing feature 2", Files: []string{"missing2.go"}},
		{ID: "F3", Description: "Missing feature 3", Files: []string{"missing3.go"}},
	}

	scanResult := ScanPreImplementation(tmpDir, requirements)
	if scanResult.IsFatal() {
		t.Fatalf("ScanPreImplementation failed: %v", scanResult.Errors)
	}

	data := scanResult.GetData()

	if data.PreImplementation.Done != 0 {
		t.Errorf("expected Done=0, got %d", data.PreImplementation.Done)
	}

	if data.PreImplementation.Partial != 0 {
		t.Errorf("expected Partial=0, got %d", data.PreImplementation.Partial)
	}

	if data.PreImplementation.Todo != 3 {
		t.Errorf("expected Todo=3, got %d", data.PreImplementation.Todo)
	}

	// All TODO: no time saved
	if data.PreImplementation.TimeSavedMinutes != 0 {
		t.Errorf("expected TimeSavedMinutes=0, got %d", data.PreImplementation.TimeSavedMinutes)
	}

	// Verify all items have correct status
	for i, item := range data.PreImplementation.ItemStatus {
		if item.Status != "TODO" {
			t.Errorf("item %d: expected status=TODO, got %s", i, item.Status)
		}
		if item.Completeness != 0.0 {
			t.Errorf("item %d: expected completeness=0.0, got %f", i, item.Completeness)
		}
	}
}

func TestClassifyFile_Done(t *testing.T) {
	tmpDir := t.TempDir()
	mainFile := filepath.Join(tmpDir, "impl.go")
	testFile := filepath.Join(tmpDir, "impl_test.go")

	mainContent := `package test

func Implementation() {
	// Complete implementation
	doSomething()
	doSomethingElse()
	finalize()
}

func doSomething() {}
func doSomethingElse() {}
func finalize() {}
`

	testContent := generateTestPadding(105) // >100 lines = high coverage

	if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatal(err)
	}

	req := Requirement{ID: "T1", Description: "Test", Files: []string{mainFile}}
	status, err := ClassifyFile(mainFile, req)
	if err != nil {
		t.Fatalf("ClassifyFile failed: %v", err)
	}

	if status.Status != "DONE" {
		t.Errorf("expected status=DONE, got %s", status.Status)
	}

	if status.Completeness != 1.0 {
		t.Errorf("expected completeness=1.0, got %f", status.Completeness)
	}

	if status.TestCoverage != "high" {
		t.Errorf("expected test coverage=high, got %s", status.TestCoverage)
	}
}

func TestClassifyFile_Partial(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "partial.go")

	content := `package test

func PartialImplementation() {
	// TODO: add error handling
	// FIXME: optimize this
	doBasicStuff()
}

func doBasicStuff() {}
func helper1() {}
func helper2() {}
func helper3() {}
func helper4() {}
func helper5() {}
`

	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	req := Requirement{ID: "T1", Description: "Test", Files: []string{file}}
	status, err := ClassifyFile(file, req)
	if err != nil {
		t.Fatalf("ClassifyFile failed: %v", err)
	}

	if status.Status != "PARTIAL" {
		t.Errorf("expected status=PARTIAL, got %s", status.Status)
	}

	if status.Completeness <= 0.0 || status.Completeness >= 1.0 {
		t.Errorf("expected completeness between 0 and 1, got %f", status.Completeness)
	}

	if len(status.Missing) == 0 {
		t.Error("expected missing items to be populated")
	}
}

func TestClassifyFile_Todo(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "nonexistent.go")

	req := Requirement{ID: "T1", Description: "Test", Files: []string{file}}
	status, err := ClassifyFile(file, req)
	if err != nil {
		t.Fatalf("ClassifyFile failed: %v", err)
	}

	if status.Status != "TODO" {
		t.Errorf("expected status=TODO, got %s", status.Status)
	}

	if status.Completeness != 0.0 {
		t.Errorf("expected completeness=0.0, got %f", status.Completeness)
	}

	if status.TestCoverage != "none" {
		t.Errorf("expected test coverage=none, got %s", status.TestCoverage)
	}

	if len(status.Missing) == 0 {
		t.Error("expected missing items to include 'file does not exist'")
	}
}

func TestClassifyFile_TestCoverage(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name             string
		testLines        int
		expectedCoverage string
	}{
		{"high coverage", 105, "high"},
		{"medium coverage", 75, "medium"},
		{"low coverage", 30, "low"},
		{"no tests", 0, "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mainFile := filepath.Join(tmpDir, tt.name+".go")
			testFile := filepath.Join(tmpDir, tt.name+"_test.go")

			mainContent := `package test

func TestableFunc() {
	implementation()
	moreImplementation()
	evenMore()
}

func implementation() {}
func moreImplementation() {}
func evenMore() {}
func helper1() {}
func helper2() {}
`

			if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
				t.Fatal(err)
			}

			if tt.testLines > 0 {
				testContent := generateTestPadding(tt.testLines)
				if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
					t.Fatal(err)
				}
			}

			req := Requirement{ID: "T1", Description: "Test", Files: []string{mainFile}}
			status, err := ClassifyFile(mainFile, req)
			if err != nil {
				t.Fatalf("ClassifyFile failed: %v", err)
			}

			if status.TestCoverage != tt.expectedCoverage {
				t.Errorf("expected test coverage=%s, got %s", tt.expectedCoverage, status.TestCoverage)
			}
		})
	}
}

// TestCheckTestCoverage_Boundaries verifies the boundary conditions of checkTestCoverage.
func TestCheckTestCoverage_Boundaries(t *testing.T) {
	tests := []struct {
		name     string
		lines    int
		expected string
	}{
		{"exactly 49 lines", 49, "low"},
		{"exactly 50 lines", 50, "medium"},
		{"exactly 100 lines", 100, "medium"},
		{"exactly 101 lines", 101, "high"},
		{"non-existent file", -1, "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lines < 0 {
				// Non-existent file path
				got := checkTestCoverage("/nonexistent/path/file_test.go")
				if got != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, got)
				}
				return
			}

			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "file_test.go")

			// Build a string with exactly tt.lines lines as counted by strings.Split.
			// strings.Split counts the trailing empty element from a trailing newline,
			// so we build lines joined by \n without a trailing newline to get exact counts.
			lines := make([]string, tt.lines)
			for i := range lines {
				lines[i] = "// line"
			}
			content := strings.Join(lines, "\n")
			if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}

			got := checkTestCoverage(testFile)
			if got != tt.expected {
				t.Errorf("lines=%d: expected %q, got %q", tt.lines, tt.expected, got)
			}
		})
	}
}

// TestContainsTodoPatterns_StubFalsePositive verifies that word-boundary anchoring
// prevents identifiers like NewStubServer and buildstub from triggering the stub pattern.
func TestContainsTodoPatterns_StubFalsePositive(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"NewStubServer identifier", "func NewStubServer() {}", false},
		{"stubService type", "type stubService struct{}", false},
		{"bare stub word in comment", "// stub implementation", true},
		{"buildstub identifier", "x := buildstub()", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsTodoPatterns(tt.content)
			if got != tt.expected {
				t.Errorf("containsTodoPatterns(%q) = %v, expected %v", tt.content, got, tt.expected)
			}
		})
	}
}

// TestParseRequirements_HeaderWithoutLocation verifies that a header with no
// Location field is still included in the result (with Files == nil), rather
// than being silently dropped.
func TestParseRequirements_HeaderWithoutLocation(t *testing.T) {
	content := "## F1: Add authentication handler\n\nSome description but no location field.\n"

	reqResult := ParseRequirements(content)
	if reqResult.IsFatal() {
		t.Fatalf("ParseRequirements returned error: %v", reqResult.Errors)
	}

	reqs := reqResult.GetData()

	if len(reqs) != 1 {
		t.Fatalf("expected 1 requirement, got %d", len(reqs))
	}

	req := reqs[0]
	if req.ID != "F1" {
		t.Errorf("expected ID=F1, got %q", req.ID)
	}

	if len(req.Files) != 0 {
		t.Errorf("expected Files to be nil/empty, got %v", req.Files)
	}
}

// TestAnalyzeSuitability_EmptyFile verifies that an empty requirementsFile
// returns success with a zero-initialized struct (not an error).
func TestAnalyzeSuitability_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	analysisResult := AnalyzeSuitability("", tmpDir)
	if !analysisResult.IsSuccess() {
		t.Errorf("expected success, got %s with errors: %v", analysisResult.Code, analysisResult.Errors)
	}
	data := analysisResult.GetData()
	if data.PreImplementation.TotalItems != 0 {
		t.Errorf("expected 0 items, got %d", data.PreImplementation.TotalItems)
	}
}

// TestAnalyzeSuitability_FileNotFound verifies that a missing requirements file
// returns success (not error), treating it as "no requirements to check".
func TestAnalyzeSuitability_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentFile := filepath.Join(tmpDir, "nonexistent-requirements.md")
	analysisResult := AnalyzeSuitability(nonexistentFile, tmpDir)
	if !analysisResult.IsSuccess() {
		t.Errorf("expected success for missing file, got %s with errors: %v", analysisResult.Code, analysisResult.Errors)
	}
	data := analysisResult.GetData()
	if data.PreImplementation.TotalItems != 0 {
		t.Errorf("expected 0 items, got %d", data.PreImplementation.TotalItems)
	}
}

// TestAnalyzeSuitability_EmptyRequirements verifies that a requirements file
// with no parseable requirements returns success with empty results.
func TestAnalyzeSuitability_EmptyRequirements(t *testing.T) {
	tmpDir := t.TempDir()
	reqFile := filepath.Join(tmpDir, "requirements.md")
	emptyContent := "# Requirements Document\n\nNo actual requirements here.\n"
	if err := os.WriteFile(reqFile, []byte(emptyContent), 0644); err != nil {
		t.Fatal(err)
	}

	analysisResult := AnalyzeSuitability(reqFile, tmpDir)
	if !analysisResult.IsSuccess() {
		t.Errorf("expected success for empty requirements, got %s with errors: %v", analysisResult.Code, analysisResult.Errors)
	}
	data := analysisResult.GetData()
	if data.PreImplementation.TotalItems != 0 {
		t.Errorf("expected 0 items, got %d", data.PreImplementation.TotalItems)
	}
}

// TestParseRequirements_HeaderWithLocation verifies that markdown parsing
// correctly extracts the Location field into the Files slice.
func TestParseRequirements_HeaderWithLocation(t *testing.T) {
	content := `## F1: Add authentication handler

Some description of the feature.

Location: pkg/auth/handler.go
Location: pkg/auth/middleware.go

More details here.

## F2: Add database migration

Another requirement.

Location: migrations/001_init.sql
`

	reqResult := ParseRequirements(content)
	if reqResult.IsFatal() {
		t.Fatalf("ParseRequirements failed: %v", reqResult.Errors)
	}

	reqs := reqResult.GetData()

	if len(reqs) != 2 {
		t.Fatalf("expected 2 requirements, got %d", len(reqs))
	}

	// Check F1
	if reqs[0].ID != "F1" {
		t.Errorf("expected ID=F1, got %q", reqs[0].ID)
	}
	if len(reqs[0].Files) != 2 {
		t.Fatalf("expected 2 files for F1, got %d", len(reqs[0].Files))
	}
	if reqs[0].Files[0] != "pkg/auth/handler.go" {
		t.Errorf("expected first file='pkg/auth/handler.go', got %q", reqs[0].Files[0])
	}
	if reqs[0].Files[1] != "pkg/auth/middleware.go" {
		t.Errorf("expected second file='pkg/auth/middleware.go', got %q", reqs[0].Files[1])
	}

	// Check F2
	if reqs[1].ID != "F2" {
		t.Errorf("expected ID=F2, got %q", reqs[1].ID)
	}
	if len(reqs[1].Files) != 1 {
		t.Fatalf("expected 1 file for F2, got %d", len(reqs[1].Files))
	}
	if reqs[1].Files[0] != "migrations/001_init.sql" {
		t.Errorf("expected file='migrations/001_init.sql', got %q", reqs[1].Files[0])
	}
}

func TestScanPreImplementation_RepoRootNotExist(t *testing.T) {
	nonexistent := "/nonexistent/path/that/does/not/exist"
	r := ScanPreImplementation(nonexistent, []Requirement{
		{ID: "R1", Description: "test", Files: []string{"foo.go"}},
	})
	if !r.IsFatal() {
		t.Fatal("expected fatal result for non-existent repoRoot")
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if r.Errors[0].Code != result.CodeSuitabilityFileStatFailed {
		t.Errorf("expected code %s, got %s",
			result.CodeSuitabilityFileStatFailed, r.Errors[0].Code)
	}
}

func TestScanPreImplementation_CountInvariant(t *testing.T) {
	tmpDir := t.TempDir()
	// Write a partial file (has a TODO marker)
	partialContent := "package x\n\nfunc F() {\n\t// TODO: implement\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "partial.go"), []byte(partialContent), 0644); err != nil {
		t.Fatal(err)
	}
	reqs := []Requirement{
		{ID: "R1", Description: "partial req", Files: []string{"partial.go"}},
		{ID: "R2", Description: "missing req", Files: []string{"missing.go"}},
	}
	r := ScanPreImplementation(tmpDir, reqs)
	if r.IsFatal() {
		t.Fatalf("unexpected fatal: %v", r.Errors)
	}
	d := r.GetData()
	total := d.PreImplementation.TotalItems
	counted := d.PreImplementation.Done + d.PreImplementation.Partial + d.PreImplementation.Todo
	if counted != total {
		t.Errorf("count invariant violated: Done(%d)+Partial(%d)+Todo(%d)=%d != TotalItems(%d)",
			d.PreImplementation.Done, d.PreImplementation.Partial,
			d.PreImplementation.Todo, counted, total)
	}
}

func TestClassifyFile_UnreadableFilePopulatesIDAndFile(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read mode-0000 files; skip unreadable-file test")
	}
	tmpDir := t.TempDir()
	unreadable := filepath.Join(tmpDir, "unreadable.go")
	if err := os.WriteFile(unreadable, []byte("package x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0000); err != nil {
		t.Fatal(err)
	}
	r := ScanPreImplementation(tmpDir, []Requirement{
		{ID: "R1", Description: "unreadable", Files: []string{"unreadable.go"}},
	})
	if !r.IsFatal() {
		t.Fatal("expected fatal result for unreadable file")
	}
}

func TestClassifyFile_NonExistReadErrorUsesS004(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can read mode-0000 files; skip unreadable-file test")
	}
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "locked.go")
	if err := os.WriteFile(f, []byte("package x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f, 0000); err != nil {
		t.Fatal(err)
	}
	req := Requirement{ID: "T1", Description: "locked", Files: []string{f}}
	_, err := ClassifyFile(f, req)
	if err == nil {
		t.Fatal("expected error for unreadable file")
	}
	var sawErr result.SAWError
	if !errors.As(err, &sawErr) {
		t.Fatalf("expected result.SAWError, got %T: %v", err, err)
	}
	if sawErr.Code != result.CodeSuitabilityFileReadFailed {
		t.Errorf("expected code %s, got %s",
			result.CodeSuitabilityFileReadFailed, sawErr.Code)
	}
}

// Helper function to generate test file padding
func generateTestPadding(lines int) string {
	result := "package test\n\nimport \"testing\"\n\n"
	for i := 0; i < lines; i++ {
		result += "// test line comment\n"
	}
	return result
}
