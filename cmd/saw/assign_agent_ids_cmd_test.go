package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestAssignAgentIDsCmd_Sequential tests sequential mode output.
func TestAssignAgentIDsCmd_Sequential(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "8"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	expected := "A B C D E F G H"

	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

// TestAssignAgentIDsCmd_Grouped tests grouped mode with JSON grouping.
func TestAssignAgentIDsCmd_Grouped(t *testing.T) {
	groupingJSON := `[["data"],["data"],["data"],["api"],["api"],["ui"],["ui"],["ui"],["ui"]]`

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "9", "--grouping", groupingJSON})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	expected := "A A2 A3 B B2 C C2 C3 C4"

	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

// TestAssignAgentIDsCmd_MissingCount tests error when --count flag is missing.
func TestAssignAgentIDsCmd_MissingCount(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{}) // No --count flag

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --count flag, got nil")
	}

	// Cobra returns error for missing required flag
	errMsg := err.Error()
	if !strings.Contains(errMsg, "required flag") && !strings.Contains(errMsg, "count") {
		t.Errorf("expected error about required 'count' flag, got: %v", err)
	}
}

// TestAssignAgentIDsCmd_InvalidJSON tests error when --grouping contains invalid JSON.
func TestAssignAgentIDsCmd_InvalidJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "3", "--grouping", "not-json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid --grouping JSON") {
		t.Errorf("expected error about invalid JSON, got: %v", err)
	}
}

// TestAssignAgentIDsCmd_InvalidCount tests error when --count is negative.
func TestAssignAgentIDsCmd_InvalidCount(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "-1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for count=-1, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "count must be > 0") {
		t.Errorf("expected error about invalid count, got: %v", err)
	}
}

// TestAssignAgentIDsCmd_OutputFormat verifies space-separated single-line output.
func TestAssignAgentIDsCmd_OutputFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "5"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := stdout.String()

	// Should be single line with trailing newline
	lines := strings.Split(output, "\n")
	if len(lines) != 2 || lines[1] != "" {
		t.Errorf("expected single line with trailing newline, got %d lines: %v", len(lines), lines)
	}

	// First line should be space-separated IDs
	firstLine := lines[0]
	ids := strings.Split(firstLine, " ")
	if len(ids) != 5 {
		t.Errorf("expected 5 space-separated IDs, got %d: %v", len(ids), ids)
	}

	// Verify format: A B C D E
	expectedIDs := []string{"A", "B", "C", "D", "E"}
	for i, id := range ids {
		if id != expectedIDs[i] {
			t.Errorf("ID %d: expected %q, got %q", i, expectedIDs[i], id)
		}
	}
}

// TestAssignAgentIDsCmd_LargeCount tests output with large count (no truncation).
func TestAssignAgentIDsCmd_LargeCount(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "100"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	ids := strings.Split(output, " ")

	if len(ids) != 100 {
		t.Errorf("expected 100 IDs, got %d", len(ids))
	}

	// Verify first and last IDs
	if ids[0] != "A" {
		t.Errorf("first ID: expected A, got %q", ids[0])
	}

	// 100th agent: 100 = 26*3 + 22, so generation 4, letter 22 (W) → W4
	if ids[99] != "W4" {
		t.Errorf("100th ID: expected W4, got %q", ids[99])
	}

	// Spot check: 27th agent (index 26) should be A2
	if ids[26] != "A2" {
		t.Errorf("27th ID: expected A2, got %q", ids[26])
	}
}

// TestAssignAgentIDsCmd_GroupingLengthMismatch tests error when grouping length != count.
func TestAssignAgentIDsCmd_GroupingLengthMismatch(t *testing.T) {
	groupingJSON := `[["data"],["api"],["ui"]]` // 3 elements

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "5", "--grouping", groupingJSON})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for grouping length mismatch, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "grouping length") || !strings.Contains(errMsg, "must match count") {
		t.Errorf("expected error about grouping length mismatch, got: %v", err)
	}
}

// TestAssignAgentIDsCmd_EmptyGrouping tests that empty grouping array triggers sequential mode.
func TestAssignAgentIDsCmd_EmptyGrouping(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "5", "--grouping", "[]"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	expected := "A B C D E"

	if output != expected {
		t.Errorf("expected sequential mode output %q, got %q", expected, output)
	}
}

