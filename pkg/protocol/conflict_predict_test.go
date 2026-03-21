package protocol

import (
	"strings"
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

	if err := PredictConflictsFromReports(manifest, 1); err != nil {
		t.Errorf("expected nil error for non-overlapping files, got: %v", err)
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

	err := PredictConflictsFromReports(manifest, 1)
	if err == nil {
		t.Fatal("expected error for file modified by 2 agents, got nil")
	}
	if !strings.Contains(err.Error(), "E11") {
		t.Errorf("error should mention E11, got: %v", err)
	}
	if !strings.Contains(err.Error(), "pkg/shared/shared.go") {
		t.Errorf("error should mention the conflicting file, got: %v", err)
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

	err := PredictConflictsFromReports(manifest, 1)
	if err == nil {
		t.Fatal("expected error for file created by 2 agents, got nil")
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

	if err := PredictConflictsFromReports(manifest, 1); err != nil {
		t.Errorf("expected nil error for IMPL file overlap, got: %v", err)
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

	if err := PredictConflictsFromReports(manifest, 1); err != nil {
		t.Errorf("expected nil error for .saw-state file overlap, got: %v", err)
	}
}

func TestPredictConflictsFromReports_NilManifest(t *testing.T) {
	if err := PredictConflictsFromReports(nil, 1); err != nil {
		t.Errorf("expected nil error for nil manifest, got: %v", err)
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

	// Wave 2 doesn't exist — should return nil.
	if err := PredictConflictsFromReports(manifest, 2); err != nil {
		t.Errorf("expected nil for missing wave, got: %v", err)
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

	if err := PredictConflictsFromReports(manifest, 1); err != nil {
		t.Errorf("expected nil when B has no report, got: %v", err)
	}
}
