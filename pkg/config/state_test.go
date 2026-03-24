package config

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func makeManifest(waves []protocol.Wave, reports map[string]protocol.CompletionReport) *protocol.IMPLManifest {
	m := &protocol.IMPLManifest{
		Waves:             waves,
		CompletionReports: reports,
	}
	return m
}

func twoAgentWave() []protocol.Wave {
	return []protocol.Wave{
		{
			Number: 1,
			Agents: []protocol.Agent{
				{ID: "A", Task: "task A"},
				{ID: "B", Task: "task B"},
			},
		},
	}
}

func TestGetWaveState_ValidWave(t *testing.T) {
	manifest := makeManifest(twoAgentWave(), nil)
	r := GetWaveState(manifest, 1)
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	ws := r.GetData()
	if ws.WaveNum != 1 {
		t.Errorf("WaveNum = %d, want 1", ws.WaveNum)
	}
	if ws.TotalAgents != 2 {
		t.Errorf("TotalAgents = %d, want 2", ws.TotalAgents)
	}
	if len(ws.PendingAgents) != 2 {
		t.Errorf("PendingAgents = %v, want [A B]", ws.PendingAgents)
	}
	if ws.IsComplete {
		t.Error("IsComplete should be false when no reports exist")
	}
}

func TestGetWaveState_WaveNotFound(t *testing.T) {
	manifest := makeManifest(twoAgentWave(), nil)
	r := GetWaveState(manifest, 99)
	if !r.IsFatal() {
		t.Fatal("expected failure for missing wave")
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
}

func TestGetWaveState_AllComplete(t *testing.T) {
	reports := map[string]protocol.CompletionReport{
		"wave1-A": {Status: "complete"},
		"wave1-B": {Status: "complete"},
	}
	manifest := makeManifest(twoAgentWave(), reports)
	r := GetWaveState(manifest, 1)
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	ws := r.GetData()
	if !ws.IsComplete {
		t.Error("IsComplete should be true when all agents completed")
	}
	if len(ws.CompletedAgents) != 2 {
		t.Errorf("CompletedAgents = %v, want [A B]", ws.CompletedAgents)
	}
	if len(ws.PendingAgents) != 0 {
		t.Errorf("PendingAgents = %v, want []", ws.PendingAgents)
	}
	if len(ws.FailedAgents) != 0 {
		t.Errorf("FailedAgents = %v, want []", ws.FailedAgents)
	}
}

func TestGetWaveState_PartialComplete(t *testing.T) {
	reports := map[string]protocol.CompletionReport{
		"wave1-A": {Status: "complete"},
	}
	manifest := makeManifest(twoAgentWave(), reports)
	r := GetWaveState(manifest, 1)
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	ws := r.GetData()
	if ws.IsComplete {
		t.Error("IsComplete should be false with pending agents")
	}
	if len(ws.CompletedAgents) != 1 || ws.CompletedAgents[0] != "A" {
		t.Errorf("CompletedAgents = %v, want [A]", ws.CompletedAgents)
	}
	if len(ws.PendingAgents) != 1 || ws.PendingAgents[0] != "B" {
		t.Errorf("PendingAgents = %v, want [B]", ws.PendingAgents)
	}
}

func TestGetWaveState_WithFailures(t *testing.T) {
	reports := map[string]protocol.CompletionReport{
		"wave1-A": {Status: "complete"},
		"wave1-B": {Status: "blocked"},
	}
	manifest := makeManifest(twoAgentWave(), reports)
	r := GetWaveState(manifest, 1)
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	ws := r.GetData()
	if ws.IsComplete {
		t.Error("IsComplete should be false when failures exist")
	}
	if len(ws.FailedAgents) != 1 || ws.FailedAgents[0] != "B" {
		t.Errorf("FailedAgents = %v, want [B]", ws.FailedAgents)
	}

	// Also test partial status counts as failed.
	reports["wave1-B"] = protocol.CompletionReport{Status: "partial"}
	r2 := GetWaveState(manifest, 1)
	ws2 := r2.GetData()
	if len(ws2.FailedAgents) != 1 || ws2.FailedAgents[0] != "B" {
		t.Errorf("partial status: FailedAgents = %v, want [B]", ws2.FailedAgents)
	}
}

func TestGetAllWaveStates(t *testing.T) {
	waves := []protocol.Wave{
		{
			Number: 1,
			Agents: []protocol.Agent{{ID: "A", Task: "task A"}},
		},
		{
			Number: 2,
			Agents: []protocol.Agent{{ID: "B", Task: "task B"}, {ID: "C", Task: "task C"}},
		},
	}
	reports := map[string]protocol.CompletionReport{
		"wave1-A": {Status: "complete"},
	}
	manifest := makeManifest(waves, reports)
	r := GetAllWaveStates(manifest)
	if r.IsFatal() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	states := r.GetData()
	if len(states) != 2 {
		t.Fatalf("got %d states, want 2", len(states))
	}
	if states[0].WaveNum != 1 || !states[0].IsComplete {
		t.Errorf("wave 1: WaveNum=%d IsComplete=%v, want 1/true", states[0].WaveNum, states[0].IsComplete)
	}
	if states[1].WaveNum != 2 || states[1].IsComplete {
		t.Errorf("wave 2: WaveNum=%d IsComplete=%v, want 2/false", states[1].WaveNum, states[1].IsComplete)
	}
	if states[1].TotalAgents != 2 {
		t.Errorf("wave 2: TotalAgents=%d, want 2", states[1].TotalAgents)
	}
}
