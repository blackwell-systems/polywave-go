package protocol

import "testing"

func TestIMPLStateToStatus_Complete(t *testing.T) {
	if got := IMPLStateToStatus(StateComplete); got != "complete" {
		t.Errorf("StateComplete: expected %q, got %q", "complete", got)
	}
}

func TestIMPLStateToStatus_InProgress(t *testing.T) {
	cases := []ProtocolState{
		StateWaveExecuting,
		StateWaveMerging,
		StateWaveVerified,
		StateWavePending,
	}
	for _, state := range cases {
		if got := IMPLStateToStatus(state); got != "in-progress" {
			t.Errorf("IMPLStateToStatus(%q): expected %q, got %q", state, "in-progress", got)
		}
	}
}

func TestIMPLStateToStatus_Pending(t *testing.T) {
	if got := IMPLStateToStatus(StateReviewed); got != "pending" {
		t.Errorf("StateReviewed: expected %q, got %q", "pending", got)
	}
}

func TestIMPLStateToStatus_Unknown(t *testing.T) {
	unknown := ProtocolState("SOME_UNKNOWN_STATE")
	if got := IMPLStateToStatus(unknown); got != string(unknown) {
		t.Errorf("unknown state: expected %q, got %q", string(unknown), got)
	}
}
