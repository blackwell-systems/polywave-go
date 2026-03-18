package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseProgramManifest_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	validManifest := `title: "Test Program"
program_slug: "test-program"
state: "PLANNING"
created: "2024-01-01"
updated: "2024-01-02"
requirements: "Build a test system"
impls:
  - slug: "impl-alpha"
    title: "Alpha Implementation"
    tier: 1
    status: "pending"
  - slug: "impl-beta"
    title: "Beta Implementation"
    tier: 2
    depends_on: ["impl-alpha"]
    status: "pending"
tiers:
  - number: 1
    impls: ["impl-alpha"]
    description: "First tier"
  - number: 2
    impls: ["impl-beta"]
    description: "Second tier"
completion:
  tiers_complete: 0
  tiers_total: 2
  impls_complete: 0
  impls_total: 2
  total_agents: 10
  total_waves: 5
`

	path := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	if err := os.WriteFile(path, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse manifest
	manifest, err := ParseProgramManifest(path)
	if err != nil {
		t.Fatalf("ParseProgramManifest failed: %v", err)
	}

	// Verify basic fields
	if manifest.Title != "Test Program" {
		t.Errorf("expected title 'Test Program', got '%s'", manifest.Title)
	}
	if manifest.ProgramSlug != "test-program" {
		t.Errorf("expected program_slug 'test-program', got '%s'", manifest.ProgramSlug)
	}
	if manifest.State != ProgramStatePlanning {
		t.Errorf("expected state 'PLANNING', got '%s'", manifest.State)
	}
	if manifest.Requirements != "Build a test system" {
		t.Errorf("expected requirements 'Build a test system', got '%s'", manifest.Requirements)
	}

	// Verify IMPLs
	if len(manifest.Impls) != 2 {
		t.Fatalf("expected 2 impls, got %d", len(manifest.Impls))
	}
	if manifest.Impls[0].Slug != "impl-alpha" {
		t.Errorf("expected first impl slug 'impl-alpha', got '%s'", manifest.Impls[0].Slug)
	}
	if manifest.Impls[0].Tier != 1 {
		t.Errorf("expected first impl tier 1, got %d", manifest.Impls[0].Tier)
	}
	if manifest.Impls[1].Slug != "impl-beta" {
		t.Errorf("expected second impl slug 'impl-beta', got '%s'", manifest.Impls[1].Slug)
	}
	if manifest.Impls[1].Tier != 2 {
		t.Errorf("expected second impl tier 2, got %d", manifest.Impls[1].Tier)
	}
	if len(manifest.Impls[1].DependsOn) != 1 || manifest.Impls[1].DependsOn[0] != "impl-alpha" {
		t.Errorf("expected second impl depends_on ['impl-alpha'], got %v", manifest.Impls[1].DependsOn)
	}

	// Verify Tiers
	if len(manifest.Tiers) != 2 {
		t.Fatalf("expected 2 tiers, got %d", len(manifest.Tiers))
	}
	if manifest.Tiers[0].Number != 1 {
		t.Errorf("expected first tier number 1, got %d", manifest.Tiers[0].Number)
	}
	if len(manifest.Tiers[0].Impls) != 1 || manifest.Tiers[0].Impls[0] != "impl-alpha" {
		t.Errorf("expected first tier impls ['impl-alpha'], got %v", manifest.Tiers[0].Impls)
	}

	// Verify Completion
	if manifest.Completion.TiersTotal != 2 {
		t.Errorf("expected tiers_total 2, got %d", manifest.Completion.TiersTotal)
	}
	if manifest.Completion.ImplsTotal != 2 {
		t.Errorf("expected impls_total 2, got %d", manifest.Completion.ImplsTotal)
	}
	if manifest.Completion.TotalAgents != 10 {
		t.Errorf("expected total_agents 10, got %d", manifest.Completion.TotalAgents)
	}
}

func TestParseProgramManifest_FileNotFound(t *testing.T) {
	// Try to parse a file that doesn't exist
	path := "/nonexistent/PROGRAM-missing.yaml"

	_, err := ParseProgramManifest(path)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	// Verify error mentions file read failure
	errMsg := err.Error()
	if !contains(errMsg, "failed to read PROGRAM manifest file") {
		t.Errorf("expected error to mention file read failure, got: %s", errMsg)
	}
}

func TestParseProgramManifest_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	invalidManifest := `this is not valid YAML
{{{{ broken syntax
: : : :::
`

	path := filepath.Join(tmpDir, "PROGRAM-invalid.yaml")
	if err := os.WriteFile(path, []byte(invalidManifest), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ParseProgramManifest(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}

	// Verify error mentions parse failure
	errMsg := err.Error()
	if !contains(errMsg, "failed to parse PROGRAM manifest YAML") {
		t.Errorf("expected error to mention YAML parse failure, got: %s", errMsg)
	}
}

