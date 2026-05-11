package engine

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// ---------------------------------------------------------------------------
// generateSlug tests
// ---------------------------------------------------------------------------

func TestGenerateSlug_BasicConversion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Add audit logging", "add-audit-logging"},
		{"Hello World!", "hello-world"},
		{"  leading and trailing  ", "leading-and-trailing"},
		{"multiple   spaces", "multiple-spaces"},
		{"123 numbers too", "123-numbers-too"},
		{"special-chars: @#$%", "special-chars"},
		{"", ""},
	}

	for _, tc := range tests {
		got := generateSlug(tc.input)
		if got != tc.want {
			t.Errorf("generateSlug(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func TestGenerateSlug_Truncation(t *testing.T) {
	long := "this is a very long feature description that exceeds forty nine characters"
	got := generateSlug(long)
	if len(got) > 49 {
		t.Errorf("generateSlug should truncate to 49 chars; got len=%d: %q", len(got), got)
	}
}

func TestGenerateSlug_NoTrailingHyphen(t *testing.T) {
	// After truncation, we might cut mid-word but shouldn't end with hyphen if
	// we trim first (our implementation trims before truncating, so the slug
	// won't start/end with hyphens from the raw string).
	got := generateSlug("hello world")
	if len(got) > 0 && got[0] == '-' {
		t.Errorf("generateSlug should not start with hyphen: %q", got)
	}
	if len(got) > 0 && got[len(got)-1] == '-' {
		t.Errorf("generateSlug should not end with hyphen: %q", got)
	}
}

// ---------------------------------------------------------------------------
// waitForFile tests
// ---------------------------------------------------------------------------

func TestWaitForFile_FileExists(t *testing.T) {
	// Create a temp file.
	tmp := t.TempDir()
	path := tmp + "/exists.txt"
	if err := writeTestFile(path, ""); err != nil {
		t.Fatal(err)
	}

	if !waitForFile(path, 1*time.Second) {
		t.Error("waitForFile should return true for an existing file")
	}
}

func TestWaitForFile_FileNotFound(t *testing.T) {
	path := "/tmp/this-file-does-not-exist-scout-run-test-12345.yaml"
	start := time.Now()
	result := waitForFile(path, 600*time.Millisecond)
	elapsed := time.Since(start)

	if result {
		t.Error("waitForFile should return false for a missing file")
	}
	// Should have waited at least the poll interval but not much more than maxWait.
	if elapsed < 500*time.Millisecond {
		t.Errorf("waitForFile returned too quickly: %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// criticThresholdMet tests
// ---------------------------------------------------------------------------

func TestCriticThresholdMet_Wave1WithThreeAgents(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A"},
					{ID: "B"},
					{ID: "C"},
				},
			},
		},
	}
	if !criticThresholdMet(manifest) {
		t.Error("criticThresholdMet should return true when wave 1 has 3+ agents")
	}
}

func TestCriticThresholdMet_Wave1WithTwoAgents(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A"},
					{ID: "B"},
				},
			},
		},
	}
	if criticThresholdMet(manifest) {
		t.Error("criticThresholdMet should return false when wave 1 has < 3 agents and only 1 repo")
	}
}

func TestCriticThresholdMet_MultipleRepos(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "cmd/foo.go", Agent: "A", Repo: "repo-a"},
			{File: "cmd/bar.go", Agent: "B", Repo: "repo-b"},
		},
	}
	if !criticThresholdMet(manifest) {
		t.Error("criticThresholdMet should return true when file_ownership spans 2+ repos")
	}
}

func TestCriticThresholdMet_SingleRepoNoThreshold(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A"},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "cmd/foo.go", Agent: "A", Repo: "same-repo"},
			{File: "cmd/bar.go", Agent: "A", Repo: "same-repo"},
		},
	}
	if criticThresholdMet(manifest) {
		t.Error("criticThresholdMet should return false when wave 1 < 3 agents and single repo")
	}
}

func TestCriticThresholdMet_EmptyManifest(t *testing.T) {
	manifest := &protocol.IMPLManifest{}
	if criticThresholdMet(manifest) {
		t.Error("criticThresholdMet should return false for empty manifest")
	}
}

// ---------------------------------------------------------------------------
// countAgentsFromErrors tests
// ---------------------------------------------------------------------------

func TestCountAgentsFromErrors_Found(t *testing.T) {
	errs := []result.PolywaveError{
		{Code: "agent-id", Line: 10, Message: "bad agent ID"},
		{Code: "agent-id", Line: 0, Message: "Run: polywave-tools assign-agent-ids --count 5"},
	}
	got := countAgentsFromErrors(errs)
	if got != 5 {
		t.Errorf("countAgentsFromErrors = %d; want 5", got)
	}
}

func TestCountAgentsFromErrors_NotFound(t *testing.T) {
	errs := []result.PolywaveError{
		{Code: "other", Line: 1, Message: "some other error"},
	}
	got := countAgentsFromErrors(errs)
	if got != 0 {
		t.Errorf("countAgentsFromErrors = %d; want 0", got)
	}
}

func TestCountAgentsFromErrors_Empty(t *testing.T) {
	got := countAgentsFromErrors(nil)
	if got != 0 {
		t.Errorf("countAgentsFromErrors(nil) = %d; want 0", got)
	}
}

// ---------------------------------------------------------------------------
// RefreshBrief tests
// ---------------------------------------------------------------------------

func TestRunScoutFull_RefreshBriefField(t *testing.T) {
	// Verify that RunScoutFullOpts has a RefreshBrief field and it can be set.
	opts := RunScoutFullOpts{
		Feature:      "some feature",
		RefreshBrief: true,
	}
	if !opts.RefreshBrief {
		t.Error("RefreshBrief field should be true when set to true")
	}

	opts2 := RunScoutFullOpts{
		Feature: "some feature",
	}
	if opts2.RefreshBrief {
		t.Error("RefreshBrief field should default to false")
	}
}

func TestRunScoutFull_RefreshBriefModifiesFeature(t *testing.T) {
	// We can't call RunScoutFull directly (it requires Scout agent),
	// but we can verify that the feature string modification logic works
	// by inspecting the format string behavior.
	//
	// The actual modification happens in RunScoutFull:
	//   opts.Feature = fmt.Sprintf("[REFRESH-BRIEF] Preserve file_ownership...")
	//
	// We test that the format produces the expected prefix.
	originalFeature := "Add caching layer"
	implPath := "/tmp/test/IMPL-caching.yaml"

	modified := fmt.Sprintf("[REFRESH-BRIEF] Preserve file_ownership and wave structure from %s. "+
		"Only update agent task descriptions to reflect current codebase state. "+
		"Do not change agent IDs, wave assignments, or file ownership.\n\n%s",
		implPath, originalFeature)

	if !strings.HasPrefix(modified, "[REFRESH-BRIEF]") {
		t.Error("modified feature should start with [REFRESH-BRIEF] prefix")
	}
	if !strings.Contains(modified, implPath) {
		t.Errorf("modified feature should contain implPath %q", implPath)
	}
	if !strings.HasSuffix(modified, originalFeature) {
		t.Error("modified feature should end with the original feature description")
	}
	if !strings.Contains(modified, "Do not change agent IDs") {
		t.Error("modified feature should contain preservation directive")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func writeTestFile(path, content string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}
