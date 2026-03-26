package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConflictTestIMPL creates a minimal IMPL YAML file at docs/IMPL/IMPL-<slug>.yaml
// with the given file_ownership entries. Each entry is a map with "file", and optionally "repo".
func writeConflictTestIMPL(t *testing.T, repoPath, slug string, files []map[string]string) {
	t.Helper()
	dir := filepath.Join(repoPath, "docs", "IMPL")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	yaml := "title: Test IMPL " + slug + "\n"
	yaml += "feature_slug: " + slug + "\n"
	yaml += "verdict: SUITABLE\n"
	yaml += "test_command: \"go test ./...\"\n"
	yaml += "lint_command: \"go vet ./...\"\n"
	yaml += "file_ownership:\n"
	for _, f := range files {
		yaml += "  - file: " + f["file"] + "\n"
		yaml += "    agent: A\n"
		yaml += "    wave: 1\n"
		if repo, ok := f["repo"]; ok {
			yaml += "    repo: " + repo + "\n"
		}
	}
	yaml += "waves:\n"
	yaml += "  - number: 1\n"
	yaml += "    agents:\n"
	yaml += "      - id: A\n"
	yaml += "        task: test task\n"

	path := filepath.Join(dir, "IMPL-"+slug+".yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
}

// writeConflictTestIMPLComplete creates a minimal IMPL YAML in docs/IMPL/complete/ directory.
func writeConflictTestIMPLComplete(t *testing.T, repoPath, slug string, files []map[string]string) {
	t.Helper()
	dir := filepath.Join(repoPath, "docs", "IMPL", "complete")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	yaml := "title: Test IMPL " + slug + "\n"
	yaml += "feature_slug: " + slug + "\n"
	yaml += "verdict: SUITABLE\n"
	yaml += "test_command: \"go test ./...\"\n"
	yaml += "lint_command: \"go vet ./...\"\n"
	yaml += "file_ownership:\n"
	for _, f := range files {
		yaml += "  - file: " + f["file"] + "\n"
		yaml += "    agent: A\n"
		yaml += "    wave: 1\n"
		if repo, ok := f["repo"]; ok {
			yaml += "    repo: " + repo + "\n"
		}
	}
	yaml += "waves:\n"
	yaml += "  - number: 1\n"
	yaml += "    agents:\n"
	yaml += "      - id: A\n"
	yaml += "        task: test task\n"

	path := filepath.Join(dir, "IMPL-"+slug+".yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckIMPLConflicts_AllDisjoint(t *testing.T) {
	tmp := t.TempDir()

	writeConflictTestIMPL(t, tmp, "alpha", []map[string]string{
		{"file": "pkg/a/file1.go"},
		{"file": "pkg/a/file2.go"},
	})
	writeConflictTestIMPL(t, tmp, "beta", []map[string]string{
		{"file": "pkg/b/file1.go"},
	})
	writeConflictTestIMPL(t, tmp, "gamma", []map[string]string{
		{"file": "pkg/c/file1.go"},
	})

	report, err := CheckIMPLConflicts([]string{"alpha", "beta", "gamma"}, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(report.Conflicts))
	}

	// All should be in tier 1 (no overlaps)
	for _, slug := range []string{"alpha", "beta", "gamma"} {
		if report.TierSuggestion[slug] != 1 {
			t.Errorf("expected %s in tier 1, got tier %d", slug, report.TierSuggestion[slug])
		}
	}

	// 3 singleton disjoint sets
	if len(report.DisjointSets) != 3 {
		t.Errorf("expected 3 disjoint sets, got %d", len(report.DisjointSets))
	}
	for _, set := range report.DisjointSets {
		if len(set) != 1 {
			t.Errorf("expected singleton set, got %v", set)
		}
	}
}

func TestCheckIMPLConflicts_TwoOverlap(t *testing.T) {
	tmp := t.TempDir()

	writeConflictTestIMPL(t, tmp, "alpha", []map[string]string{
		{"file": "pkg/shared/file.go"},
		{"file": "pkg/a/file.go"},
	})
	writeConflictTestIMPL(t, tmp, "beta", []map[string]string{
		{"file": "pkg/shared/file.go"},
		{"file": "pkg/b/file.go"},
	})
	writeConflictTestIMPL(t, tmp, "gamma", []map[string]string{
		{"file": "pkg/c/file.go"},
	})

	report, err := CheckIMPLConflicts([]string{"alpha", "beta", "gamma"}, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(report.Conflicts))
	}
	if report.Conflicts[0].File != "pkg/shared/file.go" {
		t.Errorf("expected conflict on pkg/shared/file.go, got %s", report.Conflicts[0].File)
	}

	// alpha and beta should be in separate tiers
	if report.TierSuggestion["alpha"] == report.TierSuggestion["beta"] {
		t.Errorf("alpha and beta should be in different tiers, both in tier %d", report.TierSuggestion["alpha"])
	}
	// gamma should be in tier 1 (no conflicts)
	if report.TierSuggestion["gamma"] != 1 {
		t.Errorf("expected gamma in tier 1, got tier %d", report.TierSuggestion["gamma"])
	}
}

