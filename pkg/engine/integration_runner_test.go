package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// TestRunIntegrationAgent_NoGaps verifies that RunIntegrationAgent returns
// immediately (without launching an agent) when Report.Valid is true.
func TestRunIntegrationAgent_NoGaps(t *testing.T) {
	report := &protocol.IntegrationReport{
		Wave:    1,
		Valid:   true,
		Summary: "no gaps detected",
		Gaps:    nil,
	}

	var events []string
	onEvent := func(e Event) {
		events = append(events, e.Event)
	}

	res := RunIntegrationAgent(context.Background(), RunIntegrationAgentOpts{
		IMPLPath: "/tmp/test-impl.yaml",
		RepoPath: "/tmp/test-repo",
		WaveNum:  1,
		Report:   report,
	}, onEvent)

	if res.IsFatal() {
		t.Fatalf("expected success for valid report, got fatal: %v", res.Errors)
	}

	data := res.GetData()
	if data.GapCount != 0 {
		t.Errorf("expected 0 gaps, got %d", data.GapCount)
	}

	// No events should be emitted when there are no gaps.
	if len(events) != 0 {
		t.Errorf("expected 0 events for valid report, got %d: %v", len(events), events)
	}
}

// TestRunIntegrationAgent_PublishesEvents verifies that start and failed events
// are emitted when the agent is launched (fails due to missing IMPL doc, but
// events are still published).
func TestRunIntegrationAgent_PublishesEvents(t *testing.T) {
	report := &protocol.IntegrationReport{
		Wave:  1,
		Valid: false,
		Gaps: []protocol.IntegrationGap{
			{
				ExportName: "NewWidget",
				FilePath:   "pkg/widget/widget.go",
				AgentID:    "A",
				Category:   "function_call",
				Severity:   "error",
				Reason:     "no call-site found",
			},
		},
	}

	var events []string
	onEvent := func(e Event) {
		events = append(events, e.Event)
	}

	// This will fail because the IMPL path doesn't exist, but we verify
	// that integration_agent_started and integration_agent_failed events fire.
	res := RunIntegrationAgent(context.Background(), RunIntegrationAgentOpts{
		IMPLPath: "/nonexistent/IMPL.yaml",
		RepoPath: "/tmp/test-repo",
		WaveNum:  1,
		Report:   report,
	}, onEvent)

	if !res.IsFatal() {
		t.Fatal("expected fatal result for nonexistent IMPL path, got success")
	}

	// Should have started event then failed event.
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d: %v", len(events), events)
	}
	if events[0] != "integration_agent_started" {
		t.Errorf("expected first event 'integration_agent_started', got %q", events[0])
	}
	if events[1] != "integration_agent_failed" {
		t.Errorf("expected second event 'integration_agent_failed', got %q", events[1])
	}
}

// TestRunIntegrationAgent_OptsValidation verifies that missing required fields
// produce clear errors.
func TestRunIntegrationAgent_OptsValidation(t *testing.T) {
	tests := []struct {
		name string
		opts RunIntegrationAgentOpts
		want string
	}{
		{
			name: "missing IMPLPath",
			opts: RunIntegrationAgentOpts{
				RepoPath: "/tmp/repo",
				WaveNum:  1,
				Report:   &protocol.IntegrationReport{},
			},
			want: "IMPLPath is required",
		},
		{
			name: "missing RepoPath",
			opts: RunIntegrationAgentOpts{
				IMPLPath: "/tmp/impl.yaml",
				WaveNum:  1,
				Report:   &protocol.IntegrationReport{},
			},
			want: "RepoPath is required",
		},
		{
			name: "zero WaveNum",
			opts: RunIntegrationAgentOpts{
				IMPLPath: "/tmp/impl.yaml",
				RepoPath: "/tmp/repo",
				WaveNum:  0,
				Report:   &protocol.IntegrationReport{},
			},
			want: "WaveNum must be positive",
		},
		{
			name: "nil Report",
			opts: RunIntegrationAgentOpts{
				IMPLPath: "/tmp/impl.yaml",
				RepoPath: "/tmp/repo",
				WaveNum:  1,
				Report:   nil,
			},
			want: "Report is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := RunIntegrationAgent(context.Background(), tc.opts, func(Event) {})
			if !res.IsFatal() {
				t.Fatalf("expected fatal result containing %q, got success", tc.want)
			}
			if len(res.Errors) == 0 {
				t.Fatalf("expected errors containing %q, got empty errors", tc.want)
			}
			if got := res.Errors[0].Message; !strings.Contains(got, tc.want) {
				t.Errorf("error %q does not contain %q", got, tc.want)
			}
		})
	}
}
