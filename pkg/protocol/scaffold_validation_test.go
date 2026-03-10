package protocol

import (
	"testing"
)

func TestValidateScaffolds_AllCommitted(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "file1.go", Status: "committed"},
			{FilePath: "file2.go", Status: "committed"},
		},
	}

	statuses := ValidateScaffolds(m)

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	for _, status := range statuses {
		if !status.Valid {
			t.Errorf("expected Valid=true for %s, got false: %s", status.FilePath, status.Message)
		}
		if status.Status != "committed" {
			t.Errorf("expected Status=committed for %s, got %s", status.FilePath, status.Status)
		}
	}
}

func TestValidateScaffolds_OnePending(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "file1.go", Status: "committed"},
			{FilePath: "file2.go", Status: "pending"},
		},
	}

	statuses := ValidateScaffolds(m)

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	// First should be valid
	if !statuses[0].Valid {
		t.Errorf("expected Valid=true for %s, got false", statuses[0].FilePath)
	}

	// Second should be invalid
	if statuses[1].Valid {
		t.Errorf("expected Valid=false for %s (pending), got true", statuses[1].FilePath)
	}
	if statuses[1].Message != "scaffold not yet committed" {
		t.Errorf("expected Message='scaffold not yet committed', got '%s'", statuses[1].Message)
	}
}

func TestValidateScaffolds_OneFailed(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "file1.go", Status: "committed"},
			{FilePath: "file2.go", Status: "FAILED: build error"},
		},
	}

	statuses := ValidateScaffolds(m)

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	// First should be valid
	if !statuses[0].Valid {
		t.Errorf("expected Valid=true for %s, got false", statuses[0].FilePath)
	}

	// Second should be invalid
	if statuses[1].Valid {
		t.Errorf("expected Valid=false for %s (FAILED), got true", statuses[1].FilePath)
	}
	if statuses[1].Message != "FAILED: build error" {
		t.Errorf("expected Message='FAILED: build error', got '%s'", statuses[1].Message)
	}
}

func TestValidateScaffolds_Empty(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{},
	}

	statuses := ValidateScaffolds(m)

	if len(statuses) != 0 {
		t.Fatalf("expected empty result for empty scaffolds, got %d statuses", len(statuses))
	}
}

func TestValidateScaffolds_CommittedWithSHA(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "file1.go", Status: "committed (abc123)"},
			{FilePath: "file2.go", Status: "committed (def456)"},
		},
	}

	statuses := ValidateScaffolds(m)

	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	for _, status := range statuses {
		if !status.Valid {
			t.Errorf("expected Valid=true for %s with status '%s', got false: %s",
				status.FilePath, status.Status, status.Message)
		}
	}
}

func TestValidateScaffolds_NoStatus(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "file1.go", Status: ""},
		},
	}

	statuses := ValidateScaffolds(m)

	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}

	if statuses[0].Valid {
		t.Errorf("expected Valid=false for empty status, got true")
	}
	if statuses[0].Message != "no status set" {
		t.Errorf("expected Message='no status set', got '%s'", statuses[0].Message)
	}
}

func TestValidateScaffolds_NilManifest(t *testing.T) {
	statuses := ValidateScaffolds(nil)

	if len(statuses) != 0 {
		t.Fatalf("expected empty result for nil manifest, got %d statuses", len(statuses))
	}
}

func TestAllScaffoldsCommitted_True(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "file1.go", Status: "committed"},
			{FilePath: "file2.go", Status: "committed (abc123)"},
		},
	}

	if !AllScaffoldsCommitted(m) {
		t.Error("expected AllScaffoldsCommitted=true for all committed scaffolds")
	}
}

func TestAllScaffoldsCommitted_False(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "file1.go", Status: "committed"},
			{FilePath: "file2.go", Status: "pending"},
		},
	}

	if AllScaffoldsCommitted(m) {
		t.Error("expected AllScaffoldsCommitted=false when one scaffold is pending")
	}
}

func TestAllScaffoldsCommitted_NoScaffolds(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{},
	}

	if !AllScaffoldsCommitted(m) {
		t.Error("expected AllScaffoldsCommitted=true for manifest with no scaffolds")
	}
}

func TestAllScaffoldsCommitted_NilManifest(t *testing.T) {
	if !AllScaffoldsCommitted(nil) {
		t.Error("expected AllScaffoldsCommitted=true for nil manifest")
	}
}

func TestAllScaffoldsCommitted_Failed(t *testing.T) {
	m := &IMPLManifest{
		Scaffolds: []ScaffoldFile{
			{FilePath: "file1.go", Status: "committed"},
			{FilePath: "file2.go", Status: "FAILED: syntax error"},
		},
	}

	if AllScaffoldsCommitted(m) {
		t.Error("expected AllScaffoldsCommitted=false when one scaffold has FAILED status")
	}
}
