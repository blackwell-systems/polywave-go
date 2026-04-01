package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Telegram adapter error codes (not registered in pkg/result/codes.go because
// this package is designed as an extractable library with its own error domain):
//   TELEGRAM_MARSHAL_ERROR   — JSON marshal of outbound payload failed
//   TELEGRAM_REQUEST_ERROR   — http.NewRequestWithContext failed
//   TELEGRAM_SEND_ERROR      — HTTP client.Do failed (non-context error)
//   TELEGRAM_RATE_LIMITED    — 429 Too Many Requests
//   TELEGRAM_HTTP_ERROR      — non-2xx HTTP status (not 429)

// TelegramAdapter sends notifications via the Telegram Bot API.
type TelegramAdapter struct {
	token       string
	destination string
	client      *http.Client
	// baseURL allows overriding the API endpoint for testing.
	baseURL string
}

// NewTelegramAdapter creates a Telegram Bot API adapter.
// Required cfg keys: "token" and "destination" (with fallback to "bot_token" and "chat_id").
func NewTelegramAdapter(cfg map[string]string) (Adapter, error) {
	token := readWithFallback(cfg, "token", "bot_token")
	if token == "" {
		return nil, fmt.Errorf("telegram: missing required config key \"token\"")
	}
	destination := readWithFallback(cfg, "destination", "chat_id")
	if destination == "" {
		return nil, fmt.Errorf("telegram: missing required config key \"destination\"")
	}
	return &TelegramAdapter{
		token:       token,
		destination: destination,
		client:      &http.Client{Timeout: defaultHTTPTimeout},
		baseURL:     "https://api.telegram.org",
	}, nil
}

// Name returns the adapter name.
func (a *TelegramAdapter) Name() string { return "telegram" }

// Send delivers a formatted message via the Telegram sendMessage API.
func (a *TelegramAdapter) Send(ctx context.Context, msg Message) result.Result[SendData] {
	url := fmt.Sprintf("%s/bot%s/sendMessage", a.baseURL, a.token)

	payload := map[string]string{
		"chat_id":    a.destination,
		"text":       msg.Text,
		"parse_mode": "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "TELEGRAM_MARSHAL_ERROR", Message: fmt.Sprintf("telegram: marshal payload: %v", err), Severity: "fatal"},
		})
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "TELEGRAM_REQUEST_ERROR", Message: fmt.Sprintf("telegram: create request: %v", err), Severity: "fatal"},
		})
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return result.NewFailure[SendData]([]result.SAWError{
				{Code: result.CodeContextCancelled, Message: ctx.Err().Error(), Severity: "fatal"},
			})
		}
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "TELEGRAM_SEND_ERROR", Message: fmt.Sprintf("telegram: send request: %v", err), Severity: "fatal"},
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		return result.NewFailure[SendData]([]result.SAWError{
			{
				Code:     "TELEGRAM_RATE_LIMITED",
				Message:  "telegram: rate limited",
				Severity: "error",
				Context:  map[string]string{"retry_after": retryAfter},
			},
		})
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result.NewFailure[SendData]([]result.SAWError{
			{
				Code:     "TELEGRAM_HTTP_ERROR",
				Message:  fmt.Sprintf("telegram: unexpected status %d", resp.StatusCode),
				Severity: "fatal",
			},
		})
	}

	var telegramResp struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	var msgID string
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err == nil && telegramResp.OK && telegramResp.Result.MessageID > 0 {
		msgID = fmt.Sprintf("%d", telegramResp.Result.MessageID)
	}

	return result.NewSuccess(SendData{
		MessageID: msgID,
		Timestamp: time.Now(),
		Provider:  "telegram",
	})
}

// TelegramFormatter formats events into Telegram HTML messages.
type TelegramFormatter struct{}

// Format transforms an Event into a Telegram HTML Message.
func (f *TelegramFormatter) Format(event Event) Message {
	var sb strings.Builder
	sb.WriteString("<b>")
	sb.WriteString(event.Title)
	sb.WriteString("</b>\n")
	sb.WriteString(event.Body)

	if len(event.Fields) > 0 {
		sb.WriteString("\n")
		keys := make([]string, 0, len(event.Fields))
		for k := range event.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString("\n<i>")
			sb.WriteString(k)
			sb.WriteString(":</i> ")
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
