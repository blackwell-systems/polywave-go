package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureIMPLDoc is a minimal IMPL doc used by extract tests.
const fixtureIMPLDoc = `# IMPL: Test Feature

## Interface Contracts

` + "```" + `go
type Foo interface {
    Bar() string
}
` + "```" + `

## File Ownership

| File | Agent | Wave | Action |
|------|-------|------|--------|
| pkg/foo/foo.go | A | 1 | new |
| pkg/bar/bar.go | B | 1 | new |

## Scaffolds

Some scaffold content here.

## Quality Gates

level: standard
gates:
  - type: build
    command: go build ./...
    required: true
    description: must compile

## Wave 1

### Agent A: Implement Foo

This is agent A's prompt.
It spans multiple lines.

**Goal:** implement the Foo interface.

### Agent B: Implement Bar

This is agent B's prompt.

### Agent A - Completion Report

` + "```yaml type=impl-completion-report" + `
status: complete
` + "```" + `
`

func writeFixture(t *testing.T, content string) string {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "IMPL-test.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	return path
}

// TestExtractAgentContextFound verifies that a known agent section is extracted
// correctly, including AgentPrompt, InterfaceContracts, and FileOwnership.
func TestExtractAgentContextFound(t *testing.T) {
	path := writeFixture(t, fixtureIMPLDoc)

	payload, err := ExtractAgentContext(path, "A")
	if err != nil {
		t.Fatalf("ExtractAgentContext returned unexpected error: %v", err)
	}
	if payload == nil {
		t.Fatal("ExtractAgentContext returned nil payload")
	}

	// AgentPrompt must contain the agent header and body lines.
	if !strings.Contains(payload.AgentPrompt, "### Agent A") {
		t.Errorf("AgentPrompt missing agent header; got: %q", payload.AgentPrompt)
	}
	if !strings.Contains(payload.AgentPrompt, "agent A's prompt") {
		t.Errorf("AgentPrompt missing expected body text; got: %q", payload.AgentPrompt)
	}

	// AgentPrompt must NOT include Agent B's section.
	if strings.Contains(payload.AgentPrompt, "agent B's prompt") {
		t.Errorf("AgentPrompt leaks Agent B content: %q", payload.AgentPrompt)
	}

	// Completion report header must not bleed into AgentPrompt.
	if strings.Contains(payload.AgentPrompt, "Completion Report") {
		t.Errorf("AgentPrompt contains completion report content: %q", payload.AgentPrompt)
	}

	// InterfaceContracts must contain the Foo interface.
	if !strings.Contains(payload.InterfaceContracts, "Foo") {
		t.Errorf("InterfaceContracts missing expected content; got: %q", payload.InterfaceContracts)
	}

	// FileOwnership must contain the table.
	if !strings.Contains(payload.FileOwnership, "pkg/foo/foo.go") {
		t.Errorf("FileOwnership missing expected content; got: %q", payload.FileOwnership)
	}

	// IMPLDocPath must match the path we provided.
	if payload.IMPLDocPath != path {
		t.Errorf("IMPLDocPath = %q, want %q", payload.IMPLDocPath, path)
	}

	// QualityGates section must be populated.
	if !strings.Contains(payload.QualityGates, "level: standard") {
		t.Errorf("QualityGates missing expected content; got: %q", payload.QualityGates)
	}
}

// TestExtractAgentContextNotFound verifies that an error is returned when the
// requested agent letter is not present in the document.
func TestExtractAgentContextNotFound(t *testing.T) {
	path := writeFixture(t, fixtureIMPLDoc)

	payload, err := ExtractAgentContext(path, "Z")
	if err == nil {
		t.Fatalf("expected error for missing agent Z, got payload: %+v", payload)
	}
	if !strings.Contains(err.Error(), "Z") {
		t.Errorf("error message should mention the missing letter; got: %v", err)
	}
}

// TestFormatAgentContextPayload verifies the output format of FormatAgentContextPayload.
func TestFormatAgentContextPayload(t *testing.T) {
	payload := &AgentContextPayload{
		IMPLDocPath:        "/path/to/IMPL.md",
		AgentPrompt:        "### Agent A: Do things\n\nAgent A content.",
		InterfaceContracts: "type Foo interface{}",
		FileOwnership:      "| pkg/foo.go | A | 1 | new |",
		Scaffolds:          "Scaffold content.",
		QualityGates:       "level: standard",
	}

	out := FormatAgentContextPayload(payload)

	if !strings.HasPrefix(out, "<!-- IMPL doc:") {
		prefix := out
		if len(prefix) > 60 {
			prefix = prefix[:60]
		}
		t.Errorf("output must start with HTML comment; got prefix: %q", prefix)
	}
	if !strings.Contains(out, "/path/to/IMPL.md") {
		t.Errorf("output missing IMPLDocPath; got: %q", out)
	}
	if !strings.Contains(out, "## Interface Contracts") {
		t.Error("output missing ## Interface Contracts heading")
	}
	if !strings.Contains(out, "type Foo interface{}") {
		t.Error("output missing interface contracts content")
	}
	if !strings.Contains(out, "## File Ownership") {
		t.Error("output missing ## File Ownership heading")
	}
	if !strings.Contains(out, "## Scaffolds") {
		t.Error("output missing ## Scaffolds heading")
	}
	if !strings.Contains(out, "## Quality Gates") {
		t.Error("output missing ## Quality Gates heading")
	}
	if !strings.Contains(out, "Agent A content.") {
		t.Error("output missing agent prompt content")
	}
}


