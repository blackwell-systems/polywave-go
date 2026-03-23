package protocol

import (
	"testing"
)

func TestValidateMigrationBoundaries_NoBoundary(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/auth/token.go", Wave: 1, Agent: "A"},
			{File: "pkg/db/conn.go", Wave: 2, Agent: "B"},
		},
	}
	warnings := ValidateMigrationBoundaries(m)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateMigrationBoundaries_SameDir(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/auth/token.go", Wave: 1, Agent: "A"},
			{File: "pkg/auth/middleware.go", Wave: 2, Agent: "B"},
		},
	}
	warnings := ValidateMigrationBoundaries(m)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
	w := warnings[0]
	if w.Code != "MIGRATION_BOUNDARY_WARNING" {
		t.Errorf("expected code MIGRATION_BOUNDARY_WARNING, got %s", w.Code)
	}
	if w.Field != "file_ownership" {
		t.Errorf("expected field file_ownership, got %s", w.Field)
	}
}

func TestValidateMigrationBoundaries_NonConsecutive(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/auth/token.go", Wave: 1, Agent: "A"},
			{File: "pkg/db/conn.go", Wave: 2, Agent: "B"},
			{File: "pkg/auth/middleware.go", Wave: 3, Agent: "C"},
		},
	}
	warnings := ValidateMigrationBoundaries(m)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for non-consecutive waves sharing a dir, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateMigrationBoundaries_SingleWave(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/auth/token.go", Wave: 1, Agent: "A"},
			{File: "pkg/auth/middleware.go", Wave: 1, Agent: "B"},
		},
	}
	warnings := ValidateMigrationBoundaries(m)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for single wave, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateMigrationBoundaries_MultipleOverlaps(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/auth/token.go", Wave: 1, Agent: "A"},
			{File: "pkg/db/conn.go", Wave: 1, Agent: "A"},
			{File: "pkg/auth/middleware.go", Wave: 2, Agent: "B"},
			{File: "pkg/db/query.go", Wave: 2, Agent: "B"},
		},
	}
	warnings := ValidateMigrationBoundaries(m)
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %v", len(warnings), warnings)
	}
	// Warnings should be sorted by directory name
	if warnings[0].Message == warnings[1].Message {
		t.Error("expected different messages for different directories")
	}
}

func TestValidateMigrationBoundaries_Nil(t *testing.T) {
	warnings := ValidateMigrationBoundaries(nil)
	if warnings != nil {
		t.Errorf("expected nil for nil manifest, got %v", warnings)
	}
}

func TestDiagnoseMigrationFailure_MatchesPattern(t *testing.T) {
	result := &BaselineData{
		Passed: false,
		GateResults: []GateResult{
			{
				Passed: false,
				Stderr: "pkg/auth/token.go:15: undefined: NewToken",
			},
		},
	}
	suggestion := DiagnoseMigrationFailure(result)
	if suggestion == "" {
		t.Error("expected suggestion for migration failure pattern, got empty string")
	}
}

func TestDiagnoseMigrationFailure_NoMatch(t *testing.T) {
	result := &BaselineData{
		Passed: false,
		GateResults: []GateResult{
			{
				Passed: false,
				Stderr: "syntax error: unexpected end of file",
			},
		},
	}
	suggestion := DiagnoseMigrationFailure(result)
	if suggestion != "" {
		t.Errorf("expected empty string for non-migration failure, got %q", suggestion)
	}
}

func TestDiagnoseMigrationFailure_Passed(t *testing.T) {
	result := &BaselineData{Passed: true}
	suggestion := DiagnoseMigrationFailure(result)
	if suggestion != "" {
		t.Errorf("expected empty string for passed result, got %q", suggestion)
	}
}

func TestDiagnoseMigrationFailure_Nil(t *testing.T) {
	suggestion := DiagnoseMigrationFailure(nil)
	if suggestion != "" {
		t.Errorf("expected empty string for nil result, got %q", suggestion)
	}
}
