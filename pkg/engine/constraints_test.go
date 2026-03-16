package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func TestBuildConstraints_ScoutRole(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Wave: 1},
		},
		Scaffolds: []protocol.ScaffoldFile{
			{FilePath: "pkg/types/shared.go"},
		},
	}

	c := buildConstraints(manifest, "", "scout")
	if c == nil {
		t.Fatal("expected non-nil constraints for scout role")
	}

	// Scout should have path prefix restriction
	if len(c.AllowedPathPrefixes) != 1 || c.AllowedPathPrefixes[0] != "docs/IMPL/IMPL-" {
		t.Errorf("expected AllowedPathPrefixes=[docs/IMPL/IMPL-], got %v", c.AllowedPathPrefixes)
	}

	// Scout should NOT have ownership or freeze
	if len(c.OwnedFiles) != 0 {
		t.Errorf("expected no OwnedFiles for scout, got %v", c.OwnedFiles)
	}
	if len(c.FrozenPaths) != 0 {
		t.Errorf("expected no FrozenPaths for scout, got %v", c.FrozenPaths)
	}
	if c.TrackCommits {
		t.Error("expected TrackCommits=false for scout")
	}
	if c.AgentRole != "scout" {
		t.Errorf("expected AgentRole=scout, got %s", c.AgentRole)
	}
}

func TestBuildConstraints_ScaffoldRole(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Scaffolds: []protocol.ScaffoldFile{
			{FilePath: "pkg/tools/constraints.go"},
			{FilePath: "pkg/types/shared.go"},
		},
	}

	c := buildConstraints(manifest, "", "scaffold")
	if c == nil {
		t.Fatal("expected non-nil constraints for scaffold role")
	}

	// Scaffold should have allowed path prefixes from scaffold file paths
	if len(c.AllowedPathPrefixes) != 2 {
		t.Fatalf("expected 2 AllowedPathPrefixes, got %d", len(c.AllowedPathPrefixes))
	}

	prefixSet := make(map[string]bool)
	for _, p := range c.AllowedPathPrefixes {
		prefixSet[p] = true
	}
	if !prefixSet["pkg/tools/constraints.go"] {
		t.Error("expected pkg/tools/constraints.go in AllowedPathPrefixes")
	}
	if !prefixSet["pkg/types/shared.go"] {
		t.Error("expected pkg/types/shared.go in AllowedPathPrefixes")
	}

	// No ownership or freeze for scaffold
	if len(c.OwnedFiles) != 0 {
		t.Errorf("expected no OwnedFiles for scaffold, got %v", c.OwnedFiles)
	}
	if c.TrackCommits {
		t.Error("expected TrackCommits=false for scaffold")
	}
	if c.AgentRole != "scaffold" {
		t.Errorf("expected AgentRole=scaffold, got %s", c.AgentRole)
	}
}

func TestBuildConstraints_WaveRole(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/engine/runner.go", Agent: "A", Wave: 1},
			{File: "pkg/engine/engine.go", Agent: "A", Wave: 1},
			{File: "pkg/other/file.go", Agent: "B", Wave: 1},
		},
		Scaffolds: []protocol.ScaffoldFile{
			{FilePath: "pkg/tools/constraints.go"},
		},
		InterfaceContracts: []protocol.InterfaceContract{
			{Name: "Constraints", Location: "pkg/tools/constraints.go"},
			{Name: "Config", Location: "pkg/agent/backend/backend.go"},
		},
	}

	c := buildConstraints(manifest, "A", "wave")
	if c == nil {
		t.Fatal("expected non-nil constraints for wave role")
	}

	// I1: Only agent A's files
	if len(c.OwnedFiles) != 2 {
		t.Fatalf("expected 2 OwnedFiles, got %d: %v", len(c.OwnedFiles), c.OwnedFiles)
	}
	if !c.OwnedFiles["pkg/engine/runner.go"] {
		t.Error("expected pkg/engine/runner.go in OwnedFiles")
	}
	if !c.OwnedFiles["pkg/engine/engine.go"] {
		t.Error("expected pkg/engine/engine.go in OwnedFiles")
	}
	if c.OwnedFiles["pkg/other/file.go"] {
		t.Error("agent B's file should NOT be in agent A's OwnedFiles")
	}

	// I2: Frozen paths = scaffolds + interface contract locations
	if len(c.FrozenPaths) != 2 {
		t.Fatalf("expected 2 FrozenPaths, got %d: %v", len(c.FrozenPaths), c.FrozenPaths)
	}
	if !c.FrozenPaths["pkg/tools/constraints.go"] {
		t.Error("expected scaffold path in FrozenPaths")
	}
	if !c.FrozenPaths["pkg/agent/backend/backend.go"] {
		t.Error("expected interface contract location in FrozenPaths")
	}

	// I5: Commit tracking
	if !c.TrackCommits {
		t.Error("expected TrackCommits=true for wave role")
	}

	if c.AgentRole != "wave" {
		t.Errorf("expected AgentRole=wave, got %s", c.AgentRole)
	}
	if c.AgentID != "A" {
		t.Errorf("expected AgentID=A, got %s", c.AgentID)
	}
}

