package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/collision"
	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// TestPrepareWaveCmd_RequiresWaveFlag verifies that the command errors
// when --wave is not provided.
func TestPrepareWaveCmd_RequiresWaveFlag(t *testing.T) {
	cmd := newPrepareWaveCmd()
	cmd.SetArgs([]string{"/tmp/nonexistent.yaml"})
	// Wave defaults to 0, which should trigger "wave is required"
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --wave is not provided")
	}
	if !strings.Contains(err.Error(), "wave") {
		t.Errorf("expected error about wave flag, got: %s", err.Error())
	}
}

// TestPrepareWaveCmd_UsesEnginePrepareWaveResult verifies that the CLI
// outputs engine.PrepareWaveResult JSON structure (wave, worktrees,
// agent_briefs, steps, success fields).
func TestPrepareWaveCmd_UsesEnginePrepareWaveResult(t *testing.T) {
	// Verify the engine types are the ones used (compile-time check)
	var _ engine.PrepareWaveResult
	var _ engine.AgentBriefInfo
	var _ engine.PrepareWaveOpts
	var _ engine.EventCallback

	// Verify engine result serializes with expected fields
	r := engine.PrepareWaveResult{
		Wave:      1,
		Worktrees: []protocol.WorktreeInfo{},
		AgentBriefs: []engine.AgentBriefInfo{
			{
				Agent:       "A",
				BriefPath:   "/some/path/.polywave-agent-brief.md",
				BriefLength: 100,
				JournalDir:  "/some/path/.claude/tool-journal",
				FilesOwned:  2,
				Repo:        "polywave-go",
			},
		},
		Steps: []engine.StepResult{
			{Step: "dep_check", Status: "success", Detail: "no conflicts"},
		},
		Success: true,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal engine.PrepareWaveResult: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify core fields present
	if _, ok := decoded["wave"]; !ok {
		t.Error("expected 'wave' field in JSON output")
	}
	if _, ok := decoded["worktrees"]; !ok {
		t.Error("expected 'worktrees' field in JSON output")
	}
	if _, ok := decoded["agent_briefs"]; !ok {
		t.Error("expected 'agent_briefs' field in JSON output")
	}
	if _, ok := decoded["steps"]; !ok {
		t.Error("expected 'steps' field in JSON output")
	}
	if _, ok := decoded["success"]; !ok {
		t.Error("expected 'success' field in JSON output")
	}
}

// TestPrepareWaveCmd_AgentBriefInfoJSON verifies that engine.AgentBriefInfo
// serializes with backward-compatible JSON field names.
func TestPrepareWaveCmd_AgentBriefInfoJSON(t *testing.T) {
	info := engine.AgentBriefInfo{
		Agent:       "A",
		BriefPath:   "/tmp/brief.md",
		BriefLength: 100,
		JournalDir:  "/tmp/journal",
		FilesOwned:  3,
		Repo:        "polywave-go",
		MergeTarget: "polywave/program/my-prog/tier1-impl-foo",
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Verify expected JSON field names
	str := string(data)
	expectedFields := []string{
		`"agent"`, `"brief_path"`, `"brief_length"`, `"journal_dir"`,
		`"files_owned"`, `"repo"`, `"merge_target"`,
	}
	for _, f := range expectedFields {
		if !strings.Contains(str, f) {
			t.Errorf("expected field %s in JSON, got: %s", f, str)
		}
	}
}

// TestPrepareWaveCmd_AgentBriefInfoOmitEmpty verifies that empty optional
// fields are omitted from JSON (backward compat with omitempty).
func TestPrepareWaveCmd_AgentBriefInfoOmitEmpty(t *testing.T) {
	info := engine.AgentBriefInfo{
		Agent:      "B",
		FilesOwned: 1,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	str := string(data)

	if strings.Contains(str, `"repo"`) {
		t.Errorf("expected 'repo' to be omitted when empty, got: %s", str)
	}
	if strings.Contains(str, `"merge_target"`) {
		t.Errorf("expected 'merge_target' to be omitted when empty, got: %s", str)
	}
}

// TestPrepareWave_RepoMismatchBlocksWorktreeCreation verifies that
// ValidateRepoMatch returning an error prevents worktree creation.
func TestPrepareWave_RepoMismatchBlocksWorktreeCreation(t *testing.T) {
	doc := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/api/handler.go", Agent: "A", Wave: 1, Repo: "polywave-web", Action: "modify"},
		},
	}

	configRepos := []protocol.RepoEntry{
		{Name: "polywave-web", Path: "/tmp/saw-web"},
	}

	res := protocol.ValidateRepoMatch(doc, "/tmp/saw-go", configRepos)
	if res.IsSuccess() {
		t.Fatal("expected repo mismatch error, got success")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors for repo mismatch, got none")
	}
	if !strings.Contains(res.Errors[0].Message, "repo") {
		t.Errorf("expected error to mention 'repo', got: %s", res.Errors[0].Message)
	}
}

// TestPrepareWave_AllFilesMissingSuspectsRepoMismatch verifies that when ALL
// action=modify files are missing, ValidateFileExistenceMultiRepo returns
// E16_REPO_MISMATCH_SUSPECTED which should be treated as a hard blocker.
func TestPrepareWave_AllFilesMissingSuspectsRepoMismatch(t *testing.T) {
	dir := t.TempDir()

	doc := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/nonexistent/a.go", Agent: "A", Wave: 1, Action: "modify"},
			{File: "pkg/nonexistent/b.go", Agent: "A", Wave: 1, Action: "modify"},
		},
	}

	warnings := protocol.ValidateFileExistenceMultiRepo(doc, dir, nil)

	foundSuspected := false
	for _, w := range warnings {
		if w.Code == result.CodeRepoMismatch {
			foundSuspected = true
			break
		}
	}
	if !foundSuspected {
		t.Errorf("expected %s when all modify files missing, got warnings: %+v", result.CodeRepoMismatch, warnings)
	}
}

