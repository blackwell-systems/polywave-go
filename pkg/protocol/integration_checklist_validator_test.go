package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// TestValidateIntegrationChecklist_NoHandlers verifies that a manifest with no
// API handler or component files in file_ownership produces no warnings.
func TestValidateIntegrationChecklist_NoHandlers(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/protocol/foo.go", Agent: "A", Action: "new"},
			{File: "cmd/saw/main.go", Agent: "B", Action: "modify"},
		},
		PostMergeChecklist: nil,
	}

	errs := ValidateIntegrationChecklist(m, "")
	if len(errs) != 0 {
		t.Errorf("expected no warnings, got %d: %+v", len(errs), errs)
	}
}

// TestValidateIntegrationChecklist_HasChecklist verifies that when new handlers
// are declared but the manifest already has a post_merge_checklist, no warning
// is emitted.
func TestValidateIntegrationChecklist_HasChecklist(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/api/users_handler.go", Agent: "A", Action: "new"},
		},
		PostMergeChecklist: &PostMergeChecklist{
			Groups: []ChecklistGroup{
				{
					Title: "API Integration",
					Items: []ChecklistItem{
						{Description: "Register route in router"},
					},
				},
			},
		},
	}

	errs := ValidateIntegrationChecklist(m, "")
	if len(errs) != 0 {
		t.Errorf("expected no warnings with checklist present, got %d: %+v", len(errs), errs)
	}
}

// TestValidateIntegrationChecklist_MissingChecklist verifies that a new API handler
// file combined with a nil post_merge_checklist triggers E16_MISSING_CHECKLIST.
func TestValidateIntegrationChecklist_MissingChecklist(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/api/foo_handler.go", Agent: "A", Action: "new"},
		},
		PostMergeChecklist: nil,
	}

	errs := ValidateIntegrationChecklist(m, "")
	if len(errs) != 1 {
		t.Fatalf("expected 1 warning, got %d: %+v", len(errs), errs)
	}
	if errs[0].Code != result.CodeMissingChecklist {
		t.Errorf("expected code E16_MISSING_CHECKLIST, got %q", errs[0].Code)
	}
	if errs[0].Field != "post_merge_checklist" {
		t.Errorf("expected field 'post_merge_checklist', got %q", errs[0].Field)
	}
}

// TestValidateIntegrationChecklist_ComponentsDetected verifies that a new React
// component file combined with an empty post_merge_checklist triggers a warning.
func TestValidateIntegrationChecklist_ComponentsDetected(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "web/src/components/Bar.tsx", Agent: "B", Action: "new"},
		},
		PostMergeChecklist: &PostMergeChecklist{
			Groups: []ChecklistGroup{}, // empty — no items
		},
	}

	errs := ValidateIntegrationChecklist(m, "")
	if len(errs) != 1 {
		t.Fatalf("expected 1 warning for empty checklist, got %d: %+v", len(errs), errs)
	}
	if errs[0].Code != result.CodeMissingChecklist {
		t.Errorf("expected code E16_MISSING_CHECKLIST, got %q", errs[0].Code)
	}
}

// TestValidateIntegrationChecklist_ActionModifyNotWarned verifies that modify-action
// handler files do not trigger the checklist warning (only action=new is relevant).
func TestValidateIntegrationChecklist_ActionModifyNotWarned(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/api/users_handler.go", Agent: "A", Action: "modify"},
		},
		PostMergeChecklist: nil,
	}

	errs := ValidateIntegrationChecklist(m, "")
	if len(errs) != 0 {
		t.Errorf("modify-action handler should not trigger warning, got %d: %+v", len(errs), errs)
	}
}

// TestValidateIntegrationChecklist_RepoPathSkipsMissing verifies that when repoPath
// is set and the matched file does not exist on disk, no warning is emitted (avoids
// false positives on typos in the IMPL doc).
func TestValidateIntegrationChecklist_RepoPathSkipsMissing(t *testing.T) {
	// Use a temp dir as repo root — the handler file won't exist there.
	tmpDir := t.TempDir()

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/api/ghost_handler.go", Agent: "A", Action: "new"},
		},
		PostMergeChecklist: nil,
	}

	errs := ValidateIntegrationChecklist(m, tmpDir)
	if len(errs) != 0 {
		t.Errorf("file not on disk should not trigger warning, got %d: %+v", len(errs), errs)
	}
}

// TestValidateIntegrationChecklist_RepoPathWarnsWhenPresent verifies that when
// repoPath is set and the matched file DOES exist on disk, the warning is emitted.
func TestValidateIntegrationChecklist_RepoPathWarnsWhenPresent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the handler file so existence check passes.
	handlerDir := filepath.Join(tmpDir, "pkg", "api")
	if err := os.MkdirAll(handlerDir, 0755); err != nil {
		t.Fatal(err)
	}
	handlerFile := filepath.Join(handlerDir, "real_handler.go")
	if err := os.WriteFile(handlerFile, []byte("package api"), 0644); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/api/real_handler.go", Agent: "A", Action: "new"},
		},
		PostMergeChecklist: nil,
	}

	errs := ValidateIntegrationChecklist(m, tmpDir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 warning when file exists on disk, got %d: %+v", len(errs), errs)
	}
	if errs[0].Code != result.CodeMissingChecklist {
		t.Errorf("expected E16_MISSING_CHECKLIST, got %q", errs[0].Code)
	}
}
