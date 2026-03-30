package protocol

import (
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
