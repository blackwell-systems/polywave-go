package notify

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- mock helpers ---

type mockAdapter struct {
	name    string
	sendFn  func(ctx context.Context, msg Message) error
	calls   atomic.Int32
	lastMsg Message
	mu      sync.Mutex
}

func (m *mockAdapter) Name() string { return m.name }

func (m *mockAdapter) Send(ctx context.Context, msg Message) error {
	m.calls.Add(1)
	m.mu.Lock()
	m.lastMsg = msg
	m.mu.Unlock()
	if m.sendFn != nil {
		return m.sendFn(ctx, msg)
	}
	return nil
}

type mockFormatter struct{}

func (mockFormatter) Format(e Event) Message {
	return Message{Text: e.Title + ": " + e.Body}
}

// --- tests ---

func TestDispatch_AllReceive(t *testing.T) {
	a1 := &mockAdapter{name: "a1"}
	a2 := &mockAdapter{name: "a2"}
	a3 := &mockAdapter{name: "a3"}

	d := NewDispatcher(a1, a2, a3)

	err := d.Dispatch(context.Background(), Event{
		Title: "hello",
		Body:  "world",
	}, mockFormatter{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, a := range []*mockAdapter{a1, a2, a3} {
		if got := a.calls.Load(); got != 1 {
			t.Errorf("adapter %s: got %d calls, want 1", a.name, got)
		}
	}
}

func TestDispatch_OneFailsOthersStillReceive(t *testing.T) {
	errBoom := errors.New("boom")

	a1 := &mockAdapter{name: "a1"}
	a2 := &mockAdapter{name: "a2", sendFn: func(_ context.Context, _ Message) error { return errBoom }}
	a3 := &mockAdapter{name: "a3"}

	d := NewDispatcher(a1, a2, a3)

	err := d.Dispatch(context.Background(), Event{Title: "t"}, mockFormatter{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, errBoom) {
		t.Errorf("error should contain boom: %v", err)
	}
	// The other two adapters should still have been called.
	if a1.calls.Load() != 1 {
		t.Error("a1 should have been called")
	}
	if a3.calls.Load() != 1 {
		t.Error("a3 should have been called")
	}
}

func TestDispatch_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	a1 := &mockAdapter{name: "a1", sendFn: func(ctx context.Context, _ Message) error {
		return ctx.Err()
	}}

	d := NewDispatcher(a1)

	err := d.Dispatch(ctx, Event{Title: "t"}, mockFormatter{})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestDispatch_NoAdapters(t *testing.T) {
	d := NewDispatcher()
	err := d.Dispatch(context.Background(), Event{}, mockFormatter{})
	if err != nil {
		t.Fatalf("dispatch with no adapters should succeed, got: %v", err)
	}
}

func TestDispatcher_AddRemove(t *testing.T) {
	d := NewDispatcher()
	a := &mockAdapter{name: "x"}

	d.AddAdapter(a)
	if err := d.Dispatch(context.Background(), Event{Title: "t"}, mockFormatter{}); err != nil {
		t.Fatal(err)
	}
	if a.calls.Load() != 1 {
		t.Error("adapter should have been called after Add")
	}

	d.RemoveAdapter("x")
	// Dispatch again; removed adapter should NOT be called.
	if err := d.Dispatch(context.Background(), Event{Title: "t2"}, mockFormatter{}); err != nil {
		t.Fatal(err)
	}
	if a.calls.Load() != 1 {
		t.Error("adapter should not have been called after Remove")
	}
}

func TestDispatch_FormatterOutput(t *testing.T) {
	a := &mockAdapter{name: "fmt-check"}
	d := NewDispatcher(a)

	ev := Event{
		Title:     "Build Failed",
		Body:      "pkg/foo: compile error",
		Timestamp: time.Now(),
	}

	if err := d.Dispatch(context.Background(), ev, mockFormatter{}); err != nil {
		t.Fatal(err)
	}

	a.mu.Lock()
	got := a.lastMsg.Text
	a.mu.Unlock()

	want := "Build Failed: pkg/foo: compile error"
	if got != want {
		t.Errorf("message text = %q, want %q", got, want)
	}
}
