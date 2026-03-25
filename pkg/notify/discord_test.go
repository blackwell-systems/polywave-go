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

func TestNewDiscordAdapter_Success(t *testing.T) {
	a, err := NewDiscordAdapter(map[string]string{"webhook_url": "https://discord.com/api/webhooks/123/abc"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name() != "discord" {
		t.Errorf("expected name 'discord', got %q", a.Name())
	}
}

func TestNewDiscordAdapter_MissingURL(t *testing.T) {
	_, err := NewDiscordAdapter(map[string]string{})
	if err == nil {
		t.Fatal("expected error for missing webhook_url")
	}
}

func TestNewDiscordAdapter_EmptyURL(t *testing.T) {
	_, err := NewDiscordAdapter(map[string]string{"webhook_url": ""})
	if err == nil {
		t.Fatal("expected error for empty webhook_url")
	}
}

func TestDiscordAdapter_SendSuccess(t *testing.T) {
	var receivedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	adapter := &DiscordAdapter{
		webhookURL: srv.URL,
		client:     srv.Client(),
	}

	msg := Message{
		Text: "test",
		Embeds: []map[string]interface{}{
			{"title": "Test", "description": "body", "color": 3447003},
		},
	}

	err := adapter.Send(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedBody == nil {
		t.Fatal("server did not receive body")
	}
	embeds, ok := receivedBody["embeds"]
	if !ok {
		t.Fatal("expected embeds in payload")
	}
	embedList, ok := embeds.([]interface{})
	if !ok || len(embedList) == 0 {
		t.Fatal("expected non-empty embeds array")
	}
}

func TestDiscordAdapter_SendHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	adapter := &DiscordAdapter{
		webhookURL: srv.URL,
		client:     srv.Client(),
	}

	err := adapter.Send(context.Background(), Message{Text: "test"})
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
}

func TestDiscordAdapter_SendPlainText(t *testing.T) {
	var receivedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	adapter := &DiscordAdapter{
		webhookURL: srv.URL,
		client:     srv.Client(),
	}

	err := adapter.Send(context.Background(), Message{Text: "plain text only"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, hasContent := receivedBody["content"]; !hasContent {
		t.Error("expected 'content' key for plain text message")
	}
}

func TestDiscordFormatter_Format(t *testing.T) {
	f := &DiscordFormatter{}
	event := Event{
		Type:     "wave_complete",
		Severity: SeverityInfo,
		Title:    "Wave 1 Complete",
		Body:     "All agents succeeded",
		Fields: map[string]string{
			"agents": "3",
			"wave":   "1",
		},
		Timestamp: time.Now(),
	}

	msg := f.Format(event)
	if msg.Text == "" {
		t.Error("expected non-empty text")
	}
	if msg.Embeds == nil {
		t.Fatal("expected non-nil embeds")
	}

	embeds, ok := msg.Embeds.([]map[string]interface{})
	if !ok || len(embeds) != 1 {
		t.Fatal("expected exactly one embed")
	}

	embed := embeds[0]
	if embed["title"] != "Wave 1 Complete" {
		t.Errorf("expected title 'Wave 1 Complete', got %v", embed["title"])
	}
	if embed["color"] != discordColorInfo {
		t.Errorf("expected info color %d, got %v", discordColorInfo, embed["color"])
	}

	fields, ok := embed["fields"].([]map[string]interface{})
	if !ok || len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %v", embed["fields"])
	}
	// Fields are sorted alphabetically.
	if fields[0]["name"] != "agents" {
		t.Errorf("expected first field name 'agents', got %v", fields[0]["name"])
	}
}

func TestDiscordFormatter_SeverityColors(t *testing.T) {
	f := &DiscordFormatter{}
	tests := []struct {
		severity Severity
		color    int
	}{
		{SeverityInfo, discordColorInfo},
		{SeverityWarning, discordColorWarning},
		{SeverityError, discordColorError},
	}
	for _, tt := range tests {
		msg := f.Format(Event{Severity: tt.severity, Title: "test", Body: "body"})
		embeds := msg.Embeds.([]map[string]interface{})
		if embeds[0]["color"] != tt.color {
			t.Errorf("severity %s: expected color %d, got %v", tt.severity, tt.color, embeds[0]["color"])
		}
	}
}
