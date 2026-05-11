package protocol

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func TestValidateCompletionStatuses_Valid(t *testing.T) {
	tests := []struct {
		name   string
		status CompletionStatus
	}{
		{"complete", StatusComplete},
		{"partial", StatusPartial},
		{"blocked", StatusBlocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &IMPLManifest{
				CompletionReports: map[string]CompletionReport{
					"A": {Status: tt.status},
				},
			}

			errs := ValidateCompletionStatuses(m)
			if len(errs) > 0 {
				t.Errorf("ValidateCompletionStatuses() returned errors for valid status %q: %v", tt.status, errs)
			}
		})
	}
}

func TestValidateCompletionStatuses_Invalid(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {Status: CompletionStatus("done")},
		},
	}

	errs := ValidateCompletionStatuses(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	if errs[0].Code != result.CodeInvalidEnum {
		t.Errorf("expected error code DC02_INVALID_STATUS, got %q", errs[0].Code)
	}

	if errs[0].Field != "completion_reports[A].status" {
		t.Errorf("expected field completion_reports[A].status, got %q", errs[0].Field)
	}
}

func TestValidateCompletionStatuses_Empty(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{},
	}

	errs := ValidateCompletionStatuses(m)
	if len(errs) > 0 {
		t.Errorf("ValidateCompletionStatuses() returned errors for empty reports: %v", errs)
	}
}

func TestValidateFailureTypes_Valid(t *testing.T) {
	tests := []struct {
		name        string
		failureType string
	}{
		{"transient", "transient"},
		{"fixable", "fixable"},
		{"needs_replan", "needs_replan"},
		{"escalate", "escalate"},
		{"timeout", "timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &IMPLManifest{
				CompletionReports: map[string]CompletionReport{
					"A": {Status: StatusPartial, FailureType: tt.failureType},
				},
			}

			errs := ValidateFailureTypes(m)
			if len(errs) > 0 {
				t.Errorf("ValidateFailureTypes() returned errors for valid type %q: %v", tt.failureType, errs)
			}
		})
	}
}

func TestValidateFailureTypes_Invalid(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {Status: StatusPartial, FailureType: "retry"},
		},
	}

	errs := ValidateFailureTypes(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	if errs[0].Code != result.CodeInvalidFailureType {
		t.Errorf("expected error code DC03_INVALID_FAILURE_TYPE, got %q", errs[0].Code)
	}

	if errs[0].Field != "completion_reports[A].failure_type" {
		t.Errorf("expected field completion_reports[A].failure_type, got %q", errs[0].Field)
	}
}

func TestValidateFailureTypes_EmptyOK(t *testing.T) {
	m := &IMPLManifest{
		CompletionReports: map[string]CompletionReport{
			"A": {Status: StatusComplete, FailureType: ""},
		},
	}

	errs := ValidateFailureTypes(m)
	if len(errs) > 0 {
		t.Errorf("ValidateFailureTypes() returned errors for empty failure_type: %v", errs)
	}
}

func TestValidatePreMortemRisk_Valid(t *testing.T) {
	tests := []struct {
		name string
		risk string
	}{
		{"low", "low"},
		{"medium", "medium"},
		{"high", "high"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &IMPLManifest{
				PreMortem: &PreMortem{
					OverallRisk: tt.risk,
				},
			}

			errs := ValidatePreMortemRisk(m)
			if len(errs) > 0 {
				t.Errorf("ValidatePreMortemRisk() returned errors for valid risk %q: %v", tt.risk, errs)
			}
		})
	}
}

func TestValidatePreMortemRisk_Invalid(t *testing.T) {
	m := &IMPLManifest{
		PreMortem: &PreMortem{
			OverallRisk: "critical",
		},
	}

	errs := ValidatePreMortemRisk(m)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}

	if errs[0].Code != result.CodeInvalidPreMortemRisk {
		t.Errorf("expected error code DC06_INVALID_RISK, got %q", errs[0].Code)
	}

	if errs[0].Field != "pre_mortem.overall_risk" {
		t.Errorf("expected field pre_mortem.overall_risk, got %q", errs[0].Field)
	}
}

func TestValidatePreMortemRisk_NilPreMortem(t *testing.T) {
	m := &IMPLManifest{
		PreMortem: nil,
	}

	errs := ValidatePreMortemRisk(m)
	if len(errs) > 0 {
		t.Errorf("ValidatePreMortemRisk() returned errors for nil pre-mortem: %v", errs)
	}
}

func TestValidatePreMortemRisk_EmptyRisk(t *testing.T) {
	m := &IMPLManifest{
		PreMortem: &PreMortem{
			OverallRisk: "",
		},
	}

	errs := ValidatePreMortemRisk(m)
	if len(errs) > 0 {
		t.Errorf("ValidatePreMortemRisk() returned errors for empty risk: %v", errs)
	}
}
