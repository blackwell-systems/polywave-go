package protocol

import (
	"errors"
	"os"
	"path/filepath"
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
