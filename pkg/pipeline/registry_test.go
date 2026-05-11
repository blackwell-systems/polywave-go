package pipeline

import (
	"context"
	"testing"
)

// TestRegistry_RegisterAndGet verifies basic registration and retrieval.
func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	// Unknown step should not be found.
	if _, ok := r.Get("nonexistent"); ok {
		t.Fatal("expected Get to return false for unknown step")
	}

	// Register and retrieve.
	called := false
	fn := func(ctx context.Context, state *State) error {
		called = true
		return nil
	}
	r.Register("my_step", fn)

	got, ok := r.Get("my_step")
	if !ok {
		t.Fatal("expected Get to return true after Register")
	}
	if err := got(context.Background(), makeState()); err != nil {
		t.Fatalf("unexpected error calling registered func: %v", err)
	}
	if !called {
		t.Fatal("registered function was not called")
	}
}

// TestRegistry_Overwrite verifies that registering the same name twice
// overwrites the previous entry.
func TestRegistry_Overwrite(t *testing.T) {
	r := NewRegistry()

	r.Register("step", func(ctx context.Context, state *State) error {
		state.Values["who"] = "first"
		return nil
	})
	r.Register("step", func(ctx context.Context, state *State) error {
		state.Values["who"] = "second"
		return nil
	})

	fn, ok := r.Get("step")
	if !ok {
		t.Fatal("expected step to be registered")
	}
	s := makeState()
	_ = fn(context.Background(), s)
	if s.Values["who"] != "second" {
		t.Fatalf("expected 'second', got %v", s.Values["who"])
	}
}

// TestDefaultRegistry_AllSteps verifies that DefaultRegistry contains every
// standard Polywave step by name.
func TestDefaultRegistry_AllSteps(t *testing.T) {
	r := DefaultRegistry()
	expected := []string{
		"validate_invariants",
		"create_worktrees",
		"run_quality_gates",
		"merge_agents",
		"verify_build",
		"cleanup",
	}
	for _, name := range expected {
		if _, ok := r.Get(name); !ok {
			t.Errorf("DefaultRegistry missing step %q", name)
		}
	}
}