// TestAssignAgentIDsCmd_MultiGenerationSequential tests sequential mode spanning multiple generations.
func TestAssignAgentIDsCmd_MultiGenerationSequential(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "30"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	ids := strings.Split(output, " ")

	if len(ids) != 30 {
		t.Fatalf("expected 30 IDs, got %d", len(ids))
	}

	// Verify last 4 are A2 B2 C2 D2
	lastFour := []string{"A2", "B2", "C2", "D2"}
	for i := 0; i < 4; i++ {
		idx := 26 + i
		if ids[idx] != lastFour[i] {
			t.Errorf("ID %d: expected %q, got %q", idx, lastFour[i], ids[idx])
		}
	}
}

// TestAssignAgentIDsCmd_ComplexGrouping tests grouped mode with complex category structure.
func TestAssignAgentIDsCmd_ComplexGrouping(t *testing.T) {
	// Mix of different categories with varying group sizes
	groupingJSON := `[["backend"],["backend"],["frontend"],["frontend"],["frontend"],["db"],["cache"]]`

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "7", "--grouping", groupingJSON})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())

	// Expected: backend (A, A2), frontend (B, B2, B3), db (C), cache (D)
	expected := "A A2 B B2 B3 C D"

	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

// TestAssignAgentIDsCmd_ZeroCount tests error when --count is zero.
func TestAssignAgentIDsCmd_ZeroCount(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "0"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for count=0, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "count must be > 0") {
		t.Errorf("expected error about invalid count, got: %v", err)
	}
}

// TestAssignAgentIDsCmd_ExceedsMaximum tests error when count exceeds 234.
func TestAssignAgentIDsCmd_ExceedsMaximum(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "235"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for count=235, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "exceeds maximum 234 agents") {
		t.Errorf("expected error about exceeding maximum, got: %v", err)
	}
}

// TestAssignAgentIDsCmd_MalformedGroupingArray tests error for malformed JSON array.
func TestAssignAgentIDsCmd_MalformedGroupingArray(t *testing.T) {
	testCases := []struct {
		name         string
		groupingJSON string
	}{
		{"MissingBracket", `[["data"],["api"]`},
		{"ExtraBracket", `[["data"],["api"]]]`},
		{"NotAnArray", `{"data": "value"}`},
		{"EmptyString", ``},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			cmd := newAssignAgentIDsCmd()
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--count", "2", "--grouping", tc.groupingJSON})

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error for malformed JSON, got nil")
			}

			// For empty string, the error might be different, but should still fail
			if tc.name != "EmptyString" {
				errMsg := err.Error()
				if !strings.Contains(errMsg, "invalid --grouping JSON") {
					t.Errorf("expected error about invalid JSON, got: %v", err)
				}
			}
		})
	}
}

// TestAssignAgentIDsCmd_GroupingWithMultipleTags tests grouping with multiple tags per agent.
func TestAssignAgentIDsCmd_GroupingWithMultipleTags(t *testing.T) {
	// Agents can have multiple tags, but only first tag is used for grouping
	groupingJSON := `[["data","backend"],["data","cache"],["api","frontend"]]`

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "3", "--grouping", groupingJSON})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())

	// Expected: first two "data" agents (A, A2), then "api" agent (B)
	expected := "A A2 B"

	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

// TestAssignAgentIDsCmd_ValidJSONButNotArrayOfArrays tests error for wrong JSON structure.
func TestAssignAgentIDsCmd_ValidJSONButNotArrayOfArrays(t *testing.T) {
	// Valid JSON but not [][]string
	groupingJSON := `["data","api","ui"]`

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "3", "--grouping", groupingJSON})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for wrong JSON structure, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "invalid --grouping JSON") {
		t.Errorf("expected error about invalid JSON, got: %v", err)
	}
}

// TestAssignAgentIDsCmd_NoStdoutPollution verifies no extra output besides IDs.
func TestAssignAgentIDsCmd_NoStdoutPollution(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "3"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()

	// Should only contain IDs and newline, no extra messages
	trimmed := strings.TrimSpace(output)
	if trimmed != "A B C" {
		t.Errorf("expected clean output 'A B C', got %q", trimmed)
	}

	// Stderr should be empty (no warnings or info messages)
	if stderr.Len() > 0 {
		t.Errorf("expected no stderr output, got: %s", stderr.String())
	}
}

// TestAssignAgentIDsCmd_ParsesJSONCorrectly verifies JSON unmarshaling works as expected.
func TestAssignAgentIDsCmd_ParsesJSONCorrectly(t *testing.T) {
	// Create a known grouping structure
	grouping := [][]string{
		{"layer1"},
		{"layer1"},
		{"layer2"},
	}

	// Marshal to JSON
	groupingBytes, err := json.Marshal(grouping)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newAssignAgentIDsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--count", "3", "--grouping", string(groupingBytes)})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	expected := "A A2 B"

	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}
