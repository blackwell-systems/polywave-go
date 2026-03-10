package protocol

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPostMergeChecklistUnmarshal(t *testing.T) {
	yamlData := `
groups:
  - title: "Build Verification"
    items:
      - description: "Full build passes"
        command: "go build ./..."
`
	var pmc PostMergeChecklist
	if err := yaml.Unmarshal([]byte(yamlData), &pmc); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(pmc.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(pmc.Groups))
	}
	if pmc.Groups[0].Title != "Build Verification" {
		t.Errorf("Expected group title 'Build Verification', got %q", pmc.Groups[0].Title)
	}
}

func TestKnownIssueTitleField(t *testing.T) {
	yamlData := `
title: "Flaky test"
description: "Test fails intermittently"
status: "Pre-existing"
`
	var ki KnownIssue
	if err := yaml.Unmarshal([]byte(yamlData), &ki); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if ki.Title != "Flaky test" {
		t.Errorf("Expected title 'Flaky test', got %q", ki.Title)
	}
}