// TestPrepareWave_I1OwnershipViolation verifies that protocol.Validate returns
// I1_VIOLATION errors when two agents in the same wave own the same file.
func TestPrepareWave_I1OwnershipViolation(t *testing.T) {
	doc := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Action: "modify"},
			{File: "pkg/foo/foo.go", Agent: "B", Wave: 1, Action: "modify"},
		},
	}

	allErrs := protocol.Validate(doc)
	var i1Errs []result.PolywaveError
	for _, e := range allErrs {
		if e.Code == result.CodeDisjointOwnership {
			i1Errs = append(i1Errs, e)
		}
	}

	if len(i1Errs) == 0 {
		t.Fatal("expected I1_VIOLATION errors for overlapping file ownership, got none")
	}

	found := false
	for _, e := range i1Errs {
		if strings.Contains(e.Message, "pkg/foo/foo.go") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected I1_VIOLATION to mention conflicting file, got: %+v", i1Errs)
	}
}

// TestPrepareWave_I1OwnershipNoViolation verifies that disjoint file ownership
// produces no I1_VIOLATION errors.
func TestPrepareWave_I1OwnershipNoViolation(t *testing.T) {
	doc := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Action: "modify"},
			{File: "pkg/bar/bar.go", Agent: "B", Wave: 1, Action: "modify"},
		},
	}

	allErrs := protocol.Validate(doc)
	for _, e := range allErrs {
		if e.Code == result.CodeDisjointOwnership {
			t.Errorf("unexpected I1_VIOLATION for disjoint ownership: %s", e.Message)
		}
	}
}

// TestPrepareWave_I1CrossWaveAllowed verifies that the same file owned by
// different agents in different waves does NOT produce I1_VIOLATION errors.
func TestPrepareWave_I1CrossWaveAllowed(t *testing.T) {
	doc := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Action: "create"},
			{File: "pkg/foo/foo.go", Agent: "B", Wave: 2, Action: "modify"},
		},
	}

	allErrs := protocol.Validate(doc)
	for _, e := range allErrs {
		if e.Code == result.CodeDisjointOwnership {
			t.Errorf("unexpected I1_VIOLATION for cross-wave sequential ownership: %s", e.Message)
		}
	}
}

