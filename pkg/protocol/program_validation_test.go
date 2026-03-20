package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestValidateProgram_ValidManifest verifies that a valid manifest returns no errors
func TestValidateProgram_ValidManifest(t *testing.T) {
	manifest := &PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-program",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{
				Slug:      "impl-a",
				Title:     "Implementation A",
				Tier:      1,
				DependsOn: []string{},
				Status:    "pending",
			},
			{
				Slug:      "impl-b",
				Title:     "Implementation B",
				Tier:      2,
				DependsOn: []string{"impl-a"},
				Status:    "scouting",
			},
			{
				Slug:      "impl-c",
				Title:     "Implementation C",
				Tier:      2,
				DependsOn: []string{"impl-a"},
				Status:    "complete",
			},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
			{Number: 2, Impls: []string{"impl-b", "impl-c"}},
		},
		ProgramContracts: []ProgramContract{
			{
				Name:       "TestContract",
				Definition: "func Test()",
				Location:   "pkg/test/contract.go",
				Consumers: []ProgramContractConsumer{
					{Impl: "impl-b", Usage: "test usage"},
				},
			},
		},
		Completion: ProgramCompletion{
			TiersComplete: 1,
			TiersTotal:    2,
			ImplsComplete: 1,
			ImplsTotal:    3,
		},
	}

	errs := ValidateProgram(manifest)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid manifest, got %d errors: %v", len(errs), errs)
	}
}

// TestValidateProgram_MissingRequiredFields tests that empty required fields are caught
func TestValidateProgram_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name          string
		manifest      *PROGRAMManifest
		expectedCode  string
		expectedField string
	}{
		{
			name: "missing title",
			manifest: &PROGRAMManifest{
				Title:       "",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
			},
			expectedCode:  "MISSING_FIELD",
			expectedField: "title",
		},
		{
			name: "missing program_slug",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "",
				State:       ProgramStatePlanning,
			},
			expectedCode:  "MISSING_FIELD",
			expectedField: "program_slug",
		},
		{
			name: "missing state",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       "",
			},
			expectedCode:  "MISSING_FIELD",
			expectedField: "state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateProgram(tt.manifest)
			if len(errs) == 0 {
				t.Fatalf("expected validation error, got none")
			}
			found := false
			for _, err := range errs {
				if err.Code == tt.expectedCode && err.Field == tt.expectedField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error with code=%q field=%q, got errors: %v", tt.expectedCode, tt.expectedField, errs)
			}
		})
	}
}

// TestValidateProgram_InvalidState tests that unknown state strings are caught
func TestValidateProgram_InvalidState(t *testing.T) {
	manifest := &PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-program",
		State:       ProgramState("INVALID_STATE"),
	}

	errs := ValidateProgram(manifest)
	found := false
	for _, err := range errs {
		if err.Code == "INVALID_STATE" && err.Field == "state" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected INVALID_STATE error, got errors: %v", errs)
	}
}

// TestValidateProgram_P1Violation tests that same-tier dependencies are caught
func TestValidateProgram_P1Violation(t *testing.T) {
	manifest := &PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-program",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{
				Slug:      "impl-a",
				Title:     "Implementation A",
				Tier:      1,
				DependsOn: []string{},
				Status:    "pending",
			},
			{
				Slug:      "impl-b",
				Title:     "Implementation B",
				Tier:      1,
				DependsOn: []string{"impl-a"},
				Status:    "pending",
			},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
		},
		Completion: ProgramCompletion{
			TiersTotal: 1,
			ImplsTotal: 2,
		},
	}

	errs := ValidateProgram(manifest)
	found := false
	for _, err := range errs {
		if err.Code == "P1_VIOLATION" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected P1_VIOLATION error, got errors: %v", errs)
	}
}

