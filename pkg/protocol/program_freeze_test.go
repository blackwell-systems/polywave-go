package protocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo initializes a git repository in the given directory
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "test@test.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")
	runGitCmd(t, dir, "commit", "--allow-empty", "-m", "initial commit")
}

// runGitCmd runs a git command in the specified directory
func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, string(out))
	}
}

// TestFreezeContracts_AllFrozen tests the happy path where all files exist and are committed
func TestFreezeContracts_AllFrozen(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create contract files
	contractFile := filepath.Join(tmpDir, "pkg", "api", "contract.go")
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractFile, []byte("package api\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Commit the file
	runGitCmd(t, tmpDir, "add", ".")
	runGitCmd(t, tmpDir, "commit", "-m", "add contract")

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{
			{
				Name:     "APIContract",
				Location: "pkg/api/contract.go",
				FreezeAt: "IMPL-auth completion",
			},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"auth", "storage"},
			},
		},
	}

	res := FreezeContracts(manifest, 1, tmpDir)
	if res.IsFatal() {
		t.Fatalf("FreezeContracts failed: %+v", res.Errors)
	}
	result := res.GetData()

	if !result.Success {
		t.Errorf("Expected Success=true, got false. Errors: %v", result.Errors)
	}

	if len(result.ContractsFrozen) != 1 {
		t.Fatalf("Expected 1 contract frozen, got %d", len(result.ContractsFrozen))
	}

	frozen := result.ContractsFrozen[0]
	if frozen.Name != "APIContract" {
		t.Errorf("Expected contract name APIContract, got %s", frozen.Name)
	}
	if !frozen.FileExists {
		t.Error("Expected FileExists=true")
	}
	if !frozen.Committed {
		t.Error("Expected Committed=true")
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
}

// TestFreezeContracts_FileNotFound tests when a contract file doesn't exist
func TestFreezeContracts_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{
			{
				Name:     "MissingContract",
				Location: "pkg/api/missing.go",
				FreezeAt: "IMPL-auth completion",
			},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"auth"},
			},
		},
	}

	res := FreezeContracts(manifest, 1, tmpDir)
	if res.IsFatal() {
		t.Fatalf("FreezeContracts failed: %+v", res.Errors)
	}
	result := res.GetData()

	if result.Success {
		t.Error("Expected Success=false when file not found")
	}

	if len(result.ContractsFrozen) != 1 {
		t.Fatalf("Expected 1 contract entry, got %d", len(result.ContractsFrozen))
	}

	frozen := result.ContractsFrozen[0]
	if frozen.FileExists {
		t.Error("Expected FileExists=false")
	}
	if frozen.Committed {
		t.Error("Expected Committed=false")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors when file not found")
	}
}

// TestFreezeContracts_NotCommitted tests when a contract file exists but is not committed
func TestFreezeContracts_NotCommitted(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create contract file but don't commit it
	contractFile := filepath.Join(tmpDir, "pkg", "api", "contract.go")
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractFile, []byte("package api\n"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{
			{
				Name:     "UncommittedContract",
				Location: "pkg/api/contract.go",
				FreezeAt: "IMPL-auth completion",
			},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"auth"},
			},
		},
	}

	res := FreezeContracts(manifest, 1, tmpDir)
	if res.IsFatal() {
		t.Fatalf("FreezeContracts failed: %+v", res.Errors)
	}
	result := res.GetData()

	if result.Success {
		t.Error("Expected Success=false when file not committed")
	}

	if len(result.ContractsFrozen) != 1 {
		t.Fatalf("Expected 1 contract entry, got %d", len(result.ContractsFrozen))
	}

	frozen := result.ContractsFrozen[0]
	if !frozen.FileExists {
		t.Error("Expected FileExists=true")
	}
	if frozen.Committed {
		t.Error("Expected Committed=false")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors when file not committed")
	}
}

