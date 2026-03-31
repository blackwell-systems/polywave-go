package protocol

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/commands"
)

func TestPopulateVerificationGates_GoProject(t *testing.T) {
	// Setup manifest with 2 agents
	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:    "A",
						Task:  "Implement auth handler",
						Files: []string{"pkg/auth/handler.go", "pkg/auth/handler_test.go"},
					},
					{
						ID:    "B",
						Task:  "Implement database layer",
						Files: []string{"pkg/db/store.go", "pkg/db/store_test.go"},
					},
				},
			},
		},
	}

	// Setup Go CommandSet
	commandSet := &commands.CommandSet{
		Toolchain: "go",
		Commands: commands.Commands{
			Build: "go build ./...",
			Test: commands.TestCommands{
				Full:           "go test ./...",
				FocusedPattern: "go test ./{package} -run {TestPrefix}",
			},
			Lint: commands.LintCommands{
				Check: "go vet ./...",
			},
		},
	}

	// Execute with single-repo setup
	commandSets := map[string]*commands.CommandSet{
		".": commandSet,
	}
	repoMap := map[string]string{
		".": ".",
	}
	result, err := PopulateVerificationGates(context.Background(), manifest, commandSets, repoMap)

	// Verify no error
	if err != nil {
		t.Fatalf("PopulateVerificationGates failed: %v", err)
	}

	// Verify both agents have verification blocks
	if !strings.Contains(result.Waves[0].Agents[0].Task, "## Verification Gate") {
		t.Errorf("Agent A missing verification gate")
	}
	if !strings.Contains(result.Waves[0].Agents[1].Task, "## Verification Gate") {
		t.Errorf("Agent B missing verification gate")
	}

	// Verify focused test patterns
	agentATask := result.Waves[0].Agents[0].Task
	if !strings.Contains(agentATask, "go test ./pkg/auth") {
		t.Errorf("Agent A verification gate missing focused test pattern, got: %s", agentATask)
	}

	agentBTask := result.Waves[0].Agents[1].Task
	if !strings.Contains(agentBTask, "go test ./pkg/db") {
		t.Errorf("Agent B verification gate missing focused test pattern, got: %s", agentBTask)
	}

	// Verify original manifest unchanged
	if strings.Contains(manifest.Waves[0].Agents[0].Task, "## Verification Gate") {
		t.Errorf("Original manifest was modified (should be immutable)")
	}
}

func TestPopulateVerificationGates_Idempotent(t *testing.T) {
	// Setup manifest where Agent A already has verification gate
	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:    "A",
						Task:  "Implement handler\n\n## Verification Gate\n```bash\ngo test ./pkg/auth\n```",
						Files: []string{"pkg/auth/handler.go"},
					},
					{
						ID:    "B",
						Task:  "Implement database layer",
						Files: []string{"pkg/db/store.go"},
					},
				},
			},
		},
	}

	commandSet := &commands.CommandSet{
		Toolchain: "go",
		Commands: commands.Commands{
			Build: "go build ./...",
			Test: commands.TestCommands{
				Full:           "go test ./...",
				FocusedPattern: "go test ./{package} -run {TestPrefix}",
			},
			Lint: commands.LintCommands{
				Check: "go vet ./...",
			},
		},
	}

	originalTaskA := manifest.Waves[0].Agents[0].Task

	// Execute with single-repo setup
	commandSets := map[string]*commands.CommandSet{
		".": commandSet,
	}
	repoMap := map[string]string{
		".": ".",
	}
	result, err := PopulateVerificationGates(context.Background(), manifest, commandSets, repoMap)
	if err != nil {
		t.Fatalf("PopulateVerificationGates failed: %v", err)
	}

	// Verify Agent A's task unchanged (already has gate)
	if result.Waves[0].Agents[0].Task != originalTaskA {
		t.Errorf("Agent A task was modified despite already having verification gate")
	}

	// Verify Agent B got verification gate
	if !strings.Contains(result.Waves[0].Agents[1].Task, "## Verification Gate") {
		t.Errorf("Agent B missing verification gate")
	}
}

func TestPopulateVerificationGates_MissingH2Data(t *testing.T) {
	manifest := &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{
						ID:    "A",
						Task:  "Implement handler",
						Files: []string{"pkg/auth/handler.go"},
					},
				},
			},
		},
	}

	// Execute with empty commandSets map
	_, err := PopulateVerificationGates(context.Background(), manifest, nil, nil)

	// Verify error returned
	if err == nil {
		t.Fatal("Expected error for nil CommandSet, got nil")
	}

	if !strings.Contains(err.Error(), "H2 data unavailable") {
		t.Errorf("Expected 'H2 data unavailable' error, got: %v", err)
	}
}