// TestValidateProgram_TierIMPLMismatch tests when an IMPL is not in any tier
func TestValidateProgram_TierIMPLMismatch(t *testing.T) {
	tests := []struct {
		name     string
		manifest *PROGRAMManifest
		wantCode string
	}{
		{
			name: "IMPL not in any tier",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Impls: []ProgramIMPL{
					{Slug: "impl-a", Tier: 1, Status: "pending"},
					{Slug: "impl-b", Tier: 2, Status: "pending"},
				},
				Tiers: []ProgramTier{
					{Number: 1, Impls: []string{"impl-a"}},
					// impl-b is missing from tiers
				},
				Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 2},
			},
			wantCode: "TIER_MISMATCH",
		},
		{
			name: "tier references undefined IMPL",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Impls: []ProgramIMPL{
					{Slug: "impl-a", Tier: 1, Status: "pending"},
				},
				Tiers: []ProgramTier{
					{Number: 1, Impls: []string{"impl-a", "impl-nonexistent"}},
				},
				Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
			},
			wantCode: "TIER_MISMATCH",
		},
		{
			name: "IMPL in multiple tiers",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Impls: []ProgramIMPL{
					{Slug: "impl-a", Tier: 1, Status: "pending"},
				},
				Tiers: []ProgramTier{
					{Number: 1, Impls: []string{"impl-a"}},
					{Number: 2, Impls: []string{"impl-a"}},
				},
				Completion: ProgramCompletion{TiersTotal: 2, ImplsTotal: 1},
			},
			wantCode: "TIER_MISMATCH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateProgram(tt.manifest)
			found := false
			for _, err := range errs {
				if err.Code == tt.wantCode {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected %s error, got errors: %v", tt.wantCode, errs)
			}
		})
	}
}

// TestValidateProgram_InvalidDependency tests when depends_on references a nonexistent IMPL
func TestValidateProgram_InvalidDependency(t *testing.T) {
	manifest := &PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-program",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{
				Slug:      "impl-a",
				Tier:      1,
				DependsOn: []string{"nonexistent-impl"},
				Status:    "pending",
			},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
		},
		Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
	}

	errs := ValidateProgram(manifest)
	found := false
	for _, err := range errs {
		if err.Code == "INVALID_DEPENDENCY" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected INVALID_DEPENDENCY error, got errors: %v", errs)
	}
}

// TestValidateProgram_TierOrderViolation tests when a dependency is in the same or later tier
func TestValidateProgram_TierOrderViolation(t *testing.T) {
	tests := []struct {
		name     string
		manifest *PROGRAMManifest
	}{
		{
			name: "dependency in same tier (P1 violation)",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Impls: []ProgramIMPL{
					{Slug: "impl-a", Tier: 1, DependsOn: []string{}, Status: "pending"},
					{Slug: "impl-b", Tier: 1, DependsOn: []string{"impl-a"}, Status: "pending"},
				},
				Tiers: []ProgramTier{
					{Number: 1, Impls: []string{"impl-a", "impl-b"}},
				},
				Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 2},
			},
		},
		{
			name: "dependency in later tier",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Impls: []ProgramIMPL{
					{Slug: "impl-a", Tier: 1, DependsOn: []string{"impl-b"}, Status: "pending"},
					{Slug: "impl-b", Tier: 2, DependsOn: []string{}, Status: "pending"},
				},
				Tiers: []ProgramTier{
					{Number: 1, Impls: []string{"impl-a"}},
					{Number: 2, Impls: []string{"impl-b"}},
				},
				Completion: ProgramCompletion{TiersTotal: 2, ImplsTotal: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateProgram(tt.manifest)
			found := false
			for _, err := range errs {
				// TIER_ORDER_VIOLATION for backward deps, P1_VIOLATION for same-tier deps
				if err.Code == "TIER_ORDER_VIOLATION" || err.Code == "P1_VIOLATION" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected TIER_ORDER_VIOLATION or P1_VIOLATION error, got errors: %v", errs)
			}
		})
	}
}

