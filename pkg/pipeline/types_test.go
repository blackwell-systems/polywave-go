package pipeline

import "testing"

func TestGetValue_HappyPath(t *testing.T) {
	state := &State{}
	SetValue(state, "key", "hello")
	v, ok := GetValue[string](state, "key")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if v != "hello" {
		t.Errorf("expected \"hello\", got %q", v)
	}
}

func TestGetValue_WrongType(t *testing.T) {
	state := &State{}
	SetValue(state, "key", 42)
	_, ok := GetValue[string](state, "key")
	if ok {
		t.Fatal("expected ok=false for wrong type")
	}
}

func TestGetValue_NilValues(t *testing.T) {
	state := &State{}
	v, ok := GetValue[string](state, "key")
	if ok {
		t.Fatal("expected ok=false for nil Values map")
	}
	if v != "" {
		t.Errorf("expected zero value, got %q", v)
	}
}

func TestGetValue_MissingKey(t *testing.T) {
	state := &State{Values: map[string]any{"other": "val"}}
	_, ok := GetValue[string](state, "missing")
	if ok {
		t.Fatal("expected ok=false for absent key")
	}
}

func TestSetValue_InitializesMap(t *testing.T) {
	state := &State{}
	if state.Values != nil {
		t.Fatal("precondition: Values should be nil")
	}
	SetValue(state, "x", 1)
	if state.Values == nil {
		t.Fatal("expected Values to be initialized after SetValue")
	}
	if state.Values["x"] != 1 {
		t.Errorf("expected Values[\"x\"]=1, got %v", state.Values["x"])
	}
}
