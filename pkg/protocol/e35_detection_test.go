package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDetectE35Gaps_SamePackageCallGap verifies that E35 detection catches
// same-package function calls where the caller file is not owned by the agent.
func TestDetectE35Gaps_SamePackageCallGap(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "pkg", "testpkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create function definition file (owned by Agent C)
	defFile := filepath.Join(pkgDir, "worktree.go")
	defContent := `package testpkg

// CreateProgramWorktrees creates worktrees for a program.
func CreateProgramWorktrees(programID string, tier int) error {
	return nil
}
`
	if err := os.WriteFile(defFile, []byte(defContent), 0644); err != nil {
		t.Fatalf("failed to write definition file: %v", err)
	}

	// Create caller file (owned by Agent B)
	callerFile := filepath.Join(pkgDir, "program_tier_prepare.go")
	callerContent := `package testpkg

func PrepareTier() error {
	// First call site
	err := CreateProgramWorktrees("test-program", 1)
	if err != nil {
		return err
	}

	// Second call site
	return CreateProgramWorktrees("test-program", 2)
}
`
	if err := os.WriteFile(callerFile, []byte(callerContent), 0644); err != nil {
		t.Fatalf("failed to write caller file: %v", err)
	}

	// Create manifest with Wave 1
	manifest := &IMPLManifest{
		Title:       "Test E35 Detection",
		FeatureSlug: "test-e35",
		Verdict:     "SUITABLE",
		Repository:  tmpDir,
		FileOwnership: []FileOwnership{
			{File: "pkg/testpkg/worktree.go", Agent: "C", Wave: 1},
			{File: "pkg/testpkg/program_tier_prepare.go", Agent: "B", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "B", Task: "Implement tier prep", Files: []string{"pkg/testpkg/program_tier_prepare.go"}},
					{ID: "C", Task: "Implement worktree", Files: []string{"pkg/testpkg/worktree.go"}},
				},
			},
		},
	}

	// Run E35 detection
	gaps, err := DetectE35Gaps(manifest, 1, tmpDir)
	if err != nil {
		t.Fatalf("DetectE35Gaps failed: %v", err)
	}

	// Verify gap detected
	if len(gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(gaps))
	}

	gap := gaps[0]
	if gap.Agent != "C" {
		t.Errorf("expected agent C, got %q", gap.Agent)
	}
	if gap.FunctionName != "CreateProgramWorktrees" {
		t.Errorf("expected function CreateProgramWorktrees, got %q", gap.FunctionName)
	}
	if gap.DefinedIn != "pkg/testpkg/worktree.go" {
		t.Errorf("expected definedIn pkg/testpkg/worktree.go, got %q", gap.DefinedIn)
	}
	if len(gap.CalledFrom) != 2 {
		t.Errorf("expected 2 call sites, got %d", len(gap.CalledFrom))
	}
	if gap.Package != "testpkg" {
		t.Errorf("expected package testpkg, got %q", gap.Package)
	}

	t.Logf("Gap detected successfully: %+v", gap)
}

// TestDetectE35Gaps_NoGap verifies that no gap is reported when the agent
// owns both the definition and all call sites.
func TestDetectE35Gaps_NoGap(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "pkg", "testpkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create function definition file
	defFile := filepath.Join(pkgDir, "helper.go")
	defContent := `package testpkg

func HelperFunc() {
}
`
	if err := os.WriteFile(defFile, []byte(defContent), 0644); err != nil {
		t.Fatalf("failed to write definition file: %v", err)
	}

	// Create caller file (same agent owns both)
	callerFile := filepath.Join(pkgDir, "main.go")
	callerContent := `package testpkg

func Main() {
	HelperFunc()
}
`
	if err := os.WriteFile(callerFile, []byte(callerContent), 0644); err != nil {
		t.Fatalf("failed to write caller file: %v", err)
	}

	manifest := &IMPLManifest{
		Title:       "Test E35 No Gap",
		FeatureSlug: "test-e35-no-gap",
		Verdict:     "SUITABLE",
		Repository:  tmpDir,
		FileOwnership: []FileOwnership{
			{File: "pkg/testpkg/helper.go", Agent: "A", Wave: 1},
			{File: "pkg/testpkg/main.go", Agent: "A", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement pkg", Files: []string{"pkg/testpkg/helper.go", "pkg/testpkg/main.go"}},
				},
			},
		},
	}

	gaps, err := DetectE35Gaps(manifest, 1, tmpDir)
	if err != nil {
		t.Fatalf("DetectE35Gaps failed: %v", err)
	}

	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d: %+v", len(gaps), gaps)
	}
}

