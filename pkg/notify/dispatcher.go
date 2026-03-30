package notify

import (
	"context"
	"fmt"
	"sync"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Dispatcher fans out formatted messages to N adapters concurrently,
// collecting all errors.
type Dispatcher struct {
	mu       sync.RWMutex
	adapters []Adapter
}

// NewDispatcher creates a Dispatcher pre-loaded with the given adapters.
func NewDispatcher(adapters ...Adapter) *Dispatcher {
	return &Dispatcher{
		adapters: append([]Adapter(nil), adapters...),
	}
}

// Dispatch formats the event and sends the resulting message to every
// registered adapter concurrently. Returns PARTIAL if some adapters fail,
// SUCCESS if all succeed, and FATAL if no adapters are registered or if
// context is cancelled before any send occurs.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event, formatter Formatter) result.Result[DispatchData] {
	msg := formatter.Format(event)

	d.mu.RLock()
	snapshot := make([]Adapter, len(d.adapters))
	copy(snapshot, d.adapters)
	d.mu.RUnlock()

	if len(snapshot) == 0 {
		return result.NewSuccess(DispatchData{})
	}

	// Check context before spawning goroutines.
	select {
	case <-ctx.Done():
		return result.NewFailure[DispatchData]([]result.SAWError{
			{
				Code:     "CONTEXT_CANCELLED",
				Message:  ctx.Err().Error(),
				Severity: "fatal",
			},
		})
	default:
	}

	type sendResult struct {
		adapterName string
		res         result.Result[SendData]
	}

	results := make(chan sendResult, len(snapshot))
	var wg sync.WaitGroup

	for _, a := range snapshot {
		wg.Add(1)
		go func(adapter Adapter) {
			defer wg.Done()

			// Check context before sending.
			select {
			case <-ctx.Done():
				results <- sendResult{
					adapterName: adapter.Name(),
					res: result.NewFailure[SendData]([]result.SAWError{
						{
							Code:     "CONTEXT_CANCELLED",
							Message:  ctx.Err().Error(),
							Severity: "fatal",
							Context:  map[string]string{"adapter": adapter.Name()},
						},
					}),
				}
				return
			default:
			}

			results <- sendResult{
				adapterName: adapter.Name(),
				res:         adapter.Send(ctx, msg),
			}
		}(a)
	}

	wg.Wait()
	close(results)

	var (
		sentCount   int
		failedCount int
		errs        []result.SAWError
	)

	for sr := range results {
		if sr.res.IsFatal() || sr.res.IsPartial() {
			failedCount++
			errs = append(errs, sr.res.Errors...)
		} else {
			sentCount++
		}
	}

	data := DispatchData{
		SentCount:   sentCount,
		FailedCount: failedCount,
		Errors:      errs,
	}

	if failedCount == 0 {
		return result.NewSuccess(data)
	}

	if sentCount == 0 {
		return result.NewFailure[DispatchData]([]result.SAWError{
			{
				Code:     "DISPATCH_ALL_FAILED",
				Message:  fmt.Sprintf("all %d adapters failed", failedCount),
				Severity: "fatal",
			},
		})
	}

	// Some succeeded, some failed: PARTIAL.
	return result.NewPartial(data, errs)
}

// AddAdapter appends an adapter to the dispatcher.
func (d *Dispatcher) AddAdapter(a Adapter) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.adapters = append(d.adapters, a)
}

// RemoveAdapter removes the first adapter matching the given name.
func (d *Dispatcher) RemoveAdapter(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i, a := range d.adapters {
		if a.Name() == name {
			d.adapters = append(d.adapters[:i], d.adapters[i+1:]...)
			return
		}
	}
}
