package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckAgentComplexity(t *testing.T) {
	makeOwnership := func(agent, action string, count int) []FileOwnership {
		entries := make([]FileOwnership, count)
		for i := range entries {
			entries[i] = FileOwnership{
				File:   "file" + string(rune('a'+i)),
				Agent:  agent,
				Wave:   1,
				Action: action,
			}
		}
		return entries
	}

	tests := []struct {
		name         string
		manifest     *IMPLManifest
		wantWarnings int
		wantCode     string
	}{
		{
			name: "agent with 9 files owned — expect 1 warning",
			manifest: &IMPLManifest{
				FileOwnership: makeOwnership("A", "modify", 9),
			},
			wantWarnings: 1,
			wantCode:     "W001_AGENT_SCOPE_LARGE",
		},
		{
			name: "agent with 8 files owned — expect 0 warnings (at threshold, not over)",
			manifest: &IMPLManifest{
				FileOwnership: makeOwnership("A", "modify", 8),
			},
			wantWarnings: 0,
		},
		{
			name: "agent with 6 new files — expect 1 warning",
			manifest: &IMPLManifest{
				FileOwnership: makeOwnership("A", "new", 6),
			},
			wantWarnings: 1,
			wantCode:     "W001_AGENT_SCOPE_LARGE",
		},
		{
			name: "agent with 5 new files — expect 0 warnings (at threshold, not over)",
			manifest: &IMPLManifest{
				FileOwnership: makeOwnership("A", "new", 5),
			},
			wantWarnings: 0,
		},
		{
			name: "agent with 9 total AND 6 new — expect 2 warnings",
			manifest: &IMPLManifest{
				FileOwnership: func() []FileOwnership {
					// 9 entries total: 6 "new" + 3 "modify"
					entries := makeOwnership("A", "new", 6)
					entries = append(entries, makeOwnership("A", "modify", 3)...)
					// fix file names to avoid collision
					for i := range entries {
						entries[i].File = "file_" + string(rune('a'+i))
					}
					return entries
				}(),
			},
			wantWarnings: 2,
			wantCode:     "W001_AGENT_SCOPE_LARGE",
		},
		{
			name: "empty manifest — expect 0 warnings",
			manifest: &IMPLManifest{
				FileOwnership: nil,
			},
			wantWarnings: 0,
		},
		{
			name: "two agents: one with 9 files, one with 3 — only oversized agent warns",
			manifest: &IMPLManifest{
				FileOwnership: func() []FileOwnership {
					entries := makeOwnership("A", "modify", 9)
					small := makeOwnership("B", "modify", 3)
					// fix file names to avoid collision
					for i := range small {
						small[i].File = "small_" + string(rune('a'+i))
					}
					return append(entries, small...)
				}(),
			},
			wantWarnings: 1,
			wantCode:     "W001_AGENT_SCOPE_LARGE",
		},
	}

	// V047 trivial scope cases
	v047Tests := []struct {
		name      string
		manifest  *IMPLManifest
		wantError bool
	}{
		{
			name: "1 agent 1 file SUITABLE — expect V047 error",
			manifest: &IMPLManifest{
				Verdict: "SUITABLE",
				Waves:   []Wave{{Number: 1, Agents: []Agent{{ID: "A"}}}},
				FileOwnership: []FileOwnership{
					{File: "pkg/foo.go", Agent: "A", Wave: 1},
				},
			},
			wantError: true,
		},
		{
			name: "2 agents 2 files SUITABLE — no V047",
			manifest: &IMPLManifest{
				Verdict: "SUITABLE",
				Waves:   []Wave{{Number: 1, Agents: []Agent{{ID: "A"}, {ID: "B"}}}},
				FileOwnership: []FileOwnership{
					{File: "pkg/foo.go", Agent: "A", Wave: 1},
					{File: "pkg/bar.go", Agent: "B", Wave: 1},
				},
			},
			wantError: false,
		},
		{
			name: "1 agent 1 file NOT_SUITABLE — no V047 (already rejected)",
			manifest: &IMPLManifest{
				Verdict: "NOT_SUITABLE",
				Waves:   []Wave{{Number: 1, Agents: []Agent{{ID: "A"}}}},
				FileOwnership: []FileOwnership{
					{File: "pkg/foo.go", Agent: "A", Wave: 1},
				},
			},
			wantError: false,
		},
	}
	for _, tt := range v047Tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckAgentComplexity(tt.manifest)
			hasV047 := false
			for _, e := range got {
				if e.Code == "V047_TRIVIAL_SCOPE" {
					hasV047 = true
					if e.Severity != "error" {
						t.Errorf("V047 severity = %q, want \"error\"", e.Severity)
					}
				}
			}
			if hasV047 != tt.wantError {
				t.Errorf("hasV047 = %v, want %v; errors: %v", hasV047, tt.wantError, got)
			}
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckAgentComplexity(tt.manifest)
			if len(got) != tt.wantWarnings {
				t.Errorf("CheckAgentComplexity() returned %d warnings, want %d; warnings: %v", len(got), tt.wantWarnings, got)
			}
			if tt.wantCode != "" {
				for _, w := range got {
					if w.Code != tt.wantCode {
						t.Errorf("warning code = %q, want %q", w.Code, tt.wantCode)
					}
					if w.Severity != "warning" {
						t.Errorf("warning severity = %q, want %q", w.Severity, "warning")
					}
					if w.Field != "file_ownership" {
						t.Errorf("warning field = %q, want %q", w.Field, "file_ownership")
					}
				}
			}
		})
	}
}

