package observability

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TrendResult holds time-series data for a metric over a time range.
type TrendResult struct {
	Metric  string        `json:"metric"`
	Buckets []TrendBucket `json:"buckets"`
}

// TrendBucket is a single time bucket within a trend result.
type TrendBucket struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Value float64   `json:"value"`
	Count int       `json:"count"`
}

// ComputeCostRollup aggregates cost events by requested dimensions.
func ComputeCostRollup(ctx context.Context, store Store, req RollupRequest) (*RollupResult, error) {
	events, err := store.QueryEvents(ctx, rollupFilters(req, "cost"))
	if err != nil {
		return nil, fmt.Errorf("query cost events: %w", err)
	}

	groups := make(map[string]*RollupGroup)
	var totalCost float64

	for _, ev := range events {
		ce, ok := ev.(*CostEvent)
		if !ok {
			continue
		}
		key := groupKey(req.GroupBy, costDimensions(ce))
		g := getOrCreateGroup(groups, key, req.GroupBy, costDimensions(ce))
		g.Count++
		g.TotalCost += ce.CostUSD
		totalCost += ce.CostUSD
	}

	result := &RollupResult{
		Type:      "cost",
		Groups:    groupSlice(groups),
		TotalCost: totalCost,
	}
	// Compute average cost per group.
	if len(result.Groups) > 0 {
		var sum float64
		for i := range result.Groups {
			sum += result.Groups[i].TotalCost
		}
		result.AvgRate = sum / float64(len(result.Groups))
	}
	return result, nil
}

// ComputeSuccessRateRollup aggregates agent performance events and calculates
// success rates by requested dimensions.
func ComputeSuccessRateRollup(ctx context.Context, store Store, req RollupRequest) (*RollupResult, error) {
	events, err := store.QueryEvents(ctx, rollupFilters(req, "agent_performance"))
	if err != nil {
		return nil, fmt.Errorf("query performance events: %w", err)
	}

	type bucket struct {
		total   int
		success int
	}
	buckets := make(map[string]*bucket)
	groupDims := make(map[string]map[string]string)

	for _, ev := range events {
		pe, ok := ev.(*AgentPerformanceEvent)
		if !ok {
			continue
		}
		dims := perfDimensions(pe)
		key := groupKey(req.GroupBy, dims)
		b, exists := buckets[key]
		if !exists {
			b = &bucket{}
			buckets[key] = b
			groupDims[key] = pickDims(req.GroupBy, dims)
		}
		b.total++
		if pe.Status == "success" {
			b.success++
		}
	}

	result := &RollupResult{Type: "success_rate"}
	var rateSum float64
	for key, b := range buckets {
		rate := 0.0
		if b.total > 0 {
			rate = float64(b.success) / float64(b.total)
		}
		result.Groups = append(result.Groups, RollupGroup{
			Key:         groupDims[key],
			Count:       b.total,
			SuccessRate: rate,
		})
		rateSum += rate
	}
	if len(result.Groups) > 0 {
		result.AvgRate = rateSum / float64(len(result.Groups))
	}
	return result, nil
}

// ComputeRetryRollup aggregates agent performance events and calculates
// average retry counts by requested dimensions.
func ComputeRetryRollup(ctx context.Context, store Store, req RollupRequest) (*RollupResult, error) {
	events, err := store.QueryEvents(ctx, rollupFilters(req, "agent_performance"))
	if err != nil {
		return nil, fmt.Errorf("query performance events: %w", err)
	}

	type bucket struct {
		total   int
		retries int
	}
	buckets := make(map[string]*bucket)
	groupDims := make(map[string]map[string]string)

	for _, ev := range events {
		pe, ok := ev.(*AgentPerformanceEvent)
		if !ok {
			continue
		}
		dims := perfDimensions(pe)
		key := groupKey(req.GroupBy, dims)
		b, exists := buckets[key]
		if !exists {
			b = &bucket{}
			buckets[key] = b
			groupDims[key] = pickDims(req.GroupBy, dims)
		}
		b.total++
		b.retries += pe.RetryCount
	}

	result := &RollupResult{Type: "retry_count"}
	for key, b := range buckets {
		avg := 0.0
		if b.total > 0 {
			avg = float64(b.retries) / float64(b.total)
		}
		result.Groups = append(result.Groups, RollupGroup{
			Key:        groupDims[key],
			Count:      b.total,
			AvgRetries: avg,
		})
	}
	return result, nil
}

