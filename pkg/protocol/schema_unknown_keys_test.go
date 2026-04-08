package protocol

import (
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

func TestDetectUnknownKeys_ValidManifest(t *testing.T) {
	yaml := []byte(`
title: "Test Feature"
feature_slug: test-feature
verdict: SUITABLE
test_command: "go test ./..."
lint_command: "go vet ./..."
file_ownership:
  - file: pkg/foo.go
    agent: A
    wave: 1
    action: new
waves:
  - number: 1
    agents:
      - id: A
        task: "Build foo"
        files:
          - pkg/foo.go
interface_contracts:
  - name: Foo
    description: "Foo interface"
    definition: "func Foo() error"
    location: pkg/foo.go
quality_gates:
  level: standard
  gates:
    - type: build
      command: "go build ./..."
      required: true
scaffolds:
  - file_path: pkg/types.go
    contents: "package pkg"
    status: committed
pre_mortem:
  overall_risk: low
  rows:
    - scenario: "Build fails"
      likelihood: low
      impact: medium
      mitigation: "Fix it"
known_issues:
  - title: "None"
    description: "No issues"
    status: resolved
    workaround: "N/A"
state: SCOUT_PENDING
`)
	errs := DetectUnknownKeys(yaml)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid manifest, got %d: %v", len(errs), errs)
	}
}

func TestDetectUnknownKeys_TopLevelUnknown(t *testing.T) {
	yaml := []byte(`
title: "Test"
titl: "Typo"
verdict: SUITABLE
`)
	errs := DetectUnknownKeys(yaml)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeUnknownKey {
		t.Errorf("expected code %s, got %s", result.CodeUnknownKey, errs[0].Code)
	}
	if errs[0].Field != "titl" {
		t.Errorf("expected field 'titl', got '%s'", errs[0].Field)
	}
}

func TestDetectUnknownKeys_FileOwnershipUnknown(t *testing.T) {
	yaml := []byte(`
title: "Test"
file_ownership:
  - flie: pkg/foo.go
    agent: A
    wave: 1
`)
	errs := DetectUnknownKeys(yaml)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeUnknownKey {
		t.Errorf("expected code %s, got %s", result.CodeUnknownKey, errs[0].Code)
	}
	if errs[0].Field != "file_ownership[0].flie" {
		t.Errorf("expected field 'file_ownership[0].flie', got '%s'", errs[0].Field)
	}
}

func TestDetectUnknownKeys_AgentUnknown(t *testing.T) {
	yaml := []byte(`
title: "Test"
waves:
  - number: 1
    agents:
      - id: A
        taks: "Build foo"
        files:
          - pkg/foo.go
`)
	errs := DetectUnknownKeys(yaml)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeUnknownKey {
		t.Errorf("expected code %s, got %s", result.CodeUnknownKey, errs[0].Code)
	}
	if errs[0].Field != "waves[0].agents[0].taks" {
		t.Errorf("expected field 'waves[0].agents[0].taks', got '%s'", errs[0].Field)
	}
}

func TestDetectUnknownKeys_WaveUnknown(t *testing.T) {
	yaml := []byte(`
title: "Test"
waves:
  - number: 1
    agentss: []
`)
	errs := DetectUnknownKeys(yaml)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeUnknownKey {
		t.Errorf("expected code %s, got %s", result.CodeUnknownKey, errs[0].Code)
	}
	if errs[0].Field != "waves[0].agentss" {
		t.Errorf("expected field 'waves[0].agentss', got '%s'", errs[0].Field)
	}
}

func TestDetectUnknownKeys_MultipleUnknown(t *testing.T) {
	yaml := []byte(`
title: "Test"
foo: bar
baz: qux
verdict: SUITABLE
`)
	errs := DetectUnknownKeys(yaml)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
	// Verify both are unknown key errors
	for _, e := range errs {
		if e.Code != result.CodeUnknownKey {
			t.Errorf("expected code %s, got %s", result.CodeUnknownKey, e.Code)
		}
	}
}