// TestValidateProgram_InvalidConsumer tests when a contract consumer references an unknown IMPL
func TestValidateProgram_InvalidConsumer(t *testing.T) {
	manifest := &PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-program",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
		},
		ProgramContracts: []ProgramContract{
			{
				Name:       "TestContract",
				Definition: "func Test()",
				Location:   "pkg/test/contract.go",
				Consumers: []ProgramContractConsumer{
					{Impl: "nonexistent-impl", Usage: "test usage"},
				},
			},
		},
		Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
	}

	errs := ValidateProgram(manifest)
	found := false
	for _, err := range errs {
		if err.Code == "INVALID_CONSUMER" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected INVALID_CONSUMER error, got errors: %v", errs)
	}
}

// TestValidateProgram_InvalidSlugFormat tests slugs with uppercase or special characters
func TestValidateProgram_InvalidSlugFormat(t *testing.T) {
	tests := []struct {
		name     string
		manifest *PROGRAMManifest
	}{
		{
			name: "program slug with uppercase",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "Test-Program",
				State:       ProgramStatePlanning,
				Completion:  ProgramCompletion{},
			},
		},
		{
			name: "program slug with underscores",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test_program",
				State:       ProgramStatePlanning,
				Completion:  ProgramCompletion{},
			},
		},
		{
			name: "IMPL slug with uppercase",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Impls: []ProgramIMPL{
					{Slug: "Impl-A", Tier: 1, Status: "pending"},
				},
				Tiers: []ProgramTier{
					{Number: 1, Impls: []string{"Impl-A"}},
				},
				Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
			},
		},
		{
			name: "IMPL slug with special characters",
			manifest: &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Impls: []ProgramIMPL{
					{Slug: "impl@a", Tier: 1, Status: "pending"},
				},
				Tiers: []ProgramTier{
					{Number: 1, Impls: []string{"impl@a"}},
				},
				Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateProgram(tt.manifest)
			found := false
			for _, err := range errs {
				if err.Code == "INVALID_SLUG_FORMAT" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected INVALID_SLUG_FORMAT error, got errors: %v", errs)
			}
		})
	}
}

// TestValidateProgram_CompletionBounds tests when complete exceeds total
func TestValidateProgram_CompletionBounds(t *testing.T) {
	tests := []struct {
		name       string
		completion ProgramCompletion
	}{
		{
			name: "tiers_complete exceeds tiers_total",
			completion: ProgramCompletion{
				TiersComplete: 3,
				TiersTotal:    2,
				ImplsComplete: 0,
				ImplsTotal:    5,
			},
		},
		{
			name: "impls_complete exceeds impls_total",
			completion: ProgramCompletion{
				TiersComplete: 1,
				TiersTotal:    2,
				ImplsComplete: 6,
				ImplsTotal:    5,
			},
		},
		{
			name: "both exceed totals",
			completion: ProgramCompletion{
				TiersComplete: 10,
				TiersTotal:    5,
				ImplsComplete: 20,
				ImplsTotal:    10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Completion:  tt.completion,
			}

			errs := ValidateProgram(manifest)
			found := false
			for _, err := range errs {
				if err.Code == "COMPLETION_BOUNDS" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected COMPLETION_BOUNDS error, got errors: %v", errs)
			}
		})
	}
}

// TestValidateProgram_InvalidIMPLStatus tests that invalid IMPL statuses are caught
func TestValidateProgram_InvalidIMPLStatus(t *testing.T) {
	manifest := &PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-program",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{
				Slug:   "impl-a",
				Tier:   1,
				Status: "invalid-status",
			},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
		},
		Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
	}

	errs := ValidateProgram(manifest)
	found := false
	for _, err := range errs {
		if err.Code == "INVALID_STATUS" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected INVALID_STATUS error, got errors: %v", errs)
	}
}

