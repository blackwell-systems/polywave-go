package protocol

import (
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// validManifest returns a fully populated manifest that passes all nested required field checks.
func validManifest() *IMPLManifest {
	return &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Wave: 1},
		},
		InterfaceContracts: []InterfaceContract{
			{Name: "Foo", Definition: "func Foo() error", Location: "pkg/foo/bar.go"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement foo", Files: []string{"pkg/foo/bar.go"}},
				},
			},
		},
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "go build ./...", Required: true},
			},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "pkg/foo/types.go"},
		},
		PreMortem: &PreMortem{
			OverallRisk: "low",
			Rows: []PreMortemRow{
				{Scenario: "Build fails", Likelihood: "low", Impact: "medium", Mitigation: "Fix it"},
			},
		},
	}
}

func TestValidateNestedRequiredFields_Valid(t *testing.T) {
	m := validManifest()
	errs := validateNestedRequiredFields(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid manifest, got %d: %v", len(errs), errs)
	}
}

func TestValidateNestedRequiredFields_EmptyFileOwnershipFile(t *testing.T) {
	m := validManifest()
	m.FileOwnership[0].File = ""
	errs := validateNestedRequiredFields(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeRequiredFieldsMissing && e.Field == "file_ownership[0].file" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V005_REQUIRED_FIELDS_MISSING for file_ownership[0].file, got %v", errs)
	}
}

func TestValidateNestedRequiredFields_EmptyAgentID(t *testing.T) {
	m := validManifest()
	m.Waves[0].Agents[0].ID = ""
	errs := validateNestedRequiredFields(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeRequiredFieldsMissing && e.Field == "waves[0].agents[0].id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V005_REQUIRED_FIELDS_MISSING for waves[0].agents[0].id, got %v", errs)
	}
}

func TestValidateNestedRequiredFields_EmptyAgentTask(t *testing.T) {
	m := validManifest()
	m.Waves[0].Agents[0].Task = ""
	errs := validateNestedRequiredFields(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeRequiredFieldsMissing && e.Field == "waves[0].agents[0].task" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V005_REQUIRED_FIELDS_MISSING for waves[0].agents[0].task, got %v", errs)
	}
}

func TestValidateNestedRequiredFields_ZeroWaveNumber(t *testing.T) {
	m := validManifest()
	m.Waves[0].Number = 0
	errs := validateNestedRequiredFields(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeRequiredFieldsMissing && e.Field == "waves[0].number" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V005_REQUIRED_FIELDS_MISSING for waves[0].number, got %v", errs)
	}
}

func TestValidateNestedRequiredFields_EmptyInterfaceContractName(t *testing.T) {
	m := validManifest()
	m.InterfaceContracts[0].Name = ""
	errs := validateNestedRequiredFields(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeRequiredFieldsMissing && e.Field == "interface_contracts[0].name" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V005_REQUIRED_FIELDS_MISSING for interface_contracts[0].name, got %v", errs)
	}
}

func TestValidateNestedRequiredFields_EmptyScaffoldFilePath(t *testing.T) {
	m := validManifest()
	m.Scaffolds[0].FilePath = ""
	errs := validateNestedRequiredFields(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeRequiredFieldsMissing && e.Field == "scaffolds[0].file_path" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V005_REQUIRED_FIELDS_MISSING for scaffolds[0].file_path, got %v", errs)
	}
}

func TestValidateFilePaths_Valid(t *testing.T) {
	m := validManifest()
	errs := validateFilePaths(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid paths, got %d: %v", len(errs), errs)
	}
}

func TestValidateFilePaths_LeadingSlash(t *testing.T) {
	m := validManifest()
	m.FileOwnership[0].File = "/absolute/path.go"
	errs := validateFilePaths(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeInvalidPath && e.Field == "file_ownership[0].file" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V037_INVALID_PATH for leading slash, got %v", errs)
	}
}

func TestValidateFilePaths_DotDot(t *testing.T) {
	m := validManifest()
	m.FileOwnership[0].File = "pkg/../etc/passwd"
	errs := validateFilePaths(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeInvalidPath && e.Field == "file_ownership[0].file" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V037_INVALID_PATH for '..' traversal, got %v", errs)
	}
}

func TestValidateFilePaths_Backslash(t *testing.T) {
	m := validManifest()
	m.FileOwnership[0].File = "pkg\\foo\\bar.go"
	errs := validateFilePaths(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeInvalidPath && e.Field == "file_ownership[0].file" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V037_INVALID_PATH for backslash, got %v", errs)
	}
}

func TestValidateSchema_CombinesResults(t *testing.T) {
	m := validManifest()
	// Trigger both a required field error and a path error
	m.FileOwnership[0].Agent = ""                  // required field error
	m.Waves[0].Agents[0].Files[0] = "/abs/path.go" // path error

	errs := ValidateSchema(m)

	hasRequired := false
	hasPath := false
	for _, e := range errs {
		if e.Code == result.CodeRequiredFieldsMissing {
			hasRequired = true
		}
		if e.Code == result.CodeInvalidPath {
			hasPath = true
		}
	}

	if !hasRequired {
		t.Error("expected ValidateSchema to include V005_REQUIRED_FIELDS_MISSING errors")
	}
	if !hasPath {
		t.Error("expected ValidateSchema to include V037_INVALID_PATH errors")
	}
}

// Additional edge case tests

func TestValidateNestedRequiredFields_NilOptionalFields(t *testing.T) {
	// Verify that nil QualityGates and nil PreMortem don't cause errors
	m := &IMPLManifest{
		Title:       "Test",
		FeatureSlug: "test",
		Verdict:     "SUITABLE",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1},
		},
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Task: "Do stuff"}}},
		},
		QualityGates: nil,
		PreMortem:    nil,
	}
	errs := validateNestedRequiredFields(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors when optional fields are nil, got %d: %v", len(errs), errs)
	}
}

func TestValidateFilePaths_DotDotNotFalsePositive(t *testing.T) {
	// "..." in a filename should NOT trigger dot-dot detection
	m := validManifest()
	m.FileOwnership[0].File = "pkg/foo.../bar.go"
	errs := validateFilePaths(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for '...' in path, got %d: %v", len(errs), errs)
	}
}

func TestValidateFilePaths_AgentFiles(t *testing.T) {
	m := validManifest()
	m.Waves[0].Agents[0].Files = []string{"../escape.go"}
	errs := validateFilePaths(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeInvalidPath && e.Field == "waves[0].agents[0].files[0]" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V037_INVALID_PATH for agent files with '..', got %v", errs)
	}
}

func TestValidateFilePaths_ScaffoldFiles(t *testing.T) {
	m := validManifest()
	m.Scaffolds[0].FilePath = "/root/scaffold.go"
	errs := validateFilePaths(m)
	found := false
	for _, e := range errs {
		if e.Code == result.CodeInvalidPath && e.Field == "scaffolds[0].file_path" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected V037_INVALID_PATH for scaffold with leading slash, got %v", errs)
	}
}
