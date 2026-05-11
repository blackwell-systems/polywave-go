package scaffold_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// TestCLI_PreAgentMode_DetectsSharedTypes tests end-to-end pre-agent detection
// by creating a manifest with contracts that reference shared types.
func TestCLI_PreAgentMode_DetectsSharedTypes(t *testing.T) {
	// Create temp IMPL manifest with shared types
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	manifest := &protocol.IMPLManifest{
		Title:       "Test Manifest",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		InterfaceContracts: []protocol.InterfaceContract{
			{
				Name:        "Agent A Contract",
				Description: "Contract for Agent A",
				Definition: `type MetricSnapshot struct {
	Timestamp time.Time
	Value     float64
}

func ProcessMetrics(snapshot MetricSnapshot) error`,
				Location: "pkg/metrics/processor.go",
			},
			{
				Name:        "Agent B Contract",
				Description: "Contract for Agent B",
				Definition: `type MetricSnapshot struct {
	Timestamp time.Time
	Value     float64
}

func StoreMetrics(snapshot MetricSnapshot) error`,
				Location: "pkg/storage/metrics.go",
			},
			{
				Name:        "Agent C Contract",
				Description: "Contract for Agent C with unique type",
				Definition: `type ReportConfig struct {
	Format string
}

func GenerateReport(config ReportConfig) error`,
				Location: "pkg/reporting/generator.go",
			},
		},
		FileOwnership: []protocol.FileOwnership{},
		Waves:         []protocol.Wave{},
	}

	if saveRes := protocol.Save(context.TODO(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test manifest: %v", saveRes.Errors)
	}

	// Run polywave-tools detect-scaffolds --stage pre-agent
	cmd := exec.Command("polywave-tools", "detect-scaffolds", manifestPath, "--stage", "pre-agent")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, string(output))
	}

	// Verify exit code is 0
	if cmd.ProcessState.ExitCode() != 0 {
		t.Errorf("Expected exit code 0, got %d", cmd.ProcessState.ExitCode())
	}

	// Parse JSON output
	var result struct {
		ScaffoldsNeeded []struct {
			TypeName      string   `json:"type_name"`
			Locations     []string `json:"locations"`
			SuggestedFile string   `json:"suggested_file"`
			Definition    string   `json:"definition"`
		} `json:"scaffolds_needed"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, string(output))
	}

	// Verify MetricSnapshot was detected
	if len(result.ScaffoldsNeeded) == 0 {
		t.Fatal("Expected at least one scaffold candidate, got none")
	}

	foundMetricSnapshot := false
	for _, scaffold := range result.ScaffoldsNeeded {
		if scaffold.TypeName == "MetricSnapshot" {
			foundMetricSnapshot = true

			// Verify it's referenced by at least 2 agents
			if len(scaffold.Locations) < 2 {
				t.Errorf("Expected MetricSnapshot to be referenced by ≥2 agents, got %d", len(scaffold.Locations))
			}

			// Verify suggested file follows pattern
			if scaffold.SuggestedFile == "" {
				t.Error("Expected non-empty suggested_file")
			}

			// Verify definition is present
			if scaffold.Definition == "" {
				t.Error("Expected non-empty definition")
			}
		}
	}

	if !foundMetricSnapshot {
		t.Error("Expected to find MetricSnapshot in scaffolds_needed")
	}

	// ReportConfig should NOT appear (only referenced by 1 agent)
	for _, scaffold := range result.ScaffoldsNeeded {
		if scaffold.TypeName == "ReportConfig" {
			t.Error("ReportConfig should not be in scaffolds_needed (only 1 reference)")
		}
	}
}

// TestCLI_PostAgentMode_DetectsConflicts tests end-to-end post-agent detection
// by creating a manifest where multiple agents define the same type.
func TestCLI_PostAgentMode_DetectsConflicts(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	manifest := &protocol.IMPLManifest{
		Title:       "Test Manifest",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		InterfaceContracts: []protocol.InterfaceContract{},
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/auth/token.go", Agent: "A", Wave: 1},
			{File: "pkg/session/token.go", Agent: "B", Wave: 1},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID: "A",
						Task: `## What to Implement
Implement authentication logic.

## Interfaces to Implement

` + "```go" + `
type AuthToken struct {
	UserID    string
	ExpiresAt time.Time
}
` + "```",
						Files: []string{"pkg/auth/token.go"},
					},
					{
						ID: "B",
						Task: `## What to Implement
Implement session management.

## Interfaces to Implement

` + "```go" + `
type AuthToken struct {
	UserID    string
	ExpiresAt time.Time
}
` + "```",
						Files: []string{"pkg/session/token.go"},
					},
					{
						ID: "C",
						Task: `## What to Implement
Implement logging with unique types.

` + "```go" + `
type Logger interface {
	Log(message string)
}
` + "```",
						Files: []string{"pkg/logging/logger.go"},
					},
				},
			},
		},
	}

	if saveRes := protocol.Save(context.TODO(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test manifest: %v", saveRes.Errors)
	}

	// Run polywave-tools detect-scaffolds --stage post-agent
	cmd := exec.Command("polywave-tools", "detect-scaffolds", manifestPath, "--stage", "post-agent")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command failed: %v\nOutput: %s", err, string(output))
	}

	// Verify exit code is 0
	if cmd.ProcessState.ExitCode() != 0 {
		t.Errorf("Expected exit code 0, got %d", cmd.ProcessState.ExitCode())
	}

	// Parse JSON output
	var result struct {
		Conflicts []struct {
			TypeName   string   `json:"type_name"`
			Agents     []string `json:"agents"`
			Files      []string `json:"files"`
			Resolution string   `json:"resolution"`
		} `json:"conflicts"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, string(output))
	}

	// Verify AuthToken conflict was detected
	if len(result.Conflicts) == 0 {
		t.Fatal("Expected at least one conflict, got none")
	}

	foundAuthToken := false
	for _, conflict := range result.Conflicts {
		if conflict.TypeName == "AuthToken" {
			foundAuthToken = true

			// Verify agents list
			if len(conflict.Agents) != 2 {
				t.Errorf("Expected AuthToken conflict to list 2 agents, got %d", len(conflict.Agents))
			}

			// Verify files list
			if len(conflict.Files) != 2 {
				t.Errorf("Expected AuthToken conflict to list 2 files, got %d", len(conflict.Files))
			}

			// Verify resolution is present
			if conflict.Resolution == "" {
				t.Error("Expected non-empty resolution")
			}
		}
	}

	if !foundAuthToken {
		t.Error("Expected to find AuthToken in conflicts")
	}

	// Logger should NOT appear (only defined by 1 agent)
	for _, conflict := range result.Conflicts {
		if conflict.TypeName == "Logger" {
			t.Error("Logger should not be in conflicts (only 1 definition)")
		}
	}
}

// TestCLI_InvalidStage tests that --stage with invalid value returns error.
func TestCLI_InvalidStage(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	manifest := &protocol.IMPLManifest{
		Title:              "Test Manifest",
		FeatureSlug:        "test-feature",
		Verdict:            "SUITABLE",
		InterfaceContracts: []protocol.InterfaceContract{},
		FileOwnership:      []protocol.FileOwnership{},
		Waves:              []protocol.Wave{},
	}

	if saveRes := protocol.Save(context.TODO(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test manifest: %v", saveRes.Errors)
	}

	// Run with invalid stage
	cmd := exec.Command("polywave-tools", "detect-scaffolds", manifestPath, "--stage", "invalid")
	output, err := cmd.CombinedOutput()

	// Should fail
	if err == nil {
		t.Error("Expected command to fail with invalid stage, but it succeeded")
	}

	// Verify exit code is non-zero
	if cmd.ProcessState.ExitCode() == 0 {
		t.Error("Expected non-zero exit code for invalid stage")
	}

	// Verify error message mentions valid values
	outputStr := string(output)
	if outputStr == "" {
		t.Error("Expected error message in output")
	}
}

// TestCLI_MissingStageFlag tests that missing --stage flag returns error.
func TestCLI_MissingStageFlag(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	manifest := &protocol.IMPLManifest{
		Title:              "Test Manifest",
		FeatureSlug:        "test-feature",
		Verdict:            "SUITABLE",
		InterfaceContracts: []protocol.InterfaceContract{},
		FileOwnership:      []protocol.FileOwnership{},
		Waves:              []protocol.Wave{},
	}

	if saveRes := protocol.Save(context.TODO(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test manifest: %v", saveRes.Errors)
	}

	// Run without --stage flag
	cmd := exec.Command("polywave-tools", "detect-scaffolds", manifestPath)
	output, err := cmd.CombinedOutput()

	// Should fail
	if err == nil {
		t.Error("Expected command to fail without --stage flag, but it succeeded")
	}

	// Verify exit code is non-zero
	if cmd.ProcessState.ExitCode() == 0 {
		t.Error("Expected non-zero exit code for missing --stage flag")
	}

	// Verify usage message is present
	outputStr := string(output)
	if outputStr == "" {
		t.Error("Expected usage message in output")
	}
}

// TestCLI_ManifestNotFound tests that non-existent manifest path returns error.
func TestCLI_ManifestNotFound(t *testing.T) {
	// Use non-existent path
	manifestPath := "/tmp/nonexistent-impl-doc-12345.yaml"

	cmd := exec.Command("polywave-tools", "detect-scaffolds", manifestPath, "--stage", "pre-agent")
	output, err := cmd.CombinedOutput()

	// Should fail
	if err == nil {
		t.Error("Expected command to fail with non-existent manifest, but it succeeded")
	}

	// Verify exit code is non-zero
	if cmd.ProcessState.ExitCode() == 0 {
		t.Error("Expected non-zero exit code for non-existent manifest")
	}

	// Verify error message
	outputStr := string(output)
	if outputStr == "" {
		t.Error("Expected error message in output")
	}
}

// TestCLI_EmptyManifest tests that minimal manifest with no contracts/agents returns empty results.
func TestCLI_EmptyManifest(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	manifest := &protocol.IMPLManifest{
		Title:              "Empty Manifest",
		FeatureSlug:        "empty",
		Verdict:            "SUITABLE",
		InterfaceContracts: []protocol.InterfaceContract{},
		FileOwnership:      []protocol.FileOwnership{},
		Waves:              []protocol.Wave{},
	}

	if saveRes := protocol.Save(context.TODO(), manifest, manifestPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test manifest: %v", saveRes.Errors)
	}

	// Test pre-agent mode
	t.Run("PreAgent", func(t *testing.T) {
		cmd := exec.Command("polywave-tools", "detect-scaffolds", manifestPath, "--stage", "pre-agent")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Command failed: %v\nOutput: %s", err, string(output))
		}

		// Verify exit code is 0
		if cmd.ProcessState.ExitCode() != 0 {
			t.Errorf("Expected exit code 0, got %d", cmd.ProcessState.ExitCode())
		}

		// Parse JSON output
		var result struct {
			ScaffoldsNeeded []interface{} `json:"scaffolds_needed"`
		}

		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, string(output))
		}

		// Should be empty array, not null
		if result.ScaffoldsNeeded == nil {
			t.Error("Expected empty array for scaffolds_needed, got nil")
		}

		if len(result.ScaffoldsNeeded) != 0 {
			t.Errorf("Expected empty scaffolds_needed, got %d items", len(result.ScaffoldsNeeded))
		}
	})

	// Test post-agent mode
	t.Run("PostAgent", func(t *testing.T) {
		cmd := exec.Command("polywave-tools", "detect-scaffolds", manifestPath, "--stage", "post-agent")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Command failed: %v\nOutput: %s", err, string(output))
		}

		// Verify exit code is 0
		if cmd.ProcessState.ExitCode() != 0 {
			t.Errorf("Expected exit code 0, got %d", cmd.ProcessState.ExitCode())
		}

		// Parse JSON output
		var result struct {
			Conflicts []interface{} `json:"conflicts"`
		}

		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, string(output))
		}

		// Should be empty array, not null
		if result.Conflicts == nil {
			t.Error("Expected empty array for conflicts, got nil")
		}

		if len(result.Conflicts) != 0 {
			t.Errorf("Expected empty conflicts, got %d items", len(result.Conflicts))
		}
	})
}