// TestValidateProgram_AllValidIMPLStatuses tests all valid IMPL statuses pass validation
func TestValidateProgram_AllValidIMPLStatuses(t *testing.T) {
	validStatuses := []string{"pending", "scouting", "reviewed", "executing", "complete"}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			manifest := &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       ProgramStatePlanning,
				Impls: []ProgramIMPL{
					{
						Slug:   "impl-a",
						Tier:   1,
						Status: status,
					},
				},
				Tiers: []ProgramTier{
					{Number: 1, Impls: []string{"impl-a"}},
				},
				Completion: ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
			}

			errs := ValidateProgram(manifest)
			for _, err := range errs {
				if err.Code == "INVALID_STATUS" {
					t.Errorf("valid status %q should not trigger INVALID_STATUS error: %v", status, err)
				}
			}
		})
	}
}

// TestImplsTotalExactMatch tests that impls_total must equal the actual number of impls entries.
func TestImplsTotalExactMatch(t *testing.T) {
	makeImpl := func(slug string, tier int) ProgramIMPL {
		return ProgramIMPL{Slug: slug, Title: slug, Tier: tier, Status: "pending", DependsOn: []string{}}
	}
	makeTier := func(number int, slugs ...string) ProgramTier {
		return ProgramTier{Number: number, Impls: slugs}
	}

	t.Run("exact match passes", func(t *testing.T) {
		manifest := &PROGRAMManifest{
			Title:       "Test Program",
			ProgramSlug: "test-program",
			State:       ProgramStatePlanning,
			Impls:       []ProgramIMPL{makeImpl("impl-a", 1), makeImpl("impl-b", 1)},
			Tiers:       []ProgramTier{makeTier(1, "impl-a", "impl-b")},
			Completion:  ProgramCompletion{TiersTotal: 1, ImplsTotal: 2},
		}
		errs := ValidateProgram(manifest)
		for _, e := range errs {
			if e.Code == "IMPLS_TOTAL_MISMATCH" {
				t.Errorf("exact match (impls_total=2, len(impls)=2) should not produce IMPLS_TOTAL_MISMATCH: %v", e.Message)
			}
		}
	})

	t.Run("impls_total too high fails", func(t *testing.T) {
		manifest := &PROGRAMManifest{
			Title:       "Test Program",
			ProgramSlug: "test-program",
			State:       ProgramStatePlanning,
			Impls:       []ProgramIMPL{makeImpl("impl-a", 1)},
			Tiers:       []ProgramTier{makeTier(1, "impl-a")},
			Completion:  ProgramCompletion{TiersTotal: 1, ImplsTotal: 3},
		}
		errs := ValidateProgram(manifest)
		found := false
		for _, e := range errs {
			if e.Code == "IMPLS_TOTAL_MISMATCH" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("impls_total=3 with 1 impl entry should produce IMPLS_TOTAL_MISMATCH, got: %v", errs)
		}
	})

	t.Run("impls_total too low fails", func(t *testing.T) {
		manifest := &PROGRAMManifest{
			Title:       "Test Program",
			ProgramSlug: "test-program",
			State:       ProgramStatePlanning,
			Impls:       []ProgramIMPL{makeImpl("impl-a", 1), makeImpl("impl-b", 1), makeImpl("impl-c", 1)},
			Tiers:       []ProgramTier{makeTier(1, "impl-a", "impl-b", "impl-c")},
			Completion:  ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
		}
		errs := ValidateProgram(manifest)
		found := false
		for _, e := range errs {
			if e.Code == "IMPLS_TOTAL_MISMATCH" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("impls_total=1 with 3 impl entries should produce IMPLS_TOTAL_MISMATCH, got: %v", errs)
		}
	})

	t.Run("zero impls with zero total passes", func(t *testing.T) {
		manifest := &PROGRAMManifest{
			Title:       "Test Program",
			ProgramSlug: "test-program",
			State:       ProgramStatePlanning,
			Completion:  ProgramCompletion{TiersTotal: 0, ImplsTotal: 0},
		}
		errs := ValidateProgram(manifest)
		for _, e := range errs {
			if e.Code == "IMPLS_TOTAL_MISMATCH" {
				t.Errorf("zero impls with impls_total=0 should not produce IMPLS_TOTAL_MISMATCH: %v", e.Message)
			}
		}
	})
}