// ComputeTrend computes a time-series trend for the given metric over the
// specified time range, divided into N buckets. Supported metrics: "cost",
// "success_rate", "retry_count".
func ComputeTrend(opts ComputeTrendOpts) (*TrendResult, error) {
	ctx := opts.Ctx
	store := opts.Store
	metric := opts.Metric
	timeRange := opts.TimeRange
	buckets := opts.Buckets

	if buckets <= 0 {
		buckets = 1
	}

	now := time.Now().UTC()
	start := now.Add(-timeRange)
	bucketDur := timeRange / time.Duration(buckets)

	// Determine event type to query.
	var eventType string
	switch metric {
	case "cost":
		eventType = "cost"
	case "success_rate", "retry_count":
		eventType = "agent_performance"
	default:
		return nil, fmt.Errorf("unsupported metric: %s", metric)
	}

	events, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes: []string{eventType},
		StartTime:  &start,
		EndTime:    &now,
		Limit:      0,
	})
	if err != nil {
		return nil, fmt.Errorf("query events for trend: %w", err)
	}

	result := &TrendResult{Metric: metric, Buckets: make([]TrendBucket, buckets)}

	// Initialize buckets.
	for i := 0; i < buckets; i++ {
		bStart := start.Add(bucketDur * time.Duration(i))
		bEnd := bStart.Add(bucketDur)
		if i == buckets-1 {
			bEnd = now // last bucket extends to now
		}
		result.Buckets[i] = TrendBucket{Start: bStart, End: bEnd}
	}

	// Assign events to buckets.
	type perfAccum struct {
		total   int
		success int
		retries int
	}
	perfBuckets := make([]perfAccum, buckets)

	for _, ev := range events {
		ts := ev.Timestamp()
		idx := int(ts.Sub(start) / bucketDur)
		if idx < 0 {
			idx = 0
		}
		if idx >= buckets {
			idx = buckets - 1
		}

		switch metric {
		case "cost":
			if ce, ok := ev.(*CostEvent); ok {
				result.Buckets[idx].Value += ce.CostUSD
				result.Buckets[idx].Count++
			}
		case "success_rate":
			if pe, ok := ev.(*AgentPerformanceEvent); ok {
				perfBuckets[idx].total++
				result.Buckets[idx].Count++
				if pe.Status == "success" {
					perfBuckets[idx].success++
				}
			}
		case "retry_count":
			if pe, ok := ev.(*AgentPerformanceEvent); ok {
				perfBuckets[idx].total++
				perfBuckets[idx].retries += pe.RetryCount
				result.Buckets[idx].Count++
			}
		}
	}

	// Finalize performance buckets.
	if metric == "success_rate" {
		for i, pb := range perfBuckets {
			if pb.total > 0 {
				result.Buckets[i].Value = float64(pb.success) / float64(pb.total)
			}
		}
	} else if metric == "retry_count" {
		for i, pb := range perfBuckets {
			if pb.total > 0 {
				result.Buckets[i].Value = float64(pb.retries) / float64(pb.total)
			}
		}
	}

	return result, nil
}

// --- helpers ---

func rollupFilters(req RollupRequest, eventType string) QueryFilters {
	f := QueryFilters{
		EventTypes: []string{eventType},
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		Limit:      0, // no limit for aggregation
	}
	if req.IMPLSlug != "" {
		f.IMPLSlugs = []string{req.IMPLSlug}
	}
	if req.ProgramSlug != "" {
		f.ProgramSlugs = []string{req.ProgramSlug}
	}
	return f
}

func costDimensions(ce *CostEvent) map[string]string {
	return map[string]string{
		"agent":   ce.AgentID,
		"wave":    fmt.Sprintf("%d", ce.WaveNumber),
		"impl":    ce.IMPLSlug,
		"program": ce.ProgramSlug,
		"model":   ce.Model,
	}
}

func perfDimensions(pe *AgentPerformanceEvent) map[string]string {
	return map[string]string{
		"agent":   pe.AgentID,
		"wave":    fmt.Sprintf("%d", pe.WaveNumber),
		"impl":    pe.IMPLSlug,
		"program": pe.ProgramSlug,
	}
}

func groupKey(groupBy []string, dims map[string]string) string {
	parts := make([]string, len(groupBy))
	for i, dim := range groupBy {
		parts[i] = dims[dim]
	}
	return strings.Join(parts, "\x00")
}

func pickDims(groupBy []string, dims map[string]string) map[string]string {
	m := make(map[string]string, len(groupBy))
	for _, dim := range groupBy {
		m[dim] = dims[dim]
	}
	return m
}

func getOrCreateGroup(groups map[string]*RollupGroup, key string, groupBy []string, dims map[string]string) *RollupGroup {
	g, ok := groups[key]
	if !ok {
		g = &RollupGroup{Key: pickDims(groupBy, dims)}
		groups[key] = g
	}
	return g
}

func groupSlice(groups map[string]*RollupGroup) []RollupGroup {
	s := make([]RollupGroup, 0, len(groups))
	for _, g := range groups {
		s = append(s, *g)
	}
	return s
}
