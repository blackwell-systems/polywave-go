package suitability

import (
	"os"
	"path/filepath"
	"testing"
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

// Additional test cases to reach >100 lines
func TestEdgeCase1(t *testing.T) {
	// edge case test
}

func TestEdgeCase2(t *testing.T) {
	// edge case test
}

func TestEdgeCase3(t *testing.T) {
	// edge case test
}

func TestEdgeCase4(t *testing.T) {
	// edge case test
}

func TestEdgeCase5(t *testing.T) {
	// edge case test
}

func TestEdgeCase6(t *testing.T) {
	// edge case test
}

func TestEdgeCase7(t *testing.T) {
	// edge case test
}

func TestEdgeCase8(t *testing.T) {
	// edge case test
}

func TestEdgeCase9(t *testing.T) {
	// edge case test
}

func TestEdgeCase10(t *testing.T) {
	// edge case test
}

func TestEdgeCase11(t *testing.T) {
	// edge case test
}

func TestEdgeCase12(t *testing.T) {
	// edge case test
}

func TestEdgeCase13(t *testing.T) {
	// edge case test
}

func TestEdgeCase14(t *testing.T) {
	// edge case test
}

func TestEdgeCase15(t *testing.T) {
	// edge case test
}

func TestEdgeCase16(t *testing.T) {
	// edge case test
}

func TestEdgeCase17(t *testing.T) {
	// edge case test
}

func TestEdgeCase18(t *testing.T) {
	// edge case test
}

func TestEdgeCase19(t *testing.T) {
	// edge case test
}

func TestEdgeCase20(t *testing.T) {
	// edge case test
}
`

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

	result, err := ScanPreImplementation(tmpDir, requirements)
	if err != nil {
		t.Fatalf("ScanPreImplementation failed: %v", err)
	}

	if result.PreImplementation.TotalItems != 1 {
		t.Errorf("expected TotalItems=1, got %d", result.PreImplementation.TotalItems)
	}

	if result.PreImplementation.Done != 1 {
		t.Errorf("expected Done=1, got %d", result.PreImplementation.Done)
	}

	if result.PreImplementation.Partial != 0 {
		t.Errorf("expected Partial=0, got %d", result.PreImplementation.Partial)
	}

	if result.PreImplementation.Todo != 0 {
		t.Errorf("expected Todo=0, got %d", result.PreImplementation.Todo)
	}

	if result.PreImplementation.TimeSavedMinutes != 7 {
		t.Errorf("expected TimeSavedMinutes=7, got %d", result.PreImplementation.TimeSavedMinutes)
	}

	if len(result.PreImplementation.ItemStatus) != 1 {
		t.Fatalf("expected 1 item status, got %d", len(result.PreImplementation.ItemStatus))
	}

	item := result.PreImplementation.ItemStatus[0]
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

	result, err := ScanPreImplementation(tmpDir, requirements)
	if err != nil {
		t.Fatalf("ScanPreImplementation failed: %v", err)
	}

	if result.PreImplementation.TotalItems != 3 {
		t.Errorf("expected TotalItems=3, got %d", result.PreImplementation.TotalItems)
	}

	if result.PreImplementation.Done != 1 {
		t.Errorf("expected Done=1, got %d", result.PreImplementation.Done)
	}

	if result.PreImplementation.Partial != 1 {
		t.Errorf("expected Partial=1, got %d", result.PreImplementation.Partial)
	}

	if result.PreImplementation.Todo != 1 {
		t.Errorf("expected Todo=1, got %d", result.PreImplementation.Todo)
	}

	// Time saved: 1 DONE * 7 + 1 PARTIAL * 3 = 10 minutes
	expectedTime := 10
	if result.PreImplementation.TimeSavedMinutes != expectedTime {
		t.Errorf("expected TimeSavedMinutes=%d, got %d", expectedTime, result.PreImplementation.TimeSavedMinutes)
	}
}

func TestScanPreImplementation_AllTodo(t *testing.T) {
	tmpDir := t.TempDir()

	requirements := []Requirement{
		{ID: "F1", Description: "Missing feature 1", Files: []string{"missing1.go"}},
		{ID: "F2", Description: "Missing feature 2", Files: []string{"missing2.go"}},
		{ID: "F3", Description: "Missing feature 3", Files: []string{"missing3.go"}},
	}

	result, err := ScanPreImplementation(tmpDir, requirements)
	if err != nil {
		t.Fatalf("ScanPreImplementation failed: %v", err)
	}

	if result.PreImplementation.Done != 0 {
		t.Errorf("expected Done=0, got %d", result.PreImplementation.Done)
	}

	if result.PreImplementation.Partial != 0 {
		t.Errorf("expected Partial=0, got %d", result.PreImplementation.Partial)
	}

	if result.PreImplementation.Todo != 3 {
		t.Errorf("expected Todo=3, got %d", result.PreImplementation.Todo)
	}

	// All TODO: no time saved
	if result.PreImplementation.TimeSavedMinutes != 0 {
		t.Errorf("expected TimeSavedMinutes=0, got %d", result.PreImplementation.TimeSavedMinutes)
	}

	// Verify all items have correct status
	for i, item := range result.PreImplementation.ItemStatus {
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

// Helper function to generate test file padding
func generateTestPadding(lines int) string {
	result := "package test\n\nimport \"testing\"\n\n"
	for i := 0; i < lines; i++ {
		result += "// test line comment\n"
	}
	return result
}
