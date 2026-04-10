package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// writeTestManifest writes an IMPLManifest to a temp file and returns the path.
func writeTestManifest(t *testing.T, m *protocol.IMPLManifest) string {
	t.Helper()
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "IMPL-test.yaml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

// twoAgentManifest builds a minimal manifest with two wave agents,
// where agent A has a completion report.
func twoAgentManifest() *protocol.IMPLManifest {
	return &protocol.IMPLManifest{
		Title:       "Test IMPL",
		FeatureSlug: "test-feature",
		State:       "WAVE_EXECUTING",
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Task A"},
					{ID: "B", Task: "Task B"},
				},
			},
		},
		CompletionReports: map[string]protocol.CompletionReport{
			"A": {
				Status: protocol.StatusComplete,
				Commit: "abc123",
				Branch: "saw/test-feature/wave1-agent-A",
			},
		},
	}
}

func TestAgentStatus_JSONOutput(t *testing.T) {
	m := twoAgentManifest()
	path := writeTestManifest(t, m)

	cmd := newAgentStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	cmd.SetArgs([]string{"--json", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AgentStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal JSON output: %v", err)
	}

	if result.IMPLSlug != "test-feature" {
		t.Errorf("expected impl_slug=test-feature, got %q", result.IMPLSlug)
	}

	ids := make(map[string]bool)
	for _, row := range result.Agents {
		ids[row.ID] = true
	}
	if !ids["A"] {
		t.Error("expected agent A in results")
	}
	if !ids["B"] {
		t.Error("expected agent B in results")
	}
}

func TestAgentStatus_TableOutput(t *testing.T) {
	m := twoAgentManifest()
	path := writeTestManifest(t, m)

	cmd := newAgentStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	for _, header := range []string{"Agent", "Role", "Branch", "Commits", "Report", "Status"} {
		if !strings.Contains(output, header) {
			t.Errorf("expected table header %q in output", header)
		}
	}

	// Both agents should appear as rows.
	if !strings.Contains(output, "A") {
		t.Error("expected agent A row in table")
	}
	if !strings.Contains(output, "B") {
		t.Error("expected agent B row in table")
	}
}

func TestAgentStatus_CriticRow(t *testing.T) {
	m := twoAgentManifest()
	m.CriticReport = &protocol.CriticData{
		Verdict: "PASS",
		Summary: "All good",
	}
	path := writeTestManifest(t, m)

	cmd := newAgentStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	cmd.SetArgs([]string{"--json", path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result AgentStatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal JSON output: %v", err)
	}

	found := false
	for _, row := range result.Agents {
		if row.ID == "critic" {
			found = true
			if row.Role != "critic" {
				t.Errorf("expected critic role=critic, got %q", row.Role)
			}
			if row.Branch != "(main)" {
				t.Errorf("expected critic branch=(main), got %q", row.Branch)
			}
			if !row.HasReport {
				t.Error("expected critic HasReport=true")
			}
		}
	}
	if !found {
		t.Error("expected critic row in output")
	}
}

func TestAgentStatus_NoAgents(t *testing.T) {
	m := &protocol.IMPLManifest{
		Title:       "Empty IMPL",
		FeatureSlug: "empty-impl",
		State:       "SCOUT_COMPLETE",
		Waves:       []protocol.Wave{},
	}
	path := writeTestManifest(t, m)

	cmd := newAgentStatusCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	cmd.SetArgs([]string{path})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	// Should print IMPL header without crashing.
	if !strings.Contains(output, "empty-impl") {
		t.Errorf("expected IMPL slug in output, got: %q", output)
	}
}
