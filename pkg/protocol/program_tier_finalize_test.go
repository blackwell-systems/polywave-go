package protocol

import (
	"encoding/json"
	"os"
	"testing"
)

// TestFinalizeTier_TierNotFound verifies that FinalizeTier returns an error
// when the requested tier number is not present in the manifest.
func TestFinalizeTier_TierNotFound(t *testing.T) {
	// Write a minimal PROGRAM manifest with no tiers.
	content := `
title: Test Program
program_slug: test-prog
state: TIER_EXECUTING
impls: []
tiers: []
completion:
  tiers_complete: 0
  tiers_total: 0
  impls_complete: 0
  impls_total: 0
  total_agents: 0
  total_waves: 0
`
	f, err := os.CreateTemp("", "program-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	f.Close()

	_, err = FinalizeTier(f.Name(), 99, ".")
	if err == nil {
		t.Fatal("expected error for missing tier, got nil")
	}
}

// TestFinalizeTierData_JSONFields verifies that FinalizeTierData serializes
// with the expected JSON field names as documented in the interface contract.
func TestFinalizeTierData_JSONFields(t *testing.T) {
	d := &FinalizeTierData{
		TierNumber: 1,
		ImplMergeResults: map[string]*MergeAgentsResult{
			"my-impl": {
				Wave:    1,
				Merges:  []MergeStatus{{Agent: "my-impl", Branch: "saw/program/p/tier1-impl-my-impl", Success: true}},
				Success: true,
			},
		},
		TierGateResult: &TierGateResult{
			TierNumber:   1,
			Passed:       true,
			GateResults:  []GateResult{},
			ImplStatuses: []ImplTierStatus{},
			AllImplsDone: true,
		},
		Errors: nil,
	}

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("failed to marshal FinalizeTierData: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	expectedFields := []string{"tier_number", "impl_merge_results", "tier_gate_result"}
	for _, field := range expectedFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("expected JSON field %q to be present, but it was missing", field)
		}
	}

	// Verify errors field is omitted when nil (omitempty).
	if _, ok := decoded["errors"]; ok {
		t.Error("expected 'errors' field to be omitted when nil (omitempty), but it was present")
	}

	// Verify tier_number value.
	if tierNum, ok := decoded["tier_number"].(float64); !ok || int(tierNum) != 1 {
		t.Errorf("expected tier_number=1, got %v", decoded["tier_number"])
	}

	// Verify success field is absent (moved to result.Result wrapper).
	if _, ok := decoded["success"]; ok {
		t.Error("expected 'success' field to be absent from FinalizeTierData (moved to result.Result wrapper), but it was present")
	}
}
