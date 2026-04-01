package idgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// TestAssignAgentIDs_Sequential_SmallCount tests sequential mode with a small count.
func TestAssignAgentIDs_Sequential_SmallCount(t *testing.T) {
	res := AssignAgentIDs(8, nil)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	expected := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d", len(expected), len(ids))
	}

	for i, want := range expected {
		if ids[i] != want {
			t.Errorf("index %d: expected %q, got %q", i, want, ids[i])
		}
	}
}

// TestAssignAgentIDs_Sequential_Exactly26 tests sequential mode with exactly 26 agents (A-Z).
func TestAssignAgentIDs_Sequential_Exactly26(t *testing.T) {
	res := AssignAgentIDs(26, nil)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	if len(ids) != 26 {
		t.Fatalf("expected 26 IDs, got %d", len(ids))
	}

	// Verify A-Z
	for i := 0; i < 26; i++ {
		expected := string(rune('A' + i))
		if ids[i] != expected {
			t.Errorf("index %d: expected %q, got %q", i, expected, ids[i])
		}
	}
}

// TestAssignAgentIDs_Sequential_MultiGeneration tests sequential mode spanning into second generation.
func TestAssignAgentIDs_Sequential_MultiGeneration(t *testing.T) {
	res := AssignAgentIDs(30, nil)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	if len(ids) != 30 {
		t.Fatalf("expected 30 IDs, got %d", len(ids))
	}

	// Verify A-Z (first 26)
	for i := 0; i < 26; i++ {
		expected := string(rune('A' + i))
		if ids[i] != expected {
			t.Errorf("index %d: expected %q, got %q", i, expected, ids[i])
		}
	}

	// Verify A2-D2 (next 4)
	secondGen := []string{"A2", "B2", "C2", "D2"}
	for i := 0; i < 4; i++ {
		idx := 26 + i
		if ids[idx] != secondGen[i] {
			t.Errorf("index %d: expected %q, got %q", idx, secondGen[i], ids[idx])
		}
	}
}

// TestAssignAgentIDs_Sequential_ThreeGenerations tests sequential mode spanning three generations.
func TestAssignAgentIDs_Sequential_ThreeGenerations(t *testing.T) {
	res := AssignAgentIDs(54, nil)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	if len(ids) != 54 {
		t.Fatalf("expected 54 IDs, got %d", len(ids))
	}

	// Verify first generation: A-Z (0-25)
	for i := 0; i < 26; i++ {
		expected := string(rune('A' + i))
		if ids[i] != expected {
			t.Errorf("index %d: expected %q, got %q", i, expected, ids[i])
		}
	}

	// Verify second generation: A2-Z2 (26-51)
	for i := 0; i < 26; i++ {
		expected := string(rune('A'+i)) + "2"
		idx := 26 + i
		if ids[idx] != expected {
			t.Errorf("index %d: expected %q, got %q", idx, expected, ids[idx])
		}
	}

	// Verify third generation: A3-B3 (52-53)
	thirdGen := []string{"A3", "B3"}
	for i := 0; i < 2; i++ {
		idx := 52 + i
		if ids[idx] != thirdGen[i] {
			t.Errorf("index %d: expected %q, got %q", idx, thirdGen[i], ids[idx])
		}
	}
}

// TestAssignAgentIDs_Grouped_Basic tests grouped mode with roadmap example.
func TestAssignAgentIDs_Grouped_Basic(t *testing.T) {
	// 3 data agents, 2 api agents, 4 ui agents
	grouping := [][]string{
		{"data"}, {"data"}, {"data"},
		{"api"}, {"api"},
		{"ui"}, {"ui"}, {"ui"}, {"ui"},
	}

	res := AssignAgentIDs(9, grouping)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	expected := []string{"A", "A2", "A3", "B", "B2", "C", "C2", "C3", "C4"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d", len(expected), len(ids))
	}

	for i, want := range expected {
		if ids[i] != want {
			t.Errorf("index %d: expected %q, got %q", i, want, ids[i])
		}
	}
}

