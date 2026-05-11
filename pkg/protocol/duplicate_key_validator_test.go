package protocol

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func TestValidateDuplicateKeys_NoDuplicates(t *testing.T) {
	yaml := []byte(`
title: My Feature
feature_slug: my-feature
state: SCOUT_PENDING
verdict: SUITABLE
`)
	errs := ValidateDuplicateKeys(yaml)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid YAML, got %d: %v", len(errs), errs)
	}
}

func TestValidateDuplicateKeys_SingleDuplicate(t *testing.T) {
	// Simulates the real failure case: program-web-integration IMPL had two `state:` fields
	yaml := []byte(`title: My Feature
feature_slug: my-feature
state: SCOUT_PENDING
verdict: SUITABLE
state: WAVE_EXECUTING
`)
	errs := ValidateDuplicateKeys(yaml)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error for duplicate 'state' key, got %d: %v", len(errs), errs)
	}
	err := errs[0]
	if err.Code != result.CodeDuplicateKey {
		t.Errorf("expected code E16_DUPLICATE_KEY, got %q", err.Code)
	}
	if err.Field != "state" {
		t.Errorf("expected field 'state', got %q", err.Field)
	}
	// Message should mention the key and the line numbers
	if err.Message == "" {
		t.Error("expected non-empty message")
	}
	t.Logf("error message: %s", err.Message)
}

func TestValidateDuplicateKeys_MultipleDuplicates(t *testing.T) {
	// Three occurrences of the same key — message must list all line numbers
	yaml := []byte(`title: My Feature
feature_slug: my-feature
state: SCOUT_PENDING
verdict: SUITABLE
state: WAVE_EXECUTING
state: COMPLETE
`)
	errs := ValidateDuplicateKeys(yaml)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (for 'state'), got %d: %v", len(errs), errs)
	}
	err := errs[0]
	if err.Code != result.CodeDuplicateKey {
		t.Errorf("expected code E16_DUPLICATE_KEY, got %q", err.Code)
	}
	if err.Field != "state" {
		t.Errorf("expected field 'state', got %q", err.Field)
	}
	// Message should contain all three line numbers
	// Lines 3, 5, 6 in this YAML (1-indexed, skipping blank first line since there's none here)
	t.Logf("error message: %s", err.Message)
}

func TestValidateDuplicateKeys_NestedNotDetected(t *testing.T) {
	// Duplicate keys in nested objects should NOT be flagged — only top-level
	yaml := []byte(`title: My Feature
feature_slug: my-feature
state: SCOUT_PENDING
verdict: SUITABLE
quality_gates:
  gates:
    - type: build
      type: lint
`)
	errs := ValidateDuplicateKeys(yaml)
	if len(errs) != 0 {
		t.Errorf("expected no errors for nested duplicates, got %d: %v", len(errs), errs)
	}
}

func TestValidateDuplicateKeys_EmptyYAML(t *testing.T) {
	errs := ValidateDuplicateKeys([]byte(""))
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty YAML, got %d: %v", len(errs), errs)
	}
}

func TestValidateDuplicateKeys_TwoDifferentDuplicateKeys(t *testing.T) {
	// Both 'title' and 'state' are duplicated — should return 2 errors, sorted
	yaml := []byte(`title: My Feature
feature_slug: my-feature
state: SCOUT_PENDING
verdict: SUITABLE
title: Other Title
state: COMPLETE
`)
	errs := ValidateDuplicateKeys(yaml)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
	// Errors should be sorted alphabetically: 'state' before 'title'
	if errs[0].Field != "state" {
		t.Errorf("expected first error field 'state', got %q", errs[0].Field)
	}
	if errs[1].Field != "title" {
		t.Errorf("expected second error field 'title', got %q", errs[1].Field)
	}
}
