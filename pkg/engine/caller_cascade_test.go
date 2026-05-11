package engine

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// buildCascadeTestManifest builds a minimal IMPLManifest with the given file ownership entries.
func buildCascadeTestManifest(ownerships []protocol.FileOwnership) *protocol.IMPLManifest {
	return &protocol.IMPLManifest{
		FileOwnership: ownerships,
	}
}

// TestClassifyCallerCascadeErrors_AllCascades verifies that errors only in files
// not owned by wave 1 are classified as all-cascade.
func TestClassifyCallerCascadeErrors_AllCascades(t *testing.T) {
	verifyData := &protocol.VerifyBuildData{
		TestPassed: false,
		LintPassed: false,
		TestOutput: "cmd/sawtools/some_cmd.go:36:12: undefined: SomeFunc",
		LintOutput: "cmd/sawtools/other_cmd.go:14:8: assignment mismatch: 2 variables but NewThing returns 1",
	}

	manifest := buildCascadeTestManifest([]protocol.FileOwnership{
		{File: "pkg/engine/foo.go", Wave: 1, Agent: "B"},
	})

	result := ClassifyCallerCascadeErrors(verifyData, manifest, 1)

	if !result.AllAreCascades {
		t.Errorf("AllAreCascades: got false, want true")
	}
	if result.MixedErrors {
		t.Errorf("MixedErrors: got true, want false")
	}
	if len(result.Errors) == 0 {
		t.Errorf("Errors: got empty, want non-empty")
	}
}

// TestClassifyCallerCascadeErrors_MixedErrors verifies that when errors appear
// in both owned and unowned files, MixedErrors is true and AllAreCascades is false.
func TestClassifyCallerCascadeErrors_MixedErrors(t *testing.T) {
	verifyData := &protocol.VerifyBuildData{
		TestPassed: false,
		LintPassed: false,
		// Error in a wave-1-owned file (genuine failure)
		TestOutput: "pkg/engine/foo.go:10:5: undefined: MissingFunc",
		// Error in an unowned file (cascade)
		LintOutput: "cmd/sawtools/some_cmd.go:36:12: undefined: SomeFunc",
	}

	manifest := buildCascadeTestManifest([]protocol.FileOwnership{
		{File: "pkg/engine/foo.go", Wave: 1, Agent: "B"},
	})

	result := ClassifyCallerCascadeErrors(verifyData, manifest, 1)

	if result.AllAreCascades {
		t.Errorf("AllAreCascades: got true, want false")
	}
	if !result.MixedErrors {
		t.Errorf("MixedErrors: got false, want true")
	}
	// There should still be cascade errors (from unowned file)
	if len(result.Errors) == 0 {
		t.Errorf("Errors: got empty, want at least one cascade error from unowned file")
	}
}

// TestClassifyCallerCascadeErrors_NoErrors verifies that when build passes,
// classification returns an empty result.
func TestClassifyCallerCascadeErrors_NoErrors(t *testing.T) {
	verifyData := &protocol.VerifyBuildData{
		TestPassed: true,
		LintPassed: true,
		TestOutput: "",
		LintOutput: "",
	}

	manifest := buildCascadeTestManifest([]protocol.FileOwnership{
		{File: "pkg/engine/foo.go", Wave: 1, Agent: "B"},
	})

	result := ClassifyCallerCascadeErrors(verifyData, manifest, 1)

	if result.AllAreCascades {
		t.Errorf("AllAreCascades: got true, want false")
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors: got %d, want 0", len(result.Errors))
	}
}

// TestClassifyCallerCascadeErrors_PatternMatching tests each of the three
// cascade patterns: "undefined:", "assignment mismatch:", "not enough arguments".
func TestClassifyCallerCascadeErrors_PatternMatching(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		wantMessage string
	}{
		{
			name:        "undefined",
			output:      "cmd/app/main.go:42:15: undefined: SomeType",
			wantMessage: "undefined: SomeType",
		},
		{
			name:        "assignment mismatch",
			output:      "cmd/app/main.go:17:8: assignment mismatch: 2 variables but NewFoo returns 1",
			wantMessage: "assignment mismatch: 2 variables but NewFoo returns 1",
		},
		{
			name:        "not enough arguments",
			output:      "cmd/app/main.go:99:3: not enough arguments in call to bar.Baz",
			wantMessage: "not enough arguments in call to bar.Baz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			verifyData := &protocol.VerifyBuildData{
				TestPassed: false,
				LintPassed: false,
				TestOutput: tc.output,
			}

			// Wave 1 owns a different file — so cmd/app/main.go is a cascade.
			manifest := buildCascadeTestManifest([]protocol.FileOwnership{
				{File: "pkg/engine/something.go", Wave: 1, Agent: "B"},
			})

			result := ClassifyCallerCascadeErrors(verifyData, manifest, 1)

			if len(result.Errors) == 0 {
				t.Fatalf("Errors: got empty, want at least 1 matched error")
			}

			got := result.Errors[0].Message
			if got != tc.wantMessage {
				t.Errorf("Errors[0].Message: got %q, want %q", got, tc.wantMessage)
			}

			if !result.AllAreCascades {
				t.Errorf("AllAreCascades: got false, want true")
			}
		})
	}
}

// TestClassifyCallerCascadeErrors_NilVerifyData verifies that nil verifyData
// returns an empty classification without panic.
func TestClassifyCallerCascadeErrors_NilVerifyData(t *testing.T) {
	manifest := buildCascadeTestManifest([]protocol.FileOwnership{
		{File: "pkg/engine/foo.go", Wave: 1, Agent: "B"},
	})

	result := ClassifyCallerCascadeErrors(nil, manifest, 1)

	if result.AllAreCascades {
		t.Errorf("AllAreCascades: got true, want false (nil input)")
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors: got %d, want 0 (nil input)", len(result.Errors))
	}
}

// TestClassifyCallerCascadeErrors_FileLineExtraction verifies that file and line
// are correctly extracted from matched errors.
func TestClassifyCallerCascadeErrors_FileLineExtraction(t *testing.T) {
	verifyData := &protocol.VerifyBuildData{
		TestPassed: false,
		LintOutput: "cmd/sawtools/main.go:123:7: undefined: MyFunc",
	}

	manifest := buildCascadeTestManifest([]protocol.FileOwnership{
		{File: "pkg/engine/foo.go", Wave: 1, Agent: "B"},
	})

	result := ClassifyCallerCascadeErrors(verifyData, manifest, 1)

	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}

	e := result.Errors[0]
	if e.File != "cmd/sawtools/main.go" {
		t.Errorf("File: got %q, want %q", e.File, "cmd/sawtools/main.go")
	}
	if e.Line != 123 {
		t.Errorf("Line: got %d, want 123", e.Line)
	}
	if e.Message != "undefined: MyFunc" {
		t.Errorf("Message: got %q, want %q", e.Message, "undefined: MyFunc")
	}
}