// TestFreezeContracts_NoMatchingContracts tests when no contracts should freeze at this tier
func TestFreezeContracts_NoMatchingContracts(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{
			{
				Name:     "LaterContract",
				Location: "pkg/api/contract.go",
				FreezeAt: "IMPL-payments completion",
			},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"auth"},
			},
			{
				Number: 2,
				Impls:  []string{"payments"},
			},
		},
	}

	res := FreezeContracts(manifest, 1, tmpDir)
	if res.IsFatal() {
		t.Fatalf("FreezeContracts failed: %+v", res.Errors)
	}
	result := res.GetData()

	if !result.Success {
		t.Errorf("Expected Success=true when no contracts match, got false. Errors: %v", result.Errors)
	}

	if len(result.ContractsFrozen) != 0 {
		t.Errorf("Expected 0 contracts frozen, got %d", len(result.ContractsFrozen))
	}

	if len(result.ContractsSkipped) != 1 {
		t.Errorf("Expected 1 contract skipped, got %d", len(result.ContractsSkipped))
	}

	if result.ContractsSkipped[0] != "LaterContract" {
		t.Errorf("Expected skipped contract LaterContract, got %s", result.ContractsSkipped[0])
	}
}

// TestFreezeContracts_PartialMatch tests when some contracts freeze and others are skipped
func TestFreezeContracts_PartialMatch(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit one contract file
	contractFile := filepath.Join(tmpDir, "pkg", "api", "auth.go")
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractFile, []byte("package api\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, tmpDir, "add", ".")
	runGitCmd(t, tmpDir, "commit", "-m", "add auth contract")

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{
			{
				Name:     "AuthContract",
				Location: "pkg/api/auth.go",
				FreezeAt: "IMPL-auth completion",
			},
			{
				Name:     "PaymentsContract",
				Location: "pkg/api/payments.go",
				FreezeAt: "IMPL-payments completion",
			},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"auth"},
			},
			{
				Number: 2,
				Impls:  []string{"payments"},
			},
		},
	}

	res := FreezeContracts(manifest, 1, tmpDir)
	if res.IsFatal() {
		t.Fatalf("FreezeContracts failed: %+v", res.Errors)
	}
	result := res.GetData()

	if !result.Success {
		t.Errorf("Expected Success=true, got false. Errors: %v", result.Errors)
	}

	if len(result.ContractsFrozen) != 1 {
		t.Errorf("Expected 1 contract frozen, got %d", len(result.ContractsFrozen))
	}

	if len(result.ContractsSkipped) != 1 {
		t.Errorf("Expected 1 contract skipped, got %d", len(result.ContractsSkipped))
	}

	if result.ContractsFrozen[0].Name != "AuthContract" {
		t.Errorf("Expected frozen contract AuthContract, got %s", result.ContractsFrozen[0].Name)
	}

	if result.ContractsSkipped[0] != "PaymentsContract" {
		t.Errorf("Expected skipped contract PaymentsContract, got %s", result.ContractsSkipped[0])
	}
}

// TestFreezeContracts_EmptyFreezeAt tests that contracts with empty freeze_at are skipped
func TestFreezeContracts_EmptyFreezeAt(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{
			{
				Name:     "NoFreezeContract",
				Location: "pkg/api/contract.go",
				FreezeAt: "",
			},
			{
				Name:     "WhitespaceContract",
				Location: "pkg/api/other.go",
				FreezeAt: "   ",
			},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"auth"},
			},
		},
	}

	res := FreezeContracts(manifest, 1, tmpDir)
	if res.IsFatal() {
		t.Fatalf("FreezeContracts failed: %+v", res.Errors)
	}
	result := res.GetData()

	if !result.Success {
		t.Errorf("Expected Success=true when no contracts to freeze, got false. Errors: %v", result.Errors)
	}

	if len(result.ContractsFrozen) != 0 {
		t.Errorf("Expected 0 contracts frozen, got %d", len(result.ContractsFrozen))
	}

	if len(result.ContractsSkipped) != 2 {
		t.Errorf("Expected 2 contracts skipped, got %d", len(result.ContractsSkipped))
	}
}

