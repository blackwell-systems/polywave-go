package journal

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateContext_EmptyJournal(t *testing.T) {
	result, err := GenerateContext([]ToolEntry{}, 0)
	if err != nil {
		t.Fatalf("GenerateContext failed: %v", err)
	}

	expected := "## Session Context (Recovered from Tool Journal)\n\n**No tool activity recorded yet.**\n"
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}
}

func TestExtractFilesModified_EditTool(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Edit",
			ToolUseID: "edit1",
			Input: map[string]interface{}{
				"file_path":  "pkg/api/routes.go",
				"old_string": "func Old() {\n\treturn\n}",
				"new_string": "func New() {\n\treturn \"updated\"\n}",
			},
		},
	}

	result := extractFilesModified(entries)

	if len(result) != 1 {
		t.Fatalf("Expected 1 file modification, got %d", len(result))
	}

	fm := result[0]
	if fm.Path != "pkg/api/routes.go" {
		t.Errorf("Expected path pkg/api/routes.go, got %s", fm.Path)
	}
	if fm.Operation != "modified" {
		t.Errorf("Expected operation 'modified', got %s", fm.Operation)
	}
	if fm.LinesAdded != 3 {
		t.Errorf("Expected 3 lines added, got %d", fm.LinesAdded)
	}
	if fm.LinesDeleted != 3 {
		t.Errorf("Expected 3 lines deleted, got %d", fm.LinesDeleted)
	}
}

func TestExtractFilesModified_WriteTool(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Write",
			ToolUseID: "write1",
			Input: map[string]interface{}{
				"file_path": "pkg/api/handler.go",
				"content":   "package api\n\nfunc Handler() {\n\treturn\n}",
			},
		},
	}

	result := extractFilesModified(entries)

	if len(result) != 1 {
		t.Fatalf("Expected 1 file modification, got %d", len(result))
	}

	fm := result[0]
	if fm.Path != "pkg/api/handler.go" {
		t.Errorf("Expected path pkg/api/handler.go, got %s", fm.Path)
	}
	if fm.Operation != "added" {
		t.Errorf("Expected operation 'added', got %s", fm.Operation)
	}
	if fm.LinesAdded != 5 {
		t.Errorf("Expected 5 lines added, got %d", fm.LinesAdded)
	}
}

func TestExtractFilesModified_MultipleEdits(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Edit",
			ToolUseID: "edit1",
			Input: map[string]interface{}{
				"file_path":  "pkg/api/routes.go",
				"old_string": "old",
				"new_string": "new1\nnew2",
			},
		},
		{
			Timestamp: now.Add(time.Minute),
			Kind:      "tool_use",
			ToolName:  "Edit",
			ToolUseID: "edit2",
			Input: map[string]interface{}{
				"file_path":  "pkg/api/routes.go",
				"old_string": "old2",
				"new_string": "new3\nnew4\nnew5",
			},
		},
	}

	result := extractFilesModified(entries)

	if len(result) != 1 {
		t.Fatalf("Expected 1 file modification, got %d", len(result))
	}

	fm := result[0]
	if fm.Path != "pkg/api/routes.go" {
		t.Errorf("Expected path pkg/api/routes.go, got %s", fm.Path)
	}
	if fm.LinesAdded != 5 { // 2 + 3
		t.Errorf("Expected 5 lines added, got %d", fm.LinesAdded)
	}
	if fm.LinesDeleted != 2 { // 1 + 1
		t.Errorf("Expected 2 lines deleted, got %d", fm.LinesDeleted)
	}
}

func TestExtractTestResults_GoTest(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash1",
			Input: map[string]interface{}{
				"command": "go test ./pkg/api",
			},
		},
		{
			Timestamp: now.Add(time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash1",
			Preview:   "PASS\nok  \tpkg/api\t0.123s\n14 passed",
		},
	}

	result := extractTestResults(entries)

	if len(result) != 1 {
		t.Fatalf("Expected 1 test result, got %d", len(result))
	}

	tr := result[0]
	if tr.Command != "go test ./pkg/api" {
		t.Errorf("Expected command 'go test ./pkg/api', got %s", tr.Command)
	}
	if tr.Passed != 14 {
		t.Errorf("Expected 14 passed, got %d", tr.Passed)
	}
	if tr.Failed != 0 {
		t.Errorf("Expected 0 failed, got %d", tr.Failed)
	}
}