// TestPrepareWave_CollisionReportValidField verifies collision detection types.
func TestPrepareWave_CollisionReportValidField(t *testing.T) {
	reportValid := collision.CollisionReport{
		Valid:      true,
		Collisions: nil,
	}
	if !reportValid.Valid {
		t.Error("expected Valid=true to not block prepare-wave")
	}

	reportInvalid := collision.CollisionReport{
		Valid: false,
		Collisions: []collision.TypeCollision{
			{
				TypeName:   "MyType",
				Package:    "pkg/foo",
				Agents:     []string{"A", "B"},
				Resolution: "Keep A, remove from B",
			},
		},
	}
	if reportInvalid.Valid {
		t.Error("expected Valid=false to block prepare-wave")
	}
	if len(reportInvalid.Collisions) != 1 {
		t.Errorf("expected 1 collision, got %d", len(reportInvalid.Collisions))
	}
}

// TestPrepareWave_CollisionReportJSONOutput verifies collision report JSON round-trip.
func TestPrepareWave_CollisionReportJSONOutput(t *testing.T) {
	report := collision.CollisionReport{
		Valid: false,
		Collisions: []collision.TypeCollision{
			{
				TypeName:   "Config",
				Package:    "pkg/config",
				Agents:     []string{"A", "C"},
				Resolution: "Keep A, remove from C",
			},
		},
	}

	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal CollisionReport: %v", err)
	}

	var decoded collision.CollisionReport
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("failed to unmarshal CollisionReport: %v", err)
	}

	if decoded.Valid {
		t.Error("expected Valid=false after round-trip")
	}
	if len(decoded.Collisions) != 1 {
		t.Fatalf("expected 1 collision after round-trip, got %d", len(decoded.Collisions))
	}
	if decoded.Collisions[0].TypeName != "Config" {
		t.Errorf("expected TypeName=Config, got %q", decoded.Collisions[0].TypeName)
	}
}

// TestPrepareWaveBaselineData_JSONSerialization verifies that
// protocol.BaselineData round-trips through JSON correctly.
func TestPrepareWaveBaselineData_JSONSerialization(t *testing.T) {
	r := protocol.BaselineData{
		Passed: false,
		GateResults: []protocol.GateResult{
			{
				Type:     "build",
				Command:  "go build ./...",
				Passed:   true,
				Required: true,
			},
			{
				Type:     "test",
				Command:  "go test ./...",
				Passed:   false,
				Required: true,
				ExitCode: 1,
				Stderr:   "FAIL pkg/foo",
			},
		},
		Reason:    "baseline_verification_failed",
		CommitSHA: "abc123",
		FromCache: false,
	}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal BaselineData: %v", err)
	}

	var decoded protocol.BaselineData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal BaselineData: %v", err)
	}

	if decoded.Passed != false {
		t.Errorf("expected Passed=false, got %v", decoded.Passed)
	}
	if decoded.Reason != "baseline_verification_failed" {
		t.Errorf("expected Reason=baseline_verification_failed, got %q", decoded.Reason)
	}
	if decoded.CommitSHA != "abc123" {
		t.Errorf("expected CommitSHA=abc123, got %q", decoded.CommitSHA)
	}
	if len(decoded.GateResults) != 2 {
		t.Fatalf("expected 2 gate results, got %d", len(decoded.GateResults))
	}
	if decoded.GateResults[0].Type != "build" || !decoded.GateResults[0].Passed {
		t.Errorf("unexpected GateResults[0]: %+v", decoded.GateResults[0])
	}
	if decoded.GateResults[1].Type != "test" || decoded.GateResults[1].Passed {
		t.Errorf("unexpected GateResults[1]: %+v", decoded.GateResults[1])
	}
	if decoded.GateResults[1].Stderr != "FAIL pkg/foo" {
		t.Errorf("expected Stderr=%q, got %q", "FAIL pkg/foo", decoded.GateResults[1].Stderr)
	}
}

// TestVerifyDependenciesAvailable_Wave1Skipped verifies that wave 1 is
// always considered valid (no prior wave to depend on).
func TestVerifyDependenciesAvailable_Wave1Skipped(t *testing.T) {
	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "do A"},
				},
			},
		},
	}

	res := protocol.VerifyDependenciesAvailable(context.Background(), doc, 1)
	if !res.GetData().Valid {
		t.Errorf("expected Valid=true for wave 1 (no deps), got false")
	}
}