// writeLines writes a file with n lines (each line is "x\n").
func writeLines(t *testing.T, path string, n int) {
	t.Helper()
	content := strings.Repeat("x\n", n)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeLines: %v", err)
	}
}

func TestCheckAgentLOCBudget(t *testing.T) {
	t.Run("repoPath empty returns nil (offline mode)", func(t *testing.T) {
		m := &IMPLManifest{
			FileOwnership: []FileOwnership{
				{File: "pkg/foo.go", Agent: "A", Wave: 1, Action: "modify"},
			},
		}
		got := CheckAgentLOCBudget(m, "", 2000)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("agent with one file under 2000 lines — no errors", func(t *testing.T) {
		dir := t.TempDir()
		writeLines(t, filepath.Join(dir, "foo.go"), 100)
		m := &IMPLManifest{
			FileOwnership: []FileOwnership{
				{File: "foo.go", Agent: "A", Wave: 1, Action: "modify"},
			},
		}
		got := CheckAgentLOCBudget(m, dir, 2000)
		if len(got) != 0 {
			t.Errorf("expected 0 errors, got %v", got)
		}
	})

	t.Run("agent with two files totalling over 2000 lines — one error", func(t *testing.T) {
		dir := t.TempDir()
		writeLines(t, filepath.Join(dir, "foo.go"), 1200)
		writeLines(t, filepath.Join(dir, "bar.go"), 900)
		m := &IMPLManifest{
			FileOwnership: []FileOwnership{
				{File: "foo.go", Agent: "A", Wave: 1, Action: "modify"},
				{File: "bar.go", Agent: "A", Wave: 1, Action: "modify"},
			},
		}
		got := CheckAgentLOCBudget(m, dir, 2000)
		if len(got) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(got), got)
		}
		if got[0].Code != "V048_AGENT_LOC_BUDGET" {
			t.Errorf("code = %q, want V048_AGENT_LOC_BUDGET", got[0].Code)
		}
		if got[0].Severity != "error" {
			t.Errorf("severity = %q, want error", got[0].Severity)
		}
		if !strings.Contains(got[0].Message, "foo.go") {
			t.Errorf("expected foo.go in message, got: %s", got[0].Message)
		}
		if !strings.Contains(got[0].Message, "1200") {
			t.Errorf("expected 1200 in message, got: %s", got[0].Message)
		}
		if !strings.Contains(got[0].Message, "bar.go") {
			t.Errorf("expected bar.go in message, got: %s", got[0].Message)
		}
		if !strings.Contains(got[0].Message, "900") {
			t.Errorf("expected 900 in message, got: %s", got[0].Message)
		}
	})

	t.Run("action=new files are skipped even if large", func(t *testing.T) {
		dir := t.TempDir()
		writeLines(t, filepath.Join(dir, "newfile.go"), 5000)
		m := &IMPLManifest{
			FileOwnership: []FileOwnership{
				{File: "newfile.go", Agent: "A", Wave: 1, Action: "new"},
			},
		}
		got := CheckAgentLOCBudget(m, dir, 2000)
		if len(got) != 0 {
			t.Errorf("expected 0 errors (action=new skipped), got %v", got)
		}
	})

	t.Run("single file over budget — file name and line count in message", func(t *testing.T) {
		dir := t.TempDir()
		writeLines(t, filepath.Join(dir, "bigfile.go"), 3000)
		m := &IMPLManifest{
			FileOwnership: []FileOwnership{
				{File: "bigfile.go", Agent: "A", Wave: 1, Action: "modify"},
			},
		}
		got := CheckAgentLOCBudget(m, dir, 2000)
		if len(got) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(got), got)
		}
		if !strings.Contains(got[0].Message, "bigfile.go") {
			t.Errorf("expected bigfile.go in message, got: %s", got[0].Message)
		}
		if !strings.Contains(got[0].Message, "3000") {
			t.Errorf("expected 3000 in message, got: %s", got[0].Message)
		}
	})

	t.Run("missing action=modify file is skipped silently, no panic", func(t *testing.T) {
		dir := t.TempDir()
		// Intentionally do NOT create the file
		m := &IMPLManifest{
			FileOwnership: []FileOwnership{
				{File: "nonexistent.go", Agent: "A", Wave: 1, Action: "modify"},
			},
		}
		// Should not panic
		got := CheckAgentLOCBudget(m, dir, 2000)
		if len(got) != 0 {
			t.Errorf("expected 0 errors for missing file, got %v", got)
		}
	})
}
