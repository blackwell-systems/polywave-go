package notify

import (
	"context"
	"time"
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

// Adapter delivers a formatted Message to an external service.
type Adapter interface {
	Name() string
	Send(ctx context.Context, msg Message) error
}

// Formatter transforms an Event into a Message suitable for a specific adapter.
type Formatter interface {
	Format(event Event) Message
}
