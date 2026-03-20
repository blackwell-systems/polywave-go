package observability

import (
	"context"
	"testing"
	"time"
)

// mockStore is a simple in-memory Store for testing rollups.
type mockStore struct {
	events []Event
}

func (m *mockStore) RecordEvent(_ context.Context, event Event) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockStore) QueryEvents(_ context.Context, filters QueryFilters) ([]Event, error) {
	var result []Event
	for _, ev := range m.events {
		if !matchFilters(ev, filters) {
			continue
		}
		result = append(result, ev)
	}
	if filters.Limit > 0 && len(result) > filters.Limit {
		result = result[:filters.Limit]
	}
	return result, nil
}

func (m *mockStore) GetRollup(_ context.Context, _ RollupRequest) (*RollupResult, error) {
	return nil, nil
}

func (m *mockStore) Close() error { return nil }

func matchFilters(ev Event, f QueryFilters) bool {
	if len(f.EventTypes) > 0 {
		found := false
		for _, t := range f.EventTypes {
			if ev.EventType() == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if f.StartTime != nil && ev.Timestamp().Before(*f.StartTime) {
		return false
	}
	if f.EndTime != nil && ev.Timestamp().After(*f.EndTime) {
		return false
	}
	if len(f.IMPLSlugs) > 0 {
		slug := implSlugOf(ev)
		found := false
		for _, s := range f.IMPLSlugs {
			if slug == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func implSlugOf(ev Event) string {
	switch e := ev.(type) {
	case *CostEvent:
		return e.IMPLSlug
	case *AgentPerformanceEvent:
		return e.IMPLSlug
	case *ActivityEvent:
		return e.IMPLSlug
	}
	return ""
}

func TestRollupCost(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// Insert 10 cost events across 3 agents.
	agents := []string{"A", "B", "C"}
	costs := []float64{0.10, 0.20, 0.30, 0.15, 0.25, 0.35, 0.12, 0.22, 0.32, 0.18}
	for i, cost := range costs {
		agent := agents[i%3]
		_ = store.RecordEvent(ctx, &CostEvent{
			ID:      NewEventID(),
			Type:    "cost",
			Time:    base.Add(time.Duration(i) * time.Hour),
			AgentID: agent,
			IMPLSlug: "test-impl",
			Model:   "opus",
			CostUSD: cost,
		})
	}

	req := RollupRequest{
		Type:    "cost",
		GroupBy: []string{"agent"},
	}
	result, err := ComputeCostRollup(ctx, store, req)
	if err != nil {
		t.Fatalf("ComputeCostRollup: %v", err)
	}

	if result.Type != "cost" {
		t.Errorf("expected type cost, got %s", result.Type)
	}
	if len(result.Groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(result.Groups))
	}

	// Verify total cost.
	var expectedTotal float64
	for _, c := range costs {
		expectedTotal += c
	}
	if diff := result.TotalCost - expectedTotal; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected total cost %.4f, got %.4f", expectedTotal, result.TotalCost)
	}

	// Verify each group has the right agent key.
	for _, g := range result.Groups {
		if _, ok := g.Key["agent"]; !ok {
			t.Errorf("group missing agent key: %v", g.Key)
		}
	}
}

func TestRollupCostEmpty(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}

	result, err := ComputeCostRollup(ctx, store, RollupRequest{Type: "cost", GroupBy: []string{"agent"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalCost != 0 {
		t.Errorf("expected 0 total cost, got %f", result.TotalCost)
	}
	if len(result.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(result.Groups))
	}
}

func TestRollupSuccessRate(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// Agent A: 3 success, 1 failure = 75%
	// Agent B: 1 success, 1 failure = 50%
	statuses := []struct {
		agent  string
		status string
	}{
		{"A", "success"},
		{"A", "success"},
		{"A", "success"},
		{"A", "failed"},
		{"B", "success"},
		{"B", "failed"},
	}
	for i, s := range statuses {
		_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
			ID:       NewEventID(),
			Type:     "agent_performance",
			Time:     base.Add(time.Duration(i) * time.Hour),
			AgentID:  s.agent,
			IMPLSlug: "test-impl",
			Status:   s.status,
		})
	}

	result, err := ComputeSuccessRateRollup(ctx, store, RollupRequest{
		Type:    "success_rate",
		GroupBy: []string{"agent"},
	})
	if err != nil {
		t.Fatalf("ComputeSuccessRateRollup: %v", err)
	}

	if len(result.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(result.Groups))
	}

	for _, g := range result.Groups {
		switch g.Key["agent"] {
		case "A":
			if g.SuccessRate != 0.75 {
				t.Errorf("agent A: expected 0.75 success rate, got %f", g.SuccessRate)
			}
		case "B":
			if g.SuccessRate != 0.5 {
				t.Errorf("agent B: expected 0.5 success rate, got %f", g.SuccessRate)
			}
		}
	}

	// Average: (0.75 + 0.5) / 2 = 0.625
	if diff := result.AvgRate - 0.625; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected avg rate 0.625, got %f", result.AvgRate)
	}
}

