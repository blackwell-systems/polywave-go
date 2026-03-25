package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// TelegramAdapter sends notifications via the Telegram Bot API.
type TelegramAdapter struct {
	botToken string
	chatID   string
	client   *http.Client
	// baseURL allows overriding the API endpoint for testing.
	baseURL string
}

// NewTelegramAdapter creates a Telegram Bot API adapter.
// Required cfg keys: "bot_token", "chat_id".
func NewTelegramAdapter(cfg map[string]string) (Adapter, error) {
	token, ok := cfg["bot_token"]
	if !ok || token == "" {
		return nil, fmt.Errorf("telegram: missing required config key \"bot_token\"")
	}
	chatID, ok := cfg["chat_id"]
	if !ok || chatID == "" {
		return nil, fmt.Errorf("telegram: missing required config key \"chat_id\"")
	}
	return &TelegramAdapter{
		botToken: token,
		chatID:   chatID,
		client:   &http.Client{},
		baseURL:  "https://api.telegram.org",
	}, nil
}

// Name returns the adapter name.
func (a *TelegramAdapter) Name() string { return "telegram" }

// Send delivers a formatted message via the Telegram sendMessage API.
func (a *TelegramAdapter) Send(ctx context.Context, msg Message) error {
	url := fmt.Sprintf("%s/bot%s/sendMessage", a.baseURL, a.botToken)

	payload := map[string]string{
		"chat_id":    a.chatID,
		"text":       msg.Text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// TelegramFormatter formats events into Telegram Markdown messages.
type TelegramFormatter struct{}

// Format transforms an Event into a Telegram Markdown Message.
func (f *TelegramFormatter) Format(event Event) Message {
	var sb strings.Builder
	sb.WriteString("*")
	sb.WriteString(event.Title)
	sb.WriteString("*\n")
	sb.WriteString(event.Body)

	if len(event.Fields) > 0 {
		sb.WriteString("\n")
		// Sort keys for deterministic output.
		keys := make([]string, 0, len(event.Fields))
		for k := range event.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString("\n_")
			sb.WriteString(k)
			sb.WriteString(":_ ")
			sb.WriteString(event.Fields[k])
		}
	}

	return Message{
		Text: sb.String(),
	}
}

func init() {
	Register("telegram", NewTelegramAdapter)
}
