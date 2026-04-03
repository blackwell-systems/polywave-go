package observability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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
func ComputeCostRollup(ctx context.Context, store Store, req RollupRequest) result.Result[RollupResult] {
	events, err := store.QueryEvents(ctx, rollupFilters(req, "cost"))
	if err != nil {
		return result.NewFailure[RollupResult]([]result.SAWError{
			result.NewFatal("O002_OBS_QUERY_FAILED", "query cost events: "+err.Error()).WithCause(err),
		})
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

	rollup := &RollupResult{
		Type:      "cost",
		Groups:    groupSlice(groups),
		TotalCost: totalCost,
	}
	// Compute average cost per group.
	if len(rollup.Groups) > 0 {
		var sum float64
		for i := range rollup.Groups {
			sum += rollup.Groups[i].TotalCost
		}
		rollup.AvgRate = sum / float64(len(rollup.Groups))
	}
	return result.NewSuccess(*rollup)
}

// ComputeSuccessRateRollup aggregates agent performance events and calculates
// success rates by requested dimensions.
func ComputeSuccessRateRollup(ctx context.Context, store Store, req RollupRequest) result.Result[RollupResult] {
	events, err := store.QueryEvents(ctx, rollupFilters(req, "agent_performance"))
	if err != nil {
		return result.NewFailure[RollupResult]([]result.SAWError{
			result.NewFatal("O002_OBS_QUERY_FAILED", "query performance events: "+err.Error()).WithCause(err),
		})
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

	rollup := &RollupResult{Type: "success_rate"}
	var rateSum float64
	for key, b := range buckets {
		rate := 0.0
		if b.total > 0 {
			rate = float64(b.success) / float64(b.total)
		}
		rollup.Groups = append(rollup.Groups, RollupGroup{
			Key:         groupDims[key],
			Count:       b.total,
			SuccessRate: rate,
		})
		rateSum += rate
	}
	if len(rollup.Groups) > 0 {
		rollup.AvgRate = rateSum / float64(len(rollup.Groups))
	}
	return result.NewSuccess(*rollup)
}

// ComputeRetryRollup aggregates agent performance events and calculates
// average retry counts by requested dimensions.
func ComputeRetryRollup(ctx context.Context, store Store, req RollupRequest) result.Result[RollupResult] {
	events, err := store.QueryEvents(ctx, rollupFilters(req, "agent_performance"))
	if err != nil {
		return result.NewFailure[RollupResult]([]result.SAWError{
			result.NewFatal("O002_OBS_QUERY_FAILED", "query performance events: "+err.Error()).WithCause(err),
		})
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

	rollup := &RollupResult{Type: "retry_count"}
	for key, b := range buckets {
		avg := 0.0
		if b.total > 0 {
			avg = float64(b.retries) / float64(b.total)
		}
		rollup.Groups = append(rollup.Groups, RollupGroup{
			Key:        groupDims[key],
			Count:      b.total,
			AvgRetries: avg,
		})
	}
	return result.NewSuccess(*rollup)
}

// ComputeTrend computes a time-series trend for the given metric over the
// specified time range, divided into N buckets. Supported metrics: "cost",
// "success_rate", "retry_count".
func ComputeTrend(ctx context.Context, opts ComputeTrendOpts) result.Result[TrendResult] {
	store := opts.Store
	metric := opts.Metric
	timeRange := opts.TimeRange
	numBuckets := opts.Buckets

	if numBuckets <= 0 {
		numBuckets = 1
	}

	now := time.Now().UTC()
	start := now.Add(-timeRange)
	bucketDur := timeRange / time.Duration(numBuckets)

	// Determine event type to query.
	var eventType string
	switch metric {
	case "cost":
		eventType = "cost"
	case "success_rate", "retry_count":
		eventType = "agent_performance"
	default:
		return result.NewFailure[TrendResult]([]result.SAWError{
			result.NewFatal("O002_OBS_QUERY_FAILED", "unsupported metric: "+metric),
		})
	}

	events, err := store.QueryEvents(ctx, QueryFilters{
		EventTypes: []string{eventType},
		StartTime:  &start,
		EndTime:    &now,
		Limit:      0,
	})
	if err != nil {
		return result.NewFailure[TrendResult]([]result.SAWError{
			result.NewFatal("O002_OBS_QUERY_FAILED", "query events for trend: "+err.Error()).WithCause(err),
		})
	}

	trend := &TrendResult{Metric: metric, Buckets: make([]TrendBucket, numBuckets)}

	// Initialize buckets.
	for i := 0; i < numBuckets; i++ {
		bStart := start.Add(bucketDur * time.Duration(i))
		bEnd := bStart.Add(bucketDur)
		if i == numBuckets-1 {
			bEnd = now // last bucket extends to now
		}
		trend.Buckets[i] = TrendBucket{Start: bStart, End: bEnd}
	}

	// Assign events to buckets.
	type perfAccum struct {
		total   int
		success int
		retries int
	}
	perfBuckets := make([]perfAccum, numBuckets)

	for _, ev := range events {
		ts := ev.Timestamp()
		idx := int(ts.Sub(start) / bucketDur)
		if idx < 0 {
			idx = 0
		}
		if idx >= numBuckets {
			idx = numBuckets - 1
		}

		switch metric {
		case "cost":
			if ce, ok := ev.(*CostEvent); ok {
				trend.Buckets[idx].Value += ce.CostUSD
				trend.Buckets[idx].Count++
			}
		case "success_rate":
			if pe, ok := ev.(*AgentPerformanceEvent); ok {
				perfBuckets[idx].total++
				trend.Buckets[idx].Count++
				if pe.Status == "success" {
					perfBuckets[idx].success++
				}
			}
		case "retry_count":
			if pe, ok := ev.(*AgentPerformanceEvent); ok {
				perfBuckets[idx].total++
				perfBuckets[idx].retries += pe.RetryCount
				trend.Buckets[idx].Count++
			}
		}
	}

	// Finalize performance buckets.
	if metric == "success_rate" {
		for i, pb := range perfBuckets {
			if pb.total > 0 {
				trend.Buckets[i].Value = float64(pb.success) / float64(pb.total)
			}
		}
	} else if metric == "retry_count" {
		for i, pb := range perfBuckets {
			if pb.total > 0 {
				trend.Buckets[i].Value = float64(pb.retries) / float64(pb.total)
			}
		}
	}

	return result.NewSuccess(*trend)
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