// --- Import-mode validation tests ---

// writeTestIMPL is a helper that writes a minimal IMPL YAML file to disk.
func writeTestIMPL(t *testing.T, dir string, slug string, state ProtocolState, files []string, contracts []string) string {
	t.Helper()

	// Build file_ownership entries.
	var foEntries []map[string]interface{}
	for _, f := range files {
		foEntries = append(foEntries, map[string]interface{}{
			"file":  f,
			"agent": "A",
			"wave":  1,
		})
	}

	// Build interface_contracts entries.
	var icEntries []map[string]string
	for _, c := range contracts {
		icEntries = append(icEntries, map[string]string{
			"name":       c,
			"definition": "func " + c + "()",
			"location":   "pkg/" + c + ".go",
		})
	}

	doc := map[string]interface{}{
		"title":        "IMPL " + slug,
		"feature_slug": slug,
		"verdict":      "SUITABLE",
		"test_command":  "go test ./...",
		"lint_command":  "go vet ./...",
		"state":        string(state),
		"file_ownership":     foEntries,
		"interface_contracts": icEntries,
		"waves": []map[string]interface{}{
			{
				"number": 1,
				"agents": []map[string]string{
					{"id": "A", "task": "implement"},
				},
			},
		},
	}

	data, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("failed to marshal test IMPL: %v", err)
	}

	implDir := filepath.Join(dir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatalf("failed to create IMPL dir: %v", err)
	}

	path := filepath.Join(implDir, "IMPL-"+slug+".yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test IMPL: %v", err)
	}
	return path
}

func TestValidateProgramImportMode_MissingIMPLFile(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := &PROGRAMManifest{
		Title:       "Test",
		ProgramSlug: "test",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{Slug: "missing-impl", Tier: 1, Status: "reviewed"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"missing-impl"}},
		},
	}

	errs := ValidateProgramImportMode(manifest, tmpDir)
	found := false
	for _, e := range errs {
		if e.Code == "IMPL_FILE_MISSING" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected IMPL_FILE_MISSING error, got: %v", errs)
	}
}

func TestValidateProgramImportMode_P1FileOverlap(t *testing.T) {
	tmpDir := t.TempDir()

	// Two IMPLs in same tier both owning the same file.
	writeTestIMPL(t, tmpDir, "impl-a", StateReviewed, []string{"pkg/shared.go"}, nil)
	writeTestIMPL(t, tmpDir, "impl-b", StateReviewed, []string{"pkg/shared.go"}, nil)

	manifest := &PROGRAMManifest{
		Title:       "Test",
		ProgramSlug: "test",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "reviewed"},
			{Slug: "impl-b", Tier: 1, Status: "reviewed"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
		},
	}

	errs := ValidateProgramImportMode(manifest, tmpDir)
	found := false
	for _, e := range errs {
		if e.Code == "P1_FILE_OVERLAP" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected P1_FILE_OVERLAP error, got: %v", errs)
	}
}

func TestValidateP1FileDisjointness_SameTier(t *testing.T) {
	impl1 := &IMPLManifest{
		FeatureSlug: "feature-a",
		FileOwnership: []FileOwnership{{File: "pkg/shared.go", Agent: "A"}},
	}
	impl2 := &IMPLManifest{
		FeatureSlug: "feature-b",
		FileOwnership: []FileOwnership{{File: "pkg/shared.go", Agent: "B"}},
	}

	errs := ValidateP1FileDisjointness(1, []*IMPLManifest{impl1, impl2})

	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Code != "P1_FILE_OVERLAP" {
		t.Errorf("expected P1_FILE_OVERLAP, got %s", errs[0].Code)
	}
	if !contains(errs[0].Message, "pkg/shared.go") {
		t.Errorf("error message should mention conflicting file, got: %s", errs[0].Message)
	}
	if !contains(errs[0].Message, "feature-a") || !contains(errs[0].Message, "feature-b") {
		t.Errorf("error message should mention both IMPLs, got: %s", errs[0].Message)
	}
}

