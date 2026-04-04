package observability

import (
	"context"
	"sort"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// QueryData holds the result data returned by a Query operation.
type QueryData struct {
	Events []Event
	Count  int
}

// AgentHistoryData holds the result of GetAgentHistory.
type AgentHistoryData struct {
	Events []AgentPerformanceEvent
}

// CostBreakdownData holds the result of GetCostBreakdown.
type CostBreakdownData struct {
	PerAgent map[string]float64
}

// FailurePatternsData holds the result of GetFailurePatterns.
type FailurePatternsData struct {
	Patterns []FailurePattern
}

// Query retrieves events matching the given filters and returns a result.Result[QueryData].
// Returns a successful Result with an empty slice (not an error) when no events match.
func Query(ctx context.Context, store Store, filters QueryFilters) result.Result[QueryData] {
	events, err := store.QueryEvents(ctx, filters)
	if err != nil {
		return result.NewFailure[QueryData]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}
	if events == nil {
		events = []Event{}
	}
	return result.NewSuccess(QueryData{
		Events: events,
		Count:  len(events),
	})
}

// IMPLMetrics holds aggregated metrics for a single IMPL.
type IMPLMetrics struct {
	TotalCost        float64 `json:"total_cost"`
	TotalDurationMin float64 `json:"total_duration_min"`
	SuccessRate      float64 `json:"success_rate"`
	AgentsFailed     int     `json:"agents_failed"`
	WavesCompleted   int     `json:"waves_completed"`
}

// ProgramSummary holds aggregated metrics across all IMPLs in a program.
type ProgramSummary struct {
	TotalCost          float64 `json:"total_cost"`
	IMPLCount          int     `json:"impl_count"`
	TotalDurationMin   float64 `json:"total_duration_min"`
	OverallSuccessRate float64 `json:"overall_success_rate"`
}

// FailurePattern groups a common failure type with its count and affected agents.
type FailurePattern struct {
	FailureType    string   `json:"failure_type"`
	Count          int      `json:"count"`
	AffectedAgents []string `json:"affected_agents"`
}

// GetAgentHistory retrieves recent performance events for a specific agent,
// ordered by timestamp descending.
func GetAgentHistory(ctx context.Context, store Store, agentID string, limit int) result.Result[AgentHistoryData] {
	if limit <= 0 {
		limit = 100
	}
	filters := QueryFilters{
		EventTypes: []string{"agent_performance"},
		AgentIDs:   []string{agentID},
		Limit:      limit,
	}
	rawEvents, err := store.QueryEvents(ctx, filters)
	if err != nil {
		return result.NewFailure[AgentHistoryData]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}

	events := make([]AgentPerformanceEvent, 0, len(rawEvents))
	for _, e := range rawEvents {
		if ap, ok := e.(*AgentPerformanceEvent); ok {
			events = append(events, *ap)
		}
	}

	// Sort descending by timestamp.
	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.After(events[j].Time)
	})

	return result.NewSuccess(AgentHistoryData{Events: events})
}

// GetIMPLMetrics aggregates all events for an IMPL into summary metrics.
// Returns zero-value metrics (not an error) if no data exists.
func GetIMPLMetrics(ctx context.Context, store Store, implSlug string) result.Result[IMPLMetrics] {
	metrics := &IMPLMetrics{}

	// Get cost events for total cost.
	costEvents, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes: []string{"cost"},
		IMPLSlugs:  []string{implSlug},
		Limit:      10000,
	})
	if err != nil {
		return result.NewFailure[IMPLMetrics]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}
	for _, e := range costEvents {
		if ce, ok := e.(*CostEvent); ok {
			metrics.TotalCost += ce.CostUSD
		}
	}

	// Get performance events for duration, success rate, failures.
	perfEvents, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes: []string{"agent_performance"},
		IMPLSlugs:  []string{implSlug},
		Limit:      10000,
	})
	if err != nil {
		return result.NewFailure[IMPLMetrics]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}
	var successCount int
	for _, e := range perfEvents {
		if ap, ok := e.(*AgentPerformanceEvent); ok {
			metrics.TotalDurationMin += float64(ap.DurationSeconds) / 60.0
			if ap.Status == "success" {
				successCount++
			} else if ap.Status == "failed" {
				metrics.AgentsFailed++
			}
		}
	}
	if len(perfEvents) > 0 {
		metrics.SuccessRate = float64(successCount) / float64(len(perfEvents))
	}

	// Get activity events for waves completed.
	actEvents, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes: []string{"activity"},
		IMPLSlugs:  []string{implSlug},
		Limit:      10000,
	})
	if err != nil {
		return result.NewFailure[IMPLMetrics]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}
	for _, e := range actEvents {
		if ae, ok := e.(*ActivityEvent); ok {
			if ae.ActivityType == "wave_merge" {
				metrics.WavesCompleted++
			}
		}
	}

	return result.NewSuccess(*metrics)
}