func TestCheckIMPLConflicts_ChainOverlap(t *testing.T) {
	tmp := t.TempDir()

	// A overlaps B (share file1), B overlaps C (share file2), A does not overlap C
	writeConflictTestIMPL(t, tmp, "alpha", []map[string]string{
		{"file": "pkg/shared/ab.go"},
		{"file": "pkg/a/only.go"},
	})
	writeConflictTestIMPL(t, tmp, "beta", []map[string]string{
		{"file": "pkg/shared/ab.go"},
		{"file": "pkg/shared/bc.go"},
	})
	writeConflictTestIMPL(t, tmp, "gamma", []map[string]string{
		{"file": "pkg/shared/bc.go"},
		{"file": "pkg/c/only.go"},
	})

	report, err := CheckIMPLConflicts([]string{"alpha", "beta", "gamma"}, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d", len(report.Conflicts))
	}

	// A and C can be tier 1, B must be tier 2
	// (greedy: alpha gets tier 1, beta conflicts with alpha so gets tier 2,
	//  gamma conflicts with beta but not alpha so gets tier 1)
	if report.TierSuggestion["alpha"] != 1 {
		t.Errorf("expected alpha in tier 1, got %d", report.TierSuggestion["alpha"])
	}
	if report.TierSuggestion["beta"] != 2 {
		t.Errorf("expected beta in tier 2, got %d", report.TierSuggestion["beta"])
	}
	if report.TierSuggestion["gamma"] != 1 {
		t.Errorf("expected gamma in tier 1, got %d", report.TierSuggestion["gamma"])
	}

	// All three should be in one disjoint set (connected via chain)
	if len(report.DisjointSets) != 1 {
		t.Errorf("expected 1 disjoint set (all connected), got %d", len(report.DisjointSets))
	}
	if len(report.DisjointSets) > 0 && len(report.DisjointSets[0]) != 3 {
		t.Errorf("expected 3 slugs in connected component, got %d", len(report.DisjointSets[0]))
	}
}

