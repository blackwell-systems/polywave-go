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
		Embeds: []discordEmbed{
			{Title: "Test", Description: "body", Color: 3447003},
		},
	}

	res := adapter.Send(context.Background(), msg)
	if !res.IsSuccess() {
		t.Fatalf("unexpected result %q: %v", res.Code, res.Errors)
	}
	data := res.GetData()
	if data.Provider != "discord" {
		t.Errorf("expected provider 'discord', got %q", data.Provider)
	}
	if data.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
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

	res := adapter.Send(context.Background(), Message{Text: "test"})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for 403 response, got %q", res.Code)
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

	res := adapter.Send(context.Background(), Message{Text: "plain text only"})
	if !res.IsSuccess() {
		t.Fatalf("unexpected result %q: %v", res.Code, res.Errors)
	}
	if _, hasContent := receivedBody["content"]; !hasContent {
		t.Error("expected 'content' key for plain text message")
	}
}

func TestDiscordAdapter_SendRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	adapter := &DiscordAdapter{
		webhookURL: srv.URL,
		client:     srv.Client(),
	}

	res := adapter.Send(context.Background(), Message{Text: "test"})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for rate limit, got %q", res.Code)
	}
	if len(res.Errors) == 0 || res.Errors[0].Code != "DISCORD_RATE_LIMITED" {
		t.Errorf("expected DISCORD_RATE_LIMITED error, got %v", res.Errors)
	}
}

func TestDiscordAdapter_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Use a slow server that the context will interrupt.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	adapter := &DiscordAdapter{
		webhookURL: srv.URL,
		client:     srv.Client(),
	}

	res := adapter.Send(ctx, Message{Text: "test"})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for cancelled context, got %q", res.Code)
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

	embeds, ok := msg.Embeds.([]discordEmbed)
	if !ok || len(embeds) != 1 {
		t.Fatal("expected exactly one embed")
	}

	embed := embeds[0]
	if embed.Title != "Wave 1 Complete" {
		t.Errorf("expected title 'Wave 1 Complete', got %q", embed.Title)
	}
	if embed.Color != discordColorInfo {
		t.Errorf("expected info color %d, got %d", discordColorInfo, embed.Color)
	}

	if len(embed.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(embed.Fields))
	}
	// Fields are sorted alphabetically.
	if embed.Fields[0].Name != "agents" {
		t.Errorf("expected first field name 'agents', got %q", embed.Fields[0].Name)
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
		embeds := msg.Embeds.([]discordEmbed)
		if embeds[0].Color != tt.color {
			t.Errorf("severity %s: expected color %d, got %d", tt.severity, tt.color, embeds[0].Color)
		}
	}
}

func TestDiscordFormatter_TypedEmbeds(t *testing.T) {
	f := &DiscordFormatter{}
	msg := f.Format(Event{Title: "T"})

	data, err := json.Marshal(msg.Embeds)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded []map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatal("expected at least one embed")
	}
	if _, ok := decoded[0]["title"]; !ok {
		t.Error("expected \"title\" key in embed")
	}
	if _, ok := decoded[0]["color"]; !ok {
		t.Error("expected \"color\" key in embed")
	}
}
