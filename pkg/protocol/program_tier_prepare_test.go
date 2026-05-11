package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPrepareTier_ManifestNotFound verifies that PrepareTier returns a fatal result
// when the manifest file does not exist.
func TestPrepareTier_ManifestNotFound(t *testing.T) {
	res := PrepareTier(PrepareTierOpts{Ctx: context.Background(), ProgramManifestPath: "/nonexistent/path/program.yaml", TierNumber: 1, RepoDir: "."})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for non-existent manifest path, got non-fatal")
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

	res := PrepareTier(PrepareTierOpts{Ctx: context.Background(), ProgramManifestPath: f.Name(), TierNumber: 99, RepoDir: "."})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for missing tier, got non-fatal")
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
			{ImplSlug: "my-impl", Path: "/tmp/wt", Branch: "polywave/program/p/tier1-impl-my-impl"},
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
	// Use slug in file paths to ensure uniqueness across IMPLs in the same repoDir.
	agentLines := ""
	ownershipLines := ""
	for i := 0; i < agentCount; i++ {
		id := string(rune('A' + i))
		file := fmt.Sprintf("pkg/test/%s/%s_file.go", slug, strings.ToLower(id))
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

	res := PrepareTier(PrepareTierOpts{Ctx: context.Background(), ProgramManifestPath: programPath, TierNumber: 1, RepoDir: repoDir})
	if res.IsFatal() {
		t.Fatalf("PrepareTier returned unexpected fatal: %v", res.Errors)
	}
	result := res.GetData()

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

	res := PrepareTier(PrepareTierOpts{Ctx: context.Background(), ProgramManifestPath: programPath, TierNumber: 1, RepoDir: repoDir})
	if res.IsFatal() {
		t.Fatalf("PrepareTier returned unexpected fatal: %v", res.Errors)
	}
	result := res.GetData()

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

	res := PrepareTier(PrepareTierOpts{Ctx: context.Background(), ProgramManifestPath: programPath, TierNumber: 1, RepoDir: repoDir})
	if res.IsFatal() {
		t.Fatalf("PrepareTier returned unexpected fatal: %v", res.Errors)
	}
	result := res.GetData()

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

// TestPrepareTier_CollectsAllE37Failures verifies that when multiple IMPLs in a
// tier all require E37 and none have a critic report, ALL failures are collected
// instead of returning on the first failure.
func TestPrepareTier_CollectsAllE37Failures(t *testing.T) {
	repoDir := t.TempDir()
	slugs := []string{"impl-alpha", "impl-beta", "impl-gamma"}

	// All 3 IMPLs: 3 agents each, no CriticReport — all require E37.
	for _, slug := range slugs {
		writeTempIMPL(t, repoDir, slug, 3, "")
	}

	programPath := writeTempProgram(t, slugs)

	res := PrepareTier(PrepareTierOpts{Ctx: context.Background(), ProgramManifestPath: programPath, TierNumber: 1, RepoDir: repoDir})
	if res.IsFatal() {
		t.Fatalf("PrepareTier returned unexpected fatal: %v", res.Errors)
	}
	result := res.GetData()

	if result.Success {
		t.Fatal("expected PrepareTier to fail when all IMPLs have E37 failures, got Success=true")
	}

	// Count E37 failures across all validations.
	e37FailCount := 0
	for _, vr := range result.Validations {
		for _, errMsg := range vr.Errors {
			if strings.Contains(errMsg, "E37 critic gate required") {
				e37FailCount++
			}
		}
	}

	if e37FailCount < 3 {
		t.Errorf("expected E37 failures for all 3 IMPLs, got %d; Validations: %+v", e37FailCount, result.Validations)
	}
}

// TestPrepareTier_SkipCritic verifies that when SkipCritic is true, IMPLs
// requiring E37 with no critic report are auto-skipped and validation passes.
// A worktree-creation failure at step 5 is acceptable.
func TestPrepareTier_SkipCritic(t *testing.T) {
	repoDir := t.TempDir()
	slugs := []string{"skip-impl-1", "skip-impl-2"}

	// Both IMPLs: 3 agents, no CriticReport — both require E37.
	for _, slug := range slugs {
		writeTempIMPL(t, repoDir, slug, 3, "")
	}

	programPath := writeTempProgram(t, slugs)

	res := PrepareTier(PrepareTierOpts{
		Ctx:                 context.Background(),
		ProgramManifestPath: programPath,
		TierNumber:          1,
		RepoDir:             repoDir,
		SkipCritic:          true,
	})
	if res.IsFatal() {
		t.Fatalf("PrepareTier returned unexpected fatal: %v", res.Errors)
	}
	result := res.GetData()

	// Verify no E37 failures in result.Validations.
	for _, vr := range result.Validations {
		for _, errMsg := range vr.Errors {
			if strings.Contains(errMsg, "E37 critic gate") {
				t.Errorf("unexpected E37 failure with SkipCritic=true: %s", errMsg)
			}
		}
	}
	// Note: result.Success may be false due to step 5 (worktree creation) failing
	// in a temp dir with no real git repo — that is acceptable for this test.
}

// TestSkipCriticForIMPL_WritesPass verifies that SkipCriticForIMPL writes a
// synthetic PASS critic report and the manifest reloads with the correct verdict.
func TestSkipCriticForIMPL_WritesPass(t *testing.T) {
	repoDir := t.TempDir()
	slug := "skip-test-impl"

	// 3 agents, no CriticReport — E37 required.
	implPath := writeTempIMPL(t, repoDir, slug, 3, "")

	m, err := Load(context.TODO(), implPath)
	if err != nil {
		t.Fatalf("failed to load IMPL: %v", err)
	}

	skipRes := SkipCriticForIMPL(context.TODO(), implPath, m)
	if skipRes.IsFatal() {
		t.Fatalf("SkipCriticForIMPL returned fatal: %v", skipRes.Errors)
	}
	skipped := skipRes.GetData()
	if !skipped {
		t.Fatal("expected SkipCriticForIMPL to return true (skip written), got false")
	}

	// Reload and verify.
	m2, err := Load(context.TODO(), implPath)
	if err != nil {
		t.Fatalf("failed to reload IMPL after skip: %v", err)
	}
	if m2.CriticReport == nil {
		t.Fatal("expected CriticReport to be set after skip, got nil")
	}
	if m2.CriticReport.Verdict != CriticVerdictPass {
		t.Errorf("expected CriticReport.Verdict == %q, got %q", CriticVerdictPass, m2.CriticReport.Verdict)
	}
	if !strings.Contains(m2.CriticReport.Summary, "skip-critic") {
		t.Errorf("expected CriticReport.Summary to contain 'skip-critic', got %q", m2.CriticReport.Summary)
	}
}

// TestPrepareTierResult_PrepareWaveResults_JSONFields verifies that
// PrepareTierResult serializes prepare_wave_results when set and omits it
// when nil (omitempty).
func TestPrepareTierResult_PrepareWaveResults_JSONFields(t *testing.T) {
	// When PrepareWaveResults is nil, the field should be omitted.
	rNil := &PrepareTierResult{
		Tier:    1,
		Success: true,
	}
	dataNil, err := json.Marshal(rNil)
	if err != nil {
		t.Fatalf("failed to marshal PrepareTierResult (nil PrepareWaveResults): %v", err)
	}
	var decodedNil map[string]interface{}
	if err := json.Unmarshal(dataNil, &decodedNil); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if _, ok := decodedNil["prepare_wave_results"]; ok {
		t.Error("expected 'prepare_wave_results' to be omitted when nil (omitempty), but it was present")
	}

	// When PrepareWaveResults is set, the field should be serialized.
	rSet := &PrepareTierResult{
		Tier:    1,
		Success: true,
		PrepareWaveResults: []IMPLPrepareWaveResult{
			{ImplSlug: "test-impl", Success: true},
		},
	}
	dataSet, err := json.Marshal(rSet)
	if err != nil {
		t.Fatalf("failed to marshal PrepareTierResult (with PrepareWaveResults): %v", err)
	}
	var decodedSet map[string]interface{}
	if err := json.Unmarshal(dataSet, &decodedSet); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if _, ok := decodedSet["prepare_wave_results"]; !ok {
		t.Error("expected 'prepare_wave_results' to be present when set, but it was missing")
	}
	items, ok := decodedSet["prepare_wave_results"].([]interface{})
	if !ok || len(items) != 1 {
		t.Errorf("expected prepare_wave_results to have 1 item, got %v", decodedSet["prepare_wave_results"])
	}
}

// TestIMPLPrepareWaveResult_JSONFields verifies that IMPLPrepareWaveResult
// serializes with expected field names and that error is omitted when empty.
func TestIMPLPrepareWaveResult_JSONFields(t *testing.T) {
	r := &IMPLPrepareWaveResult{
		ImplSlug:  "my-impl",
		Success:   true,
		Worktrees: []WorktreeInfo{},
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal IMPLPrepareWaveResult: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	// Verify required fields are present.
	expectedFields := []string{"impl_slug", "success", "worktrees"}
	for _, field := range expectedFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("expected JSON field %q to be present, but it was missing", field)
		}
	}

	// Verify error is omitted when empty (omitempty).
	if _, ok := decoded["error"]; ok {
		t.Error("expected 'error' field to be omitted when empty (omitempty), but it was present")
	}

	// Verify error is present when set.
	rWithErr := &IMPLPrepareWaveResult{
		ImplSlug: "fail-impl",
		Success:  false,
		Error:    "something went wrong",
	}
	dataErr, err := json.Marshal(rWithErr)
	if err != nil {
		t.Fatalf("failed to marshal IMPLPrepareWaveResult with error: %v", err)
	}
	var decodedErr map[string]interface{}
	if err := json.Unmarshal(dataErr, &decodedErr); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if _, ok := decodedErr["error"]; !ok {
		t.Error("expected 'error' field to be present when set, but it was missing")
	}
}

// TestPrepareTierOpts_NewFields verifies that the new fields exist on
// PrepareTierOpts by constructing the struct (compile-time check).
func TestPrepareTierOpts_NewFields(t *testing.T) {
	opts := PrepareTierOpts{
		Ctx:                 context.Background(),
		ProgramManifestPath: "test.yaml",
		TierNumber:          1,
		RepoDir:             ".",
		SkipCritic:          false,
		RunPrepareWave:      true,
		WaveNum:             1,
		MergeTarget:         "main",
	}
	if !opts.RunPrepareWave {
		t.Error("expected RunPrepareWave=true")
	}
	if opts.WaveNum != 1 {
		t.Error("expected WaveNum=1")
	}
	if opts.MergeTarget != "main" {
		t.Errorf("expected MergeTarget=%q, got %q", "main", opts.MergeTarget)
	}
}

// TestSkipCriticForIMPL_NotRequired verifies that SkipCriticForIMPL returns
// (false, nil) for an IMPL that does not meet the E37 threshold (2 agents).
func TestSkipCriticForIMPL_NotRequired(t *testing.T) {
	repoDir := t.TempDir()
	slug := "small-skip-impl"

	// 2 agents — below E37 threshold.
	implPath := writeTempIMPL(t, repoDir, slug, 2, "")

	m, err := Load(context.TODO(), implPath)
	if err != nil {
		t.Fatalf("failed to load IMPL: %v", err)
	}

	skipRes := SkipCriticForIMPL(context.TODO(), implPath, m)
	if skipRes.IsFatal() {
		t.Fatalf("SkipCriticForIMPL returned unexpected fatal: %v", skipRes.Errors)
	}
	skipped := skipRes.GetData()
	if skipped {
		t.Fatal("expected SkipCriticForIMPL to return false (not required), got true")
	}
}
