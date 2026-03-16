package protocol

import (
	"testing"
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
	if errs[0].Code != SV01UnknownKey {
		t.Errorf("expected code %s, got %s", SV01UnknownKey, errs[0].Code)
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
	if errs[0].Code != SV01UnknownKey {
		t.Errorf("expected code %s, got %s", SV01UnknownKey, errs[0].Code)
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
	if errs[0].Code != SV01UnknownKey {
		t.Errorf("expected code %s, got %s", SV01UnknownKey, errs[0].Code)
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
	if errs[0].Code != SV01UnknownKey {
		t.Errorf("expected code %s, got %s", SV01UnknownKey, errs[0].Code)
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
		if e.Code != SV01UnknownKey {
			t.Errorf("expected code %s, got %s", SV01UnknownKey, e.Code)
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
	if errs[0].Code != SV01UnknownKey {
		t.Errorf("expected code %s, got %s", SV01UnknownKey, errs[0].Code)
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
