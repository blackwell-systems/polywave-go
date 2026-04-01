package notify

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// --- mock helpers ---

type mockAdapter struct {
	name   string
	sendFn func(ctx context.Context, msg Message) result.Result[SendData]
	calls  atomic.Int32
	lastMsg Message
	mu      sync.Mutex
}

func (m *mockAdapter) Name() string { return m.name }

func (m *mockAdapter) Send(ctx context.Context, msg Message) result.Result[SendData] {
	m.calls.Add(1)
	m.mu.Lock()
	m.lastMsg = msg
	m.mu.Unlock()
	if m.sendFn != nil {
		return m.sendFn(ctx, msg)
	}
	return result.NewSuccess(SendData{Timestamp: time.Now(), Provider: m.name})
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

	res := d.Dispatch(context.Background(), Event{
		Title: "hello",
		Body:  "world",
	}, mockFormatter{})

	if !res.IsSuccess() {
		t.Fatalf("unexpected result code %q, errors: %v", res.Code, res.Errors)
	}
	data := res.GetData()
	if data.SentCount != 3 {
		t.Errorf("expected SentCount=3, got %d", data.SentCount)
	}
	if data.FailedCount != 0 {
		t.Errorf("expected FailedCount=0, got %d", data.FailedCount)
	}
	for _, a := range []*mockAdapter{a1, a2, a3} {
		if got := a.calls.Load(); got != 1 {
			t.Errorf("adapter %s: got %d calls, want 1", a.name, got)
		}
	}
}

func TestDispatch_OneFailsOthersStillReceive(t *testing.T) {
	a1 := &mockAdapter{name: "a1"}
	a2 := &mockAdapter{name: "a2", sendFn: func(_ context.Context, _ Message) result.Result[SendData] {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "BOOM", Message: "boom", Severity: "fatal"},
		})
	}}
	a3 := &mockAdapter{name: "a3"}

	d := NewDispatcher(a1, a2, a3)

	res := d.Dispatch(context.Background(), Event{Title: "t"}, mockFormatter{})
	if !res.IsPartial() {
		t.Fatalf("expected PARTIAL result, got %q", res.Code)
	}
	data := res.GetData()
	if data.SentCount != 2 {
		t.Errorf("expected SentCount=2, got %d", data.SentCount)
	}
	if data.FailedCount != 1 {
		t.Errorf("expected FailedCount=1, got %d", data.FailedCount)
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

	a1 := &mockAdapter{name: "a1"}
	d := NewDispatcher(a1)

	res := d.Dispatch(ctx, Event{Title: "t"}, mockFormatter{})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result for cancelled context, got %q", res.Code)
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors in FATAL result")
	}
	if res.Errors[0].Code != result.CodeContextCancelled {
		t.Errorf("expected CONTEXT_CANCELLED error code, got %q", res.Errors[0].Code)
	}
}

func TestDispatch_NoAdapters(t *testing.T) {
	d := NewDispatcher()
	res := d.Dispatch(context.Background(), Event{}, mockFormatter{})
	if !res.IsFatal() {
		t.Fatalf("dispatch with no adapters should be FATAL, got: %q", res.Code)
	}
	if len(res.Errors) == 0 || res.Errors[0].Code != result.CodeDispatchNoAdapters {
		t.Errorf("expected DISPATCH_NO_ADAPTERS error, got %v", res.Errors)
	}
}

func TestDispatcher_AddRemove(t *testing.T) {
	d := NewDispatcher()
	a := &mockAdapter{name: "x"}

	d.AddAdapter(a)
	res := d.Dispatch(context.Background(), Event{Title: "t"}, mockFormatter{})
	if !res.IsSuccess() {
		t.Fatalf("unexpected result: %q errors: %v", res.Code, res.Errors)
	}
	if a.calls.Load() != 1 {
		t.Error("adapter should have been called after Add")
	}

	d.RemoveAdapter("x")
	// After remove, dispatch should fail with no adapters.
	res2 := d.Dispatch(context.Background(), Event{Title: "t2"}, mockFormatter{})
	if !res2.IsFatal() {
		t.Fatalf("expected FATAL after remove, got %q", res2.Code)
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

	res := d.Dispatch(context.Background(), ev, mockFormatter{})
	if !res.IsSuccess() {
		t.Fatalf("unexpected result: %q", res.Code)
	}

	a.mu.Lock()
	got := a.lastMsg.Text
	a.mu.Unlock()

	want := "Build Failed: pkg/foo: compile error"
	if got != want {
		t.Errorf("message text = %q, want %q", got, want)
	}
}

func TestDispatch_AllFail(t *testing.T) {
	failFn := func(_ context.Context, _ Message) result.Result[SendData] {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "ERR", Message: "fail", Severity: "fatal"},
		})
	}
	a1 := &mockAdapter{name: "a1", sendFn: failFn}
	a2 := &mockAdapter{name: "a2", sendFn: failFn}

	d := NewDispatcher(a1, a2)
	res := d.Dispatch(context.Background(), Event{Title: "t"}, mockFormatter{})
	if !res.IsFatal() {
		t.Fatalf("expected FATAL when all adapters fail, got %q", res.Code)
	}
}

func TestDispatch_PartialWhenSomeFail(t *testing.T) {
	a1 := &mockAdapter{name: "ok"}
	a2 := &mockAdapter{name: "fail", sendFn: func(_ context.Context, _ Message) result.Result[SendData] {
		return result.NewFailure[SendData]([]result.SAWError{
			{Code: "ERR", Message: "fail", Severity: "error"},
		})
	}}

	d := NewDispatcher(a1, a2)
	res := d.Dispatch(context.Background(), Event{Title: "t"}, mockFormatter{})
	if !res.IsPartial() {
		t.Fatalf("expected PARTIAL result, got %q", res.Code)
	}
	data := res.GetData()
	if data.SentCount != 1 || data.FailedCount != 1 {
		t.Errorf("expected SentCount=1 FailedCount=1, got %d/%d", data.SentCount, data.FailedCount)
	}
}