// GetProgramSummary aggregates metrics across all IMPLs in a program.
// Returns zero-value summary (not an error) if no data exists.
func GetProgramSummary(ctx context.Context, store Store, programSlug string) result.Result[ProgramSummary] {
	summary := &ProgramSummary{}

	// Get cost events.
	costEvents, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes:   []string{"cost"},
		ProgramSlugs: []string{programSlug},
		Limit:        10000,
	})
	if err != nil {
		return result.NewFailure[ProgramSummary]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}
	for _, e := range costEvents {
		if ce, ok := e.(*CostEvent); ok {
			summary.TotalCost += ce.CostUSD
		}
	}

	// Get performance events.
	perfEvents, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes:   []string{"agent_performance"},
		ProgramSlugs: []string{programSlug},
		Limit:        10000,
	})
	if err != nil {
		return result.NewFailure[ProgramSummary]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}
	var successCount int
	implSet := make(map[string]struct{})
	for _, e := range perfEvents {
		if ap, ok := e.(*AgentPerformanceEvent); ok {
			summary.TotalDurationMin += float64(ap.DurationSeconds) / 60.0
			if ap.Status == "success" {
				successCount++
			}
			if ap.IMPLSlug != "" {
				implSet[ap.IMPLSlug] = struct{}{}
			}
		}
	}
	summary.IMPLCount = len(implSet)
	if len(perfEvents) > 0 {
		summary.OverallSuccessRate = float64(successCount) / float64(len(perfEvents))
	}

	return result.NewSuccess(*summary)
}

// GetCostBreakdown returns a per-agent cost breakdown for an IMPL.
func GetCostBreakdown(ctx context.Context, store Store, implSlug string) result.Result[CostBreakdownData] {
	events, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes: []string{"cost"},
		IMPLSlugs:  []string{implSlug},
		Limit:      10000,
	})
	if err != nil {
		return result.NewFailure[CostBreakdownData]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}

	breakdown := make(map[string]float64)
	for _, e := range events {
		if ce, ok := e.(*CostEvent); ok {
			breakdown[ce.AgentID] += ce.CostUSD
		}
	}
	return result.NewSuccess(CostBreakdownData{PerAgent: breakdown})
}

// GetFailurePatterns identifies common failure types across agents.
// Results are sorted by count descending.
func GetFailurePatterns(ctx context.Context, store Store, filters QueryFilters) result.Result[FailurePatternsData] {
	filters.EventTypes = []string{"agent_performance"}
	if filters.Limit == 0 {
		filters.Limit = 10000
	}

	events, err := store.QueryEvents(ctx, filters)
	if err != nil {
		return result.NewFailure[FailurePatternsData]([]result.SAWError{
			result.NewFatal(result.CodeObsQueryFailed, err.Error()).WithCause(err),
		})
	}

	type patternData struct {
		count  int
		agents map[string]struct{}
	}
	patterns := make(map[string]*patternData)

	for _, e := range events {
		ap, ok := e.(*AgentPerformanceEvent)
		if !ok || ap.FailureType == "" {
			continue
		}
		pd, exists := patterns[ap.FailureType]
		if !exists {
			pd = &patternData{agents: make(map[string]struct{})}
			patterns[ap.FailureType] = pd
		}
		pd.count++
		pd.agents[ap.AgentID] = struct{}{}
	}

	failurePatterns := make([]FailurePattern, 0, len(patterns))
	for ft, pd := range patterns {
		agents := make([]string, 0, len(pd.agents))
		for a := range pd.agents {
			agents = append(agents, a)
		}
		sort.Strings(agents)
		failurePatterns = append(failurePatterns, FailurePattern{
			FailureType:    ft,
			Count:          pd.count,
			AffectedAgents: agents,
		})
	}

	sort.Slice(failurePatterns, func(i, j int) bool {
		return failurePatterns[i].Count > failurePatterns[j].Count
	})

	return result.NewSuccess(FailurePatternsData{Patterns: failurePatterns})
}