// TestAssignAgentIDs_Grouped_SingleAgentPerGroup tests one agent per category.
func TestAssignAgentIDs_Grouped_SingleAgentPerGroup(t *testing.T) {
	grouping := [][]string{
		{"data"},
		{"api"},
		{"ui"},
	}

	res := AssignAgentIDs(3, grouping)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	expected := []string{"A", "B", "C"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d", len(expected), len(ids))
	}

	for i, want := range expected {
		if ids[i] != want {
			t.Errorf("index %d: expected %q, got %q", i, want, ids[i])
		}
	}
}

// TestAssignAgentIDs_Grouped_AllSameTag tests all agents with the same category.
func TestAssignAgentIDs_Grouped_AllSameTag(t *testing.T) {
	grouping := [][]string{
		{"data"}, {"data"}, {"data"}, {"data"}, {"data"},
	}

	res := AssignAgentIDs(5, grouping)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	expected := []string{"A", "A2", "A3", "A4", "A5"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d", len(expected), len(ids))
	}

	for i, want := range expected {
		if ids[i] != want {
			t.Errorf("index %d: expected %q, got %q", i, want, ids[i])
		}
	}
}

// TestAssignAgentIDs_Grouped_EmptyTags tests mix of empty and non-empty tags.
func TestAssignAgentIDs_Grouped_EmptyTags(t *testing.T) {
	grouping := [][]string{
		{},           // empty tag (treated as separate category)
		{"data"},     // first "data" agent
		{},           // another empty tag
		{"data"},     // second "data" agent
		{"api"},      // first "api" agent
	}

	res := AssignAgentIDs(5, grouping)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	// Expected behavior:
	// - Empty tag groups get their own letter (first appearance)
	// - "data" gets next letter, with multi-gen for subsequent "data" agents
	// - "api" gets next letter
	// Order of first appearance: "", "data", "api"
	// So: A (empty), B (data), A2 (empty), B2 (data), C (api)
	expected := []string{"A", "B", "A2", "B2", "C"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d", len(expected), len(ids))
	}

	for i, want := range expected {
		if ids[i] != want {
			t.Errorf("index %d: expected %q, got %q", i, want, ids[i])
		}
	}
}

// TestAssignAgentIDs_InvalidCount_Zero tests error on count=0.
func TestAssignAgentIDs_InvalidCount_Zero(t *testing.T) {
	res := AssignAgentIDs(0, nil)
	if !res.IsFatal() {
		t.Fatal("expected fatal error for count=0, got success")
	}
	if res.Errors[0].Code != result.CodeAgentCountInvalid {
		t.Errorf("expected code %s, got %s", result.CodeAgentCountInvalid, res.Errors[0].Code)
	}
	if !strings.Contains(res.Errors[0].Message, "count must be > 0") {
		t.Errorf("expected message to contain 'count must be > 0', got: %s", res.Errors[0].Message)
	}
}

// TestAssignAgentIDs_InvalidCount_Negative tests error on negative count.
func TestAssignAgentIDs_InvalidCount_Negative(t *testing.T) {
	res := AssignAgentIDs(-5, nil)
	if !res.IsFatal() {
		t.Fatal("expected fatal error for count=-5, got success")
	}
	if res.Errors[0].Code != result.CodeAgentCountInvalid {
		t.Errorf("expected code %s, got %s", result.CodeAgentCountInvalid, res.Errors[0].Code)
	}
	if !strings.Contains(res.Errors[0].Message, "count must be > 0") {
		t.Errorf("expected message to contain 'count must be > 0', got: %s", res.Errors[0].Message)
	}
}

// TestAssignAgentIDs_GroupingLengthMismatch tests error when grouping length != count.
func TestAssignAgentIDs_GroupingLengthMismatch(t *testing.T) {
	grouping := [][]string{
		{"data"}, {"api"}, {"ui"},
	}

	res := AssignAgentIDs(5, grouping)
	if !res.IsFatal() {
		t.Fatal("expected fatal error for grouping length mismatch, got success")
	}
	if res.Errors[0].Code != result.CodeAgentCountMismatch {
		t.Errorf("expected code %s, got %s", result.CodeAgentCountMismatch, res.Errors[0].Code)
	}
	if !strings.Contains(res.Errors[0].Message, "grouping length") {
		t.Errorf("expected message to contain 'grouping length', got: %s", res.Errors[0].Message)
	}
}

