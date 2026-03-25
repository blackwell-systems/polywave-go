package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewSlackAdapter_MissingWebhookURL(t *testing.T) {
	_, err := NewSlackAdapter(map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing webhook_url, got nil")
	}
}

func TestNewSlackAdapter_Success(t *testing.T) {
	adapter, err := NewSlackAdapter(map[string]string{
		"webhook_url": "https://hooks.slack.com/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter.Name() != "slack" {
		t.Errorf("expected name \"slack\", got %q", adapter.Name())
	}
}

func TestSlackAdapter_Send_Success(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &receivedBody); err != nil {
			t.Errorf("failed to unmarshal body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := NewSlackAdapter(map[string]string{
		"webhook_url": server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	formatter := &SlackFormatter{}
	event := Event{
		Type:     "wave_complete",
		Severity: SeverityInfo,
		Title:    "Wave 1 Complete",
		Body:     "All agents finished successfully",
		Fields: map[string]string{
			"agents": "3",
			"wave":   "1",
		},
		Timestamp: time.Now(),
	}
	msg := formatter.Format(event)

	err = adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify payload structure
	blocks, ok := receivedBody["blocks"].([]interface{})
	if !ok {
		t.Fatal("expected blocks array in payload")
	}
	if len(blocks) < 2 {
		t.Fatalf("expected at least 2 blocks, got %d", len(blocks))
	}

	// First block should be the title section
	firstBlock := blocks[0].(map[string]interface{})
	if firstBlock["type"] != "section" {
		t.Errorf("expected first block type \"section\", got %v", firstBlock["type"])
	}

	// Verify text fallback
	if receivedBody["text"] != "Wave 1 Complete" {
		t.Errorf("expected text fallback \"Wave 1 Complete\", got %v", receivedBody["text"])
	}

	// Should NOT have channel when not configured
	if _, exists := receivedBody["channel"]; exists {
		t.Error("expected no channel field when not configured")
	}
}

func TestSlackAdapter_Send_ChannelOverride(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := NewSlackAdapter(map[string]string{
		"webhook_url": server.URL,
		"channel":     "#deployments",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := Message{Text: "test", Embeds: []interface{}{}}
	err = adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	channel, ok := receivedBody["channel"].(string)
	if !ok || channel != "#deployments" {
		t.Errorf("expected channel \"#deployments\", got %v", receivedBody["channel"])
	}
}

func TestSlackAdapter_Send_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	adapter, err := NewSlackAdapter(map[string]string{
		"webhook_url": server.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	msg := Message{Text: "test"}
	err = adapter.Send(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

func TestSlackFormatter_Info(t *testing.T) {
	f := &SlackFormatter{}
	event := Event{
		Type:     "test",
		Severity: SeverityInfo,
		Title:    "Info Event",
		Fields:   map[string]string{"key": "value"},
	}
	msg := f.Format(event)

	if msg.Text != "Info Event" {
		t.Errorf("expected text \"Info Event\", got %q", msg.Text)
	}

	blocks, ok := msg.Embeds.([]interface{})
	if !ok {
		t.Fatal("expected Embeds to be []interface{}")
	}

	// Should have: title section, fields section, context block (no body)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks (title, fields, context), got %d", len(blocks))
	}

	// Verify context block has info color
	contextBlock := blocks[2].(map[string]interface{})
	elements := contextBlock["elements"].([]interface{})
	elem := elements[0].(map[string]interface{})
	text := elem["text"].(string)
	if text != "Severity: info | Color: #36a64f" {
		t.Errorf("unexpected context text: %s", text)
	}
}

func TestSlackFormatter_Warning(t *testing.T) {
	f := &SlackFormatter{}
	event := Event{
		Type:     "test",
		Severity: SeverityWarning,
		Title:    "Warning Event",
	}
	msg := f.Format(event)

	blocks := msg.Embeds.([]interface{})
	// title + context (no body, no fields)
	contextBlock := blocks[len(blocks)-1].(map[string]interface{})
	elements := contextBlock["elements"].([]interface{})
	elem := elements[0].(map[string]interface{})
	text := elem["text"].(string)
	if text != "Severity: warning | Color: #daa038" {
		t.Errorf("unexpected context text: %s", text)
	}
}

func TestSlackFormatter_Error(t *testing.T) {
	f := &SlackFormatter{}
	event := Event{
		Type:     "test",
		Severity: SeverityError,
		Title:    "Error Event",
	}
	msg := f.Format(event)

	blocks := msg.Embeds.([]interface{})
	contextBlock := blocks[len(blocks)-1].(map[string]interface{})
	elements := contextBlock["elements"].([]interface{})
	elem := elements[0].(map[string]interface{})
	text := elem["text"].(string)
	if text != "Severity: error | Color: #cc0000" {
		t.Errorf("unexpected context text: %s", text)
	}
}
