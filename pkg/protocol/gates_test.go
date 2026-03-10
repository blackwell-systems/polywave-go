package protocol

import (
	"testing"
)

func TestRunGates_NoGates(t *testing.T) {
	// Test with nil QualityGates
	manifest := &IMPLManifest{}
	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d results", len(results))
	}

	// Test with empty Gates slice
	manifest.QualityGates = &QualityGates{
		Level: "quick",
		Gates: []QualityGate{},
	}
	results, err = RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d results", len(results))
	}
}

func TestRunGates_PassingGate(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "test",
					Command:  "echo ok",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Type != "test" {
		t.Errorf("expected Type 'test', got '%s'", result.Type)
	}
	if result.Command != "echo ok" {
		t.Errorf("expected Command 'echo ok', got '%s'", result.Command)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", result.ExitCode)
	}
	if !result.Passed {
		t.Errorf("expected Passed=true, got false")
	}
	if result.Stdout != "ok\n" {
		t.Errorf("expected Stdout 'ok\\n', got '%s'", result.Stdout)
	}
	if !result.Required {
		t.Errorf("expected Required=true, got false")
	}
}

func TestRunGates_FailingGate(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  "exit 1",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.ExitCode != 1 {
		t.Errorf("expected ExitCode 1, got %d", result.ExitCode)
	}
	if result.Passed {
		t.Errorf("expected Passed=false, got true")
	}
}

func TestRunGates_MixedGates(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  "echo build success",
					Required: true,
				},
				{
					Type:     "test",
					Command:  "false",
					Required: false,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First gate should pass
	if !results[0].Passed {
		t.Errorf("expected first gate to pass")
	}
	if results[0].ExitCode != 0 {
		t.Errorf("expected first gate ExitCode 0, got %d", results[0].ExitCode)
	}

	// Second gate should fail
	if results[1].Passed {
		t.Errorf("expected second gate to fail")
	}
	if results[1].ExitCode == 0 {
		t.Errorf("expected second gate ExitCode != 0, got 0")
	}
}

func TestRunGates_CapturesOutput(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "test",
					Command:  "echo stdout message && echo stderr message >&2",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Stdout != "stdout message\n" {
		t.Errorf("expected Stdout 'stdout message\\n', got '%s'", result.Stdout)
	}
	if result.Stderr != "stderr message\n" {
		t.Errorf("expected Stderr 'stderr message\\n', got '%s'", result.Stderr)
	}
}

func TestRunGates_NonExistentCommand(t *testing.T) {
	manifest := &IMPLManifest{
		QualityGates: &QualityGates{
			Level: "quick",
			Gates: []QualityGate{
				{
					Type:     "build",
					Command:  "nonexistent_command_xyz",
					Required: true,
				},
			},
		},
	}

	results, err := RunGates(manifest, 1, "/tmp")
	if err != nil {
		t.Fatalf("expected no error (gate failures should not return errors), got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Passed {
		t.Errorf("expected non-existent command to fail")
	}
	if result.ExitCode == 0 {
		t.Errorf("expected non-zero exit code for non-existent command")
	}
	// Stderr should contain an error message
	if result.Stderr == "" {
		t.Errorf("expected Stderr to contain error message for non-existent command")
	}
}
