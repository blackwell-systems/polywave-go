package protocol

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPredictConflictsFromReports_NoConflicts(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A"},
					{ID: "B"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/foo/foo.go"},
				FilesCreated: []string{"pkg/foo/foo_test.go"},
			},
			"B": {
				Status:       "complete",
				FilesChanged: []string{"pkg/bar/bar.go"},
				FilesCreated: []string{"pkg/bar/bar_test.go"},
			},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if res.IsFatal() {
		t.Errorf("expected non-fatal result for non-overlapping files, got: %v", res.Errors)
	}
	if res.IsPartial() {
		t.Errorf("expected success result for non-overlapping files, got partial with: %v", res.Errors)
	}
	if !res.IsSuccess() {
		t.Errorf("expected success result for non-overlapping files")
	}
	if res.GetData().ConflictsDetected != 0 {
		t.Errorf("expected 0 conflicts, got %d", res.GetData().ConflictsDetected)
	}
}

func TestPredictConflictsFromReports_ConflictDetected(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A"},
					{ID: "B"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/shared/shared.go"},
			},
			"B": {
				Status:       "complete",
				FilesChanged: []string{"pkg/shared/shared.go"},
			},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if res.IsSuccess() {
		t.Fatal("expected non-success result for file modified by 2 agents, got success")
	}
	if !res.IsPartial() {
		t.Errorf("expected partial result for conflict, got code: %s", res.Code)
	}
	if res.GetData().ConflictsDetected == 0 {
		t.Error("expected ConflictsDetected > 0")
	}
	// Check that at least one warning mentions E11
	found := false
	for _, e := range res.Errors {
		if len(e.Message) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected errors in partial result, got none")
	}
}

func TestPredictConflictsFromReports_FilesCreatedConflict(t *testing.T) {
	// Two agents claiming to create the same file is also a conflict.
	manifest := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A"},
					{ID: "B"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesCreated: []string{"pkg/new/new.go"},
			},
			"B": {
				Status:       "complete",
				FilesCreated: []string{"pkg/new/new.go"},
			},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if res.IsSuccess() {
		t.Fatal("expected non-success for file created by 2 agents, got success")
	}
	if res.GetData().ConflictsDetected == 0 {
		t.Error("expected ConflictsDetected > 0")
	}
}

func TestPredictConflictsFromReports_IMPLFilesIgnored(t *testing.T) {
	// IMPL docs and .saw-state files should not trigger conflict errors,
	// since multiple agents are expected to update them.
	manifest := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A"},
					{ID: "B"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"docs/IMPL/IMPL-feature.yaml"},
			},
			"B": {
				Status:       "complete",
				FilesChanged: []string{"docs/IMPL/IMPL-feature.yaml"},
			},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if !res.IsSuccess() {
		t.Errorf("expected success for IMPL file overlap, got: %v", res.Errors)
	}
}

func TestPredictConflictsFromReports_SawStateFilesIgnored(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A"},
					{ID: "B"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{".saw-state/wave1/agent-A/brief.md"},
			},
			"B": {
				Status:       "complete",
				FilesChanged: []string{".saw-state/wave1/agent-A/brief.md"},
			},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if !res.IsSuccess() {
		t.Errorf("expected success for .saw-state file overlap, got: %v", res.Errors)
	}
}

func TestPredictConflictsFromReports_NilManifest(t *testing.T) {
	res := PredictConflictsFromReports(context.Background(), nil, 1)
	if !res.IsSuccess() {
		t.Errorf("expected success for nil manifest, got: %v", res.Errors)
	}
}

func TestPredictConflictsFromReports_WaveNotFound(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", FilesChanged: []string{"pkg/foo.go"}},
		},
	}

	// Wave 2 doesn't exist — should return success with no conflicts.
	res := PredictConflictsFromReports(context.Background(), manifest, 2)
	if !res.IsSuccess() {
		t.Errorf("expected success for missing wave, got: %v", res.Errors)
	}
}

func TestPredictConflictsFromReports_MissingReportSkipped(t *testing.T) {
	// Agent B has no completion report — only A's files are counted.
	manifest := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A"},
					{ID: "B"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/foo/foo.go"},
			},
			// B has no report
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if !res.IsSuccess() {
		t.Errorf("expected success when B has no report, got: %v", res.Errors)
	}
}

