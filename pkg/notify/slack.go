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

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackSection struct {
	Type   string       `json:"type"`
	Text   *slackText   `json:"text,omitempty"`
	Fields []*slackText `json:"fields,omitempty"`
}

type slackContext struct {
	Type     string       `json:"type"`
	Elements []*slackText `json:"elements"`
}

type slackPayload struct {
	Text    string        `json:"text,omitempty"`
	Channel string        `json:"channel,omitempty"`
	Blocks  []interface{} `json:"blocks,omitempty"`
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
		client:      &http.Client{Timeout: defaultHTTPTimeout},
	}, nil
}

// Name returns the adapter name.
func (s *SlackAdapter) Name() string { return "slack" }

// Send delivers a formatted message via webhook or Bot API.
func (s *SlackAdapter) Send(ctx context.Context, msg Message) result.Result[SendData] {
	p := slackPayload{Text: msg.Text}
	if s.destination != "" {
		p.Channel = s.destination
	}
	// msg.Embeds must be []interface{} (as produced by SlackFormatter.Format)
	// for the assertion to succeed. If Embeds is any other type (e.g. nil
	// or []discordEmbed), the assertion fails silently and the message falls
	// back to plain text (msg.Text only). Callers using a non-Slack formatter
	// must ensure Embeds is []interface{} or leave it nil.
	if msg.Embeds != nil {
		if blocks, ok := msg.Embeds.([]interface{}); ok {
			p.Blocks = blocks
		}
	}

	body, err := json.Marshal(p)
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
				{Code: result.CodeContextCancelled, Message: ctx.Err().Error(), Severity: "fatal"},
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
		return "#808080" // neutral gray for unrecognized severity
	}
}

// SlackFormatter formats events using Slack Block Kit structures.
type SlackFormatter struct{}

// Format transforms an Event into a Slack Block Kit Message.
func (f *SlackFormatter) Format(event Event) Message {
	blocks := []interface{}{}

	// Section block with title as text
	titleBlock := &slackSection{
		Type: "section",
		Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf("*%s*", event.Title)},
	}
	blocks = append(blocks, titleBlock)

	// Body as a section if present
	if event.Body != "" {
		bodyBlock := &slackSection{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: event.Body},
		}
		blocks = append(blocks, bodyBlock)
	}

	// Fields block with key-value pairs
	if len(event.Fields) > 0 {
		// Sort keys for deterministic output
		keys := make([]string, 0, len(event.Fields))
		for k := range event.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		fieldItems := make([]*slackText, 0, len(event.Fields))
		for _, k := range keys {
			fieldItems = append(fieldItems, &slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s:* %s", k, event.Fields[k]),
			})
		}
		fieldsBlock := &slackSection{Type: "section", Fields: fieldItems}
		blocks = append(blocks, fieldsBlock)
	}

	// Color context block
	colorBlock := &slackContext{
		Type: "context",
		Elements: []*slackText{{
			Type: "mrkdwn",
			Text: fmt.Sprintf("Severity: %s | Color: %s", event.Severity, severityColor(event.Severity)),
		}},
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