// TestAssignAgentIDs_NilGrouping tests that nil grouping triggers sequential mode.
func TestAssignAgentIDs_NilGrouping(t *testing.T) {
	res := AssignAgentIDs(5, nil)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	expected := []string{"A", "B", "C", "D", "E"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d", len(expected), len(ids))
	}

	for i, want := range expected {
		if ids[i] != want {
			t.Errorf("index %d: expected %q, got %q", i, want, ids[i])
		}
	}
}

// TestAssignAgentIDs_EmptyGrouping tests that empty grouping slice fails length-mismatch validation.
func TestAssignAgentIDs_EmptyGrouping(t *testing.T) {
	// Empty slice [][]string{} has length 0, which fails length-mismatch validation when count != 0.
	// Only nil triggers sequential mode; a non-nil empty slice is treated as grouped mode
	// and rejected by the length-mismatch check.
	res := AssignAgentIDs(5, [][]string{})
	if !res.IsFatal() {
		t.Fatal("expected fatal error for empty grouping with count=5, got success")
	}
	if res.Errors[0].Code != result.CodeAgentCountMismatch {
		t.Errorf("expected code %s, got %s", result.CodeAgentCountMismatch, res.Errors[0].Code)
	}
	if !strings.Contains(res.Errors[0].Message, "grouping length") {
		t.Errorf("expected message to contain 'grouping length', got: %s", res.Errors[0].Message)
	}
}

// TestAssignAgentIDs_MaxSupportedAgents tests maximum supported count (234 = 26*9).
func TestAssignAgentIDs_MaxSupportedAgents(t *testing.T) {
	res := AssignAgentIDs(234, nil)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got: %v", res.Errors)
	}
	ids := res.GetData()

	if len(ids) != 234 {
		t.Fatalf("expected 234 IDs, got %d", len(ids))
	}

	// Verify first ID is A
	if ids[0] != "A" {
		t.Errorf("first ID: expected A, got %q", ids[0])
	}

	// Verify last ID is Z9 (234th agent = 26*9)
	if ids[233] != "Z9" {
		t.Errorf("last ID: expected Z9, got %q", ids[233])
	}

	// Spot check some middle values
	// 26th agent (index 25) should be Z
	if ids[25] != "Z" {
		t.Errorf("26th agent: expected Z, got %q", ids[25])
	}

	// 27th agent (index 26) should be A2
	if ids[26] != "A2" {
		t.Errorf("27th agent: expected A2, got %q", ids[26])
	}

	// 52nd agent (index 51) should be Z2
	if ids[51] != "Z2" {
		t.Errorf("52nd agent: expected Z2, got %q", ids[51])
	}
}

// TestAssignAgentIDs_ExceedsMaximum tests error when count exceeds 234.
func TestAssignAgentIDs_ExceedsMaximum(t *testing.T) {
	res := AssignAgentIDs(235, nil)
	if !res.IsFatal() {
		t.Fatal("expected fatal error for count=235, got success")
	}
	if res.Errors[0].Code != result.CodeAgentLimitExceeded {
		t.Errorf("expected code %s, got %s", result.CodeAgentLimitExceeded, res.Errors[0].Code)
	}
	if !strings.Contains(res.Errors[0].Message, "exceeds maximum 234") {
		t.Errorf("expected message to contain 'exceeds maximum 234', got: %s", res.Errors[0].Message)
	}
}

// TestAssignAgentIDs_OutputMatchesRegex verifies all outputs match ^[A-Z][2-9]?$.
func TestAssignAgentIDs_OutputMatchesRegex(t *testing.T) {
	testCases := []struct {
		name     string
		count    int
		grouping [][]string
	}{
		{"Sequential_10", 10, nil},
		{"Sequential_50", 50, nil},
		{"Grouped_Basic", 9, [][]string{
			{"data"}, {"data"}, {"data"},
			{"api"}, {"api"},
			{"ui"}, {"ui"}, {"ui"}, {"ui"},
		}},
		{"MaxAgents", 234, nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := AssignAgentIDs(tc.count, tc.grouping)
			if !res.IsSuccess() {
				t.Fatalf("expected success, got: %v", res.Errors)
			}
			ids := res.GetData()

			for i, id := range ids {
				if !agentIDRegex.MatchString(id) {
					t.Errorf("index %d: ID %q does not match regex ^[A-Z][2-9]?$", i, id)
				}
			}
		})
	}
}

