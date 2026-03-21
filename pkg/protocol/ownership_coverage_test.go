package protocol

import (
	"testing"
)

// TestOwnershipCoverage_AllOwned verifies that an agent reporting only files
// within its declared ownership returns Valid=true with no violations.
func TestOwnershipCoverage_AllOwned(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "my-feature",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/a.go", Agent: "B", Wave: 1},
			{File: "pkg/foo/b.go", Agent: "B", Wave: 1},
		},
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "B", Task: "test"}}},
		},
		CompletionReports: map[string]CompletionReport{
			"B": {
				Status:       "complete",
				FilesChanged: []string{"pkg/foo/a.go"},
				FilesCreated: []string{"pkg/foo/b.go"},
			},
		},
	}

	result, err := ValidateFileOwnershipCoverage(manifest, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true, got false. Violations: %v", result.Violations)
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(result.Violations))
	}
}

// TestOwnershipCoverage_BonusFile verifies that a file not in the agent's
// ownership table is recorded as a violation and Valid is false.
func TestOwnershipCoverage_BonusFile(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "my-feature",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/a.go", Agent: "B", Wave: 1},
		},
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "B", Task: "test"}}},
		},
		CompletionReports: map[string]CompletionReport{
			"B": {
				Status:       "complete",
				FilesChanged: []string{"pkg/foo/a.go", "pkg/OTHER/bonus.go"},
			},
		},
	}

	result, err := ValidateFileOwnershipCoverage(manifest, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected Valid=false for bonus file outside ownership")
	}
	if len(result.Violations) == 0 {
		t.Fatal("expected at least one violation")
	}
	v := result.Violations[0]
	if v.Agent != "B" {
		t.Errorf("expected violation for agent B, got %s", v.Agent)
	}
	found := false
	for _, f := range v.UnownedFiles {
		if f == "pkg/OTHER/bonus.go" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected bonus.go in unowned files, got: %v", v.UnownedFiles)
	}
}

// TestOwnershipCoverage_FrozenPathAllowed verifies that scaffold (frozen) paths
// are exempt from the ownership check and do not cause violations.
func TestOwnershipCoverage_FrozenPathAllowed(t *testing.T) {
	scaffoldPath := "pkg/scaffold/types.go"
	manifest := &IMPLManifest{
		FeatureSlug: "my-feature",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/a.go", Agent: "B", Wave: 1},
		},
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "B", Task: "test"}}},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: scaffoldPath},
		},
		CompletionReports: map[string]CompletionReport{
			"B": {
				Status: "complete",
				// Reports the scaffold file as changed — should be allowed.
				FilesChanged: []string{"pkg/foo/a.go", scaffoldPath},
			},
		},
	}

	result, err := ValidateFileOwnershipCoverage(manifest, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true (scaffold path allowed), got false. Violations: %v", result.Violations)
	}
}

// TestOwnershipCoverage_WaveNotFound verifies that requesting a non-existent
// wave number returns an error.
func TestOwnershipCoverage_WaveNotFound(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "my-feature",
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "B"}}},
		},
	}

	_, err := ValidateFileOwnershipCoverage(manifest, 99)
	if err == nil {
		t.Error("expected error for non-existent wave, got nil")
	}
}

// TestOwnershipCoverage_NoReport verifies that an agent with no completion
// report is silently skipped (no violation, Valid=true).
func TestOwnershipCoverage_NoReport(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "my-feature",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/a.go", Agent: "B", Wave: 1},
		},
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "B", Task: "test"}}},
		},
		// No CompletionReports
	}

	result, err := ValidateFileOwnershipCoverage(manifest, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true when no report exists, got false")
	}
}
