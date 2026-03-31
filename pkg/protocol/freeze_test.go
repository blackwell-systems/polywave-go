package protocol

import (
	"testing"
	"time"
)

func TestCheckFreeze_NoTimestamp(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "TestInterface", Definition: "func Test()", Location: "test.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "scaffold.go", Contents: "package test"},
		},
	}

	violations, err := CheckFreeze(manifest)
	if err != nil {
		t.Fatalf("CheckFreeze returned error: %v", err)
	}

	if len(violations) != 0 {
		t.Errorf("Expected no violations when WorktreesCreatedAt is nil, got %d", len(violations))
	}
}

func TestCheckFreeze_NoChanges(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "TestInterface", Definition: "func Test()", Location: "test.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "scaffold.go", Contents: "package test"},
		},
	}

	// Set freeze timestamp and compute hashes
	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	// Check freeze with no changes
	violations, err := CheckFreeze(manifest)
	if err != nil {
		t.Fatalf("CheckFreeze returned error: %v", err)
	}

	if len(violations) != 0 {
		t.Errorf("Expected no violations when nothing changed, got %d: %+v", len(violations), violations)
	}
}

func TestCheckFreeze_ContractsChanged(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "TestInterface", Definition: "func Test()", Location: "test.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "scaffold.go", Contents: "package test"},
		},
	}

	// Set freeze timestamp
	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	// Modify interface contracts
	manifest.InterfaceContracts[0].Definition = "func TestModified()"

	// Check freeze - should detect violation
	violations, err := CheckFreeze(manifest)
	if err != nil {
		t.Fatalf("CheckFreeze returned error: %v", err)
	}

	if len(violations) != 1 {
		t.Fatalf("Expected 1 violation, got %d: %+v", len(violations), violations)
	}

	if violations[0].Section != "interface_contracts" {
		t.Errorf("Expected violation section 'interface_contracts', got '%s'", violations[0].Section)
	}

	if violations[0].Message == "" {
		t.Error("Expected non-empty violation message")
	}
}

func TestCheckFreeze_ScaffoldsChanged(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "TestInterface", Definition: "func Test()", Location: "test.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "scaffold.go", Contents: "package test"},
		},
	}

	// Set freeze timestamp
	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	// Modify scaffolds
	manifest.Scaffolds[0].Contents = "package test\n// modified"

	// Check freeze - should detect violation
	violations, err := CheckFreeze(manifest)
	if err != nil {
		t.Fatalf("CheckFreeze returned error: %v", err)
	}

	if len(violations) != 1 {
		t.Fatalf("Expected 1 violation, got %d: %+v", len(violations), violations)
	}

	if violations[0].Section != "scaffolds" {
		t.Errorf("Expected violation section 'scaffolds', got '%s'", violations[0].Section)
	}

	if violations[0].Message == "" {
		t.Error("Expected non-empty violation message")
	}
}

func TestCheckFreeze_BothChanged(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "TestInterface", Definition: "func Test()", Location: "test.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "scaffold.go", Contents: "package test"},
		},
	}

	// Set freeze timestamp
	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	// Modify both contracts and scaffolds
	manifest.InterfaceContracts[0].Definition = "func TestModified()"
	manifest.Scaffolds[0].Contents = "package test\n// modified"

	// Check freeze - should detect both violations
	violations, err := CheckFreeze(manifest)
	if err != nil {
		t.Fatalf("CheckFreeze returned error: %v", err)
	}

	if len(violations) != 2 {
		t.Fatalf("Expected 2 violations, got %d: %+v", len(violations), violations)
	}

	// Check that both sections are reported
	sections := make(map[string]bool)
	for _, v := range violations {
		sections[v.Section] = true
	}

	if !sections["interface_contracts"] {
		t.Error("Expected interface_contracts violation")
	}
	if !sections["scaffolds"] {
		t.Error("Expected scaffolds violation")
	}
}