func TestBuildConstraints_WaveRole_FreezeTime(t *testing.T) {
	freezeTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	manifest := &protocol.IMPLManifest{
		WorktreesCreatedAt: &freezeTime,
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1},
		},
		Scaffolds: []protocol.ScaffoldFile{
			{FilePath: "pkg/scaffold.go"},
		},
	}

	c := buildConstraints(manifest, "A", "wave")
	if c == nil {
		t.Fatal("expected non-nil constraints")
	}

	if c.FreezeTime == nil {
		t.Fatal("expected non-nil FreezeTime")
	}
	if !c.FreezeTime.Equal(freezeTime) {
		t.Errorf("expected FreezeTime=%v, got %v", freezeTime, *c.FreezeTime)
	}
}

func TestBuildConstraints_NilManifest(t *testing.T) {
	c := buildConstraints(nil, "A", "wave")
	if c != nil {
		t.Errorf("expected nil constraints for nil manifest, got %+v", c)
	}
}

func TestBuildWaveConstraints_Integration(t *testing.T) {
	// Create a temporary IMPL manifest file
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	yamlContent := `title: Test Feature
feature_slug: test
verdict: SUITABLE
test_command: "go test ./..."
file_ownership:
  - file: pkg/engine/runner.go
    agent: A
    wave: 1
  - file: pkg/engine/types.go
    agent: A
    wave: 1
  - file: pkg/other/file.go
    agent: B
    wave: 1
scaffolds:
  - file_path: pkg/tools/constraints.go
    contents: "package tools"
interface_contracts:
  - name: Constraints
    description: Shared config struct
    definition: "type Constraints struct {}"
    location: pkg/tools/constraints.go
waves:
  - number: 1
    agents:
      - id: A
        task: Implement runner
      - id: B
        task: Implement other
`

	if err := os.WriteFile(implPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := BuildWaveConstraints(implPath, "A")
	if err != nil {
		t.Fatalf("BuildWaveConstraints failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil constraints")
	}

	// Verify wave role properties
	if c.AgentRole != "wave" {
		t.Errorf("expected role=wave, got %s", c.AgentRole)
	}
	if c.AgentID != "A" {
		t.Errorf("expected agentID=A, got %s", c.AgentID)
	}
	if len(c.OwnedFiles) != 2 {
		t.Errorf("expected 2 owned files, got %d", len(c.OwnedFiles))
	}
	if !c.OwnedFiles["pkg/engine/runner.go"] {
		t.Error("expected pkg/engine/runner.go in owned files")
	}
	if !c.TrackCommits {
		t.Error("expected TrackCommits=true")
	}

	// Verify frozen paths include scaffold + interface contract
	if !c.FrozenPaths["pkg/tools/constraints.go"] {
		t.Error("expected scaffold file in frozen paths")
	}
}

func TestBuildWaveConstraints_Integration_BadPath(t *testing.T) {
	_, err := BuildWaveConstraints("/nonexistent/path/IMPL.yaml", "A")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}
