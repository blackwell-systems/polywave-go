// Package sqlite provides a SQLite-backed implementation of observability.Store.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	obs "github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS events (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	time TEXT NOT NULL,
	data TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_time ON events(time);
`

// store implements observability.Store backed by SQLite.
type store struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at path and returns an
// observability.Store. The database schema is created automatically
// if it does not exist. Uses modernc.org/sqlite (pure Go, no CGo).
func Open(path string) (obs.Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite open %s: %w", path, err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite pragma: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite schema: %w", err)
	}

	return &store{db: db}, nil
}

// RecordEvent writes a new event to storage.
func (s *store) RecordEvent(ctx context.Context, event obs.Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO events (id, type, time, data) VALUES (?, ?, ?, ?)`,
		event.EventID(),
		event.EventType(),
		event.Timestamp().UTC().Format(time.RFC3339Nano),
		string(data),
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// QueryEvents retrieves events matching the given filters.
func (s *store) QueryEvents(ctx context.Context, filters obs.QueryFilters) ([]obs.Event, error) {
	var where []string
	var args []any

	if len(filters.EventTypes) > 0 {
		placeholders := make([]string, len(filters.EventTypes))
		for i, t := range filters.EventTypes {
			placeholders[i] = "?"
			args = append(args, t)
		}
		where = append(where, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ",")))
	}

	if filters.StartTime != nil {
		where = append(where, "time >= ?")
		args = append(args, filters.StartTime.UTC().Format(time.RFC3339Nano))
	}
	if filters.EndTime != nil {
		where = append(where, "time <= ?")
		args = append(args, filters.EndTime.UTC().Format(time.RFC3339Nano))
	}

	query := "SELECT type, data FROM events"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY time ASC"

	if filters.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filters.Limit)
		if filters.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", filters.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []obs.Event
	for rows.Next() {
		var eventType, data string
		if err := rows.Scan(&eventType, &data); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		ev, err := unmarshalEvent(eventType, []byte(data))
		if err != nil {
			return nil, fmt.Errorf("unmarshal event: %w", err)
		}

		// Apply in-Go filters for fields stored inside the JSON data.
		if !matchesJSONFilters(ev, filters) {
			continue
		}

		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return events, nil
}

// GetRollup computes aggregated metrics over stored events.
// Delegates to the rollup functions in pkg/observability and unwraps their
// result.Result[obs.RollupResult] return values to satisfy the Store interface.
func (s *store) GetRollup(ctx context.Context, req obs.RollupRequest) (*obs.RollupResult, error) {
	var r result.Result[obs.RollupResult]
	switch req.Type {
	case "cost":
		r = obs.ComputeCostRollup(ctx, s, req)
	case "success_rate":
		r = obs.ComputeSuccessRateRollup(ctx, s, req)
	case "retry_count":
		r = obs.ComputeRetryRollup(ctx, s, req)
	default:
		return nil, fmt.Errorf("unsupported rollup type: %s", req.Type)
	}
	if !r.IsSuccess() {
		return nil, fmt.Errorf("%s", r.Errors[0].Message)
	}
	data := r.GetData()
	return &data, nil
}

// Close releases the database connection.
func (s *store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// unmarshalEvent deserializes JSON data into the correct concrete Event type.
func unmarshalEvent(eventType string, data []byte) (obs.Event, error) {
	switch eventType {
	case "cost":
		var ev obs.CostEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, err
		}
		return &ev, nil
	case "agent_performance":
		var ev obs.AgentPerformanceEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, err
		}
		return &ev, nil
	case "activity":
		var ev obs.ActivityEvent
		if err := json.Unmarshal(data, &ev); err != nil {
			return nil, err
		}
		return &ev, nil
	default:
		return nil, fmt.Errorf("unknown event type: %s", eventType)
	}
}

// matchesJSONFilters checks whether an event matches the filter criteria
// that are stored inside the JSON data (impl slugs, program slugs, agent IDs).
func matchesJSONFilters(ev obs.Event, f obs.QueryFilters) bool {
	if len(f.IMPLSlugs) > 0 {
		slug := eventIMPLSlug(ev)
		if !contains(f.IMPLSlugs, slug) {
			return false
		}
	}
	if len(f.ProgramSlugs) > 0 {
		slug := eventProgramSlug(ev)
		if !contains(f.ProgramSlugs, slug) {
			return false
		}
	}
	if len(f.AgentIDs) > 0 {
		aid := eventAgentID(ev)
		if !contains(f.AgentIDs, aid) {
			return false
		}
	}
	return true
}

func eventIMPLSlug(ev obs.Event) string {
	switch e := ev.(type) {
	case *obs.CostEvent:
		return e.IMPLSlug
	case *obs.AgentPerformanceEvent:
		return e.IMPLSlug
	case *obs.ActivityEvent:
		return e.IMPLSlug
	}
	return ""
}

func eventProgramSlug(ev obs.Event) string {
	switch e := ev.(type) {
	case *obs.CostEvent:
		return e.ProgramSlug
	case *obs.AgentPerformanceEvent:
		return e.ProgramSlug
	case *obs.ActivityEvent:
		return e.ProgramSlug
	}
	return ""
}

func eventAgentID(ev obs.Event) string {
	switch e := ev.(type) {
	case *obs.CostEvent:
		return e.AgentID
	case *obs.AgentPerformanceEvent:
		return e.AgentID
	}
	return ""
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
