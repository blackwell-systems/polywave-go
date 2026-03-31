package engine

import (
	"encoding/json"
	"testing"
)

func TestRunStartedPayload_JSON(t *testing.T) {
	original := RunStartedPayload{Slug: "foo", IMPLPath: "/bar"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var got RunStartedPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.Slug != original.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, original.Slug)
	}
	if got.IMPLPath != original.IMPLPath {
		t.Errorf("IMPLPath: got %q, want %q", got.IMPLPath, original.IMPLPath)
	}
}

func TestRunFailedPayload_JSON(t *testing.T) {
	original := RunFailedPayload{Error: "test error"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var got RunFailedPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.Error != original.Error {
		t.Errorf("Error: got %q, want %q", got.Error, original.Error)
	}
}

func TestWaveGatePendingPayload_JSON(t *testing.T) {
	original := WaveGatePendingPayload{Wave: 1, NextWave: 2, Slug: "my-feature"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	var got WaveGatePendingPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if got.Wave != original.Wave {
		t.Errorf("Wave: got %d, want %d", got.Wave, original.Wave)
	}
	if got.NextWave != original.NextWave {
		t.Errorf("NextWave: got %d, want %d", got.NextWave, original.NextWave)
	}
	if got.Slug != original.Slug {
		t.Errorf("Slug: got %q, want %q", got.Slug, original.Slug)
	}
}