// TestAssignAgentIDs_Grouped_TooManyAgentsPerCategory tests that grouped mode rejects
// more than 9 agents in a single category.
func TestAssignAgentIDs_Grouped_TooManyAgentsPerCategory(t *testing.T) {
	// 10 entries all tagged "data" — exceeds the 9-per-category limit
	grouping := make([][]string, 10)
	for i := range grouping {
		grouping[i] = []string{"data"}
	}

	res := AssignAgentIDs(10, grouping)
	if !res.IsFatal() {
		t.Fatal("expected fatal error for >9 agents in a single category, got success")
	}
	if res.Errors[0].Code != result.CodeCategoryLimitExceeded {
		t.Errorf("expected code %s, got %s", result.CodeCategoryLimitExceeded, res.Errors[0].Code)
	}
	if !strings.Contains(res.Errors[0].Message, "category") || !strings.Contains(res.Errors[0].Message, "9") {
		t.Errorf("error message should mention \"category\" and \"9\", got: %q", res.Errors[0].Message)
	}
}

// TestAssignAgentIDs_Grouped_TooManyCategories tests that grouped mode rejects
// more than 26 distinct categories.
func TestAssignAgentIDs_Grouped_TooManyCategories(t *testing.T) {
	// 27 entries each with a unique category tag
	grouping := make([][]string, 27)
	for i := range grouping {
		grouping[i] = []string{fmt.Sprintf("cat%d", i)}
	}

	res := AssignAgentIDs(27, grouping)
	if !res.IsFatal() {
		t.Fatal("expected fatal error for >26 distinct categories, got success")
	}
	if res.Errors[0].Code != result.CodeCategoryCountExceeded {
		t.Errorf("expected code %s, got %s", result.CodeCategoryCountExceeded, res.Errors[0].Code)
	}
	if !strings.Contains(res.Errors[0].Message, "26") && !strings.Contains(res.Errors[0].Message, "categories") {
		t.Errorf("error message should mention \"26\" or \"categories\", got: %q", res.Errors[0].Message)
	}
}

// TestAssignAgentIDs_StabilityCheck verifies same inputs produce same outputs.
func TestAssignAgentIDs_StabilityCheck(t *testing.T) {
	testCases := []struct {
		name     string
		count    int
		grouping [][]string
	}{
		{"Sequential", 30, nil},
		{"Grouped", 9, [][]string{
			{"data"}, {"data"}, {"data"},
			{"api"}, {"api"},
			{"ui"}, {"ui"}, {"ui"}, {"ui"},
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate IDs three times
			res1 := AssignAgentIDs(tc.count, tc.grouping)
			if !res1.IsSuccess() {
				t.Fatalf("run 1 failed: %v", res1.Errors)
			}
			ids1 := res1.GetData()

			res2 := AssignAgentIDs(tc.count, tc.grouping)
			if !res2.IsSuccess() {
				t.Fatalf("run 2 failed: %v", res2.Errors)
			}
			ids2 := res2.GetData()

			res3 := AssignAgentIDs(tc.count, tc.grouping)
			if !res3.IsSuccess() {
				t.Fatalf("run 3 failed: %v", res3.Errors)
			}
			ids3 := res3.GetData()

			// Verify all three runs produce identical output
			if len(ids1) != len(ids2) || len(ids1) != len(ids3) {
				t.Fatalf("length mismatch: %d vs %d vs %d", len(ids1), len(ids2), len(ids3))
			}

			for i := 0; i < len(ids1); i++ {
				if ids1[i] != ids2[i] || ids1[i] != ids3[i] {
					t.Errorf("index %d: unstable output: %q vs %q vs %q", i, ids1[i], ids2[i], ids3[i])
				}
			}
		})
	}
}
