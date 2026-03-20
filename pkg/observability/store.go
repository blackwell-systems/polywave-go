package observability

import (
	"context"
	"time"
)

// Store defines the storage abstraction for observability events.
// Implementations exist for SQLite (local) and PostgreSQL (shared).
type Store interface {
	// RecordEvent writes a new event to storage.
	RecordEvent(ctx context.Context, event Event) error

	// QueryEvents retrieves events matching the given filters.
	QueryEvents(ctx context.Context, filters QueryFilters) ([]Event, error)

	// GetRollup computes aggregated metrics over stored events.
	GetRollup(ctx context.Context, rollup RollupRequest) (*RollupResult, error)

	// Close releases any held resources (connections, file handles).
	Close() error
}

// QueryFilters specifies criteria for retrieving events.
type QueryFilters struct {
	EventTypes   []string   `json:"event_types,omitempty"`
	IMPLSlugs    []string   `json:"impl_slugs,omitempty"`
	ProgramSlugs []string   `json:"program_slugs,omitempty"`
	AgentIDs     []string   `json:"agent_ids,omitempty"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Limit        int        `json:"limit,omitempty"`
	Offset       int        `json:"offset,omitempty"`
}

// RollupRequest describes an aggregation query.
type RollupRequest struct {
	Type        string     `json:"type"`                   // "cost" | "success_rate" | "retry_count"
	GroupBy     []string   `json:"group_by,omitempty"`     // "agent" | "wave" | "impl" | "program" | "model"
	IMPLSlug    string     `json:"impl_slug,omitempty"`
	ProgramSlug string     `json:"program_slug,omitempty"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
}

// RollupResult holds aggregated metrics.
type RollupResult struct {
	Type      string        `json:"type"`
	Groups    []RollupGroup `json:"groups"`
	TotalCost float64       `json:"total_cost,omitempty"`
	AvgRate   float64       `json:"avg_rate,omitempty"`
}

// RollupGroup is a single bucket within a rollup result.
type RollupGroup struct {
	Key         map[string]string `json:"key"`
	Count       int               `json:"count"`
	TotalCost   float64           `json:"total_cost,omitempty"`
	SuccessRate float64           `json:"success_rate,omitempty"`
	AvgRetries  float64           `json:"avg_retries,omitempty"`
}
