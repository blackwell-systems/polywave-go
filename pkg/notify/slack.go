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

// readWithFallback reads key from cfg, falling back to fallbackKey.
func readWithFallback(cfg map[string]string, key, fallbackKey string) string {
	if v := cfg[key]; v != "" {
		return v
	}
	return cfg[fallbackKey]
}

// SlackAdapter sends notifications via Slack incoming webhooks or Bot API.
// Supports two modes:
//   - Webhook mode: set "webhook_url" — posts to the channel configured in the webhook
//   - Bot token mode: set "token" + "destination" — posts to any channel via chat.postMessage
type SlackAdapter struct {
	webhookURL  string
	token       string
	destination string
	client      *http.Client
}

// NewSlackAdapter creates a new Slack adapter from configuration.
// Requires either "webhook_url" OR ("token" + "destination").
// Accepts legacy field names "bot_token" and "channel" as fallbacks.
// Optional in webhook mode: "destination" (override, only works with legacy webhooks).
func NewSlackAdapter(cfg map[string]string) (Adapter, error) {
	url := cfg["webhook_url"]
	token := readWithFallback(cfg, "token", "bot_token")
	destination := readWithFallback(cfg, "destination", "channel")

	if url == "" && token == "" {
		return nil, fmt.Errorf("slack: requires either \"webhook_url\" or \"token\"")
	}
	if token != "" && destination == "" {
		return nil, fmt.Errorf("slack: \"destination\" is required when using \"token\"")
	}

	return &SlackAdapter{
		webhookURL:  url,
		token:       token,
		destination: destination,
		client:      &http.Client{},
	}, nil
}

// Name returns the adapter name.
func (s *SlackAdapter) Name() string { return "slack" }

// Send delivers a formatted message via webhook or Bot API.
func (s *SlackAdapter) Send(ctx context.Context, msg Message) result.Result[SendData] {
	payload := make(map[string]interface{})

	if msg.Embeds != nil {
		if blocks, ok := msg.Embeds.([]interface{}); ok {
			payload["blocks"] = blocks
		}
	}

	if msg.Text != "" {
		payload["text"] = msg.Text
	}

	if s.destination != "" {
		payload["channel"] = s.destination
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "SLACK_MARSHAL_ERROR", Message: fmt.Sprintf("slack: marshal payload: %v", err), Severity: "fatal"},
		})
	}

	// Bot token mode: POST to chat.postMessage API
	var targetURL string
	if s.token != "" {
		targetURL = "https://slack.com/api/chat.postMessage"
	} else {
		targetURL = s.webhookURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "SLACK_REQUEST_ERROR", Message: fmt.Sprintf("slack: create request: %v", err), Severity: "fatal"},
		})
	}
	req.Header.Set("Content-Type", "application/json")

	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return result.NewFailure[SendData]([]result.SAWError{
				{Code: "CONTEXT_CANCELLED", Message: ctx.Err().Error(), Severity: "fatal"},
			})
		}
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "SLACK_SEND_ERROR", Message: fmt.Sprintf("slack: send request: %v", err), Severity: "fatal"},
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := resp.Header.Get("Retry-After")
		return result.NewFailure[SendData]([]result.SAWError{
			{
				Code:     "SLACK_RATE_LIMITED",
				Message:  "slack: rate limited",
				Severity: "error",
				Context:  map[string]string{"retry_after": retryAfter},
			},
		})
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result.NewFailure[SendData]([]result.SAWError{
			{
				Code:     "SLACK_HTTP_ERROR",
				Message:  fmt.Sprintf("slack: unexpected status %d", resp.StatusCode),
				Severity: "fatal",
			},
		})
	}

	// Bot API returns {"ok": false, "error": "..."} on failure even with 200
	if s.token != "" {
		var apiResp struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err == nil && !apiResp.OK {
			return result.NewFailure[SendData]([]result.SAWError{
				{Code: "SLACK_API_ERROR", Message: fmt.Sprintf("slack: API error: %s", apiResp.Error), Severity: "fatal"},
			})
		}
	}

	return result.NewSuccess(SendData{
		Timestamp: time.Now(),
		Provider:  "slack",
	})
}

// severityColor maps severity levels to Slack color codes.
func severityColor(s Severity) string {
	switch s {
	case SeverityInfo:
		return "#36a64f" // good (green)
	case SeverityWarning:
		return "#daa038" // warning (amber)
	case SeverityError:
		return "#cc0000" // danger (red)
	default:
		return "#36a64f"
	}
}

// SlackFormatter formats events using Slack Block Kit structures.
type SlackFormatter struct{}

// Format transforms an Event into a Slack Block Kit Message.
func (f *SlackFormatter) Format(event Event) Message {
	blocks := []interface{}{}

	// Section block with title as text
	titleBlock := map[string]interface{}{
		"type": "section",
		"text": map[string]interface{}{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*%s*", event.Title),
		},
	}
	blocks = append(blocks, titleBlock)

	// Body as a section if present
	if event.Body != "" {
		bodyBlock := map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": event.Body,
			},
		}
		blocks = append(blocks, bodyBlock)
	}

	// Fields block with key-value pairs
	if len(event.Fields) > 0 {
		fields := []interface{}{}
		// Sort keys for deterministic output
		keys := make([]string, 0, len(event.Fields))
		for k := range event.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fields = append(fields, map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s:* %s", k, event.Fields[k]),
			})
		}
		fieldsBlock := map[string]interface{}{
			"type":   "section",
			"fields": fields,
		}
		blocks = append(blocks, fieldsBlock)
	}

	// Color context block
	colorBlock := map[string]interface{}{
		"type": "context",
		"elements": []interface{}{
			map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("Severity: %s | Color: %s", event.Severity, severityColor(event.Severity)),
			},
		},
	}
	blocks = append(blocks, colorBlock)

	return Message{
		Text:   event.Title,
		Embeds: blocks,
	}
}

func init() {
	Register("slack", NewSlackAdapter)
}
