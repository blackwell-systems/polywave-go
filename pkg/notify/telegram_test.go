package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewTelegramAdapter_Success(t *testing.T) {
	a, err := NewTelegramAdapter(map[string]string{
		"token":       "123:ABC",
		"destination": "-100123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name() != "telegram" {
		t.Errorf("expected name 'telegram', got %q", a.Name())
	}
}

func TestNewTelegramAdapter_MissingToken(t *testing.T) {
	_, err := NewTelegramAdapter(map[string]string{"destination": "-100123"})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestNewTelegramAdapter_MissingDestination(t *testing.T) {
	_, err := NewTelegramAdapter(map[string]string{"token": "123:ABC"})
	if err == nil {
		t.Fatal("expected error for missing destination")
	}
}

func TestTelegramAdapter_SendSuccess(t *testing.T) {
	var receivedBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}
		// Verify the URL contains the bot token path.
		if !strings.Contains(r.URL.Path, "/bot123:ABC/sendMessage") {
			t.Errorf("unexpected URL path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	adapter := &TelegramAdapter{
		token:       "123:ABC",
		destination: "-100123",
		client:      srv.Client(),
		baseURL:     srv.URL,
	}

	res := adapter.Send(context.Background(), Message{Text: "*Hello*\nWorld"})
	if !res.IsSuccess() {
		t.Fatalf("unexpected result %q: %v", res.Code, res.Errors)
	}
	data := res.GetData()
	if data.Provider != "telegram" {
		t.Errorf("expected provider 'telegram', got %q", data.Provider)
	}
	if data.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if receivedBody["chat_id"] != "-100123" {
		t.Errorf("expected chat_id '-100123', got %q", receivedBody["chat_id"])
	}
	if receivedBody["parse_mode"] != "Markdown" {
		t.Errorf("expected parse_mode 'Markdown', got %q", receivedBody["parse_mode"])
	}
}

func TestTelegramAdapter_SendHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	adapter := &TelegramAdapter{
		token:       "bad-token",
		destination: "-100123",
		client:      srv.Client(),
		baseURL:     srv.URL,
	}

	res := adapter.Send(context.Background(), Message{Text: "test"})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for 401 response, got %q", res.Code)
	}
}

func TestTelegramAdapter_SendRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	adapter := &TelegramAdapter{
		token:       "123:ABC",
		destination: "-100123",
		client:      srv.Client(),
		baseURL:     srv.URL,
	}

	res := adapter.Send(context.Background(), Message{Text: "test"})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for rate limit, got %q", res.Code)
	}
	if len(res.Errors) == 0 || res.Errors[0].Code != "TELEGRAM_RATE_LIMITED" {
		t.Errorf("expected TELEGRAM_RATE_LIMITED error, got %v", res.Errors)
	}
}

func TestTelegramAdapter_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	adapter := &TelegramAdapter{
		token:       "123:ABC",
		destination: "-100123",
		client:      srv.Client(),
		baseURL:     srv.URL,
	}

	res := adapter.Send(ctx, Message{Text: "test"})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for cancelled context, got %q", res.Code)
	}
}

func TestNewTelegramAdapter_BackwardCompat(t *testing.T) {
	a, err := NewTelegramAdapter(map[string]string{
		"bot_token": "123:ABC",
		"chat_id":   "-100123",
	})
	if err != nil {
		t.Fatalf("expected backward-compat fields to work, got error: %v", err)
	}
	if a.Name() != "telegram" {
		t.Errorf("expected name 'telegram', got %q", a.Name())
	}
}

func TestTelegramFormatter_Format(t *testing.T) {
	f := &TelegramFormatter{}
	event := Event{
		Type:     "agent_failed",
		Severity: SeverityError,
		Title:    "Agent B Failed",
		Body:     "Build error in pkg/api",
		Fields: map[string]string{
			"agent": "B",
			"wave":  "2",
		},
		Timestamp: time.Now(),
	}

	msg := f.Format(event)
	if !strings.Contains(msg.Text, "*Agent B Failed*") {
		t.Error("expected Markdown bold title")
	}
	if !strings.Contains(msg.Text, "Build error in pkg/api") {
		t.Error("expected body text")
	}
	if !strings.Contains(msg.Text, "_agent:_ B") {
		t.Errorf("expected field formatting, got: %s", msg.Text)
	}
	if !strings.Contains(msg.Text, "_wave:_ 2") {
		t.Errorf("expected wave field, got: %s", msg.Text)
	}
}

func TestTelegramFormatter_FormatNoFields(t *testing.T) {
	f := &TelegramFormatter{}
	event := Event{
		Title: "Simple Event",
		Body:  "No extra fields",
	}

	msg := f.Format(event)
	expected := "*Simple Event*\nNo extra fields"
	if msg.Text != expected {
		t.Errorf("expected %q, got %q", expected, msg.Text)
	}
}
