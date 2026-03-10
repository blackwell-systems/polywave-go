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

func TestIMPLManifestPostMergeChecklistField(t *testing.T) {
	yamlData := `
title: "test"
feature_slug: "test"
post_merge_checklist:
  groups:
    - title: "Build"
      items:
        - description: "Full build"
          command: "go build"
`
	var manifest IMPLManifest
	if err := yaml.Unmarshal([]byte(yamlData), &manifest); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if manifest.PostMergeChecklist == nil {
		t.Fatal("Expected PostMergeChecklist to be populated")
	}
	if len(manifest.PostMergeChecklist.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(manifest.PostMergeChecklist.Groups))
	}
}

func TestKnownIssueTitleFieldInManifest(t *testing.T) {
	yamlData := `
title: "test"
feature_slug: "test"
known_issues:
  - title: "Known issue 1"
    description: "Description 1"
    status: "Pre-existing"
  - title: "Known issue 2"
    description: "Description 2"
`
	var manifest IMPLManifest
	if err := yaml.Unmarshal([]byte(yamlData), &manifest); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(manifest.KnownIssues) != 2 {
		t.Fatalf("Expected 2 known issues, got %d", len(manifest.KnownIssues))
	}
	if manifest.KnownIssues[0].Title != "Known issue 1" {
		t.Errorf("Expected title 'Known issue 1', got %q", manifest.KnownIssues[0].Title)
	}
	if manifest.KnownIssues[1].Title != "Known issue 2" {
		t.Errorf("Expected title 'Known issue 2', got %q", manifest.KnownIssues[1].Title)
	}
}
