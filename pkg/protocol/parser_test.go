package protocol

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseIMPLDocMinimal writes a minimal IMPL doc with a Wave 1 header and
// one agent, parses it, and verifies wave count = 1 and agent letter = "A".
func TestParseIMPLDocMinimal(t *testing.T) {
	content := `# IMPL: Engine Extraction

## Wave 1

### Agent A: Copy protocol package

Copy the protocol package files.
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "IMPL-test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	doc, err := ParseIMPLDoc(path)
	if err != nil {
		t.Fatalf("ParseIMPLDoc returned error: %v", err)
	}
	if doc == nil {
		t.Fatal("ParseIMPLDoc returned nil doc")
	}

	if len(doc.Waves) != 1 {
		t.Fatalf("expected 1 wave, got %d", len(doc.Waves))
	}
	if doc.Waves[0].Number != 1 {
		t.Errorf("expected wave number 1, got %d", doc.Waves[0].Number)
	}
	if len(doc.Waves[0].Agents) != 1 {
		t.Fatalf("expected 1 agent in wave 1, got %d", len(doc.Waves[0].Agents))
	}
	if doc.Waves[0].Agents[0].Letter != "A" {
		t.Errorf("expected agent letter 'A', got %q", doc.Waves[0].Agents[0].Letter)
	}
}

// TestParseCompletionReportNotFound verifies that ErrReportNotFound is returned
// for a doc that has no completion report section.
func TestParseCompletionReportNotFound(t *testing.T) {
	content := `# IMPL: Engine Extraction

## Wave 1

### Agent A: Copy protocol package

Copy the protocol package files.
`
	tmp := t.TempDir()
	path := filepath.Join(tmp, "IMPL-test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := ParseCompletionReport(path, "A")
	if err == nil {
		t.Fatal("expected ErrReportNotFound, got nil error")
	}
	if !errors.Is(err, ErrReportNotFound) {
		t.Errorf("expected errors.Is(err, ErrReportNotFound) to be true, got: %v", err)
	}
}

func TestExtractTypedBlock(t *testing.T) {
	input := "```yaml type=impl-test\nline1\nline2\n```\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Scan() // consume opening fence
	lines, found := extractTypedBlock(scanner)
	if !found {
		t.Fatal("Expected block to be found")
	}
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "line1" {
		t.Errorf("Expected first line to be 'line1', got %q", lines[0])
	}
	if lines[1] != "line2" {
		t.Errorf("Expected second line to be 'line2', got %q", lines[1])
	}
}

func TestParseQualityGatesTypedBlock(t *testing.T) {
	input := `## Quality Gates

` + "```yaml type=impl-quality-gates\nlevel: standard\ngates:\n  - type: build\n    command: go build\n    required: true\n```\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Scan() // skip header
	gates := parseQualityGatesSection(scanner)
	if gates == nil {
		t.Fatal("Expected gates to be parsed")
	}
	if gates.Level != "standard" {
		t.Errorf("Expected level 'standard', got %q", gates.Level)
	}
	if len(gates.Gates) != 1 {
		t.Errorf("Expected 1 gate, got %d", len(gates.Gates))
	}
	if len(gates.Gates) > 0 && gates.Gates[0].Type != "build" {
		t.Errorf("Expected gate type 'build', got %q", gates.Gates[0].Type)
	}
}

func TestParsePostMergeChecklistTypedBlock(t *testing.T) {
	input := `## Post-Merge Checklist

` + "```yaml type=impl-post-merge-checklist\ngroups:\n  - title: \"Build Verification\"\n    items:\n      - description: \"Full build passes\"\n        command: \"go build ./...\"\n```\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Scan() // skip header
	pmc := parsePostMergeChecklistSection(scanner)
	if pmc == nil {
		t.Fatal("Expected checklist to be parsed")
	}
	if len(pmc.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(pmc.Groups))
	}
	if len(pmc.Groups) > 0 && pmc.Groups[0].Title != "Build Verification" {
		t.Errorf("Expected group title 'Build Verification', got %q", pmc.Groups[0].Title)
	}
	if len(pmc.Groups) > 0 && len(pmc.Groups[0].Items) != 1 {
		t.Errorf("Expected 1 item in group, got %d", len(pmc.Groups[0].Items))
	}
}

func TestParseKnownIssuesTypedBlock(t *testing.T) {
	input := `## Known Issues

` + "```yaml type=impl-known-issues\n- title: \"Flaky test\"\n  description: \"Test fails intermittently\"\n  status: \"open\"\n```\n"
	scanner := bufio.NewScanner(strings.NewReader(input))
	scanner.Scan() // skip header
	issues := parseKnownIssuesSection(scanner)
	if issues == nil {
		t.Fatal("Expected issues to be parsed")
	}
	if len(issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(issues))
	}
	if len(issues) > 0 && issues[0].Description != "Test fails intermittently" {
		t.Errorf("Expected description 'Test fails intermittently', got %q", issues[0].Description)
	}
}
