// Package observability provides event types and storage abstractions for
// tracking cost, agent performance, and orchestrator activity in the
// Scout-and-Wave protocol.
//
// # Architecture
//
// The package defines a Store interface that abstracts event persistence.
// Concrete implementations (SQLite, PostgreSQL) live in separate packages
// and are selected at runtime based on configuration.
//
//	┌──────────────┐
//	│  Orchestrator │──RecordEvent──▶ Store interface
//	└──────────────┘                     │
//	                          ┌──────────┼──────────┐
//	                          ▼          ▼          ▼
//	                       SQLite    PostgreSQL   (future)
//
// # Event Types
//
// Three event types capture different observability signals:
//
//   - CostEvent: Token usage and cost per agent invocation
//   - AgentPerformanceEvent: Execution outcome, duration, test results
//   - ActivityEvent: High-level orchestrator actions (scout launch, wave start, merge)
//
// All event types implement the Event interface.
//
// # Usage
//
// Record events through the Store interface:
//
//	store, err := sqlite.Open("observability.db")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer store.Close()
//
//	evt := &observability.CostEvent{
//	    ID:           observability.NewEventID(),
//	    Type:         "cost",
//	    Time:         time.Now().UTC(),
//	    AgentID:      "A",
//	    WaveNumber:   1,
//	    IMPLSlug:     "add-auth",
//	    Model:        "claude-sonnet-4-6",
//	    InputTokens:  1500,
//	    OutputTokens: 800,
//	    CostUSD:      0.012,
//	}
//
//	if err := store.RecordEvent(ctx, evt); err != nil {
//	    log.Fatal(err)
//	}
//
// Query events with filters:
//
//	events, err := store.QueryEvents(ctx, observability.QueryFilters{
//	    EventTypes: []string{"cost"},
//	    IMPLSlugs:  []string{"add-auth"},
//	    Limit:      50,
//	})
package observability