// TestFreezeContracts_WholeWordMatching tests that slug matching uses word boundaries
func TestFreezeContracts_WholeWordMatching(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit contract file
	contractFile := filepath.Join(tmpDir, "pkg", "api", "contract.go")
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractFile, []byte("package api\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, tmpDir, "add", ".")
	runGitCmd(t, tmpDir, "commit", "-m", "add contract")

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{
			{
				Name:     "AuthContract",
				Location: "pkg/api/contract.go",
				// "auth" should match as whole word
				FreezeAt: "IMPL-auth completion",
			},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				// "auth" should match "IMPL-auth completion"
				Impls: []string{"auth"},
			},
			{
				Number: 2,
				// "authorization" should NOT match "IMPL-auth completion"
				Impls: []string{"authorization"},
			},
		},
	}

	// Test tier 1 - should match
	res := FreezeContracts(manifest, 1, tmpDir)
	if res.IsFatal() {
		t.Fatalf("FreezeContracts tier 1 failed: %+v", res.Errors)
	}
	result := res.GetData()

	if !result.Success {
		t.Errorf("Tier 1: Expected Success=true, got false. Errors: %v", result.Errors)
	}

	if len(result.ContractsFrozen) != 1 {
		t.Errorf("Tier 1: Expected 1 contract frozen, got %d", len(result.ContractsFrozen))
	}

	// Test tier 2 - should NOT match (authorization vs auth)
	res2 := FreezeContracts(manifest, 2, tmpDir)
	if res2.IsFatal() {
		t.Fatalf("FreezeContracts tier 2 failed: %+v", res2.Errors)
	}
	result2 := res2.GetData()

	if len(result2.ContractsFrozen) != 0 {
		t.Errorf("Tier 2: Expected 0 contracts frozen, got %d", len(result2.ContractsFrozen))
	}

	if len(result2.ContractsSkipped) != 1 {
		t.Errorf("Tier 2: Expected 1 contract skipped, got %d", len(result2.ContractsSkipped))
	}
}

// TestFreezeContracts_InvalidTier tests handling of non-existent tier numbers
func TestFreezeContracts_InvalidTier(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"auth"},
			},
		},
	}

	res := FreezeContracts(manifest, 99, tmpDir)
	if !res.IsFatal() {
		t.Error("Expected fatal result for invalid tier number")
	}
}

// TestFreezeContracts_ModifiedFile tests when a file is modified but not staged/committed
func TestFreezeContracts_ModifiedFile(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create and commit contract file
	contractFile := filepath.Join(tmpDir, "pkg", "api", "contract.go")
	if err := os.MkdirAll(filepath.Dir(contractFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(contractFile, []byte("package api\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitCmd(t, tmpDir, "add", ".")
	runGitCmd(t, tmpDir, "commit", "-m", "add contract")

	// Modify the file without committing
	if err := os.WriteFile(contractFile, []byte("package api\n// modified\n"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := &PROGRAMManifest{
		ProgramContracts: []ProgramContract{
			{
				Name:     "ModifiedContract",
				Location: "pkg/api/contract.go",
				FreezeAt: "IMPL-auth completion",
			},
		},
		Tiers: []ProgramTier{
			{
				Number: 1,
				Impls:  []string{"auth"},
			},
		},
	}

	res := FreezeContracts(manifest, 1, tmpDir)
	if res.IsFatal() {
		t.Fatalf("FreezeContracts failed: %+v", res.Errors)
	}
	result := res.GetData()

	if result.Success {
		t.Error("Expected Success=false when file is modified")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors when file is modified but not committed")
	}

	frozen := result.ContractsFrozen[0]
	if !frozen.FileExists {
		t.Error("Expected FileExists=true")
	}
	if frozen.Committed {
		t.Error("Expected Committed=false for modified file")
	}
}