func TestExtractTestResults_GoTestWithFailures(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash1",
			Input: map[string]interface{}{
				"command": "go test ./pkg/api",
			},
		},
		{
			Timestamp: now.Add(time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash1",
			Preview:   "FAIL\tpkg/api\t0.123s\n12 passed 2 failed",
		},
	}

	result := extractTestResults(entries)

	if len(result) != 1 {
		t.Fatalf("Expected 1 test result, got %d", len(result))
	}

	tr := result[0]
	if tr.Passed != 12 {
		t.Errorf("Expected 12 passed, got %d", tr.Passed)
	}
	if tr.Failed != 2 {
		t.Errorf("Expected 2 failed, got %d", tr.Failed)
	}
}

func TestExtractTestResults_MultipleRuns(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash1",
			Input: map[string]interface{}{
				"command": "go test ./pkg/api",
			},
		},
		{
			Timestamp: now.Add(time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash1",
			Preview:   "PASS\n10 passed",
		},
		{
			Timestamp: now.Add(time.Minute),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash2",
			Input: map[string]interface{}{
				"command": "go test ./pkg/engine",
			},
		},
		{
			Timestamp: now.Add(time.Minute + time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash2",
			Preview:   "PASS\n5 passed",
		},
	}

	result := extractTestResults(entries)

	if len(result) != 2 {
		t.Fatalf("Expected 2 test results, got %d", len(result))
	}

	if result[0].Passed != 10 {
		t.Errorf("Expected first test 10 passed, got %d", result[0].Passed)
	}
	if result[1].Passed != 5 {
		t.Errorf("Expected second test 5 passed, got %d", result[1].Passed)
	}
}

func TestExtractGitCommits_ParseSHA(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash1",
			Input: map[string]interface{}{
				"command": "git commit -m \"feat: add API endpoints\"",
			},
		},
		{
			Timestamp: now.Add(time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash1",
			Preview:   "[wave1-agent-A abc123d] feat: add API endpoints\n 4 files changed, 126 insertions(+), 10 deletions(-)",
		},
	}

	result := extractGitCommits(entries)

	if len(result) != 1 {
		t.Fatalf("Expected 1 git commit, got %d", len(result))
	}

	gc := result[0]
	if gc.SHA != "abc123d" {
		t.Errorf("Expected SHA abc123d, got %s", gc.SHA)
	}
	if gc.Branch != "wave1-agent-A" {
		t.Errorf("Expected branch wave1-agent-A, got %s", gc.Branch)
	}
	if gc.Message != "feat: add API endpoints" {
		t.Errorf("Expected message 'feat: add API endpoints', got %s", gc.Message)
	}
}

func TestExtractGitCommits_ParseStats(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash1",
			Input: map[string]interface{}{
				"command": "git commit -m \"fix: bug fix\"",
			},
		},
		{
			Timestamp: now.Add(time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash1",
			Preview:   "[main 1a2b3c4] fix: bug fix\n 4 files changed, 126 insertions(+), 10 deletions(-)",
		},
	}

	result := extractGitCommits(entries)

	if len(result) != 1 {
		t.Fatalf("Expected 1 git commit, got %d", len(result))
	}

	gc := result[0]
	if gc.Files != 4 {
		t.Errorf("Expected 4 files changed, got %d", gc.Files)
	}
	if gc.Insertions != 126 {
		t.Errorf("Expected 126 insertions, got %d", gc.Insertions)
	}
	if gc.Deletions != 10 {
		t.Errorf("Expected 10 deletions, got %d", gc.Deletions)
	}
}

func TestExtractScaffoldImports_DetectsReadOfScaffoldPaths(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "read1",
			Input: map[string]interface{}{
				"file_path": "pkg/journal/types.go",
			},
		},
		{
			Timestamp: now.Add(time.Second),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "read2",
			Input: map[string]interface{}{
				"file_path": "pkg/api/handler.go",
			},
		},
		{
			Timestamp: now.Add(2 * time.Second),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "read3",
			Input: map[string]interface{}{
				"file_path": "scaffold/user_interface.go",
			},
		},
	}

	result := extractScaffoldImports(entries)

	if len(result) != 2 {
		t.Fatalf("Expected 2 scaffold imports, got %d", len(result))
	}

	// Check that types.go and scaffold path are detected
	foundTypes := false
	foundScaffold := false
	for _, path := range result {
		if path == "pkg/journal/types.go" {
			foundTypes = true
		}
		if path == "scaffold/user_interface.go" {
			foundScaffold = true
		}
	}

	if !foundTypes {
		t.Error("Expected to find pkg/journal/types.go in scaffold imports")
	}
	if !foundScaffold {
		t.Error("Expected to find scaffold/user_interface.go in scaffold imports")
	}
}

