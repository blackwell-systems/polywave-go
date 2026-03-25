package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

// SlackAdapter sends notifications via Slack incoming webhooks.
type SlackAdapter struct {
	webhookURL string
	channel    string // optional override
	client     *http.Client
}

// NewSlackAdapter creates a new Slack adapter from configuration.
// Required cfg keys: "webhook_url". Optional: "channel".
func NewSlackAdapter(cfg map[string]string) (Adapter, error) {
	url := cfg["webhook_url"]
	if url == "" {
		return nil, fmt.Errorf("slack: missing required config key \"webhook_url\"")
	}
	return &SlackAdapter{
		webhookURL: url,
		channel:    cfg["channel"],
		client:     &http.Client{},
	}, nil
}

// Name returns the adapter name.
func (s *SlackAdapter) Name() string { return "slack" }

// Send delivers a formatted message to the Slack webhook.
func (s *SlackAdapter) Send(ctx context.Context, msg Message) error {
	payload := make(map[string]interface{})

	if msg.Embeds != nil {
		if blocks, ok := msg.Embeds.([]interface{}); ok {
			payload["blocks"] = blocks
		}
	}

	// Fallback text is required by Slack for notifications
	if msg.Text != "" {
		payload["text"] = msg.Text
	}

	if s.channel != "" {
		payload["channel"] = s.channel
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("slack: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack: unexpected status %d", resp.StatusCode)
	}

	return nil
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

// RegisterSlack registers the Slack adapter factory with the global registry.
// Call this from an init() function once the registry package is available,
// or invoke it explicitly during application startup:
//
//	notify.Register("slack", func(cfg map[string]string) (notify.Adapter, error) {
//	    return notify.NewSlackAdapter(cfg)
//	})