func TestPredictConflictsFromReports_ConflictData_Count(t *testing.T) {
	// Verify ConflictsDetected matches the number of conflicting files.
	manifest := &IMPLManifest{
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A"},
					{ID: "B"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/shared/shared.go", "pkg/other/other.go"},
			},
			"B": {
				Status:       "complete",
				FilesChanged: []string{"pkg/shared/shared.go", "pkg/other/other.go"},
			},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if res.IsSuccess() {
		t.Fatal("expected partial result for conflicts, got success")
	}
	data := res.GetData()
	if data.ConflictsDetected != 2 {
		t.Errorf("expected ConflictsDetected=2, got %d", data.ConflictsDetected)
	}
	if len(data.Conflicts) != 2 {
		t.Errorf("expected 2 conflict predictions, got %d", len(data.Conflicts))
	}
}

func TestPredictConflictsFromReports_IdenticalEditsAllowed(t *testing.T) {
	// Create a temporary git repo with two branches having identical file content
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Initialize git repo
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "init.defaultBranch", "main")

	// Create initial commit on main branch
	testFile := filepath.Join(repoPath, "shared.go")
	if err := os.WriteFile(testFile, []byte("package shared\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "initial")
	runGit("branch", "-M", "main")

	// Create branch D with edited file
	runGit("checkout", "-b", "saw/test-feature/wave1-agent-D")
	if err := os.WriteFile(testFile, []byte("package shared\n\nfunc Foo() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "agent D changes")

	// Create branch F with IDENTICAL edit
	runGit("checkout", "main")
	runGit("checkout", "-b", "saw/test-feature/wave1-agent-F")
	if err := os.WriteFile(testFile, []byte("package shared\n\nfunc Foo() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "agent F changes")

	// Test that identical edits are allowed
	manifest := &IMPLManifest{
		Repository:  repoPath,
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "D"},
					{ID: "F"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"D": {
				Status:       "complete",
				FilesChanged: []string{"shared.go"},
			},
			"F": {
				Status:       "complete",
				FilesChanged: []string{"shared.go"},
			},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if !res.IsSuccess() {
		t.Errorf("expected success for identical edits, got: %v", res.Errors)
	}
}

func TestParseDiffHunks_Basic(t *testing.T) {
	diff := `diff --git a/foo.go b/foo.go
--- a/foo.go
+++ b/foo.go
@@ -10,5 +10,7 @@
 context line
-old line
+new line A
+new line B
 context line
@@ -100 +102 @@
-single line
+replaced line
@@ -50,0 +53,3 @@
+pure insertion line 1
+pure insertion line 2
+pure insertion line 3`

	hunks := parseDiffHunks(diff)
	// Expects 3 hunks: [10,14], [100,100], and insertion anchor [50,50].
	if len(hunks) != 3 {
		t.Fatalf("expected 3 hunks, got %d: %+v", len(hunks), hunks)
	}
	if hunks[0].Start != 10 || hunks[0].End != 14 {
		t.Errorf("hunk 0: expected [10,14], got [%d,%d]", hunks[0].Start, hunks[0].End)
	}
	if hunks[1].Start != 100 || hunks[1].End != 100 {
		t.Errorf("hunk 1: expected [100,100], got [%d,%d]", hunks[1].Start, hunks[1].End)
	}
	if hunks[2].Start != 50 || hunks[2].End != 50 {
		t.Errorf("hunk 2 (insertion anchor): expected [50,50], got [%d,%d]", hunks[2].Start, hunks[2].End)
	}
}

func TestHunksOverlap(t *testing.T) {
	cases := []struct {
		name    string
		a, b    []HunkRange
		overlap bool
	}{
		{"non-overlapping", []HunkRange{{10, 20}}, []HunkRange{{30, 40}}, false},
		{"overlapping", []HunkRange{{10, 25}}, []HunkRange{{20, 35}}, true},
		{"adjacent no overlap", []HunkRange{{10, 20}}, []HunkRange{{21, 30}}, false},
		{"same range", []HunkRange{{10, 20}}, []HunkRange{{10, 20}}, true},
		{"one inside other", []HunkRange{{5, 50}}, []HunkRange{{10, 20}}, true},
		{"empty a", nil, []HunkRange{{10, 20}}, false},
		{"empty b", []HunkRange{{10, 20}}, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := hunksOverlap(tc.a, tc.b)
			if got != tc.overlap {
				t.Errorf("hunksOverlap(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.overlap)
			}
		})
	}
}

func TestPredictConflictsFromReports_NonOverlappingEditsAllowed(t *testing.T) {
	// Simulate the cascade-patch pattern: two agents modify different functions
	// in the same file. Agents branch off the same base commit.
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "init.defaultBranch", "main")

	// Base file: two independent functions far apart
	baseContent := "package shared\n\n" +
		"func FuncA(x int) int { return x }\n\n" +
		"// lots of lines in between\n" +
		"// line 5\n// line 6\n// line 7\n// line 8\n// line 9\n// line 10\n" +
		"// line 11\n// line 12\n// line 13\n// line 14\n// line 15\n" +
		"func FuncB(x int) int { return x }\n"
	testFile := filepath.Join(repoPath, "shared.go")
	if err := os.WriteFile(testFile, []byte(baseContent), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "initial")
	runGit("branch", "-M", "main")

	// Agent A: modifies FuncA (near top of file, line 3)
	runGit("checkout", "-b", "saw/cascade-test/wave1-agent-A")
	contentA := "package shared\n\n" +
		"func FuncA(ctx context.Context, x int) int { return x }\n\n" +
		"// lots of lines in between\n" +
		"// line 5\n// line 6\n// line 7\n// line 8\n// line 9\n// line 10\n" +
		"// line 11\n// line 12\n// line 13\n// line 14\n// line 15\n" +
		"func FuncB(x int) int { return x }\n"
	if err := os.WriteFile(testFile, []byte(contentA), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "agent A: add ctx to FuncA")

	// Agent B: modifies FuncB (near bottom of file, line 18)
	runGit("checkout", "main")
	runGit("checkout", "-b", "saw/cascade-test/wave1-agent-B")
	contentB := "package shared\n\n" +
		"func FuncA(x int) int { return x }\n\n" +
		"// lots of lines in between\n" +
		"// line 5\n// line 6\n// line 7\n// line 8\n// line 9\n// line 10\n" +
		"// line 11\n// line 12\n// line 13\n// line 14\n// line 15\n" +
		"func FuncB(ctx context.Context, x int) int { return x }\n"
	if err := os.WriteFile(testFile, []byte(contentB), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "agent B: add ctx to FuncB")

	manifest := &IMPLManifest{
		Repository:  repoPath,
		FeatureSlug: "cascade-test",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{{ID: "A"}, {ID: "B"}},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete", FilesChanged: []string{"shared.go"}},
			"B": {Status: "complete", FilesChanged: []string{"shared.go"}},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if !res.IsSuccess() {
		t.Errorf("expected success for non-overlapping cascade edits, got: %v", res.Errors)
	}
	if res.GetData().ConflictsDetected != 0 {
		t.Errorf("expected 0 conflicts for cascade patch, got %d", res.GetData().ConflictsDetected)
	}
}

func TestPredictConflictsFromReports_DifferingEditsBlocked(t *testing.T) {
	// Create a temporary git repo with two branches having DIFFERENT file content
	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo")

	// Initialize git repo
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")
	runGit("config", "init.defaultBranch", "main")

	// Create initial commit on main branch
	testFile := filepath.Join(repoPath, "shared.go")
	if err := os.WriteFile(testFile, []byte("package shared\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "initial")
	runGit("branch", "-M", "main")

	// Create branch D with one edit
	runGit("checkout", "-b", "saw/test-feature/wave1-agent-D")
	if err := os.WriteFile(testFile, []byte("package shared\n\nfunc Foo() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "agent D changes")

	// Create branch F with DIFFERENT edit
	runGit("checkout", "main")
	runGit("checkout", "-b", "saw/test-feature/wave1-agent-F")
	if err := os.WriteFile(testFile, []byte("package shared\n\nfunc Bar() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "agent F changes")

	// Test that differing edits are blocked
	manifest := &IMPLManifest{
		Repository:  repoPath,
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "D"},
					{ID: "F"},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"D": {
				Status:       "complete",
				FilesChanged: []string{"shared.go"},
			},
			"F": {
				Status:       "complete",
				FilesChanged: []string{"shared.go"},
			},
		},
	}

	res := PredictConflictsFromReports(context.Background(), manifest, 1)
	if res.IsSuccess() {
		t.Fatal("expected non-success for differing edits, got success")
	}
	if !res.IsPartial() {
		t.Errorf("expected partial result, got code: %s", res.Code)
	}
	data := res.GetData()
	if data.ConflictsDetected == 0 {
		t.Error("expected ConflictsDetected > 0 for differing edits")
	}
	// Verify conflict is about shared.go
	found := false
	for _, cp := range data.Conflicts {
		if cp.File == "shared.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected conflict prediction for shared.go")
	}
}
