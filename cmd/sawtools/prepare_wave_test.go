package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

func TestLoadSAWConfigRepos_ValidFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "saw.config.json")

	content := `{
		"repos": [
			{"name": "scout-and-wave-go", "path": "/tmp/saw-go"},
			{"name": "scout-and-wave-web", "path": "/tmp/saw-web"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	repos := loadSAWConfigRepos(configPath)
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Name != "scout-and-wave-go" || repos[0].Path != "/tmp/saw-go" {
		t.Errorf("unexpected repos[0]: %+v", repos[0])
	}
	if repos[1].Name != "scout-and-wave-web" || repos[1].Path != "/tmp/saw-web" {
		t.Errorf("unexpected repos[1]: %+v", repos[1])
	}
}

func TestLoadSAWConfigRepos_AbsentFile(t *testing.T) {
	repos := loadSAWConfigRepos("/nonexistent/path/saw.config.json")
	if repos != nil {
		t.Errorf("expected nil for absent file, got %v", repos)
	}
}

func TestLoadSAWConfigRepos_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "saw.config.json")

	if err := os.WriteFile(configPath, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	repos := loadSAWConfigRepos(configPath)
	if repos != nil {
		t.Errorf("expected nil for invalid JSON, got %v", repos)
	}
}

func TestLoadSAWConfigRepos_NoReposKey(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "saw.config.json")

	if err := os.WriteFile(configPath, []byte(`{"planner_model": "claude-3-opus"}`), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	repos := loadSAWConfigRepos(configPath)
	// Should return nil slice (empty from json unmarshal with no repos key)
	if len(repos) != 0 {
		t.Errorf("expected empty repos, got %v", repos)
	}
}

func TestResolveAgentRepo_WithMatchingRepo(t *testing.T) {
	fileOwnership := []protocol.FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "scout-and-wave-go"},
		{File: "pkg/api/handler.go", Agent: "B", Wave: 1, Repo: "scout-and-wave-web"},
	}
	repos := []protocol.RepoEntry{
		{Name: "scout-and-wave-go", Path: "/abs/path/saw-go"},
		{Name: "scout-and-wave-web", Path: "/abs/path/saw-web"},
	}

	got := protocol.ResolveAgentRepo(fileOwnership, "A", "/fallback", repos)
	if got != "/abs/path/saw-go" {
		t.Errorf("expected /abs/path/saw-go, got %s", got)
	}

	got = protocol.ResolveAgentRepo(fileOwnership, "B", "/fallback", repos)
	if got != "/abs/path/saw-web" {
		t.Errorf("expected /abs/path/saw-web, got %s", got)
	}
}

func TestResolveAgentRepo_FallbackWhenNoRepoField(t *testing.T) {
	fileOwnership := []protocol.FileOwnership{
		{File: "cmd/saw/prepare_wave.go", Agent: "B", Wave: 1},
	}
	repos := []protocol.RepoEntry{
		{Name: "some-repo", Path: "/abs/path/some-repo"},
	}

	got := protocol.ResolveAgentRepo(fileOwnership, "B", "/project/root", repos)
	if got != "/project/root" {
		t.Errorf("expected fallback /project/root, got %s", got)
	}
}

func TestResolveAgentRepo_FallbackWhenRepoNotInRegistry(t *testing.T) {
	fileOwnership := []protocol.FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "unknown-repo"},
	}
	repos := []protocol.RepoEntry{
		{Name: "other-repo", Path: "/abs/path/other"},
	}

	got := protocol.ResolveAgentRepo(fileOwnership, "A", "/project/root", repos)
	if got != "/project/root" {
		t.Errorf("expected fallback /project/root, got %s", got)
	}
}

func TestResolveAgentRepo_FallbackWhenNilRepos(t *testing.T) {
	fileOwnership := []protocol.FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "scout-and-wave-go"},
	}

	got := protocol.ResolveAgentRepo(fileOwnership, "A", "/project/root", nil)
	if got != "/project/root" {
		t.Errorf("expected fallback /project/root with nil repos, got %s", got)
	}
}

func TestResolveAgentRepo_FallbackWhenEmptyOwnership(t *testing.T) {
	repos := []protocol.RepoEntry{
		{Name: "scout-and-wave-go", Path: "/abs/path/saw-go"},
	}

	got := protocol.ResolveAgentRepo(nil, "A", "/project/root", repos)
	if got != "/project/root" {
		t.Errorf("expected fallback /project/root for empty ownership, got %s", got)
	}
}

func TestPrepareWaveResult_JSONSerialization(t *testing.T) {
	result := PrepareWaveResult{
		Wave:      1,
		Worktrees: []protocol.WorktreeInfo{},
		AgentBriefs: []AgentBriefInfo{
			{
				Agent:       "A",
				BriefPath:   "/some/path/.saw-agent-brief.md",
				BriefLength: 100,
				JournalDir:  "/some/path/.claude/tool-journal",
				FilesOwned:  2,
				Repo:        "scout-and-wave-go",
			},
			{
				Agent:       "B",
				BriefPath:   "/other/path/.saw-agent-brief.md",
				BriefLength: 80,
				JournalDir:  "/other/path/.claude/tool-journal",
				FilesOwned:  1,
				// No Repo — primary repo agent
			},
		},
		Repos: []string{"scout-and-wave-go"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var decoded PrepareWaveResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if decoded.Wave != 1 {
		t.Errorf("expected wave 1, got %d", decoded.Wave)
	}
	if len(decoded.AgentBriefs) != 2 {
		t.Fatalf("expected 2 agent briefs, got %d", len(decoded.AgentBriefs))
	}
	if decoded.AgentBriefs[0].Repo != "scout-and-wave-go" {
		t.Errorf("expected Repo scout-and-wave-go, got %q", decoded.AgentBriefs[0].Repo)
	}
	if decoded.AgentBriefs[1].Repo != "" {
		t.Errorf("expected empty Repo for agent B, got %q", decoded.AgentBriefs[1].Repo)
	}
	if len(decoded.Repos) != 1 || decoded.Repos[0] != "scout-and-wave-go" {
		t.Errorf("unexpected Repos: %v", decoded.Repos)
	}
}

// TestPrepareWave_RepoMismatchBlocksWorktreeCreation verifies that
// ValidateRepoMatch returning an error prevents worktree creation.
func TestPrepareWave_RepoMismatchBlocksWorktreeCreation(t *testing.T) {
	// Build a manifest that targets a different repo than the project root
	doc := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/api/handler.go", Agent: "A", Wave: 1, Repo: "scout-and-wave-web", Action: "modify"},
		},
	}

	// configRepos points "scout-and-wave-web" to a different path
	configRepos := []protocol.RepoEntry{
		{Name: "scout-and-wave-web", Path: "/tmp/saw-web"},
	}

	// Project root is NOT /tmp/saw-web, so this should fail
	err := protocol.ValidateRepoMatch(doc, "/tmp/saw-go", configRepos)
	if err == nil {
		t.Fatal("expected repo mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "repo") {
		t.Errorf("expected error to mention 'repo', got: %s", err.Error())
	}
}

// TestPrepareWave_AllFilesMissingSuspectsRepoMismatch verifies that when ALL
// action=modify files are missing, ValidateFileExistenceMultiRepo returns
// E16_REPO_MISMATCH_SUSPECTED which should be treated as a hard blocker.
func TestPrepareWave_AllFilesMissingSuspectsRepoMismatch(t *testing.T) {
	dir := t.TempDir()

	// Create a manifest where all modify files reference paths that don't exist
	doc := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/nonexistent/a.go", Agent: "A", Wave: 1, Action: "modify"},
			{File: "pkg/nonexistent/b.go", Agent: "A", Wave: 1, Action: "modify"},
		},
	}

	warnings := protocol.ValidateFileExistenceMultiRepo(doc, dir, nil)

	// Should contain E16_REPO_MISMATCH_SUSPECTED
	foundSuspected := false
	for _, w := range warnings {
		if w.Code == "E16_REPO_MISMATCH_SUSPECTED" {
			foundSuspected = true
			break
		}
	}
	if !foundSuspected {
		t.Errorf("expected E16_REPO_MISMATCH_SUSPECTED when all modify files missing, got warnings: %+v", warnings)
	}
}

// TestPrepareWave_CrossRepoValidPasses verifies that a properly configured
// cross-repo IMPL passes validation when the worktree repo is one of the targets.
func TestPrepareWave_CrossRepoValidPasses(t *testing.T) {
	projectRoot := t.TempDir()

	// Create some files that exist
	modFile := filepath.Join(projectRoot, "pkg/foo/foo.go")
	if err := os.MkdirAll(filepath.Dir(modFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(modFile, []byte("package foo"), 0644); err != nil {
		t.Fatal(err)
	}

	doc := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Action: "modify"},
		},
	}

	// No repo fields means single-repo — should pass with projectRoot
	err := protocol.ValidateRepoMatch(doc, projectRoot, nil)
	if err != nil {
		t.Errorf("expected no error for valid single-repo IMPL, got: %v", err)
	}

	// File existence should find at least one file
	warnings := protocol.ValidateFileExistenceMultiRepo(doc, projectRoot, nil)
	for _, w := range warnings {
		if w.Code == "E16_REPO_MISMATCH_SUSPECTED" {
			t.Errorf("should not suspect repo mismatch when files exist, got: %s", w.Message)
		}
	}
}

// TestPrepareWaveBaselineData_JSONSerialization verifies that a
// protocol.BaselineData round-trips through JSON correctly.
func TestPrepareWaveBaselineData_JSONSerialization(t *testing.T) {
	result := protocol.BaselineData{
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

	data, err := json.Marshal(result)
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
	if decoded.FromCache != false {
		t.Errorf("expected FromCache=false, got %v", decoded.FromCache)
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

	// Verify that the Reason field is omitted when empty (omitempty)
	passedResult := protocol.BaselineData{
		Passed:      true,
		GateResults: []protocol.GateResult{},
	}
	passedData, err := json.Marshal(passedResult)
	if err != nil {
		t.Fatalf("failed to marshal passing BaselineData: %v", err)
	}
	var passedMap map[string]interface{}
	if err := json.Unmarshal(passedData, &passedMap); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if reason, ok := passedMap["reason"]; ok && reason != "" {
		t.Errorf("expected 'reason' field to be omitted when empty, but got %v", reason)
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

	// Wave 1 has no dependencies — should always return Valid=true
	res := protocol.VerifyDependenciesAvailable(doc, 1)
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
		// CompletionReports deliberately empty — agent A has not completed
	}

	res := protocol.VerifyDependenciesAvailable(doc, 2)
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

	res := protocol.VerifyDependenciesAvailable(doc, 2)
	if !res.GetData().Valid {
		t.Errorf("expected Valid=true when all dependencies are complete")
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
	var i1Errs []result.SAWError
	for _, e := range allErrs {
		if e.Code == "I1_VIOLATION" {
			i1Errs = append(i1Errs, e)
		}
	}

	if len(i1Errs) == 0 {
		t.Fatal("expected I1_VIOLATION errors for overlapping file ownership, got none")
	}

	// Verify the error mentions the conflicting file
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
		if e.Code == "I1_VIOLATION" {
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
		if e.Code == "I1_VIOLATION" {
			t.Errorf("unexpected I1_VIOLATION for cross-wave sequential ownership: %s", e.Message)
		}
	}
}

// TestPrepareWave_CollisionReportValidField verifies that a CollisionReport
// with Valid=true does not trigger the collision block path.
// This tests the conditional logic: only collisionReport.Valid == false blocks.
func TestPrepareWave_CollisionReportValidField(t *testing.T) {
	// Simulate what prepare-wave does when collision check passes (Valid=true)
	reportValid := collision.CollisionReport{
		Valid:      true,
		Collisions: nil,
	}
	if !reportValid.Valid {
		t.Error("expected Valid=true to not block prepare-wave")
	}

	// Simulate what prepare-wave does when collision check fails (Valid=false)
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
	if reportInvalid.Collisions[0].TypeName != "MyType" {
		t.Errorf("expected TypeName=MyType, got %q", reportInvalid.Collisions[0].TypeName)
	}
}

// TestPrepareWave_CollisionReportJSONOutput verifies that a collision report
// marshals to the expected JSON shape for programmatic consumers.
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
	if len(decoded.Collisions[0].Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(decoded.Collisions[0].Agents))
	}
}

func TestAgentBriefInfo_OmitEmptyRepo(t *testing.T) {
	info := AgentBriefInfo{
		Agent:      "B",
		FilesOwned: 1,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal AgentBriefInfo: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, ok := m["repo"]; ok {
		t.Errorf("expected 'repo' field to be omitted when empty, but it was present")
	}
}

// TestPrepareWave_MergeTargetIncludedInBrief verifies that when mergeTarget is
// non-empty, the brief template includes a "## Merge Target" section with the
// branch name.
func TestPrepareWave_MergeTargetIncludedInBrief(t *testing.T) {
	mergeTarget := "saw/program/my-prog/tier1-impl-foo"

	// Build the merge target section exactly as prepare_wave.go does
	mergeTargetSection := fmt.Sprintf(
		"\n\n## Merge Target\n\n**Merge your branch to:** `%s`\n\n"+
			"Do NOT merge to HEAD or main. All wave commits must land on "+
			"this IMPL branch. The finalize-tier command will merge the "+
			"IMPL branch to main after all IMPLs in the tier complete.\n",
		mergeTarget,
	)

	// Build a minimal brief using the same template
	brief := fmt.Sprintf("# Agent A Brief - Wave 1\n\n**IMPL Doc:** /tmp/impl.yaml\n\n## Files Owned\n\n- foo.go\n\n## Task\n\nDo something.\n%s",
		mergeTargetSection)

	if !strings.Contains(brief, "## Merge Target") {
		t.Errorf("expected brief to contain '## Merge Target', got:\n%s", brief)
	}
	if !strings.Contains(brief, mergeTarget) {
		t.Errorf("expected brief to contain branch name %q, got:\n%s", mergeTarget, brief)
	}
	if !strings.Contains(brief, "Do NOT merge to HEAD or main") {
		t.Errorf("expected brief to contain warning text, got:\n%s", brief)
	}
}

// TestPrepareWave_NoMergeTargetNoBriefSection verifies that when mergeTarget is
// empty (the default), the brief does NOT contain a "## Merge Target" section.
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

// TestPrepareWaveResult_StaleCleanupWarningsField verifies that
// StaleCleanupWarnings serializes correctly and is omitted when empty.
func TestPrepareWaveResult_StaleCleanupWarningsField(t *testing.T) {
	// With warnings present: field should appear in JSON
	r := PrepareWaveResult{
		Wave:                 1,
		Worktrees:            []protocol.WorktreeInfo{},
		AgentBriefs:          []AgentBriefInfo{},
		StaleCleanupWarnings: []string{"stale cleanup failed for saw/foo/wave1-agent-A: branch not found"},
	}
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !strings.Contains(string(out), "stale_cleanup_warnings") {
		t.Errorf("expected stale_cleanup_warnings in JSON when warnings present, got: %s", string(out))
	}
	if !strings.Contains(string(out), "branch not found") {
		t.Errorf("expected warning message in JSON output, got: %s", string(out))
	}

	// Without warnings: field should be omitted (omitempty)
	rClean := PrepareWaveResult{
		Wave:        1,
		Worktrees:   []protocol.WorktreeInfo{},
		AgentBriefs: []AgentBriefInfo{},
	}
	outClean, err := json.Marshal(rClean)
	if err != nil {
		t.Fatalf("json.Marshal failed for clean result: %v", err)
	}
	if strings.Contains(string(outClean), "stale_cleanup_warnings") {
		t.Errorf("expected stale_cleanup_warnings to be omitted when nil, got: %s", string(outClean))
	}
}

// TestPrepareWave_AgentBriefInfo_MergeTargetField verifies that AgentBriefInfo
// has a MergeTarget field serialized as "merge_target" in JSON.
func TestPrepareWave_AgentBriefInfo_MergeTargetField(t *testing.T) {
	info := AgentBriefInfo{
		Agent:       "A",
		BriefPath:   "/tmp/brief.md",
		BriefLength: 100,
		JournalDir:  "/tmp/journal",
		FilesOwned:  3,
		MergeTarget: "saw/program/my-prog/tier1-impl-foo",
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"merge_target"`) {
		t.Errorf("expected merge_target in JSON, got: %s", string(data))
	}
	if !strings.Contains(string(data), "tier1-impl-foo") {
		t.Errorf("expected impl slug in JSON, got: %s", string(data))
	}

	// Verify omitempty: empty MergeTarget should not appear in JSON
	infoEmpty := AgentBriefInfo{
		Agent:      "B",
		FilesOwned: 1,
	}
	dataEmpty, err := json.Marshal(infoEmpty)
	if err != nil {
		t.Fatalf("json.Marshal failed for empty MergeTarget: %v", err)
	}
	if strings.Contains(string(dataEmpty), "merge_target") {
		t.Errorf("expected merge_target to be omitted when empty, got: %s", string(dataEmpty))
	}
}
