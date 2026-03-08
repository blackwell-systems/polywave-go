package orchestrator

import (
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

// TestRunQualityGatesNil verifies that nil gates returns nil results and nil error.
func TestRunQualityGatesNil(t *testing.T) {
	results, err := RunQualityGates("/tmp", nil)
	if err != nil {
		t.Fatalf("expected nil error for nil gates, got: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results for nil gates, got: %v", results)
	}
}

// TestRunQualityGatesQuick verifies that level="quick" skips all gates.
func TestRunQualityGatesQuick(t *testing.T) {
	gates := &types.QualityGates{
		Level: "quick",
		Gates: []types.QualityGate{
			{Type: "build", Command: "go build ./...", Required: true},
		},
	}
	results, err := RunQualityGates("/tmp", gates)
	if err != nil {
		t.Fatalf("expected nil error for quick level, got: %v", err)
	}
	if results != nil {
		t.Fatalf("expected nil results for quick level, got: %v", results)
	}
}

// TestRunQualityGatesRequiredFail verifies that a required gate with a failing
// command produces a non-nil error.
func TestRunQualityGatesRequiredFail(t *testing.T) {
	gates := &types.QualityGates{
		Level: "standard",
		Gates: []types.QualityGate{
			{
				Type:     "build",
				Command:  "false", // always exits non-zero
				Required: true,
				Description: "always fails",
			},
		},
	}
	results, err := RunQualityGates(t.TempDir(), gates)
	if err == nil {
		t.Fatal("expected blocking error for required failing gate, got nil")
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Error("expected Passed=false for failing gate")
	}
	if !strings.Contains(err.Error(), "build") {
		t.Errorf("error should mention gate type 'build'; got: %v", err)
	}
}

// TestRunQualityGatesOptionalFail verifies that an optional gate failing does
// not produce a blocking error, but result shows Passed=false.
func TestRunQualityGatesOptionalFail(t *testing.T) {
	gates := &types.QualityGates{
		Level: "standard",
		Gates: []types.QualityGate{
			{
				Type:     "lint",
				Command:  "false", // always exits non-zero
				Required: false,
				Description: "optional check",
			},
		},
	}
	results, err := RunQualityGates(t.TempDir(), gates)
	if err != nil {
		t.Fatalf("expected nil error for optional failing gate, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Error("expected Passed=false for failing optional gate")
	}
}