func TestDetermineFocusedTestPattern_SinglePackage(t *testing.T) {
	files := []string{"pkg/auth/handler.go", "pkg/auth/handler_test.go"}
	commandSet := &commands.CommandSet{
		Toolchain: "go",
		Commands: commands.Commands{
			Test: commands.TestCommands{
				Full:           "go test ./...",
				FocusedPattern: "go test ./{package} -run {TestPrefix}",
			},
		},
	}

	result := DetermineFocusedTestPattern(files, commandSet)

	expected := "go test ./pkg/auth -run TestAuth"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDetermineFocusedTestPattern_MultiplePackages(t *testing.T) {
	files := []string{"pkg/auth/handler.go", "pkg/db/store.go"}
	commandSet := &commands.CommandSet{
		Toolchain: "go",
		Commands: commands.Commands{
			Test: commands.TestCommands{
				Full:           "go test ./...",
				FocusedPattern: "go test ./{package} -run {TestPrefix}",
			},
		},
	}

	result := DetermineFocusedTestPattern(files, commandSet)

	// Should return full suite command (cannot focus across packages)
	if result != "go test ./..." {
		t.Errorf("Expected full suite command for multiple packages, got: %q", result)
	}
}

func TestDetermineFocusedTestPattern_EmptyFiles(t *testing.T) {
	files := []string{}
	commandSet := &commands.CommandSet{
		Toolchain: "go",
		Commands: commands.Commands{
			Test: commands.TestCommands{
				Full:           "go test ./...",
				FocusedPattern: "go test ./{package} -run {TestPrefix}",
			},
		},
	}

	result := DetermineFocusedTestPattern(files, commandSet)

	// Should return full suite command (no files to focus on)
	if result != "go test ./..." {
		t.Errorf("Expected full suite command for empty files, got: %q", result)
	}
}

func TestDetermineFocusedTestPattern_NoFocusedPattern(t *testing.T) {
	files := []string{"pkg/auth/handler.go"}
	commandSet := &commands.CommandSet{
		Toolchain: "go",
		Commands: commands.Commands{
			Test: commands.TestCommands{
				Full:           "go test ./...",
				FocusedPattern: "", // No focused pattern available
			},
		},
	}

	result := DetermineFocusedTestPattern(files, commandSet)

	// Should return full suite command
	if result != "go test ./..." {
		t.Errorf("Expected full suite command when no focused pattern, got: %q", result)
	}
}

func TestDetermineFocusedTestPattern_Rust(t *testing.T) {
	files := []string{"src/auth/mod.rs"}
	commandSet := &commands.CommandSet{
		Toolchain: "rust",
		Commands: commands.Commands{
			Test: commands.TestCommands{
				Full:           "cargo test",
				FocusedPattern: "cargo test {module_name}",
			},
		},
	}

	result := DetermineFocusedTestPattern(files, commandSet)

	expected := "cargo test auth"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDetermineFocusedTestPattern_JavaScript(t *testing.T) {
	files := []string{"src/components/auth.js"}
	commandSet := &commands.CommandSet{
		Toolchain: "javascript",
		Commands: commands.Commands{
			Test: commands.TestCommands{
				Full:           "npm test",
				FocusedPattern: "npm test -- --grep \"{module}\"",
			},
		},
	}

	result := DetermineFocusedTestPattern(files, commandSet)

	// Module pattern uses directory name, not filename
	expected := "npm test -- --grep \"components\""
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestDetermineFocusedTestPattern_Python(t *testing.T) {
	files := []string{"pkg/auth/handler.py"}
	commandSet := &commands.CommandSet{
		Toolchain: "python",
		Commands: commands.Commands{
			Test: commands.TestCommands{
				Full:           "pytest",
				FocusedPattern: "pytest {path}/test_*.py",
			},
		},
	}

	result := DetermineFocusedTestPattern(files, commandSet)

	expected := "pytest pkg/auth/test_*.py"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestFormatVerificationBlock(t *testing.T) {
	buildCmd := "go build ./..."
	lintCmd := "go vet ./..."
	testCmd := "go test ./pkg/auth"

	result := FormatVerificationBlock(buildCmd, lintCmd, testCmd)

	// Verify structure
	if !strings.Contains(result, "## Verification Gate") {
		t.Errorf("Missing verification gate header")
	}

	if !strings.Contains(result, "```bash") {
		t.Errorf("Missing bash fence opening")
	}

	if !strings.Contains(result, buildCmd) {
		t.Errorf("Missing build command")
	}

	if !strings.Contains(result, lintCmd) {
		t.Errorf("Missing lint command")
	}

	if !strings.Contains(result, testCmd) {
		t.Errorf("Missing test command")
	}

	// Verify proper fence closure
	lines := strings.Split(result, "\n")
	lastLine := lines[len(lines)-2] // -2 because last line is empty
	if lastLine != "```" {
		t.Errorf("Expected closing fence, got: %q", lastLine)
	}
}

func TestFinalizeIMPL_Success(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()

	// Create minimal IMPL file
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	implContent := `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
test_command: "go test ./..."
lint_command: "go vet ./..."
file_ownership:
  - file: "pkg/auth/handler.go"
    agent: "A"
    wave: 1
  - file: "pkg/auth/handler_test.go"
    agent: "A"
    wave: 1
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: "A"
        task: "Implement auth handler"
        files: ["pkg/auth/handler.go", "pkg/auth/handler_test.go"]
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to write test IMPL file: %v", err)
	}

	// Create minimal repo structure for H2 extraction
	repoRoot := tmpDir
	goModPath := filepath.Join(repoRoot, "go.mod")
	goModContent := "module github.com/test/project\n\ngo 1.21\n"
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Execute
	res := FinalizeIMPL(implPath, repoRoot)
	if !res.IsSuccess() {
		t.Fatalf("FinalizeIMPL failed. Errors: %v", res.Errors)
	}
	data := res.GetData()

	// Verify initial validation passed
	if !data.Validation.Passed {
		t.Errorf("Initial validation should pass")
	}

	// Verify gate population stats
	if data.GatePopulation.AgentsUpdated != 1 {
		t.Errorf("Expected 1 agent updated, got %d", data.GatePopulation.AgentsUpdated)
	}

	if data.GatePopulation.Toolchain != "go" {
		t.Errorf("Expected toolchain=go, got %s", data.GatePopulation.Toolchain)
	}

	if !data.GatePopulation.H2DataAvailable {
		t.Errorf("Expected H2DataAvailable=true")
	}

	// Verify final validation passed
	if !data.FinalValidation.Passed {
		t.Errorf("Final validation should pass")
	}

	// Verify file was updated
	updatedManifest, err := Load(context.Background(), implPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	if !strings.Contains(updatedManifest.Waves[0].Agents[0].Task, "## Verification Gate") {
		t.Errorf("Agent A task should have verification gate after finalization")
	}
}

func TestFinalizeIMPL_ValidationFailure(t *testing.T) {
	// Setup temp directory
	tmpDir := t.TempDir()

	// Create IMPL file with I1 violation (duplicate file ownership)
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	implContent := `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
test_command: "go test ./..."
lint_command: "go vet ./..."
file_ownership:
  - file: "pkg/auth/handler.go"
    agent: "A"
    wave: 1
  - file: "pkg/auth/handler.go"
    agent: "B"
    wave: 1
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: "A"
        task: "Implement auth handler"
        files: ["pkg/auth/handler.go"]
      - id: "B"
        task: "Also implement auth handler"
        files: ["pkg/auth/handler.go"]
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to write test IMPL file: %v", err)
	}

	repoRoot := tmpDir

	// Execute
	res := FinalizeIMPL(implPath, repoRoot)

	// Verify failure
	if res.IsSuccess() {
		t.Errorf("Expected failure for validation failure, got success")
	}

	if len(res.Errors) == 0 {
		t.Errorf("Expected structured errors for validation failure")
	}

	// Verify file unchanged (rollback)
	reloadedManifest, err := Load(context.Background(), implPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	// Should still have violation (unchanged)
	validationErrors := Validate(reloadedManifest)
	if len(validationErrors) == 0 {
		t.Errorf("File should be unchanged after validation failure")
	}
}

func TestFinalizeIMPL_PopulationFailure(t *testing.T) {
	// Setup temp directory with no go.mod (H2 extraction will fail)
	tmpDir := t.TempDir()

	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	implContent := `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
test_command: "go test ./..."
lint_command: "go vet ./..."
file_ownership:
  - file: "pkg/auth/handler.go"
    agent: "A"
    wave: 1
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: "A"
        task: "Implement auth handler"
        files: ["pkg/auth/handler.go"]
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to write test IMPL file: %v", err)
	}

	repoRoot := tmpDir // No go.mod, H2 extraction should fail

	// Execute
	res := FinalizeIMPL(implPath, repoRoot)

	// Verify failure (H2 data unavailable)
	if res.IsSuccess() {
		t.Errorf("Expected failure when H2 data unavailable, got success")
	}

	if len(res.Errors) == 0 {
		t.Errorf("Expected structured errors for H2 failure")
	}

	// Verify file unchanged
	reloadedManifest, err := Load(context.Background(), implPath)
	if err != nil {
		t.Fatalf("Failed to reload manifest: %v", err)
	}

	if strings.Contains(reloadedManifest.Waves[0].Agents[0].Task, "## Verification Gate") {
		t.Errorf("File should be unchanged after population failure")
	}
}