func TestValidateP1FileDisjointness_DisjointFiles(t *testing.T) {
	impl1 := &IMPLManifest{
		FeatureSlug: "feature-a",
		FileOwnership: []FileOwnership{{File: "pkg/a.go", Agent: "A"}},
	}
	impl2 := &IMPLManifest{
		FeatureSlug: "feature-b",
		FileOwnership: []FileOwnership{{File: "pkg/b.go", Agent: "B"}},
	}

	errs := ValidateP1FileDisjointness(1, []*IMPLManifest{impl1, impl2})

	if len(errs) != 0 {
		t.Errorf("expected no errors for disjoint files, got: %v", errs)
	}
}

func TestValidateP1FileDisjointness_MultipleOverlaps(t *testing.T) {
	impl1 := &IMPLManifest{
		FeatureSlug: "feature-a",
		FileOwnership: []FileOwnership{
			{File: "pkg/shared1.go", Agent: "A"},
			{File: "pkg/shared2.go", Agent: "A"},
		},
	}
	impl2 := &IMPLManifest{
		FeatureSlug: "feature-b",
		FileOwnership: []FileOwnership{
			{File: "pkg/shared1.go", Agent: "B"},
			{File: "pkg/shared2.go", Agent: "B"},
		},
	}

	errs := ValidateP1FileDisjointness(2, []*IMPLManifest{impl1, impl2})

	if len(errs) != 2 {
		t.Fatalf("expected 2 errors (one per overlapping file), got %d: %v", len(errs), errs)
	}
	for _, err := range errs {
		if err.Code != "P1_FILE_OVERLAP" {
			t.Errorf("expected all errors to be P1_FILE_OVERLAP, got: %s", err.Code)
		}
	}
}

func TestValidateP1FileDisjointness_EmptyIMPLs(t *testing.T) {
	errs := ValidateP1FileDisjointness(1, []*IMPLManifest{})

	if len(errs) != 0 {
		t.Errorf("expected no errors for empty IMPL list, got: %v", errs)
	}
}

func TestValidateProgramImportMode_P2ContractRedefinition(t *testing.T) {
	tmpDir := t.TempDir()

	// IMPL redefines a frozen contract.
	writeTestIMPL(t, tmpDir, "impl-a", StateComplete, nil, nil)
	writeTestIMPL(t, tmpDir, "impl-b", StateReviewed, nil, []string{"SharedAPI"})

	manifest := &PROGRAMManifest{
		Title:       "Test",
		ProgramSlug: "test",
		State:       ProgramStateTierExecuting,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "complete"},
			{Slug: "impl-b", Tier: 2, Status: "reviewed"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a"}},
			{Number: 2, Impls: []string{"impl-b"}},
		},
		ProgramContracts: []ProgramContract{
			{
				Name:       "SharedAPI",
				Definition: "func SharedAPI()",
				Location:   "pkg/api.go",
				FreezeAt:   "tier-1",
			},
		},
	}

	errs := ValidateProgramImportMode(manifest, tmpDir)
	found := false
	for _, e := range errs {
		if e.Code == "P2_CONTRACT_REDEFINITION" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected P2_CONTRACT_REDEFINITION error, got: %v", errs)
	}
}

func TestValidateProgramImportMode_ValidPreExisting(t *testing.T) {
	tmpDir := t.TempDir()

	writeTestIMPL(t, tmpDir, "impl-a", StateReviewed, []string{"pkg/a.go"}, nil)
	writeTestIMPL(t, tmpDir, "impl-b", StateReviewed, []string{"pkg/b.go"}, nil)

	manifest := &PROGRAMManifest{
		Title:       "Test",
		ProgramSlug: "test",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "reviewed"},
			{Slug: "impl-b", Tier: 1, Status: "reviewed"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b"}},
		},
	}

	errs := ValidateProgramImportMode(manifest, tmpDir)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid pre-existing IMPLs, got: %v", errs)
	}
}

