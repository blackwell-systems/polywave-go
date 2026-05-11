package resume

import (
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

func TestBuildRecoverySteps_NoFailures(t *testing.T) {
	state := SessionState{
		ResumeCommand:   "polywave-tools run-wave docs/IMPL/IMPL-test.yaml --wave 1",
		SuggestedAction: "Resume wave 1",
	}
	steps := BuildRecoverySteps(state, nil)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Priority != 3 {
		t.Errorf("expected priority 3, got %d", steps[0].Priority)
	}
}

func TestBuildRecoverySteps_ResumeCommandEmpty(t *testing.T) {
	state := SessionState{}
	steps := BuildRecoverySteps(state, nil)
	if len(steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(steps))
	}
}

func TestBuildRecoverySteps_FailedAgent(t *testing.T) {
	state := SessionState{
		FailedAgents:  []string{"X"},
		ResumeCommand: "polywave-tools run-wave x",
	}
	manifest := &protocol.IMPLManifest{
		CompletionReports: map[string]protocol.CompletionReport{
			"X": {FailureType: "fixable"},
		},
	}
	steps := BuildRecoverySteps(state, manifest)
	found := false
	for _, s := range steps {
		if s.Priority == 1 && s.Category == protocol.FailureCategoryFixable {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one step with priority 1 and category fixable")
	}
}

func TestBuildRecoverySteps_DirtyWorktree(t *testing.T) {
	state := SessionState{
		DirtyWorktrees: []DirtyWorktree{
			{Path: "/tmp/wt", AgentID: "Y", HasChanges: true},
		},
	}
	steps := BuildRecoverySteps(state, nil)
	found := false
	for _, s := range steps {
		if s.Priority == 1 && strings.Contains(s.Command, "git -C") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one step with priority 1 and command containing 'git -C'")
	}
}

func TestBuildRecoverySteps_OrphanedClean(t *testing.T) {
	state := SessionState{
		OrphanedWorktrees: []string{"/tmp/wt"},
		DirtyWorktrees:    []DirtyWorktree{{HasChanges: false}},
		CurrentWave:       1,
		IMPLPath:          "docs/IMPL/IMPL-test.yaml",
	}
	steps := BuildRecoverySteps(state, nil)
	found := false
	for _, s := range steps {
		if s.Priority == 2 && strings.Contains(s.Command, "polywave-tools cleanup") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected at least one step with priority 2 and command containing 'polywave-tools cleanup'")
	}
}
