package notify

import (
	"context"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Severity represents the importance level of a notification event.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Event is a generic notification event. CRITICAL: no SAW-specific imports.
type Event struct {
	Type      string            // e.g. "wave_complete", "agent_failed"
	Severity  Severity
	Title     string
	Body      string
	Fields    map[string]string // arbitrary key-value metadata
	Timestamp time.Time
}

// Message is the formatted output ready for delivery to a specific adapter.
type Message struct {
	Text   string      // plain text fallback
	Embeds interface{} // adapter-specific rich content (Slack blocks, Discord embeds)
}

// SendData carries metadata about a successfully delivered message.
type SendData struct {
	MessageID string
	Timestamp time.Time
	Provider  string
}

// DispatchData carries aggregate metrics from a fan-out Dispatch call.
type DispatchData struct {
	SentCount   int
	FailedCount int
	Errors      []result.SAWError
}

// Adapter delivers a formatted Message to an external service.
type Adapter interface {
	Name() string
	Send(ctx context.Context, msg Message) result.Result[SendData]
}

// Formatter transforms an Event into a Message suitable for a specific adapter.
type Formatter interface {
	Format(event Event) Message
}

// defaultHTTPTimeout is the timeout applied to all outbound HTTP clients
// in notify adapters (Slack, Discord, Telegram).
const defaultHTTPTimeout = 10 * time.Second

// readWithFallback reads key from cfg, falling back to fallbackKey.
func readWithFallback(cfg map[string]string, key, fallbackKey string) string {
	if v := cfg[key]; v != "" {
		return v
	}
	return cfg[fallbackKey]
}
