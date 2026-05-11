package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
)

// allCompleteManifestYAML is a PROGRAM manifest where all IMPLs are complete.
const allCompleteManifestYAML = `title: Test Program
program_slug: test-program
state: TIER_VERIFIED
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    status: complete
  - slug: impl-b
    title: Implementation B
    tier: 2
    status: complete
tiers:
  - number: 1
    impls:
      - impl-a
  - number: 2
    impls:
      - impl-b
completion:
  tiers_complete: 2
  tiers_total: 2
  impls_complete: 2
  impls_total: 2
  total_agents: 4
  total_waves: 2
`

// tierPendingManifestYAML is a PROGRAM manifest where tier 2 is still pending.
const tierPendingManifestYAML = `title: Pending Program
program_slug: pending-program
state: TIER_EXECUTING
impls:
  - slug: impl-a
    title: Implementation A
    tier: 1
    status: complete
  - slug: impl-b
    title: Implementation B
    tier: 2
    status: pending
tiers:
  - number: 1
    impls:
      - impl-a
  - number: 2
    impls:
      - impl-b
completion:
  tiers_complete: 1
  tiers_total: 2
  impls_complete: 1
  impls_total: 2
  total_agents: 2
  total_waves: 1
`

// TestMarkProgramComplete_AllComplete tests the happy path: all tiers complete.
func TestMarkProgramComplete_AllComplete(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(manifestPath, []byte(allCompleteManifestYAML), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newMarkProgramCompleteCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath, "--date", "2026-01-15", "--repo-dir", tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s\nstdout: %s", err, stderr.String(), stdout.String())
	}

	var result engine.MarkProgramCompleteResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	if !result.Completed {
		t.Errorf("expected completed: true, got false")
	}
	if result.ProgramSlug != "test-program" {
		t.Errorf("expected program_slug: test-program, got %s", result.ProgramSlug)
	}
	if result.Date != "2026-01-15" {
		t.Errorf("expected date: 2026-01-15, got %s", result.Date)
	}
	if result.TiersComplete != 2 {
		t.Errorf("expected tiers_complete: 2, got %d", result.TiersComplete)
	}
	if result.ImplsComplete != 2 {
		t.Errorf("expected impls_complete: 2, got %d", result.ImplsComplete)
	}

	// Verify archived path is set
	if result.ArchivedPath == "" {
		t.Errorf("expected archived_path to be set")
	}

	// Verify manifest was archived and updated with SAW:PROGRAM:COMPLETE marker
	archivedPath := result.ArchivedPath
	if archivedPath == "" {
		archivedPath = manifestPath // fallback for test
	}
	data, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatalf("failed to read archived manifest at %s: %v", archivedPath, err)
	}
	content := string(data)
	if !strings.Contains(content, "SAW:PROGRAM:COMPLETE") {
		t.Errorf("expected SAW:PROGRAM:COMPLETE marker in manifest")
	}
	if !strings.Contains(content, "state: COMPLETE") {
		t.Errorf("expected state: COMPLETE in manifest")
	}
	if !strings.Contains(content, "2026-01-15") {
		t.Errorf("expected date 2026-01-15 in manifest")
	}
}

// TestMarkProgramComplete_NotComplete_TierPending tests that MarkProgramComplete
// returns an error when not all IMPLs are complete.
func TestMarkProgramComplete_NotComplete_TierPending(t *testing.T) {
	tmpDir := t.TempDir()

	manifestPath := filepath.Join(tmpDir, "PROGRAM-pending.yaml")
	if err := os.WriteFile(manifestPath, []byte(tierPendingManifestYAML), 0644); err != nil {
		t.Fatal(err)
	}

	res := engine.MarkProgramComplete(context.Background(), engine.MarkProgramCompleteOpts{
		ManifestPath: manifestPath,
		RepoDir:      tmpDir,
	})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for incomplete tiers, got success/partial")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors for incomplete tiers, got none")
	}
	errMsg := res.Errors[0].Message
	if !strings.Contains(errMsg, "impl-b") {
		t.Errorf("expected error to mention impl-b, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "pending") {
		t.Errorf("expected error to mention 'pending' status, got: %s", errMsg)
	}
}

// TestMarkProgramComplete_UpdatesContext tests that CONTEXT.md is created/updated.
func TestMarkProgramComplete_UpdatesContext(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)

	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(manifestPath, []byte(allCompleteManifestYAML), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	cmd := newMarkProgramCompleteCmd()
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{manifestPath, "--date", "2026-01-20", "--repo-dir", tmpDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	contextPath := filepath.Join(tmpDir, "docs", "CONTEXT.md")
	data, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("CONTEXT.md not created: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "Test Program") {
		t.Errorf("expected program title in CONTEXT.md, got:\n%s", content)
	}
	if !strings.Contains(content, "test-program") {
		t.Errorf("expected program slug in CONTEXT.md, got:\n%s", content)
	}
	if !strings.Contains(content, "2026-01-20") {
		t.Errorf("expected date in CONTEXT.md, got:\n%s", content)
	}
	if !strings.Contains(content, "2 tiers") {
		t.Errorf("expected tier count in CONTEXT.md, got:\n%s", content)
	}

	var result engine.MarkProgramCompleteResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if !result.ContextUpdated {
		t.Errorf("expected context_updated: true, got false")
	}
}

// TestMarkProgramComplete_InvalidManifest tests that an IMPL with no status
// (empty string) is treated as incomplete.
func TestMarkProgramComplete_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	missingStatusManifest := `title: Broken Program
program_slug: broken-program
state: TIER_EXECUTING
impls:
  - slug: impl-x
    title: X
    tier: 1
tiers:
  - number: 1
    impls:
      - impl-x
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 0
  total_waves: 0
`
	manifestPath := filepath.Join(tmpDir, "PROGRAM-broken.yaml")
	if err := os.WriteFile(manifestPath, []byte(missingStatusManifest), 0644); err != nil {
		t.Fatal(err)
	}

	res := engine.MarkProgramComplete(context.Background(), engine.MarkProgramCompleteOpts{
		ManifestPath: manifestPath,
		RepoDir:      tmpDir,
	})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for impl with missing status, got success/partial")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors for impl with missing status, got none")
	}
	errMsg := res.Errors[0].Message
	if !strings.Contains(errMsg, "impl-x") {
		t.Errorf("expected error to mention impl-x, got: %s", errMsg)
	}
}
