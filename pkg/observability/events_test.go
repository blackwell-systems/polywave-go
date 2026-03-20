package observability

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEventID(t *testing.T) {
	id1 := NewEventID()
	id2 := NewEventID()
	if id1 == "" {
		t.Fatal("NewEventID returned empty string")
	}
	if id1 == id2 {
		t.Fatalf("NewEventID returned duplicate IDs: %s", id1)
	}
	// UUID v4 format: 8-4-4-4-12 hex chars
	if len(id1) != 36 {
		t.Fatalf("expected 36-char UUID, got %d chars: %s", len(id1), id1)
	}
}

func TestCostEvent_Interface(t *testing.T) {
	now := time.Now().UTC()
	evt := &CostEvent{
		ID:           NewEventID(),
		Type:         "cost",
		Time:         now,
		AgentID:      "A",
		WaveNumber:   1,
		IMPLSlug:     "add-auth",
		Model:        "claude-sonnet-4-6",
		InputTokens:  1500,
		OutputTokens: 800,
		CostUSD:      0.012,
		Meta:         map[string]any{"provider": "anthropic"},
	}

	var e Event = evt
	if e.EventID() != evt.ID {
		t.Errorf("EventID() = %q, want %q", e.EventID(), evt.ID)
	}
	if e.EventType() != "cost" {
		t.Errorf("EventType() = %q, want %q", e.EventType(), "cost")
	}
	if !e.Timestamp().Equal(now) {
		t.Errorf("Timestamp() = %v, want %v", e.Timestamp(), now)
	}
	meta := e.Metadata()
	if meta["provider"] != "anthropic" {
		t.Errorf("Metadata()[\"provider\"] = %v, want %q", meta["provider"], "anthropic")
	}
}

func TestAgentPerformanceEvent_Interface(t *testing.T) {
	now := time.Now().UTC()
	evt := &AgentPerformanceEvent{
		ID:              NewEventID(),
		Type:            "agent_performance",
		Time:            now,
		AgentID:         "B",
		WaveNumber:      2,
		IMPLSlug:        "add-auth",
		Status:          "success",
		DurationSeconds: 120,
		FilesModified:   []string{"pkg/auth/handler.go"},
		TestsPassed:     5,
		TestsFailed:     0,
		Meta:            map[string]any{"ci": true},
	}

	var e Event = evt
	if e.EventID() != evt.ID {
		t.Errorf("EventID() = %q, want %q", e.EventID(), evt.ID)
	}
	if e.EventType() != "agent_performance" {
		t.Errorf("EventType() = %q, want %q", e.EventType(), "agent_performance")
	}
	if !e.Timestamp().Equal(now) {
		t.Errorf("Timestamp() = %v, want %v", e.Timestamp(), now)
	}
	if e.Metadata()["ci"] != true {
		t.Errorf("Metadata()[\"ci\"] = %v, want true", e.Metadata()["ci"])
	}
}

func TestActivityEvent_Interface(t *testing.T) {
	now := time.Now().UTC()
	evt := &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         now,
		ActivityType: "wave_start",
		IMPLSlug:     "add-auth",
		WaveNumber:   1,
		User:         "dayna",
		Details:      "launched wave 1 with 3 agents",
		Meta:         map[string]any{"agents": 3},
	}

	var e Event = evt
	if e.EventID() != evt.ID {
		t.Errorf("EventID() = %q, want %q", e.EventID(), evt.ID)
	}
	if e.EventType() != "activity" {
		t.Errorf("EventType() = %q, want %q", e.EventType(), "activity")
	}
	if !e.Timestamp().Equal(now) {
		t.Errorf("Timestamp() = %v, want %v", e.Timestamp(), now)
	}
	if e.Metadata()["agents"] != 3 {
		t.Errorf("Metadata()[\"agents\"] = %v, want 3", e.Metadata()["agents"])
	}
}

func TestCostEvent_JSON(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	evt := &CostEvent{
		ID:           "test-id-1",
		Type:         "cost",
		Time:         now,
		AgentID:      "A",
		WaveNumber:   1,
		IMPLSlug:     "add-auth",
		Model:        "claude-sonnet-4-6",
		InputTokens:  1500,
		OutputTokens: 800,
		CostUSD:      0.012,
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got CostEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ID != evt.ID {
		t.Errorf("ID = %q, want %q", got.ID, evt.ID)
	}
	if got.Type != "cost" {
		t.Errorf("Type = %q, want %q", got.Type, "cost")
	}
	if !got.Time.Equal(now) {
		t.Errorf("Time = %v, want %v", got.Time, now)
	}
	if got.InputTokens != 1500 {
		t.Errorf("InputTokens = %d, want 1500", got.InputTokens)
	}
	if got.CostUSD != 0.012 {
		t.Errorf("CostUSD = %f, want 0.012", got.CostUSD)
	}
}

func TestAgentPerformanceEvent_JSON(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	evt := &AgentPerformanceEvent{
		ID:              "test-id-2",
		Type:            "agent_performance",
		Time:            now,
		AgentID:         "B",
		WaveNumber:      1,
		IMPLSlug:        "add-auth",
		Status:          "failed",
		FailureType:     "transient",
		RetryCount:      2,
		DurationSeconds: 60,
		FilesModified:   []string{"a.go", "b.go"},
		TestsPassed:     3,
		TestsFailed:     1,
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got AgentPerformanceEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Status != "failed" {
		t.Errorf("Status = %q, want %q", got.Status, "failed")
	}
	if got.FailureType != "transient" {
		t.Errorf("FailureType = %q, want %q", got.FailureType, "transient")
	}
	if len(got.FilesModified) != 2 {
		t.Errorf("FilesModified length = %d, want 2", len(got.FilesModified))
	}
}

func TestActivityEvent_JSON(t *testing.T) {
	now := time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC)
	evt := &ActivityEvent{
		ID:           "test-id-3",
		Type:         "activity",
		Time:         now,
		ActivityType: "scout_launch",
		IMPLSlug:     "add-auth",
		User:         "dayna",
		Details:      "started scout",
		Meta:         map[string]any{"dry_run": false},
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got ActivityEvent
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.ActivityType != "scout_launch" {
		t.Errorf("ActivityType = %q, want %q", got.ActivityType, "scout_launch")
	}
	if got.User != "dayna" {
		t.Errorf("User = %q, want %q", got.User, "dayna")
	}
	if got.Meta["dry_run"] != false {
		t.Errorf("Meta[\"dry_run\"] = %v, want false", got.Meta["dry_run"])
	}
}

func TestEvent_NilMetadata(t *testing.T) {
	evt := &CostEvent{
		ID:   "test-nil-meta",
		Type: "cost",
		Time: time.Now().UTC(),
	}
	if evt.Metadata() != nil {
		t.Errorf("expected nil Metadata for zero-value Meta, got %v", evt.Metadata())
	}
}