// TestDetectE35Gaps_CrossPackageNoGap verifies that cross-package calls do not
// trigger E35 violations (E35 only applies to same-package calls).
func TestDetectE35Gaps_CrossPackageNoGap(t *testing.T) {
	tmpDir := t.TempDir()
	pkg1Dir := filepath.Join(tmpDir, "pkg", "pkg1")
	pkg2Dir := filepath.Join(tmpDir, "pkg", "pkg2")
	if err := os.MkdirAll(pkg1Dir, 0755); err != nil {
		t.Fatalf("failed to create pkg1 directory: %v", err)
	}
	if err := os.MkdirAll(pkg2Dir, 0755); err != nil {
		t.Fatalf("failed to create pkg2 directory: %v", err)
	}

	// Create function in pkg1 (owned by Agent A)
	defFile := filepath.Join(pkg1Dir, "helper.go")
	defContent := `package pkg1

func SharedFunc() {
}
`
	if err := os.WriteFile(defFile, []byte(defContent), 0644); err != nil {
		t.Fatalf("failed to write definition file: %v", err)
	}

	// Create caller in pkg2 (owned by Agent B)
	callerFile := filepath.Join(pkg2Dir, "main.go")
	callerContent := `package pkg2

import "pkg/pkg1"

func Main() {
	pkg1.SharedFunc()
}
`
	if err := os.WriteFile(callerFile, []byte(callerContent), 0644); err != nil {
		t.Fatalf("failed to write caller file: %v", err)
	}

	manifest := &IMPLManifest{
		Title:       "Test E35 Cross Package",
		FeatureSlug: "test-e35-cross-pkg",
		Verdict:     "SUITABLE",
		Repository:  tmpDir,
		FileOwnership: []FileOwnership{
			{File: "pkg/pkg1/helper.go", Agent: "A", Wave: 1},
			{File: "pkg/pkg2/main.go", Agent: "B", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement pkg1", Files: []string{"pkg/pkg1/helper.go"}},
					{ID: "B", Task: "Implement pkg2", Files: []string{"pkg/pkg2/main.go"}},
				},
			},
		},
	}

	gaps, err := DetectE35Gaps(manifest, 1, tmpDir)
	if err != nil {
		t.Fatalf("DetectE35Gaps failed: %v", err)
	}

	// No gap expected: cross-package calls are not E35 violations
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps (cross-package calls OK), got %d: %+v", len(gaps), gaps)
	}
}