func TestRollupRetry(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// Agent A: retries 0, 1, 5 → avg 2.0
	// Agent B: retries 3, 4 → avg 3.5 (high retry, >2)
	retries := []struct {
		agent string
		count int
	}{
		{"A", 0},
		{"A", 1},
		{"A", 5},
		{"B", 3},
		{"B", 4},
	}
	for i, r := range retries {
		_ = store.RecordEvent(ctx, &AgentPerformanceEvent{
			ID:         NewEventID(),
			Type:       "agent_performance",
			Time:       base.Add(time.Duration(i) * time.Hour),
			AgentID:    r.agent,
			IMPLSlug:   "test-impl",
			Status:     "success",
			RetryCount: r.count,
		})
	}

	result, err := ComputeRetryRollup(ctx, store, RollupRequest{
		Type:    "retry_count",
		GroupBy: []string{"agent"},
	})
	if err != nil {
		t.Fatalf("ComputeRetryRollup: %v", err)
	}

	if len(result.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(result.Groups))
	}

	for _, g := range result.Groups {
		switch g.Key["agent"] {
		case "A":
			if g.AvgRetries != 2.0 {
				t.Errorf("agent A: expected 2.0 avg retries, got %f", g.AvgRetries)
			}
		case "B":
			if g.AvgRetries != 3.5 {
				t.Errorf("agent B: expected 3.5 avg retries, got %f", g.AvgRetries)
			}
			if g.AvgRetries <= 2.0 {
				t.Errorf("agent B should be flagged as high retry rate")
			}
		}
	}
}

func TestRollupTrend(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}
	now := time.Now().UTC()

	// Insert cost events over 7 days.
	for day := 0; day < 7; day++ {
		ts := now.Add(-time.Duration(7-day) * 24 * time.Hour).Add(time.Hour) // offset 1hr into each day
		_ = store.RecordEvent(ctx, &CostEvent{
			ID:       NewEventID(),
			Type:     "cost",
			Time:     ts,
			AgentID:  "A",
			IMPLSlug: "test-impl",
			Model:    "opus",
			CostUSD:  float64(day+1) * 0.10,
		})
	}

	result, err := ComputeTrend(ctx, store, "cost", 7*24*time.Hour, 7)
	if err != nil {
		t.Fatalf("ComputeTrend: %v", err)
	}

	if result.Metric != "cost" {
		t.Errorf("expected metric cost, got %s", result.Metric)
	}
	if len(result.Buckets) != 7 {
		t.Fatalf("expected 7 buckets, got %d", len(result.Buckets))
	}

	// Verify at least some buckets have data.
	totalCount := 0
	for _, b := range result.Buckets {
		totalCount += b.Count
	}
	if totalCount != 7 {
		t.Errorf("expected 7 total events across buckets, got %d", totalCount)
	}
}

func TestRollupTrendUnsupportedMetric(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}
	_, err := ComputeTrend(ctx, store, "unknown", time.Hour, 5)
	if err == nil {
		t.Fatal("expected error for unsupported metric")
	}
}

func TestRollupMultiDimensionGroupBy(t *testing.T) {
	ctx := context.Background()
	store := &mockStore{}
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// 2 agents x 2 waves = 4 groups.
	entries := []struct {
		agent string
		wave  int
		cost  float64
	}{
		{"A", 1, 0.10},
		{"A", 1, 0.20},
		{"A", 2, 0.30},
		{"B", 1, 0.40},
		{"B", 2, 0.50},
		{"B", 2, 0.60},
	}
	for i, e := range entries {
		_ = store.RecordEvent(ctx, &CostEvent{
			ID:         NewEventID(),
			Type:       "cost",
			Time:       base.Add(time.Duration(i) * time.Hour),
			AgentID:    e.agent,
			WaveNumber: e.wave,
			IMPLSlug:   "test-impl",
			Model:      "opus",
			CostUSD:    e.cost,
		})
	}

	result, err := ComputeCostRollup(ctx, store, RollupRequest{
		Type:    "cost",
		GroupBy: []string{"agent", "wave"},
	})
	if err != nil {
		t.Fatalf("ComputeCostRollup: %v", err)
	}

	if len(result.Groups) != 4 {
		t.Errorf("expected 4 groups (agent+wave), got %d", len(result.Groups))
	}
}
