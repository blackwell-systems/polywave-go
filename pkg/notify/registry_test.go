package notify

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// simpleAdapter is a minimal Adapter for registry tests.
type simpleAdapter struct{ name string }

func (s *simpleAdapter) Name() string { return s.name }
func (s *simpleAdapter) Send(_ context.Context, _ Message) result.Result[SendData] {
	return result.NewSuccess(SendData{Timestamp: time.Now(), Provider: s.name})
}

func resetRegistry() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]AdapterFactory{}
}

func TestRegistry_RegisterAndCreate(t *testing.T) {
	resetRegistry()

	Register("test-adapter", func(cfg map[string]string) (Adapter, error) {
		return &simpleAdapter{name: cfg["name"]}, nil
	})

	a, err := NewFromConfig("test-adapter", map[string]string{"name": "my-instance"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Name() != "my-instance" {
		t.Errorf("Name() = %q, want %q", a.Name(), "my-instance")
	}
}

func TestRegistry_UnknownName(t *testing.T) {
	resetRegistry()

	_, err := NewFromConfig("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown adapter")
	}
	if !strings.Contains(err.Error(), "unknown adapter") {
		t.Errorf("error should mention 'unknown adapter': %v", err)
	}
}

func TestRegistry_RegisteredNames(t *testing.T) {
	resetRegistry()

	Register("bravo", func(_ map[string]string) (Adapter, error) { return &simpleAdapter{}, nil })
	Register("alpha", func(_ map[string]string) (Adapter, error) { return &simpleAdapter{}, nil })

	names := RegisteredNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	// Should be sorted.
	if names[0] != "alpha" || names[1] != "bravo" {
		t.Errorf("names = %v, want [alpha bravo]", names)
	}
}

func TestRegistry_DuplicatePanics(t *testing.T) {
	resetRegistry()

	factory := func(_ map[string]string) (Adapter, error) { return &simpleAdapter{}, nil }
	Register("dup", factory)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate register")
		}
	}()
	Register("dup", factory)
}