func TestExtractVerificationGates_DetectsGateCommands(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash1",
			Input: map[string]interface{}{
				"command": "go build ./pkg/journal/...",
			},
		},
		{
			Timestamp: now.Add(time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash1",
			Preview:   "",
		},
		{
			Timestamp: now.Add(time.Minute),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash2",
			Input: map[string]interface{}{
				"command": "go test ./pkg/journal/...",
			},
		},
		{
			Timestamp: now.Add(time.Minute + time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash2",
			Preview:   "PASS\nok  \tpkg/journal\t0.123s",
		},
		{
			Timestamp: now.Add(2 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash3",
			Input: map[string]interface{}{
				"command": "go vet ./pkg/journal/...",
			},
		},
		{
			Timestamp: now.Add(2*time.Minute + time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash3",
			Preview:   "",
		},
	}

	result := extractVerificationGates(entries)

	if len(result) != 3 {
		t.Fatalf("Expected 3 verification gates, got %d", len(result))
	}

	if buildGate, ok := result["Build"]; !ok {
		t.Error("Expected Build gate to be present")
	} else if buildGate.Status != "PASS" {
		t.Errorf("Expected Build gate status PASS, got %s", buildGate.Status)
	}

	if testGate, ok := result["Tests"]; !ok {
		t.Error("Expected Tests gate to be present")
	} else if testGate.Status != "PASS" {
		t.Errorf("Expected Tests gate status PASS, got %s", testGate.Status)
	}

	if lintGate, ok := result["Lint"]; !ok {
		t.Error("Expected Lint gate to be present")
	} else if lintGate.Status != "PASS" {
		t.Errorf("Expected Lint gate status PASS, got %s", lintGate.Status)
	}
}

func TestExtractVerificationGates_DetectsFailures(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash1",
			Input: map[string]interface{}{
				"command": "go test ./pkg/journal/...",
			},
		},
		{
			Timestamp: now.Add(time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash1",
			Preview:   "FAIL\tpkg/journal\t0.123s\nerror: test failed",
		},
	}

	result := extractVerificationGates(entries)

	if testGate, ok := result["Tests"]; !ok {
		t.Error("Expected Tests gate to be present")
	} else if testGate.Status != "FAIL" {
		t.Errorf("Expected Tests gate status FAIL, got %s", testGate.Status)
	}
}

func TestExtractCompletionReportStatus_DetectsReport(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Edit",
			ToolUseID: "edit1",
			Input: map[string]interface{}{
				"file_path": "docs/IMPL/IMPL-tool-journaling.yaml",
				"new_string": "## Agent B - Completion Report\n\n```yaml type=impl-completion-report\nstatus: complete\n```",
			},
		},
	}

	result := extractCompletionReportStatus(entries)

	if !result {
		t.Error("Expected completion report to be detected")
	}
}

func TestExtractCompletionReportStatus_NoReport(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Edit",
			ToolUseID: "edit1",
			Input: map[string]interface{}{
				"file_path":  "pkg/api/routes.go",
				"new_string": "func Handler() {}",
			},
		},
	}

	result := extractCompletionReportStatus(entries)

	if result {
		t.Error("Expected no completion report to be detected")
	}
}