func TestPartitionIMPLsByStatus(t *testing.T) {
	manifest := &PROGRAMManifest{
		Title:       "Test",
		ProgramSlug: "test",
		State:       ProgramStatePlanning,
		Impls: []ProgramIMPL{
			{Slug: "impl-a", Tier: 1, Status: "pending"},
			{Slug: "impl-b", Tier: 1, Status: "reviewed"},
			{Slug: "impl-c", Tier: 1, Status: "scouting"},
			{Slug: "impl-d", Tier: 1, Status: "complete"},
			{Slug: "impl-e", Tier: 2, Status: "pending"},
		},
		Tiers: []ProgramTier{
			{Number: 1, Impls: []string{"impl-a", "impl-b", "impl-c", "impl-d"}},
			{Number: 2, Impls: []string{"impl-e"}},
		},
	}

	needsScout, preExisting := PartitionIMPLsByStatus(manifest, 1)

	if len(needsScout) != 2 {
		t.Errorf("expected 2 needsScout IMPLs, got %d: %v", len(needsScout), needsScout)
	}
	if len(preExisting) != 2 {
		t.Errorf("expected 2 preExisting IMPLs, got %d: %v", len(preExisting), preExisting)
	}

	// Verify correct slugs in each group.
	scoutSlugs := make(map[string]bool)
	for _, impl := range needsScout {
		scoutSlugs[impl.Slug] = true
	}
	if !scoutSlugs["impl-a"] || !scoutSlugs["impl-c"] {
		t.Errorf("needsScout should contain impl-a and impl-c, got: %v", scoutSlugs)
	}

	preSlugs := make(map[string]bool)
	for _, impl := range preExisting {
		preSlugs[impl.Slug] = true
	}
	if !preSlugs["impl-b"] || !preSlugs["impl-d"] {
		t.Errorf("preExisting should contain impl-b and impl-d, got: %v", preSlugs)
	}

	// Tier 2 should only have impl-e as needsScout.
	ns2, pe2 := PartitionIMPLsByStatus(manifest, 2)
	if len(ns2) != 1 || ns2[0].Slug != "impl-e" {
		t.Errorf("tier 2 needsScout expected [impl-e], got: %v", ns2)
	}
	if len(pe2) != 0 {
		t.Errorf("tier 2 preExisting expected empty, got: %v", pe2)
	}

	// Non-existent tier should return empty.
	ns99, pe99 := PartitionIMPLsByStatus(manifest, 99)
	if len(ns99) != 0 || len(pe99) != 0 {
		t.Errorf("non-existent tier should return empty, got ns=%v pe=%v", ns99, pe99)
	}
}

// TestValidateProgram_AllValidProgramStates tests all valid program states pass validation
func TestValidateProgram_AllValidProgramStates(t *testing.T) {
	validStates := []ProgramState{
		ProgramStatePlanning,
		ProgramStateValidating,
		ProgramStateReviewed,
		ProgramStateScaffold,
		ProgramStateTierExecuting,
		ProgramStateTierVerified,
		ProgramStateComplete,
		ProgramStateBlocked,
		ProgramStateNotSuitable,
	}

	for _, state := range validStates {
		t.Run(string(state), func(t *testing.T) {
			manifest := &PROGRAMManifest{
				Title:       "Test Program",
				ProgramSlug: "test-program",
				State:       state,
				Completion:  ProgramCompletion{},
			}

			errs := ValidateProgram(manifest)
			for _, err := range errs {
				if err.Code == "INVALID_STATE" {
					t.Errorf("valid state %q should not trigger INVALID_STATE error: %v", state, err)
				}
			}
		})
	}
}
