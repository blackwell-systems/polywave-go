package protocol

import (
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

	res := PredictConflictsFromReports(manifest, 1)
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

	res := PredictConflictsFromReports(manifest, 1)
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

	res := PredictConflictsFromReports(manifest, 1)
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

	res := PredictConflictsFromReports(manifest, 1)
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

	res := PredictConflictsFromReports(manifest, 1)
	if !res.IsSuccess() {
		t.Errorf("expected success for .saw-state file overlap, got: %v", res.Errors)
	}
}

func TestPredictConflictsFromReports_NilManifest(t *testing.T) {
	res := PredictConflictsFromReports(nil, 1)
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
	res := PredictConflictsFromReports(manifest, 2)
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

	res := PredictConflictsFromReports(manifest, 1)
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

	res := PredictConflictsFromReports(manifest, 1)
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

	res := PredictConflictsFromReports(manifest, 1)
	if !res.IsSuccess() {
		t.Errorf("expected success for identical edits, got: %v", res.Errors)
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

	res := PredictConflictsFromReports(manifest, 1)
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
