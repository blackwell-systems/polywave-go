package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Discord embed color constants by severity.
const (
	discordColorInfo    = 3447003  // blue
	discordColorWarning = 16776960 // yellow
	discordColorError   = 15158332 // red
)

// DiscordAdapter sends notifications via Discord webhook with embed formatting.
type DiscordAdapter struct {
	webhookURL string
	client     *http.Client
}

// NewDiscordAdapter creates a Discord webhook adapter.
// Required cfg keys: "webhook_url".
func NewDiscordAdapter(cfg map[string]string) (Adapter, error) {
	url, ok := cfg["webhook_url"]
	if !ok || url == "" {
		return nil, fmt.Errorf("discord: missing required config key \"webhook_url\"")
	}
	return &DiscordAdapter{
		webhookURL: url,
		client:     &http.Client{},
	}, nil
}

// Name returns the adapter name.
func (a *DiscordAdapter) Name() string { return "discord" }

// Send delivers a formatted message to the Discord webhook endpoint.
func (a *DiscordAdapter) Send(ctx context.Context, msg Message) result.Result[SendData] {
	var payload interface{}
	if msg.Embeds != nil {
		payload = map[string]interface{}{
			"embeds": msg.Embeds,
		}
	} else {
		payload = map[string]interface{}{
			"content": msg.Text,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "DISCORD_MARSHAL_ERROR", Message: fmt.Sprintf("discord: marshal payload: %v", err), Severity: "fatal"},
		})
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, bytes.NewReader(body))
	if err != nil {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "DISCORD_REQUEST_ERROR", Message: fmt.Sprintf("discord: create request: %v", err), Severity: "fatal"},
		})
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		// Context cancellation surfaces here.
		if ctx.Err() != nil {
			return result.NewFailure[SendData]([]result.SAWError{
				{Code: "CONTEXT_CANCELLED", Message: ctx.Err().Error(), Severity: "fatal"},
			})
		}
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "DISCORD_SEND_ERROR", Message: fmt.Sprintf("discord: send request: %v", err), Severity: "fatal"},
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		return result.NewFailure[SendData]([]result.SAWError{
			{
				Code:     "DISCORD_RATE_LIMITED",
				Message:  "discord: rate limited",
				Severity: "error",
				Context:  map[string]string{"retry_after": retryAfter},
			},
		})
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result.NewFailure[SendData]([]result.SAWError{
			{
				Code:     "DISCORD_HTTP_ERROR",
				Message:  fmt.Sprintf("discord: unexpected status %d", resp.StatusCode),
				Severity: "fatal",
			},
		})
	}

	return result.NewSuccess(SendData{
		Timestamp: time.Now(),
		Provider:  "discord",
	})
}

// DiscordFormatter formats events into Discord embed messages.
type DiscordFormatter struct{}

// Format transforms an Event into a Discord embed Message.
func (f *DiscordFormatter) Format(event Event) Message {
	color := discordColorInfo
	switch event.Severity {
	case SeverityWarning:
		color = discordColorWarning
	case SeverityError:
		color = discordColorError
	}

	fields := make([]map[string]interface{}, 0, len(event.Fields))
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(event.Fields))
	for k := range event.Fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fields = append(fields, map[string]interface{}{
			"name":   k,
			"value":  event.Fields[k],
			"inline": true,
		})
	}

	embed := map[string]interface{}{
		"title":       event.Title,
		"description": event.Body,
		"color":       color,
	}
	if len(fields) > 0 {
		embed["fields"] = fields
	}

	return Message{
		Text:   event.Title + ": " + event.Body,
		Embeds: []map[string]interface{}{embed},
	}
}

func init() {
	Register("discord", NewDiscordAdapter)
}
