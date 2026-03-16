package protocol

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
)

// scoutOutputSchema is the subset of IMPLManifest that a Scout produces.
// Runtime-only fields (completion_reports, stub_reports, merge_state,
// worktrees_created_at, frozen_contracts_hash, frozen_scaffolds_hash) are
// intentionally omitted so that the generated JSON Schema describes only the
// fields a Scout is expected to populate.
type scoutOutputSchema struct {
	Title                 string              `json:"title"`
	FeatureSlug           string              `json:"feature_slug"`
	Verdict               string              `json:"verdict"`
	SuitabilityAssessment string              `json:"suitability_assessment,omitempty"`
	TestCommand           string              `json:"test_command"`
	LintCommand           string              `json:"lint_command"`
	State                 string              `json:"state,omitempty"`
	FileOwnership         []FileOwnership     `json:"file_ownership"`
	InterfaceContracts    []InterfaceContract `json:"interface_contracts"`
	Waves                 []Wave              `json:"waves"`
	QualityGates          *QualityGates       `json:"quality_gates,omitempty"`
	Scaffolds             []ScaffoldFile      `json:"scaffolds,omitempty"`
	PreMortem             *PreMortem          `json:"pre_mortem,omitempty"`
	KnownIssues           []KnownIssue        `json:"known_issues,omitempty"`
	CompletionDate        string              `json:"completion_date,omitempty"`
}

// GenerateScoutSchema returns a JSON Schema (as map[string]any) describing the
// Scout-output subset of IMPLManifest. The schema is generated via reflection
// using github.com/invopop/jsonschema and excludes runtime-only fields such as
// completion_reports, stub_reports, merge_state, worktrees_created_at,
// frozen_contracts_hash, and frozen_scaffolds_hash.
func GenerateScoutSchema() (map[string]any, error) {
	r := &jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	schema := r.Reflect(&scoutOutputSchema{})

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// completionReportOutputSchema is the schema definition for structured completion
// report output. It matches the CompletionReport type defined in protocol/types.go.
type completionReportOutputSchema struct {
	Status              string                `json:"status"`
	Worktree            string                `json:"worktree,omitempty"`
	Branch              string                `json:"branch,omitempty"`
	Commit              string                `json:"commit,omitempty"`
	FilesChanged        []string              `json:"files_changed,omitempty"`
	FilesCreated        []string              `json:"files_created,omitempty"`
	InterfaceDeviations []InterfaceDeviation  `json:"interface_deviations,omitempty"`
	OutOfScopeDeps      []string              `json:"out_of_scope_deps,omitempty"`
	TestsAdded          []string              `json:"tests_added,omitempty"`
	Verification        string                `json:"verification,omitempty"`
	FailureType         string                `json:"failure_type,omitempty"`
	Notes               string                `json:"notes,omitempty"`
}

// GenerateCompletionReportSchema returns a JSON Schema (as map[string]any)
// describing the CompletionReport structure for structured output.
// The schema is generated via reflection using github.com/invopop/jsonschema.
func GenerateCompletionReportSchema() (map[string]any, error) {
	r := &jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	schema := r.Reflect(&completionReportOutputSchema{})

	data, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return result, nil
}