func TestDetectUnknownKeys_NestedDeep(t *testing.T) {
	yaml := []byte(`
title: "Test"
quality_gates:
  level: standard
  gates:
    - type: build
      command: "go build ./..."
      required: true
      bogus_field: "should fail"
`)
	errs := DetectUnknownKeys(yaml)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != result.CodeUnknownKey {
		t.Errorf("expected code %s, got %s", result.CodeUnknownKey, errs[0].Code)
	}
	if errs[0].Field != "quality_gates.gates[0].bogus_field" {
		t.Errorf("expected field 'quality_gates.gates[0].bogus_field', got '%s'", errs[0].Field)
	}
}

func TestDetectUnknownKeys_EmptyManifest(t *testing.T) {
	yaml := []byte(``)
	errs := DetectUnknownKeys(yaml)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty manifest, got %d: %v", len(errs), errs)
	}
}

func TestDetectUnknownKeys_IntegrationConnectors(t *testing.T) {
	yamlData := []byte(`
title: "Test"
integration_connectors:
  - name: "wire A->B"
integration_reports:
  A:
    status: complete
`)
	errs := DetectUnknownKeys(yamlData)
	if len(errs) != 0 {
		t.Errorf("integration_connectors and integration_reports should be valid, got %d errors: %v", len(errs), errs)
	}
}

func TestDetectUnknownKeys_QualityGateRepo(t *testing.T) {
	yamlData := []byte(`
title: "Test"
quality_gates:
  level: standard
  gates:
    - type: build
      command: "go build ./..."
      required: true
      repo: scout-and-wave-go
`)
	errs := DetectUnknownKeys(yamlData)
	if len(errs) != 0 {
		t.Errorf("quality_gate.repo should be valid, got %d errors: %v", len(errs), errs)
	}
}

func TestStripUnknownKeys_RemovesUnknown(t *testing.T) {
	input := []byte(`title: "Test Feature"
verdict: SUITABLE
dep_graph: "some graph text"
cascade_candidates:
  - foo
  - bar
state: SCOUT_PENDING
`)
	cleaned, stripped, err := StripUnknownKeys(input)
	if err != nil {
		t.Fatalf("StripUnknownKeys returned error: %v", err)
	}
	if len(stripped) != 2 {
		t.Fatalf("expected 2 stripped keys, got %d: %v", len(stripped), stripped)
	}
	// stripped is sorted
	if stripped[0] != "cascade_candidates" || stripped[1] != "dep_graph" {
		t.Errorf("expected [cascade_candidates dep_graph], got %v", stripped)
	}
	// Verify known keys are preserved
	cleanedStr := string(cleaned)
	if !strings.Contains(cleanedStr, "title:") {
		t.Error("cleaned YAML should contain 'title'")
	}
	if !strings.Contains(cleanedStr, "verdict:") {
		t.Error("cleaned YAML should contain 'verdict'")
	}
	if !strings.Contains(cleanedStr, "state:") {
		t.Error("cleaned YAML should contain 'state'")
	}
	// Verify unknown keys are gone
	if strings.Contains(cleanedStr, "dep_graph") {
		t.Error("cleaned YAML should not contain 'dep_graph'")
	}
	if strings.Contains(cleanedStr, "cascade_candidates") {
		t.Error("cleaned YAML should not contain 'cascade_candidates'")
	}
}

func TestStripUnknownKeys_NoUnknown(t *testing.T) {
	input := []byte(`title: "Test Feature"
verdict: SUITABLE
state: SCOUT_PENDING
`)
	cleaned, stripped, err := StripUnknownKeys(input)
	if err != nil {
		t.Fatalf("StripUnknownKeys returned error: %v", err)
	}
	if len(stripped) != 0 {
		t.Errorf("expected 0 stripped keys, got %d: %v", len(stripped), stripped)
	}
	// Should return original bytes unchanged
	if string(cleaned) != string(input) {
		t.Error("expected original bytes returned when no unknown keys")
	}
}
