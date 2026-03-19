package protocol

import (
	"path/filepath"
	"testing"
)

func TestBranchName(t *testing.T) {
	tests := []struct {
		name     string
		slug     string
		waveNum  int
		agentID  string
		expected string
	}{
		{
			name:     "basic case",
			slug:     "my-feature",
			waveNum:  1,
			agentID:  "A",
			expected: "saw/my-feature/wave1-agent-A",
		},
		{
			name:     "multi-generation agent",
			slug:     "my-feature",
			waveNum:  2,
			agentID:  "B3",
			expected: "saw/my-feature/wave2-agent-B3",
		},
		{
			name:     "slug with numbers",
			slug:     "v2-migration",
			waveNum:  5,
			agentID:  "C",
			expected: "saw/v2-migration/wave5-agent-C",
		},
		{
			name:     "slug with multiple hyphens",
			slug:     "add-new-feature-flag",
			waveNum:  10,
			agentID:  "D2",
			expected: "saw/add-new-feature-flag/wave10-agent-D2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchName(tt.slug, tt.waveNum, tt.agentID)
			if got != tt.expected {
				t.Errorf("BranchName(%q, %d, %q) = %q, want %q",
					tt.slug, tt.waveNum, tt.agentID, got, tt.expected)
			}
		})
	}
}

func TestLegacyBranchName(t *testing.T) {
	tests := []struct {
		name     string
		waveNum  int
		agentID  string
		expected string
	}{
		{
			name:     "basic case",
			waveNum:  1,
			agentID:  "A",
			expected: "wave1-agent-A",
		},
		{
			name:     "multi-generation agent",
			waveNum:  3,
			agentID:  "B2",
			expected: "wave3-agent-B2",
		},
		{
			name:     "high wave number",
			waveNum:  99,
			agentID:  "Z",
			expected: "wave99-agent-Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LegacyBranchName(tt.waveNum, tt.agentID)
			if got != tt.expected {
				t.Errorf("LegacyBranchName(%d, %q) = %q, want %q",
					tt.waveNum, tt.agentID, got, tt.expected)
			}
		})
	}
}

func TestWorktreeDir(t *testing.T) {
	tests := []struct {
		name     string
		repoDir  string
		slug     string
		waveNum  int
		agentID  string
		expected string
	}{
		{
			name:     "unix path",
			repoDir:  "/home/user/repo",
			slug:     "my-feature",
			waveNum:  1,
			agentID:  "A",
			expected: "/home/user/repo/.claude/worktrees/saw/my-feature/wave1-agent-A",
		},
		{
			name:     "relative path",
			repoDir:  ".",
			slug:     "test-slug",
			waveNum:  2,
			agentID:  "B",
			expected: filepath.Join(".", ".claude", "worktrees", "saw", "test-slug", "wave2-agent-B"),
		},
		{
			name:     "slug with hyphens",
			repoDir:  "/repo",
			slug:     "add-new-api",
			waveNum:  3,
			agentID:  "C2",
			expected: "/repo/.claude/worktrees/saw/add-new-api/wave3-agent-C2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorktreeDir(tt.repoDir, tt.slug, tt.waveNum, tt.agentID)
			if got != tt.expected {
				t.Errorf("WorktreeDir(%q, %q, %d, %q) = %q, want %q",
					tt.repoDir, tt.slug, tt.waveNum, tt.agentID, got, tt.expected)
			}
		})
	}
}

func TestScopedBranchRegex_NewFormat(t *testing.T) {
	tests := []struct {
		branch  string
		matches bool
		wave    string
		agent   string
	}{
		{"saw/my-slug/wave1-agent-A", true, "1", "A"},
		{"saw/test/wave2-agent-B", true, "2", "B"},
		{"saw/feature-x/wave10-agent-C2", true, "10", "C2"},
		{"saw/v2-migration/wave99-agent-Z9", true, "99", "Z9"},
		{"saw/hyphenated-slug/wave5-agent-D", true, "5", "D"},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			matches := ScopedBranchRegex.MatchString(tt.branch)
			if matches != tt.matches {
				t.Errorf("ScopedBranchRegex.MatchString(%q) = %v, want %v",
					tt.branch, matches, tt.matches)
				return
			}

			if matches {
				m := ScopedBranchRegex.FindStringSubmatch(tt.branch)
				if m == nil {
					t.Errorf("Expected match groups for %q", tt.branch)
					return
				}
				if m[1] != tt.wave {
					t.Errorf("Wave number = %q, want %q", m[1], tt.wave)
				}
				if m[2] != tt.agent {
					t.Errorf("Agent ID = %q, want %q", m[2], tt.agent)
				}
			}
		})
	}
}

