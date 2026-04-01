package protocol

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrepareTier_ManifestNotFound verifies that PrepareTier returns an error
// when the manifest file does not exist.
func TestPrepareTier_ManifestNotFound(t *testing.T) {
	_, err := PrepareTier("/nonexistent/path/program.yaml", 1, ".")
	if err == nil {
		t.Fatal("expected error for non-existent manifest path, got nil")
	}
}

// TestPrepareTier_TierNotFound verifies that PrepareTier returns an error
// when the requested tier number is not present in the manifest.
func TestPrepareTier_TierNotFound(t *testing.T) {
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

	_, err = PrepareTier(f.Name(), 99, ".")
	if err == nil {
		t.Fatal("expected error for missing tier, got nil")
	}
}

// TestPrepareTierResult_JSONFields verifies that PrepareTierResult serializes
// with the expected JSON field names as documented in the interface contract.
func TestPrepareTierResult_JSONFields(t *testing.T) {
	r := &PrepareTierResult{
		Tier: 1,
		ConflictCheck: &ConflictCheckResult{
			Conflicts: []IMPLFileConflict{},
			Disjoint:  true,
		},
		Validations: []IMPLValidationResult{
			{ImplSlug: "my-impl", Valid: true, Fixed: 0},
		},
		Branches: []ProgramWorktreeInfo{
			{ImplSlug: "my-impl", Path: "/tmp/wt", Branch: "saw/program/p/tier1-impl-my-impl"},
		},
		Success: true,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal PrepareTierResult: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	expectedFields := []string{"tier", "conflict_check", "validations", "branches", "success"}
	for _, field := range expectedFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("expected JSON field %q to be present, but it was missing", field)
		}
	}

	if tierNum, ok := decoded["tier"].(float64); !ok || int(tierNum) != 1 {
		t.Errorf("expected tier=1, got %v", decoded["tier"])
	}

	if success, ok := decoded["success"].(bool); !ok || !success {
		t.Errorf("expected success=true, got %v", decoded["success"])
	}
}