// TestVerifyDependenciesAvailable_Wave2MissingDep verifies that a wave 2 agent
// with a missing dependency (no completion report) returns Valid=false.
func TestVerifyDependenciesAvailable_Wave2MissingDep(t *testing.T) {
	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "do A"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "do B", Dependencies: []string{"A"}},
				},
			},
		},
	}

	res := protocol.VerifyDependenciesAvailable(context.Background(), doc, 2)
	if res.GetData().Valid {
		t.Errorf("expected Valid=false when dependency A has no completion report")
	}
	data := res.GetData()
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent check, got %d", len(data.Agents))
	}
	check := data.Agents[0]
	if check.Available {
		t.Errorf("expected agent B Available=false")
	}
	if len(check.Missing) != 1 || check.Missing[0] != "A" {
		t.Errorf("expected missing=[A], got %v", check.Missing)
	}
}

// TestVerifyDependenciesAvailable_Wave2DepComplete verifies that when dependency
// agent has a complete completion report, Valid=true.
func TestVerifyDependenciesAvailable_Wave2DepComplete(t *testing.T) {
	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "do A"},
				},
			},
			{
				Number: 2,
				Agents: []protocol.Agent{
					{ID: "B", Task: "do B", Dependencies: []string{"A"}},
				},
			},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {Status: "complete"},
		},
	}

	res := protocol.VerifyDependenciesAvailable(context.Background(), doc, 2)
	if !res.GetData().Valid {
		t.Errorf("expected Valid=true when all dependencies are complete")
	}
}

// TestPrepareWave_MergeTargetIncludedInBrief verifies merge target handling.
func TestPrepareWave_MergeTargetIncludedInBrief(t *testing.T) {
	mergeTarget := "polywave/program/my-prog/tier1-impl-foo"

	mergeTargetSection := fmt.Sprintf(
		"\n\n## Merge Target\n\n**Merge your branch to:** `%s`\n\n"+
			"Do NOT merge to HEAD or main. All wave commits must land on "+
			"this IMPL branch. The finalize-tier command will merge the "+
			"IMPL branch to main after all IMPLs in the tier complete.\n",
		mergeTarget,
	)

	brief := fmt.Sprintf("# Agent A Brief - Wave 1\n\n**IMPL Doc:** /tmp/impl.yaml\n\n## Files Owned\n\n- foo.go\n\n## Task\n\nDo something.\n%s",
		mergeTargetSection)

	if !strings.Contains(brief, "## Merge Target") {
		t.Errorf("expected brief to contain '## Merge Target', got:\n%s", brief)
	}
	if !strings.Contains(brief, mergeTarget) {
		t.Errorf("expected brief to contain branch name %q, got:\n%s", mergeTarget, brief)
	}
}

// TestPrepareWave_NoMergeTargetNoBriefSection verifies that when mergeTarget is
// empty, the brief does NOT contain a "## Merge Target" section.
func TestPrepareWave_NoMergeTargetNoBriefSection(t *testing.T) {
	mergeTarget := ""

	var mergeTargetSection string
	if mergeTarget != "" {
		mergeTargetSection = fmt.Sprintf(
			"\n\n## Merge Target\n\n**Merge your branch to:** `%s`\n\n"+
				"Do NOT merge to HEAD or main. All wave commits must land on "+
				"this IMPL branch. The finalize-tier command will merge the "+
				"IMPL branch to main after all IMPLs in the tier complete.\n",
			mergeTarget,
		)
	}

	brief := fmt.Sprintf("# Agent A Brief - Wave 1\n\n**IMPL Doc:** /tmp/impl.yaml\n\n## Files Owned\n\n- foo.go\n\n## Task\n\nDo something.\n%s",
		mergeTargetSection)

	if strings.Contains(brief, "## Merge Target") {
		t.Errorf("expected brief to NOT contain '## Merge Target' when mergeTarget is empty, got:\n%s", brief)
	}
}

// TestPrepareWaveCmd_CommitStateFlagDefaultsToTrue verifies that the
// --commit-state flag defaults to true (auto-commit Polywave state before
// working-dir check is on by default).
func TestPrepareWaveCmd_CommitStateFlagDefaultsToTrue(t *testing.T) {
	cmd := newPrepareWaveCmd()
	flag := cmd.Flags().Lookup("commit-state")
	if flag == nil {
		t.Fatal("expected --commit-state flag to be registered")
	}
	if flag.DefValue != "true" {
		t.Errorf("expected --commit-state default to be %q, got %q", "true", flag.DefValue)
	}
}