func TestParseProgramManifest_AllFields(t *testing.T) {
	tmpDir := t.TempDir()

	fullManifest := `title: "Complete Program"
program_slug: "complete-program"
state: "TIER_EXECUTING"
created: "2024-01-01"
updated: "2024-01-15"
requirements: "Comprehensive requirements"
program_contracts:
  - name: "CoreAPI"
    description: "Core API interface"
    definition: "type CoreAPI interface { DoWork() error }"
    location: "pkg/core/api.go"
    freeze_at: "tier-2-start"
    consumers:
      - impl: "impl-consumer"
        usage: "Calls DoWork() in main flow"
impls:
  - slug: "impl-alpha"
    title: "Alpha Implementation"
    tier: 1
    estimated_agents: 3
    estimated_waves: 2
    key_outputs: ["pkg/alpha/main.go", "pkg/alpha/helper.go"]
    status: "complete"
  - slug: "impl-beta"
    title: "Beta Implementation"
    tier: 2
    depends_on: ["impl-alpha"]
    estimated_agents: 5
    estimated_waves: 3
    key_outputs: ["pkg/beta/main.go"]
    status: "in-progress"
tiers:
  - number: 1
    impls: ["impl-alpha"]
    description: "Foundation tier"
  - number: 2
    impls: ["impl-beta"]
    description: "Integration tier"
tier_gates:
  - type: "build"
    command: "go build ./..."
    required: true
  - type: "test"
    command: "go test ./..."
    required: true
    description: "Full test suite"
completion:
  tiers_complete: 1
  tiers_total: 2
  impls_complete: 1
  impls_total: 2
  total_agents: 8
  total_waves: 5
pre_mortem:
  - scenario: "Integration complexity"
    likelihood: "medium"
    impact: "high"
    mitigation: "Add extra integration wave"
`

	path := filepath.Join(tmpDir, "PROGRAM-complete.yaml")
	if err := os.WriteFile(path, []byte(fullManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse manifest
	manifest, err := ParseProgramManifest(path)
	if err != nil {
		t.Fatalf("ParseProgramManifest failed: %v", err)
	}

	// Verify all sections are populated
	if manifest.Title != "Complete Program" {
		t.Errorf("expected title 'Complete Program', got '%s'", manifest.Title)
	}
	if manifest.ProgramSlug != "complete-program" {
		t.Errorf("expected program_slug 'complete-program', got '%s'", manifest.ProgramSlug)
	}
	if manifest.State != ProgramStateTierExecuting {
		t.Errorf("expected state 'TIER_EXECUTING', got '%s'", manifest.State)
	}
	if manifest.Created != "2024-01-01" {
		t.Errorf("expected created '2024-01-01', got '%s'", manifest.Created)
	}
	if manifest.Updated != "2024-01-15" {
		t.Errorf("expected updated '2024-01-15', got '%s'", manifest.Updated)
	}

	// Verify program_contracts
	if len(manifest.ProgramContracts) != 1 {
		t.Fatalf("expected 1 program contract, got %d", len(manifest.ProgramContracts))
	}
	contract := manifest.ProgramContracts[0]
	if contract.Name != "CoreAPI" {
		t.Errorf("expected contract name 'CoreAPI', got '%s'", contract.Name)
	}
	if contract.Location != "pkg/core/api.go" {
		t.Errorf("expected contract location 'pkg/core/api.go', got '%s'", contract.Location)
	}
	if contract.FreezeAt != "tier-2-start" {
		t.Errorf("expected freeze_at 'tier-2-start', got '%s'", contract.FreezeAt)
	}
	if len(contract.Consumers) != 1 {
		t.Fatalf("expected 1 consumer, got %d", len(contract.Consumers))
	}
	if contract.Consumers[0].Impl != "impl-consumer" {
		t.Errorf("expected consumer impl 'impl-consumer', got '%s'", contract.Consumers[0].Impl)
	}

	// Verify impls with all fields
	if len(manifest.Impls) != 2 {
		t.Fatalf("expected 2 impls, got %d", len(manifest.Impls))
	}
	impl1 := manifest.Impls[0]
	if impl1.EstimatedAgents != 3 {
		t.Errorf("expected estimated_agents 3, got %d", impl1.EstimatedAgents)
	}
	if impl1.EstimatedWaves != 2 {
		t.Errorf("expected estimated_waves 2, got %d", impl1.EstimatedWaves)
	}
	if len(impl1.KeyOutputs) != 2 {
		t.Fatalf("expected 2 key_outputs, got %d", len(impl1.KeyOutputs))
	}
	if impl1.Status != "complete" {
		t.Errorf("expected status 'complete', got '%s'", impl1.Status)
	}

	// Verify tier_gates
	if len(manifest.TierGates) != 2 {
		t.Fatalf("expected 2 tier_gates, got %d", len(manifest.TierGates))
	}
	gate1 := manifest.TierGates[0]
	if gate1.Type != "build" {
		t.Errorf("expected gate type 'build', got '%s'", gate1.Type)
	}
	if !gate1.Required {
		t.Errorf("expected gate to be required")
	}
	gate2 := manifest.TierGates[1]
	if gate2.Description != "Full test suite" {
		t.Errorf("expected gate description 'Full test suite', got '%s'", gate2.Description)
	}

	// Verify completion
	if manifest.Completion.TiersComplete != 1 {
		t.Errorf("expected tiers_complete 1, got %d", manifest.Completion.TiersComplete)
	}
	if manifest.Completion.ImplsComplete != 1 {
		t.Errorf("expected impls_complete 1, got %d", manifest.Completion.ImplsComplete)
	}

	// Verify pre_mortem
	if len(manifest.PreMortem) != 1 {
		t.Fatalf("expected 1 pre_mortem row, got %d", len(manifest.PreMortem))
	}
	preMortem := manifest.PreMortem[0]
	if preMortem.Scenario != "Integration complexity" {
		t.Errorf("expected scenario 'Integration complexity', got '%s'", preMortem.Scenario)
	}
	if preMortem.Likelihood != "medium" {
		t.Errorf("expected likelihood 'medium', got '%s'", preMortem.Likelihood)
	}
	if preMortem.Mitigation != "Add extra integration wave" {
		t.Errorf("expected mitigation 'Add extra integration wave', got '%s'", preMortem.Mitigation)
	}
}
