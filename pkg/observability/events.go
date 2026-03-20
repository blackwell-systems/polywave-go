package observability

import (
	"crypto/rand"
	"fmt"
	"time"
)

// Event is the base interface for all observability events.
type Event interface {
	EventID() string
	EventType() string
	Timestamp() time.Time
	Metadata() map[string]any
}

// NewEventID generates a random UUID v4 string for use as an event identifier.
func NewEventID() string {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		panic(fmt.Sprintf("observability: failed to generate UUID: %v", err))
	}
	// Set version 4 and variant bits per RFC 4122.
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// CostEvent tracks token usage and cost for a single agent invocation.
type CostEvent struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`          // "cost"
	Time         time.Time              `json:"time"`
	AgentID      string                 `json:"agent_id"`
	WaveNumber   int                    `json:"wave_number"`
	IMPLSlug     string                 `json:"impl_slug"`
	ProgramSlug  string                 `json:"program_slug,omitempty"`
	Model        string                 `json:"model"`
	InputTokens  int                    `json:"input_tokens"`
	OutputTokens int                    `json:"output_tokens"`
	CostUSD      float64                `json:"cost_usd"`
	Meta         map[string]any `json:"metadata,omitempty"`
}

func (e *CostEvent) EventID() string                  { return e.ID }
func (e *CostEvent) EventType() string                { return e.Type }
func (e *CostEvent) Timestamp() time.Time             { return e.Time }
func (e *CostEvent) Metadata() map[string]any { return e.Meta }

// AgentPerformanceEvent tracks execution outcome for a single agent run.
type AgentPerformanceEvent struct {
	ID              string                 `json:"id"`
	Type            string                 `json:"type"`             // "agent_performance"
	Time            time.Time              `json:"time"`
	AgentID         string                 `json:"agent_id"`
	WaveNumber      int                    `json:"wave_number"`
	IMPLSlug        string                 `json:"impl_slug"`
	ProgramSlug     string                 `json:"program_slug,omitempty"`
	Status          string                 `json:"status"`           // "success" | "failed" | "blocked" | "partial"
	FailureType     string                 `json:"failure_type,omitempty"` // "transient" | "fixable" | "needs_replan" | "escalate"
	RetryCount      int                    `json:"retry_count"`
	DurationSeconds int                    `json:"duration_seconds"`
	FilesModified   []string               `json:"files_modified,omitempty"`
	TestsPassed     int                    `json:"tests_passed"`
	TestsFailed     int                    `json:"tests_failed"`
	Meta            map[string]any `json:"metadata,omitempty"`
}

func (e *AgentPerformanceEvent) EventID() string                  { return e.ID }
func (e *AgentPerformanceEvent) EventType() string                { return e.Type }
func (e *AgentPerformanceEvent) Timestamp() time.Time             { return e.Time }
func (e *AgentPerformanceEvent) Metadata() map[string]any { return e.Meta }

// ActivityEvent tracks high-level orchestrator actions.
type ActivityEvent struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`          // "activity"
	Time         time.Time              `json:"time"`
	ActivityType string                 `json:"activity_type"` // "scout_launch" | "wave_start" | "wave_merge" | "impl_complete"
	IMPLSlug     string                 `json:"impl_slug"`
	ProgramSlug  string                 `json:"program_slug,omitempty"`
	WaveNumber   int                    `json:"wave_number,omitempty"`
	User         string                 `json:"user,omitempty"`
	Details      string                 `json:"details,omitempty"`
	Meta         map[string]any `json:"metadata,omitempty"`
}

func (e *ActivityEvent) EventID() string                  { return e.ID }
func (e *ActivityEvent) EventType() string                { return e.Type }
func (e *ActivityEvent) Timestamp() time.Time             { return e.Time }
func (e *ActivityEvent) Metadata() map[string]any { return e.Meta }