func TestScopedBranchRegex_LegacyFormat(t *testing.T) {
	tests := []struct {
		branch  string
		matches bool
		wave    string
		agent   string
	}{
		{"wave1-agent-A", true, "1", "A"},
		{"wave2-agent-B", true, "2", "B"},
		{"wave10-agent-C2", true, "10", "C2"},
		{"wave99-agent-Z9", true, "99", "Z9"},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			matches := ScopedBranchRegex.MatchString(tt.branch)
			if matches != tt.matches {
				t.Errorf("ScopedBranchRegex.MatchString(%q) = %v, want %v",
					tt.branch, matches, tt.matches)
				return
			}

			if matches {
				m := ScopedBranchRegex.FindStringSubmatch(tt.branch)
				if m == nil {
					t.Errorf("Expected match groups for %q", tt.branch)
					return
				}
				if m[1] != tt.wave {
					t.Errorf("Wave number = %q, want %q", m[1], tt.wave)
				}
				if m[2] != tt.agent {
					t.Errorf("Agent ID = %q, want %q", m[2], tt.agent)
				}
			}
		})
	}
}

func TestScopedBranchRegex_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"wave1",                        // missing agent
		"agent-A",                      // missing wave
		"wave1-A",                      // missing "agent-" prefix
		"wave-1-agent-A",               // wrong wave format
		"wave1-agent-a",                // lowercase agent (invalid)
		"saw/wave1-agent-A",            // missing slug between saw/ and wave
		"saw//wave1-agent-A",           // empty slug
		"main",                         // not a SAW branch
		"feature/my-feature",           // different branch pattern
		"saw/MY-SLUG/wave1-agent-A",    // uppercase in slug (invalid)
		"saw/my_slug/wave1-agent-A",    // underscore in slug (invalid)
		"wave1-agent-A-extra",          // trailing content
		"prefix-wave1-agent-A",         // leading content
		"saw/my-slug/wave1-agent-A/extra", // trailing content after agent
	}

	for _, branch := range invalid {
		t.Run(branch, func(t *testing.T) {
			if ScopedBranchRegex.MatchString(branch) {
				t.Errorf("ScopedBranchRegex should not match %q", branch)
			}
		})
	}
}

func TestParseBranch_NewFormat(t *testing.T) {
	tests := []struct {
		branch      string
		expectedW   int
		expectedID  string
		expectedOK  bool
	}{
		{"saw/my-slug/wave1-agent-A", 1, "A", true},
		{"saw/test/wave2-agent-B2", 2, "B2", true},
		{"saw/feature/wave10-agent-C", 10, "C", true},
		{"saw/v2-api/wave99-agent-Z9", 99, "Z9", true},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			wave, agentID, ok := ParseBranch(tt.branch)
			if ok != tt.expectedOK {
				t.Errorf("ParseBranch(%q) ok = %v, want %v", tt.branch, ok, tt.expectedOK)
				return
			}
			if wave != tt.expectedW {
				t.Errorf("ParseBranch(%q) wave = %d, want %d", tt.branch, wave, tt.expectedW)
			}
			if agentID != tt.expectedID {
				t.Errorf("ParseBranch(%q) agentID = %q, want %q", tt.branch, agentID, tt.expectedID)
			}
		})
	}
}

func TestParseBranch_LegacyFormat(t *testing.T) {
	tests := []struct {
		branch      string
		expectedW   int
		expectedID  string
		expectedOK  bool
	}{
		{"wave1-agent-A", 1, "A", true},
		{"wave2-agent-B2", 2, "B2", true},
		{"wave10-agent-C", 10, "C", true},
		{"wave99-agent-Z9", 99, "Z9", true},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			wave, agentID, ok := ParseBranch(tt.branch)
			if ok != tt.expectedOK {
				t.Errorf("ParseBranch(%q) ok = %v, want %v", tt.branch, ok, tt.expectedOK)
				return
			}
			if wave != tt.expectedW {
				t.Errorf("ParseBranch(%q) wave = %d, want %d", tt.branch, wave, tt.expectedW)
			}
			if agentID != tt.expectedID {
				t.Errorf("ParseBranch(%q) agentID = %q, want %q", tt.branch, agentID, tt.expectedID)
			}
		})
	}
}