// TestDetectTestCascades_OrphanedTestFile verifies that test cascade detection
// catches test files that reference interfaces but are not owned by any agent.
func TestDetectTestCascades_OrphanedTestFile(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "pkg", "parser")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create interface definition file (owned by Agent A)
	defFile := filepath.Join(pkgDir, "lockfile.go")
	defContent := `package parser

type LockFileParser interface {
	Parse(path string) error
	Detect() bool
}

type CargoLockParser struct {
	data []byte
}
`
	if err := os.WriteFile(defFile, []byte(defContent), 0644); err != nil {
		t.Fatalf("failed to write definition file: %v", err)
	}

	// Create orphaned test file (NOT owned by any agent)
	testFile := filepath.Join(pkgDir, "lockfile_test.go")
	testContent := `package parser

import "testing"

func TestCargoLockParser(t *testing.T) {
	parser := &CargoLockParser{}
	if parser == nil {
		t.Fatal("parser is nil")
	}
}

func TestLockFileParser(t *testing.T) {
	var p LockFileParser = &CargoLockParser{}
	if p == nil {
		t.Fatal("parser is nil")
	}
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create manifest with Wave 1
	manifest := &IMPLManifest{
		Title:       "Test Cascade Detection",
		FeatureSlug: "test-cascade",
		Verdict:     "SUITABLE",
		Repository:  tmpDir,
		FileOwnership: []FileOwnership{
			{File: "pkg/parser/lockfile.go", Agent: "A", Wave: 1},
			// Note: lockfile_test.go is NOT in file ownership
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement parser", Files: []string{"pkg/parser/lockfile.go"}},
				},
			},
		},
	}

	// Run E35 detection (which now includes test cascade detection)
	gaps, err := DetectE35Gaps(manifest, 1, tmpDir)
	if err != nil {
		t.Fatalf("DetectE35Gaps failed: %v", err)
	}

	// Verify gap detected for orphaned test file
	if len(gaps) == 0 {
		t.Fatalf("expected at least 1 gap for orphaned test file, got 0")
	}

	// Find gaps related to test files
	var testGaps []E35Gap
	for _, gap := range gaps {
		for _, caller := range gap.CalledFrom {
			if strings.Contains(caller, "lockfile_test.go") {
				testGaps = append(testGaps, gap)
				break
			}
		}
	}

	if len(testGaps) == 0 {
		t.Fatalf("expected gaps related to lockfile_test.go, got none. All gaps: %+v", gaps)
	}

	// Verify gap structure
	gap := testGaps[0]
	if gap.Agent != "A" {
		t.Errorf("expected agent A, got %q", gap.Agent)
	}
	if gap.FunctionName != "LockFileParser" && gap.FunctionName != "CargoLockParser" {
		t.Errorf("expected function name LockFileParser or CargoLockParser, got %q", gap.FunctionName)
	}
	if gap.DefinedIn != "pkg/parser/lockfile.go" {
		t.Errorf("expected definedIn pkg/parser/lockfile.go, got %q", gap.DefinedIn)
	}
	if gap.Package != "parser" {
		t.Errorf("expected package parser, got %q", gap.Package)
	}

	t.Logf("Test cascade gap detected successfully: %+v", gap)
}

// TestDetectTestCascades_TestFileOwned verifies that no gap is reported when
// the test file IS owned by an agent.
func TestDetectTestCascades_TestFileOwned(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "pkg", "parser")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create interface definition file
	defFile := filepath.Join(pkgDir, "lockfile.go")
	defContent := `package parser

type LockFileParser interface {
	Parse(path string) error
}
`
	if err := os.WriteFile(defFile, []byte(defContent), 0644); err != nil {
		t.Fatalf("failed to write definition file: %v", err)
	}

	// Create test file (owned by same agent)
	testFile := filepath.Join(pkgDir, "lockfile_test.go")
	testContent := `package parser

import "testing"

func TestLockFileParser(t *testing.T) {
	var p LockFileParser
	if p != nil {
		t.Fatal("parser should be nil")
	}
}
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	manifest := &IMPLManifest{
		Title:       "Test Cascade No Gap",
		FeatureSlug: "test-cascade-no-gap",
		Verdict:     "SUITABLE",
		Repository:  tmpDir,
		FileOwnership: []FileOwnership{
			{File: "pkg/parser/lockfile.go", Agent: "A", Wave: 1},
			{File: "pkg/parser/lockfile_test.go", Agent: "A", Wave: 1}, // Test file IS owned
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement parser", Files: []string{"pkg/parser/lockfile.go", "pkg/parser/lockfile_test.go"}},
				},
			},
		},
	}

	gaps, err := DetectE35Gaps(manifest, 1, tmpDir)
	if err != nil {
		t.Fatalf("DetectE35Gaps failed: %v", err)
	}

	// No gaps expected: test file is owned by agent
	// (Filter out any non-test-related gaps)
	var testGaps []E35Gap
	for _, gap := range gaps {
		for _, caller := range gap.CalledFrom {
			if strings.Contains(caller, "_test.go") {
				testGaps = append(testGaps, gap)
				break
			}
		}
	}

	if len(testGaps) != 0 {
		t.Errorf("expected 0 test cascade gaps (test file is owned), got %d: %+v", len(testGaps), testGaps)
	}
}
