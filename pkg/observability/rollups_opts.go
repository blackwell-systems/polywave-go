package observability

import (
	"context"
	"time"
)

// ComputeTrendOpts holds parameters for ComputeTrend.
type ComputeTrendOpts struct {
	Ctx       context.Context
	Store     Store
	Metric    string
	TimeRange time.Duration
	Buckets   int
}
