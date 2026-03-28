package protocol

import (
	"errors"
	"testing"
)

func TestCompletionReportBuilder_ValidComplete(t *testing.T) {
	err := NewCompletionReport("A").
		WithStatus(StatusComplete).
		WithCommit("abc123").
		Validate()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestCompletionReportBuilder_BlockedWithoutFailureType(t *testing.T) {
	err := NewCompletionReport("A").
		WithStatus(StatusBlocked).
		Validate()
	if err == nil {
		t.Error("expected error for blocked without failure_type")
	}
}

func TestCompletionReportBuilder_PartialRequiresFailureType(t *testing.T) {
	err := NewCompletionReport("A").
		WithStatus(StatusPartial).
		WithCommit("abc123").
		Validate()
	if err == nil {
		t.Error("expected error for partial without failure_type")
	}
}

func TestCompletionReportBuilder_CompleteWithFailureType(t *testing.T) {
	err := NewCompletionReport("A").
		WithStatus(StatusComplete).
		WithCommit("abc123").
		WithFailureType("transient").
		Validate()
	if err == nil {
		t.Error("expected error for complete with failure_type set")
	}
}

func TestCompletionReportBuilder_EmptyCommitOnComplete(t *testing.T) {
	err := NewCompletionReport("A").
		WithStatus(StatusComplete).
		Validate()
	if err == nil {
		t.Error("expected error for complete with empty commit")
	}
}

func TestCompletionReportBuilder_InvalidStatus(t *testing.T) {
	err := NewCompletionReport("A").
		WithStatus(CompletionStatus("done")).
		Validate()
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestCompletionReportBuilder_InvalidFailureType(t *testing.T) {
	err := NewCompletionReport("A").
		WithStatus(StatusBlocked).
		WithFailureType("bad").
		Validate()
	if err == nil {
		t.Error("expected error for invalid failure_type")
	}
}

func TestCompletionReportBuilder_AppendToManifest(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "X", Task: "test"}}},
		},
		CompletionReports: make(map[string]CompletionReport),
	}

	err := NewCompletionReport("X").
		WithStatus(StatusComplete).
		WithCommit("deadbeef").
		AppendToManifest(manifest)
	if err != nil {
		t.Fatalf("AppendToManifest failed: %v", err)
	}

	report, ok := manifest.CompletionReports["X"]
	if !ok {
		t.Fatal("report not stored in manifest")
	}
	if report.Status != StatusComplete {
		t.Errorf("status = %q, want %q", report.Status, StatusComplete)
	}
	if report.Commit != "deadbeef" {
		t.Errorf("commit = %q, want %q", report.Commit, "deadbeef")
	}
}

func TestCompletionReportBuilder_AppendToManifest_UnknownAgent(t *testing.T) {
	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "X", Task: "test"}}},
		},
		CompletionReports: make(map[string]CompletionReport),
	}

	err := NewCompletionReport("Z").
		WithStatus("complete").
		WithCommit("abc123").
		AppendToManifest(manifest)
	if err == nil {
		t.Error("expected ErrAgentNotFound for unknown agent")
	}
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("expected ErrAgentNotFound, got: %v", err)
	}
}
