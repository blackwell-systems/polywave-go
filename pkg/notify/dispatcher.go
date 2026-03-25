package notify

import (
	"context"
	"errors"
	"sync"
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
// registered adapter concurrently. All errors are collected and returned
// as a joined error. Context cancellation is respected.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event, formatter Formatter) error {
	msg := formatter.Format(event)

	d.mu.RLock()
	snapshot := make([]Adapter, len(d.adapters))
	copy(snapshot, d.adapters)
	d.mu.RUnlock()

	if len(snapshot) == 0 {
		return nil
	}

	var (
		mu   sync.Mutex
		errs []error
		wg   sync.WaitGroup
	)

	for _, a := range snapshot {
		wg.Add(1)
		go func(adapter Adapter) {
			defer wg.Done()

			// Check context before sending.
			select {
			case <-ctx.Done():
				mu.Lock()
				errs = append(errs, ctx.Err())
				mu.Unlock()
				return
			default:
			}

			if err := adapter.Send(ctx, msg); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}(a)
	}

	wg.Wait()
	return errors.Join(errs...)
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