func TestCheckIMPLConflicts_MissingIMPL(t *testing.T) {
	tmp := t.TempDir()
	// Create docs/IMPL directory but no IMPL files
	if err := os.MkdirAll(filepath.Join(tmp, "docs", "IMPL"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := CheckIMPLConflicts([]string{"nonexistent"}, tmp)
	if err == nil {
		t.Fatal("expected error for missing IMPL, got nil")
	}
}

func TestCheckIMPLConflicts_CrossRepo(t *testing.T) {
	tmp := t.TempDir()

	// Same file path but different repos -> no conflict
	writeConflictTestIMPL(t, tmp, "alpha", []map[string]string{
		{"file": "pkg/shared/file.go", "repo": "repo-a"},
	})
	writeConflictTestIMPL(t, tmp, "beta", []map[string]string{
		{"file": "pkg/shared/file.go", "repo": "repo-b"},
	})

	report, err := CheckIMPLConflicts([]string{"alpha", "beta"}, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts (different repos), got %d: %+v", len(report.Conflicts), report.Conflicts)
	}

	// Both should be tier 1
	if report.TierSuggestion["alpha"] != 1 {
		t.Errorf("expected alpha in tier 1, got %d", report.TierSuggestion["alpha"])
	}
	if report.TierSuggestion["beta"] != 1 {
		t.Errorf("expected beta in tier 1, got %d", report.TierSuggestion["beta"])
	}
}

func TestCheckIMPLConflicts_Empty(t *testing.T) {
	report, err := CheckIMPLConflicts([]string{}, "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d", len(report.Conflicts))
	}
	if len(report.DisjointSets) != 0 {
		t.Errorf("expected 0 disjoint sets, got %d", len(report.DisjointSets))
	}
	if len(report.TierSuggestion) != 0 {
		t.Errorf("expected empty tier suggestions, got %v", report.TierSuggestion)
	}
}

// writeAbsPathTestIMPL creates a minimal IMPL YAML at the given absolute path.
func writeAbsPathTestIMPL(t *testing.T, absPath, slug string, files []map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		t.Fatal(err)
	}

	yaml := "title: Test IMPL " + slug + "\n"
	yaml += "feature_slug: " + slug + "\n"
	yaml += "verdict: SUITABLE\n"
	yaml += "test_command: \"go test ./...\"\n"
	yaml += "lint_command: \"go vet ./...\"\n"
	yaml += "file_ownership:\n"
	for _, f := range files {
		yaml += "  - file: " + f["file"] + "\n"
		yaml += "    agent: A\n"
		yaml += "    wave: 1\n"
		if repo, ok := f["repo"]; ok {
			yaml += "    repo: " + repo + "\n"
		}
	}
	yaml += "waves:\n"
	yaml += "  - number: 1\n"
	yaml += "    agents:\n"
	yaml += "      - id: A\n"
	yaml += "        task: test task\n"

	if err := os.WriteFile(absPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveIMPLPathOrAbs_AbsoluteExists(t *testing.T) {
	// Test A: absolute path that exists
	tmp := t.TempDir()
	absPath := filepath.Join(tmp, "IMPL-my-feature.yaml")
	if err := os.WriteFile(absPath, []byte("feature_slug: my-feature\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveIMPLPathOrAbs("", absPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != absPath {
		t.Errorf("expected %q, got %q", absPath, got)
	}
}

func TestResolveIMPLPathOrAbs_AbsoluteNotExists(t *testing.T) {
	// Test B: absolute path that does not exist
	_, err := resolveIMPLPathOrAbs("", "/nonexistent/path/IMPL-missing.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent absolute path, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected error to contain 'does not exist', got: %v", err)
	}
}

func TestResolveIMPLPathOrAbs_SlugStillWorks(t *testing.T) {
	// Test C: slug still works
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "docs", "IMPL")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	implPath := filepath.Join(dir, "IMPL-my-slug.yaml")
	if err := os.WriteFile(implPath, []byte("feature_slug: my-slug\n"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveIMPLPathOrAbs(tmp, "my-slug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != implPath {
		t.Errorf("expected %q, got %q", implPath, got)
	}
}

// writeConflictTestIMPLWithWaves creates a minimal IMPL YAML file with per-wave file ownership.
// files is a slice of maps with "file", optionally "repo", and "wave" (as string, e.g. "1", "2").
func writeConflictTestIMPLWithWaves(t *testing.T, repoPath, slug string, files []map[string]string) {
	t.Helper()
	dir := filepath.Join(repoPath, "docs", "IMPL")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Collect unique wave numbers for the waves section
	waveSet := make(map[string]bool)
	for _, f := range files {
		w := f["wave"]
		if w == "" {
			w = "1"
		}
		waveSet[w] = true
	}

	yaml := "title: Test IMPL " + slug + "\n"
	yaml += "feature_slug: " + slug + "\n"
	yaml += "verdict: SUITABLE\n"
	yaml += "test_command: \"go test ./...\"\n"
	yaml += "lint_command: \"go vet ./...\"\n"
	yaml += "file_ownership:\n"
	for _, f := range files {
		yaml += "  - file: " + f["file"] + "\n"
		yaml += "    agent: A\n"
		w := f["wave"]
		if w == "" {
			w = "1"
		}
		yaml += "    wave: " + w + "\n"
		if repo, ok := f["repo"]; ok {
			yaml += "    repo: " + repo + "\n"
		}
	}
	yaml += "waves:\n"
	for w := range waveSet {
		yaml += "  - number: " + w + "\n"
		yaml += "    agents:\n"
		yaml += "      - id: A\n"
		yaml += "        task: test task\n"
	}

	path := filepath.Join(dir, "IMPL-"+slug+".yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckIMPLConflictsWaveLevel_DisjointWave1ConflictWave2(t *testing.T) {
	// 2 IMPLs share a file only in Wave 2. Wave 1 is disjoint.
	// Expect: both in tier 1 (wave-level TierSuggestion), SerialWaves has wave 2
	// for both IMPLs, WaveConflicts has 1 entry with WaveNum=2.
	tmp := t.TempDir()

	writeConflictTestIMPLWithWaves(t, tmp, "alpha", []map[string]string{
		{"file": "pkg/a/file1.go", "wave": "1"},
		{"file": "pkg/shared/file.go", "wave": "2"},
	})
	writeConflictTestIMPLWithWaves(t, tmp, "beta", []map[string]string{
		{"file": "pkg/b/file1.go", "wave": "1"},
		{"file": "pkg/shared/file.go", "wave": "2"},
	})

	report, err := CheckIMPLConflictsWaveLevel([]string{"alpha", "beta"}, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should be in tier 1 (no Wave 1 conflict)
	if report.TierSuggestion["alpha"] != 1 {
		t.Errorf("expected alpha in tier 1, got %d", report.TierSuggestion["alpha"])
	}
	if report.TierSuggestion["beta"] != 1 {
		t.Errorf("expected beta in tier 1, got %d", report.TierSuggestion["beta"])
	}

	// WaveConflicts should have exactly 1 entry at wave 2
	if len(report.WaveConflicts) != 1 {
		t.Fatalf("expected 1 wave conflict, got %d: %+v", len(report.WaveConflicts), report.WaveConflicts)
	}
	if report.WaveConflicts[0].WaveNum != 2 {
		t.Errorf("expected WaveNum=2, got %d", report.WaveConflicts[0].WaveNum)
	}
	if report.WaveConflicts[0].File != "pkg/shared/file.go" {
		t.Errorf("expected conflict on pkg/shared/file.go, got %s", report.WaveConflicts[0].File)
	}

	// SerialWaves: both slugs should have wave 2
	if len(report.SerialWaves["alpha"]) != 1 || report.SerialWaves["alpha"][0] != 2 {
		t.Errorf("expected alpha SerialWaves=[2], got %v", report.SerialWaves["alpha"])
	}
	if len(report.SerialWaves["beta"]) != 1 || report.SerialWaves["beta"][0] != 2 {
		t.Errorf("expected beta SerialWaves=[2], got %v", report.SerialWaves["beta"])
	}
}

func TestCheckIMPLConflictsWaveLevel_Wave1Conflict(t *testing.T) {
	// 2 IMPLs share a file in Wave 1.
	// Expect: different tiers, SerialWaves has wave 1 for both.
	tmp := t.TempDir()

	writeConflictTestIMPLWithWaves(t, tmp, "alpha", []map[string]string{
		{"file": "pkg/shared/file.go", "wave": "1"},
		{"file": "pkg/a/file.go", "wave": "1"},
	})
	writeConflictTestIMPLWithWaves(t, tmp, "beta", []map[string]string{
		{"file": "pkg/shared/file.go", "wave": "1"},
		{"file": "pkg/b/file.go", "wave": "1"},
	})

	report, err := CheckIMPLConflictsWaveLevel([]string{"alpha", "beta"}, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Different tiers (Wave 1 conflict)
	if report.TierSuggestion["alpha"] == report.TierSuggestion["beta"] {
		t.Errorf("alpha and beta should be in different tiers (Wave 1 conflict), both in tier %d", report.TierSuggestion["alpha"])
	}

	// WaveConflicts: 1 entry at wave 1
	if len(report.WaveConflicts) != 1 {
		t.Fatalf("expected 1 wave conflict, got %d", len(report.WaveConflicts))
	}
	if report.WaveConflicts[0].WaveNum != 1 {
		t.Errorf("expected WaveNum=1, got %d", report.WaveConflicts[0].WaveNum)
	}

	// SerialWaves: both slugs should have wave 1
	if len(report.SerialWaves["alpha"]) != 1 || report.SerialWaves["alpha"][0] != 1 {
		t.Errorf("expected alpha SerialWaves=[1], got %v", report.SerialWaves["alpha"])
	}
	if len(report.SerialWaves["beta"]) != 1 || report.SerialWaves["beta"][0] != 1 {
		t.Errorf("expected beta SerialWaves=[1], got %v", report.SerialWaves["beta"])
	}
}

func TestCheckIMPLConflictsWaveLevel_NoConflicts(t *testing.T) {
	// 2 IMPLs fully disjoint across all waves.
	// Expect: empty WaveConflicts, empty SerialWaves, both in tier 1.
	tmp := t.TempDir()

	writeConflictTestIMPLWithWaves(t, tmp, "alpha", []map[string]string{
		{"file": "pkg/a/file1.go", "wave": "1"},
		{"file": "pkg/a/file2.go", "wave": "2"},
	})
	writeConflictTestIMPLWithWaves(t, tmp, "beta", []map[string]string{
		{"file": "pkg/b/file1.go", "wave": "1"},
		{"file": "pkg/b/file2.go", "wave": "2"},
	})

	report, err := CheckIMPLConflictsWaveLevel([]string{"alpha", "beta"}, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No wave conflicts
	if len(report.WaveConflicts) != 0 {
		t.Errorf("expected 0 wave conflicts, got %d: %+v", len(report.WaveConflicts), report.WaveConflicts)
	}

	// Empty SerialWaves
	if len(report.SerialWaves) != 0 {
		t.Errorf("expected empty SerialWaves, got %v", report.SerialWaves)
	}

	// Both in tier 1
	if report.TierSuggestion["alpha"] != 1 {
		t.Errorf("expected alpha in tier 1, got %d", report.TierSuggestion["alpha"])
	}
	if report.TierSuggestion["beta"] != 1 {
		t.Errorf("expected beta in tier 1, got %d", report.TierSuggestion["beta"])
	}
}

func TestCheckIMPLConflictsWaveLevel_MultipleWaveConflicts(t *testing.T) {
	// 3 IMPLs: A+B share a file in Wave 1, B+C share a file in Wave 2.
	// Expect: A and B in different tiers (Wave 1 conflict),
	//         C in tier 1 (Wave 1 disjoint from A and B).
	tmp := t.TempDir()

	writeConflictTestIMPLWithWaves(t, tmp, "alpha", []map[string]string{
		{"file": "pkg/shared/ab.go", "wave": "1"},
		{"file": "pkg/a/only.go", "wave": "1"},
	})
	writeConflictTestIMPLWithWaves(t, tmp, "beta", []map[string]string{
		{"file": "pkg/shared/ab.go", "wave": "1"},
		{"file": "pkg/shared/bc.go", "wave": "2"},
	})
	writeConflictTestIMPLWithWaves(t, tmp, "gamma", []map[string]string{
		{"file": "pkg/shared/bc.go", "wave": "2"},
		{"file": "pkg/c/only.go", "wave": "1"},
	})

	report, err := CheckIMPLConflictsWaveLevel([]string{"alpha", "beta", "gamma"}, tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A and B should be in different tiers (Wave 1 conflict between them)
	if report.TierSuggestion["alpha"] == report.TierSuggestion["beta"] {
		t.Errorf("alpha and beta should be in different tiers (Wave 1 conflict), both in tier %d", report.TierSuggestion["alpha"])
	}

	// C should be in tier 1 (no Wave 1 conflict with A or B)
	if report.TierSuggestion["gamma"] != 1 {
		t.Errorf("expected gamma in tier 1, got %d (gamma has no Wave 1 conflict with alpha or beta)", report.TierSuggestion["gamma"])
	}

	// WaveConflicts: 2 entries (one at wave 1 for A+B, one at wave 2 for B+C)
	if len(report.WaveConflicts) != 2 {
		t.Fatalf("expected 2 wave conflicts, got %d: %+v", len(report.WaveConflicts), report.WaveConflicts)
	}

	// Verify conflict files and wave numbers (sorted by wave then file)
	waveNums := map[int]string{}
	for _, wc := range report.WaveConflicts {
		waveNums[wc.WaveNum] = wc.File
	}
	if waveNums[1] != "pkg/shared/ab.go" {
		t.Errorf("expected wave 1 conflict on pkg/shared/ab.go, got %q", waveNums[1])
	}
	if waveNums[2] != "pkg/shared/bc.go" {
		t.Errorf("expected wave 2 conflict on pkg/shared/bc.go, got %q", waveNums[2])
	}

	// alpha should have serial wave 1, beta should have serial waves 1 and 2
	if len(report.SerialWaves["alpha"]) != 1 || report.SerialWaves["alpha"][0] != 1 {
		t.Errorf("expected alpha SerialWaves=[1], got %v", report.SerialWaves["alpha"])
	}
	if len(report.SerialWaves["beta"]) != 2 {
		t.Errorf("expected beta SerialWaves=[1, 2], got %v", report.SerialWaves["beta"])
	}
	if len(report.SerialWaves["gamma"]) != 1 || report.SerialWaves["gamma"][0] != 2 {
		t.Errorf("expected gamma SerialWaves=[2], got %v", report.SerialWaves["gamma"])
	}
}

func TestCheckIMPLConflicts_AbsolutePaths(t *testing.T) {
	// Test D: CheckIMPLConflicts with absolute paths (cross-repo scenario)
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	absPath1 := filepath.Join(dir1, "IMPL-cross-alpha.yaml")
	absPath2 := filepath.Join(dir2, "IMPL-cross-beta.yaml")

	writeAbsPathTestIMPL(t, absPath1, "cross-alpha", []map[string]string{
		{"file": "pkg/alpha/main.go"},
	})
	writeAbsPathTestIMPL(t, absPath2, "cross-beta", []map[string]string{
		{"file": "pkg/beta/main.go"},
	})

	report, err := CheckIMPLConflicts([]string{absPath1, absPath2}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No conflicts since files are disjoint
	if len(report.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts, got %d: %+v", len(report.Conflicts), report.Conflicts)
	}

	// TierSuggestion must be keyed by feature slug (from loaded IMPL doc), not raw path
	if tier, ok := report.TierSuggestion["cross-alpha"]; !ok {
		t.Errorf("expected TierSuggestion[\"cross-alpha\"] to be set; got map: %v", report.TierSuggestion)
	} else if tier != 1 {
		t.Errorf("expected cross-alpha in tier 1, got tier %d", tier)
	}
	if tier, ok := report.TierSuggestion["cross-beta"]; !ok {
		t.Errorf("expected TierSuggestion[\"cross-beta\"] to be set; got map: %v", report.TierSuggestion)
	} else if tier != 1 {
		t.Errorf("expected cross-beta in tier 1, got tier %d", tier)
	}

	// Path strings must NOT appear as keys
	for _, ref := range []string{absPath1, absPath2} {
		if _, ok := report.TierSuggestion[ref]; ok {
			t.Errorf("TierSuggestion must not be keyed by raw path %q", ref)
		}
	}

	// DisjointSets should contain feature slugs, not paths
	if len(report.DisjointSets) != 2 {
		t.Errorf("expected 2 disjoint sets, got %d: %v", len(report.DisjointSets), report.DisjointSets)
	}
}