// TestConflictCheckResult_JSONFields verifies ConflictCheckResult serialization.
func TestConflictCheckResult_JSONFields(t *testing.T) {
	r := &ConflictCheckResult{
		Conflicts: []IMPLFileConflict{},
		Disjoint:  true,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal ConflictCheckResult: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if _, ok := decoded["conflicts"]; !ok {
		t.Error("expected JSON field 'conflicts' to be present")
	}
	if _, ok := decoded["disjoint"]; !ok {
		t.Error("expected JSON field 'disjoint' to be present")
	}
}

// TestIMPLValidationResult_JSONOmitEmpty verifies that Errors field is omitted
// when empty (omitempty tag).
func TestIMPLValidationResult_JSONOmitEmpty(t *testing.T) {
	r := &IMPLValidationResult{
		ImplSlug: "test-impl",
		Valid:    true,
		Fixed:    0,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal IMPLValidationResult: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if _, ok := decoded["errors"]; ok {
		t.Error("expected 'errors' field to be omitted when nil (omitempty), but it was present")
	}
}

// writeTempIMPL creates a minimal valid IMPL YAML at {repoDir}/docs/IMPL/IMPL-{slug}.yaml.
// agentCount controls how many agents appear in wave 1.
// criticReport is an optional YAML block to include under critic_report:.
// Returns the IMPL path.
func writeTempIMPL(t *testing.T, repoDir, slug string, agentCount int, criticReport string) string {
	t.Helper()

	implDir := filepath.Join(repoDir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatalf("failed to create docs/IMPL dir: %v", err)
	}

	// Build agents and file_ownership entries.
	agentLines := ""
	ownershipLines := ""
	for i := 0; i < agentCount; i++ {
		id := string(rune('A' + i))
		file := fmt.Sprintf("pkg/test/%s_file.go", strings.ToLower(id))
		agentLines += fmt.Sprintf("      - id: %s\n        task: implement part %s\n        files:\n          - %s\n", id, id, file)
		ownershipLines += fmt.Sprintf("  - file: %s\n    agent: %s\n    wave: 1\n    action: new\n", file, id)
	}

	criticBlock := ""
	if criticReport != "" {
		criticBlock = "critic_report:\n" + criticReport
	}

	content := fmt.Sprintf(`title: Test IMPL %s
feature_slug: %s
verdict: SUITABLE
waves:
  - number: 1
    agents:
%sfile_ownership:
%sinterface_contracts: []
%s`, slug, slug, agentLines, ownershipLines, criticBlock)

	implPath := filepath.Join(implDir, fmt.Sprintf("IMPL-%s.yaml", slug))
	if err := os.WriteFile(implPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write IMPL YAML: %v", err)
	}
	return implPath
}

// writeTempProgram creates a minimal program YAML referencing the given slugs in tier 1.
// Returns the program YAML path.
func writeTempProgram(t *testing.T, slugs []string) string {
	t.Helper()

	implsBlock := ""
	for _, slug := range slugs {
		implsBlock += fmt.Sprintf("  - slug: %s\n    title: Test %s\n    tier: 1\n    status: pending\n", slug, slug)
	}

	implRefsBlock := ""
	for _, slug := range slugs {
		implRefsBlock += fmt.Sprintf("    - %s\n", slug)
	}

	content := fmt.Sprintf(`title: Test Program
program_slug: test-prog
state: TIER_EXECUTING
impls:
%stiers:
  - number: 1
    impls:
%scompletion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: %d
  total_agents: 0
  total_waves: 0
`, implsBlock, implRefsBlock, len(slugs))

	f, err := os.CreateTemp("", "program-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp program file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		t.Fatalf("failed to write program YAML: %v", err)
	}
	f.Close()
	return f.Name()
}

// TestPrepareTier_E37NotRequired_SmallIMPL verifies that an IMPL with 2 agents in
// wave 1 and no CriticReport is not blocked by E37 — E37 is not required at
// that scale.
//
// The test passes if PrepareTier does NOT return an E37 validation error.
// A worktree-creation failure at step 5 (no real git repo) is acceptable.
func TestPrepareTier_E37NotRequired_SmallIMPL(t *testing.T) {
	repoDir := t.TempDir()
	slug := "small-impl"

	// 2 agents in wave 1, no CriticReport.
	writeTempIMPL(t, repoDir, slug, 2, "")

	programPath := writeTempProgram(t, []string{slug})

	result, err := PrepareTier(programPath, 1, repoDir)
	if err != nil {
		t.Fatalf("PrepareTier returned unexpected error: %v", err)
	}

	// Verify no E37 error appears in Validations.
	for _, vr := range result.Validations {
		for _, errMsg := range vr.Errors {
			if strings.Contains(errMsg, "E37 critic gate") {
				t.Errorf("unexpected E37 block for 2-agent IMPL: %s", errMsg)
			}
		}
	}
	// Note: result.Success may be false due to step 5 (worktree creation) failing
	// in a temp dir with no real git repo — that is acceptable for this test.
}

// TestPrepareTier_E37Required_MissingReport verifies that an IMPL with 3 agents in
// wave 1 and no CriticReport is blocked by E37.
func TestPrepareTier_E37Required_MissingReport(t *testing.T) {
	repoDir := t.TempDir()
	slug := "large-impl"

	// 3 agents in wave 1, no CriticReport — E37 threshold met.
	writeTempIMPL(t, repoDir, slug, 3, "")

	programPath := writeTempProgram(t, []string{slug})

	result, err := PrepareTier(programPath, 1, repoDir)
	if err != nil {
		t.Fatalf("PrepareTier returned unexpected error: %v", err)
	}

	if result.Success {
		t.Fatal("expected PrepareTier to fail for 3-agent IMPL with no CriticReport, got Success=true")
	}

	// Verify E37 error message appears in Validations.
	found := false
	for _, vr := range result.Validations {
		for _, errMsg := range vr.Errors {
			if strings.Contains(errMsg, "E37 critic gate required") {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("expected E37 critic gate validation error in Validations, but none found; Validations: %+v", result.Validations)
	}
}

// TestPrepareTier_E37Required_WithPassReport verifies that an IMPL with 3 agents in
// wave 1 and a PASS CriticReport is NOT blocked by E37.
//
// The test passes if PrepareTier does NOT return an E37 validation error.
// A worktree-creation failure at step 5 (no real git repo) is acceptable.
func TestPrepareTier_E37Required_WithPassReport(t *testing.T) {
	repoDir := t.TempDir()
	slug := "large-impl-with-pass"

	// PASS critic report — all good.
	criticBlock := `  verdict: PASS
  agent_reviews: {}
  summary: all good
  reviewed_at: "2026-01-01T00:00:00Z"
  issue_count: 0
`
	// 3 agents in wave 1, PASS CriticReport — E37 required but satisfied.
	writeTempIMPL(t, repoDir, slug, 3, criticBlock)

	programPath := writeTempProgram(t, []string{slug})

	result, err := PrepareTier(programPath, 1, repoDir)
	if err != nil {
		t.Fatalf("PrepareTier returned unexpected error: %v", err)
	}

	// Verify no E37 error appears in Validations.
	for _, vr := range result.Validations {
		for _, errMsg := range vr.Errors {
			if strings.Contains(errMsg, "E37 critic gate") {
				t.Errorf("unexpected E37 block for 3-agent IMPL with PASS critic report: %s", errMsg)
			}
		}
	}
	// Note: result.Success may be false due to step 5 (worktree creation) failing
	// in a temp dir with no real git repo — that is acceptable for this test.
}