func TestParseBranch_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"wave1",
		"agent-A",
		"main",
		"feature/branch",
		"saw/my-slug/invalid",
	}

	for _, branch := range invalid {
		t.Run(branch, func(t *testing.T) {
			_, _, ok := ParseBranch(branch)
			if ok {
				t.Errorf("ParseBranch(%q) should return ok=false", branch)
			}
		})
	}
}

func TestExtractSlug_NewFormat(t *testing.T) {
	tests := []struct {
		branch       string
		expectedSlug string
	}{
		{"saw/my-slug/wave1-agent-A", "my-slug"},
		{"saw/test/wave2-agent-B", "test"},
		{"saw/feature-x/wave10-agent-C2", "feature-x"},
		{"saw/v2-migration/wave99-agent-Z", "v2-migration"},
		{"saw/hyphenated-slug-here/wave5-agent-D", "hyphenated-slug-here"},
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := ExtractSlug(tt.branch)
			if got != tt.expectedSlug {
				t.Errorf("ExtractSlug(%q) = %q, want %q", tt.branch, got, tt.expectedSlug)
			}
		})
	}
}

func TestExtractSlug_LegacyFormat(t *testing.T) {
	legacy := []string{
		"wave1-agent-A",
		"wave2-agent-B",
		"wave10-agent-C2",
	}

	for _, branch := range legacy {
		t.Run(branch, func(t *testing.T) {
			got := ExtractSlug(branch)
			if got != "" {
				t.Errorf("ExtractSlug(%q) = %q, want empty string for legacy format", branch, got)
			}
		})
	}
}

func TestExtractSlug_Invalid(t *testing.T) {
	invalid := []string{
		"",
		"main",
		"feature/branch",
		"saw/",     // no slug
		"sawx/test/wave1-agent-A", // not starting with saw/
	}

	for _, branch := range invalid {
		t.Run(branch, func(t *testing.T) {
			got := ExtractSlug(branch)
			if got != "" {
				t.Errorf("ExtractSlug(%q) = %q, want empty string for invalid input", branch, got)
			}
		})
	}
}

func TestBranchName_SpecialSlugs(t *testing.T) {
	tests := []struct {
		name     string
		slug     string
		waveNum  int
		agentID  string
		expected string
	}{
		{
			name:     "slug with number prefix",
			slug:     "2fa-integration",
			waveNum:  1,
			agentID:  "A",
			expected: "saw/2fa-integration/wave1-agent-A",
		},
		{
			name:     "slug with many hyphens",
			slug:     "add-new-feature-for-api-v2",
			waveNum:  3,
			agentID:  "B",
			expected: "saw/add-new-feature-for-api-v2/wave3-agent-B",
		},
		{
			name:     "short slug",
			slug:     "a",
			waveNum:  1,
			agentID:  "C",
			expected: "saw/a/wave1-agent-C",
		},
		{
			name:     "numeric slug",
			slug:     "123",
			waveNum:  2,
			agentID:  "D",
			expected: "saw/123/wave2-agent-D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchName(tt.slug, tt.waveNum, tt.agentID)
			if got != tt.expected {
				t.Errorf("BranchName(%q, %d, %q) = %q, want %q",
					tt.slug, tt.waveNum, tt.agentID, got, tt.expected)
			}

			// Verify it matches the regex
			if !ScopedBranchRegex.MatchString(got) {
				t.Errorf("BranchName result %q should match ScopedBranchRegex", got)
			}

			// Verify round-trip through ParseBranch
			wave, agent, ok := ParseBranch(got)
			if !ok || wave != tt.waveNum || agent != tt.agentID {
				t.Errorf("ParseBranch(%q) = (%d, %q, %v), want (%d, %q, true)",
					got, wave, agent, ok, tt.waveNum, tt.agentID)
			}

			// Verify ExtractSlug
			slug := ExtractSlug(got)
			if slug != tt.slug {
				t.Errorf("ExtractSlug(%q) = %q, want %q", got, slug, tt.slug)
			}
		})
	}
}
