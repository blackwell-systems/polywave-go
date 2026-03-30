package protocol

import (
	"os"
	"path/filepath"
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
