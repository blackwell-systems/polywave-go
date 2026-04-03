package observability

import (
	"time"
)

// ComputeTrendOpts holds parameters for ComputeTrend.
type ComputeTrendOpts struct {
	Store     Store
	Metric    string
	TimeRange time.Duration
	Buckets   int
}
