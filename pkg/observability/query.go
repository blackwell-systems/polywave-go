package observability

import (
	"context"
	"sort"
)

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
func GetAgentHistory(ctx context.Context, store Store, agentID string, limit int) ([]AgentPerformanceEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	filters := QueryFilters{
		EventTypes: []string{"agent_performance"},
		AgentIDs:   []string{agentID},
		Limit:      limit,
	}
	events, err := store.QueryEvents(ctx, filters)
	if err != nil {
		return nil, err
	}

	result := make([]AgentPerformanceEvent, 0, len(events))
	for _, e := range events {
		if ap, ok := e.(*AgentPerformanceEvent); ok {
			result = append(result, *ap)
		}
	}

	// Sort descending by timestamp.
	sort.Slice(result, func(i, j int) bool {
		return result[i].Time.After(result[j].Time)
	})

	return result, nil
}

// GetIMPLMetrics aggregates all events for an IMPL into summary metrics.
// Returns zero-value metrics (not an error) if no data exists.
func GetIMPLMetrics(ctx context.Context, store Store, implSlug string) (*IMPLMetrics, error) {
	metrics := &IMPLMetrics{}

	// Get cost events for total cost.
	costEvents, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes: []string{"cost"},
		IMPLSlugs:  []string{implSlug},
		Limit:      10000,
	})
	if err != nil {
		return nil, err
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
		return nil, err
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
		return nil, err
	}
	for _, e := range actEvents {
		if ae, ok := e.(*ActivityEvent); ok {
			if ae.ActivityType == "wave_merge" {
				metrics.WavesCompleted++
			}
		}
	}

	return metrics, nil
}

// GetProgramSummary aggregates metrics across all IMPLs in a program.
// Returns zero-value summary (not an error) if no data exists.
func GetProgramSummary(ctx context.Context, store Store, programSlug string) (*ProgramSummary, error) {
	summary := &ProgramSummary{}

	// Get cost events.
	costEvents, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes:   []string{"cost"},
		ProgramSlugs: []string{programSlug},
		Limit:        10000,
	})
	if err != nil {
		return nil, err
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
		return nil, err
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

	return summary, nil
}

// GetCostBreakdown returns a per-agent cost breakdown for an IMPL.
func GetCostBreakdown(ctx context.Context, store Store, implSlug string) (map[string]float64, error) {
	events, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes: []string{"cost"},
		IMPLSlugs:  []string{implSlug},
		Limit:      10000,
	})
	if err != nil {
		return nil, err
	}

	breakdown := make(map[string]float64)
	for _, e := range events {
		if ce, ok := e.(*CostEvent); ok {
			breakdown[ce.AgentID] += ce.CostUSD
		}
	}
	return breakdown, nil
}

// GetFailurePatterns identifies common failure types across agents.
// Results are sorted by count descending.
func GetFailurePatterns(ctx context.Context, store Store, filters QueryFilters) ([]FailurePattern, error) {
	filters.EventTypes = []string{"agent_performance"}
	if filters.Limit == 0 {
		filters.Limit = 10000
	}

	events, err := store.QueryEvents(ctx, filters)
	if err != nil {
		return nil, err
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

	result := make([]FailurePattern, 0, len(patterns))
	for ft, pd := range patterns {
		agents := make([]string, 0, len(pd.agents))
		for a := range pd.agents {
			agents = append(agents, a)
		}
		sort.Strings(agents)
		result = append(result, FailurePattern{
			FailureType:    ft,
			Count:          pd.count,
			AffectedAgents: agents,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result, nil
}