func TestGenerateContext_FullIntegration(t *testing.T) {
	now := time.Now()
	entries := []ToolEntry{
		// Read scaffold
		{
			Timestamp: now,
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "read1",
			Input: map[string]interface{}{
				"file_path": "pkg/journal/types.go",
			},
		},
		// Write new file
		{
			Timestamp: now.Add(time.Minute),
			Kind:      "tool_use",
			ToolName:  "Write",
			ToolUseID: "write1",
			Input: map[string]interface{}{
				"file_path": "pkg/journal/context.go",
				"content":   "package journal\n\nfunc GenerateContext() {}",
			},
		},
		// Edit file
		{
			Timestamp: now.Add(2 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Edit",
			ToolUseID: "edit1",
			Input: map[string]interface{}{
				"file_path":  "pkg/journal/context.go",
				"old_string": "func GenerateContext() {}",
				"new_string": "func GenerateContext(entries []ToolEntry) string {\n\treturn \"\"\n}",
			},
		},
		// Run build
		{
			Timestamp: now.Add(3 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash1",
			Input: map[string]interface{}{
				"command": "go build ./pkg/journal/...",
			},
		},
		{
			Timestamp: now.Add(3*time.Minute + time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash1",
			Preview:   "",
		},
		// Run tests
		{
			Timestamp: now.Add(4 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash2",
			Input: map[string]interface{}{
				"command": "go test ./pkg/journal/...",
			},
		},
		{
			Timestamp: now.Add(4*time.Minute + time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash2",
			Preview:   "PASS\nok  \tpkg/journal\t0.123s\n10 passed",
		},
		// Git commit
		{
			Timestamp: now.Add(5 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "bash3",
			Input: map[string]interface{}{
				"command": "git commit -m \"feat: implement context generator\"",
			},
		},
		{
			Timestamp: now.Add(5*time.Minute + time.Second),
			Kind:      "tool_result",
			ToolUseID: "bash3",
			Preview:   "[wave1-agent-B abc123d] feat: implement context generator\n 2 files changed, 150 insertions(+)",
		},
	}

	result, err := GenerateContext(entries, 0)
	if err != nil {
		t.Fatalf("GenerateContext failed: %v", err)
	}

	// Check key sections are present
	expectedSections := []string{
		"## Session Context (Recovered from Tool Journal)",
		"**Last activity:**",
		"**Total tool calls:** 9",
		"### Files Modified (1)",
		"pkg/journal/context.go",
		"### Tests Run",
		"go test ./pkg/journal/...",
		"10 passed",
		"### Git Commits",
		"abc123d",
		"feat: implement context generator",
		"### Scaffold Files Imported",
		"pkg/journal/types.go",
		"### Verification Status (Field 6 Gates)",
		"Build:",
		"Tests:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(result, section) {
			t.Errorf("Expected context to contain '%s', but it didn't.\nFull output:\n%s", section, result)
		}
	}
}

func TestGenerateContext_MaxEntriesLimit(t *testing.T) {
	now := time.Now()
	entries := make([]ToolEntry, 100)
	for i := 0; i < 100; i++ {
		entries[i] = ToolEntry{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "read" + string(rune(i)),
			Input: map[string]interface{}{
				"file_path": "test.go",
			},
		}
	}

	result, err := GenerateContext(entries, 10)
	if err != nil {
		t.Fatalf("GenerateContext failed: %v", err)
	}

	// Should only process last 10 entries
	if !strings.Contains(result, "**Total tool calls:** 100") {
		t.Error("Expected total tool calls to still be 100")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "just now"},
		{1 * time.Minute, "1 minute"},
		{5 * time.Minute, "5 minutes"},
		{1 * time.Hour, "1 hour"},
		{3 * time.Hour, "3 hours"},
		{24 * time.Hour, "1 day"},
		{48 * time.Hour, "2 days"},
	}

	for _, tt := range tests {
		result := formatRelativeTime(tt.duration)
		if result != tt.expected {
			t.Errorf("formatRelativeTime(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{1 * time.Minute, "1m"},
		{5 * time.Minute, "5m"},
		{1 * time.Hour, "1h 0m"},
		{1*time.Hour + 23*time.Minute, "1h 23m"},
		{24 * time.Hour, "1d 0h"},
		{25*time.Hour + 30*time.Minute, "1d 1h"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", tt.duration, result, tt.expected)
		}
	}
}

func TestFormatFileChange(t *testing.T) {
	tests := []struct {
		fm       FileModification
		expected string
	}{
		{
			FileModification{Operation: "added", LinesAdded: 50},
			"added, 50 lines",
		},
		{
			FileModification{Operation: "modified", LinesAdded: 30, LinesDeleted: 0},
			"added 30 lines",
		},
		{
			FileModification{Operation: "modified", LinesAdded: 0, LinesDeleted: 20},
			"deleted 20 lines",
		},
		{
			FileModification{Operation: "modified", LinesAdded: 30, LinesDeleted: 20},
			"+30/-20 lines",
		},
	}

	for _, tt := range tests {
		result := formatFileChange(tt.fm)
		if result != tt.expected {
			t.Errorf("formatFileChange(%+v) = %s, expected %s", tt.fm, result, tt.expected)
		}
	}
}