func TestSetFreezeTimestamp_SetsHash(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "TestInterface", Definition: "func Test()", Location: "test.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "scaffold.go", Contents: "package test"},
		},
	}

	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	if manifest.FrozenContractsHash == "" {
		t.Error("Expected FrozenContractsHash to be set")
	}

	if manifest.FrozenScaffoldsHash == "" {
		t.Error("Expected FrozenScaffoldsHash to be set")
	}

	// Verify hashes are valid hex strings (64 chars for SHA256)
	if len(manifest.FrozenContractsHash) != 64 {
		t.Errorf("Expected contracts hash length 64, got %d", len(manifest.FrozenContractsHash))
	}

	if len(manifest.FrozenScaffoldsHash) != 64 {
		t.Errorf("Expected scaffolds hash length 64, got %d", len(manifest.FrozenScaffoldsHash))
	}
}

func TestSetFreezeTimestamp_SetsTime(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "TestInterface", Definition: "func Test()", Location: "test.go"},
		},
	}

	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	if manifest.WorktreesCreatedAt == nil {
		t.Fatal("Expected WorktreesCreatedAt to be set")
	}

	// Check that the timestamp matches (within a second to account for rounding)
	diff := manifest.WorktreesCreatedAt.Sub(now)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Expected timestamp to be %v, got %v (diff: %v)", now, *manifest.WorktreesCreatedAt, diff)
	}
}

func TestCheckFreeze_EmptyCollections(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{},
		Scaffolds:          []ScaffoldFile{},
	}

	// Set freeze timestamp
	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	// Check freeze - should not error with empty collections
	violations, err := CheckFreeze(manifest)
	if err != nil {
		t.Fatalf("CheckFreeze returned error with empty collections: %v", err)
	}

	if len(violations) != 0 {
		t.Errorf("Expected no violations with empty collections, got %d", len(violations))
	}
}

func TestCheckFreeze_AddingItems(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "Interface1", Definition: "func Test1()", Location: "test.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "scaffold1.go", Contents: "package test"},
		},
	}

	// Set freeze timestamp
	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	// Add new items (which changes the hash)
	manifest.InterfaceContracts = append(manifest.InterfaceContracts, InterfaceContract{
		Name:       "Interface2",
		Definition: "func Test2()",
		Location:   "test2.go",
	})

	// Check freeze - should detect violation
	violations, err := CheckFreeze(manifest)
	if err != nil {
		t.Fatalf("CheckFreeze returned error: %v", err)
	}

	if len(violations) != 1 {
		t.Fatalf("Expected 1 violation when adding items, got %d: %+v", len(violations), violations)
	}

	if violations[0].Section != "interface_contracts" {
		t.Errorf("Expected violation section 'interface_contracts', got '%s'", violations[0].Section)
	}
}

func TestComputeHash_Deterministic(t *testing.T) {
	data := []InterfaceContract{
		{Name: "Test", Definition: "func Test()", Location: "test.go"},
	}

	hash1, err := computeHash(data)
	if err != nil {
		t.Fatalf("computeHash failed: %v", err)
	}

	hash2, err := computeHash(data)
	if err != nil {
		t.Fatalf("computeHash failed on second call: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic: got %s and %s", hash1, hash2)
	}
}

// TestSetFreezeTimestamp_ContractsFrozenTrue verifies FreezeData.ContractsFrozen is true after freeze.
func TestSetFreezeTimestamp_ContractsFrozenTrue(t *testing.T) {
	manifest := &IMPLManifest{
		InterfaceContracts: []InterfaceContract{
			{Name: "TestInterface", Definition: "func Test()", Location: "test.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "scaffold.go", Contents: "package test"},
		},
	}

	now := time.Now()
	res := SetFreezeTimestamp(manifest, now)
	if res.IsFatal() {
		t.Fatalf("SetFreezeTimestamp failed: %v", res.Errors)
	}

	if !res.IsSuccess() {
		t.Fatalf("Expected SUCCESS result, got code: %s", res.Code)
	}

	data := res.GetData()
	if !data.ContractsFrozen {
		t.Error("Expected FreezeData.ContractsFrozen to be true after freeze")
	}

	if data.FreezeTimestamp.IsZero() {
		t.Error("Expected FreezeData.FreezeTimestamp to be set")
	}
}
