package protocol

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func TestValidateActionEnums_AllValid(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/new_file.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/foo/existing.go", Agent: "B", Wave: 1, Action: "modify"},
			{File: "pkg/foo/old.go", Agent: "C", Wave: 1, Action: "delete"},
		},
	}

	errs := ValidateActionEnums(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %d: %+v", len(errs), errs)
	}
}

func TestValidateActionEnums_EmptyActionAllowed(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/file.go", Agent: "A", Wave: 1, Action: ""},
			{File: "pkg/bar/file.go", Agent: "B", Wave: 1}, // no Action field at all
		},
	}

	errs := ValidateActionEnums(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty/omitted action, got %d: %+v", len(errs), errs)
	}
}

func TestValidateActionEnums_InvalidAction(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/file.go", Agent: "A", Wave: 1, Action: "update"},
		},
	}

	errs := ValidateActionEnums(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(errs), errs)
	}

	err := errs[0]
	if err.Code != result.CodeInvalidActionEnum {
		t.Errorf("expected code E16_INVALID_ACTION, got %q", err.Code)
	}
	if err.Field != "file_ownership[0].action" {
		t.Errorf("expected field file_ownership[0].action, got %q", err.Field)
	}
	// Verify message contains the invalid value
	expected := `file_ownership[0].action has invalid value "update" — must be new, modify, or delete`
	if err.Message != expected {
		t.Errorf("unexpected message:\n  got:  %q\n  want: %q", err.Message, expected)
	}
}

func TestValidateActionEnums_MultipleInvalid(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/file.go", Agent: "A", Wave: 1, Action: "update"},
			{File: "pkg/bar/file.go", Agent: "B", Wave: 1, Action: "modify"}, // valid
			{File: "pkg/baz/file.go", Agent: "C", Wave: 1, Action: "create"},
		},
	}

	errs := ValidateActionEnums(m)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %+v", len(errs), errs)
	}

	// First invalid: index 0, action "update"
	if errs[0].Code != result.CodeInvalidActionEnum {
		t.Errorf("errs[0]: expected code E16_INVALID_ACTION, got %q", errs[0].Code)
	}
	if errs[0].Field != "file_ownership[0].action" {
		t.Errorf("errs[0]: expected field file_ownership[0].action, got %q", errs[0].Field)
	}

	// Second invalid: index 2, action "create"
	if errs[1].Code != result.CodeInvalidActionEnum {
		t.Errorf("errs[1]: expected code E16_INVALID_ACTION, got %q", errs[1].Code)
	}
	if errs[1].Field != "file_ownership[2].action" {
		t.Errorf("errs[1]: expected field file_ownership[2].action, got %q", errs[1].Field)
	}
}
