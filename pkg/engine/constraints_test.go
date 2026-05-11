package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
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

	res := BuildWaveConstraints(context.Background(), implPath, "A")
	if !res.IsSuccess() {
		t.Fatalf("BuildWaveConstraints failed: %v", res.Errors)
	}
	c := res.GetData()
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
	res := BuildWaveConstraints(context.Background(), "/nonexistent/path/IMPL.yaml", "A")
	if res.IsSuccess() {
		t.Error("expected failure for nonexistent path")
	}
}

func TestBuildConstraints_IntegratorRole(t *testing.T) {
	connectors := []protocol.IntegrationConnector{
		{File: "pkg/engine/orchestrator.go", Reason: "wire NewRunner into orchestrator"},
		{File: "cmd/saw/main.go", Reason: "register subcommand"},
	}
	manifest := &protocol.IMPLManifest{}

	c := buildIntegratorConstraints(manifest, connectors)
	if c == nil {
		t.Fatal("expected non-nil constraints for integrator role")
	}

	// Integrator should have AllowedPathPrefixes from connectors
	if len(c.AllowedPathPrefixes) != 2 {
		t.Fatalf("expected 2 AllowedPathPrefixes, got %d: %v", len(c.AllowedPathPrefixes), c.AllowedPathPrefixes)
	}

	prefixSet := make(map[string]bool)
	for _, p := range c.AllowedPathPrefixes {
		prefixSet[p] = true
	}
	if !prefixSet["pkg/engine/orchestrator.go"] {
		t.Error("expected pkg/engine/orchestrator.go in AllowedPathPrefixes")
	}
	if !prefixSet["cmd/saw/main.go"] {
		t.Error("expected cmd/saw/main.go in AllowedPathPrefixes")
	}

	// Integrator should NOT have OwnedFiles or FrozenPaths
	if len(c.OwnedFiles) != 0 {
		t.Errorf("expected no OwnedFiles for integrator, got %v", c.OwnedFiles)
	}
	if len(c.FrozenPaths) != 0 {
		t.Errorf("expected no FrozenPaths for integrator, got %v", c.FrozenPaths)
	}

	// Integrator should track commits for audit trail
	if !c.TrackCommits {
		t.Error("expected TrackCommits=true for integrator")
	}
	if c.AgentRole != "integrator" {
		t.Errorf("expected AgentRole=integrator, got %s", c.AgentRole)
	}
}

func TestBuildConstraints_IntegratorEmpty(t *testing.T) {
	manifest := &protocol.IMPLManifest{}

	// Empty connectors: no path prefixes
	c := buildIntegratorConstraints(manifest, nil)
	if c == nil {
		t.Fatal("expected non-nil constraints for integrator role")
	}

	if len(c.AllowedPathPrefixes) != 0 {
		t.Errorf("expected 0 AllowedPathPrefixes for empty connectors, got %d: %v",
			len(c.AllowedPathPrefixes), c.AllowedPathPrefixes)
	}

	if !c.TrackCommits {
		t.Error("expected TrackCommits=true for integrator even with no connectors")
	}

	// Also test via buildConstraints switch case (no connectors available on struct)
	c2 := buildConstraints(manifest, "", "integrator")
	if c2 == nil {
		t.Fatal("expected non-nil constraints from buildConstraints integrator case")
	}
	if len(c2.AllowedPathPrefixes) != 0 {
		t.Errorf("expected 0 AllowedPathPrefixes from switch case, got %d", len(c2.AllowedPathPrefixes))
	}
	if !c2.TrackCommits {
		t.Error("expected TrackCommits=true from switch case")
	}
}

func TestBuildIntegratorConstraints_LoadAndBuild(t *testing.T) {
	// Create a temporary IMPL manifest with integration_connectors
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
scaffolds:
  - file_path: pkg/tools/constraints.go
    contents: "package tools"
waves:
  - number: 1
    agents:
      - id: A
        task: Implement runner
integration_connectors:
  - file: pkg/engine/orchestrator.go
    reason: wire NewRunner into orchestrator
  - file: cmd/saw/main.go
    reason: register subcommand
`

	if err := os.WriteFile(implPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	res := BuildIntegratorConstraints(context.Background(), implPath)
	if !res.IsSuccess() {
		t.Fatalf("BuildIntegratorConstraints failed: %v", res.Errors)
	}
	c := res.GetData()
	if c == nil {
		t.Fatal("expected non-nil constraints")
	}

	// Verify integrator role properties
	if c.AgentRole != "integrator" {
		t.Errorf("expected role=integrator, got %s", c.AgentRole)
	}
	if len(c.AllowedPathPrefixes) != 2 {
		t.Fatalf("expected 2 AllowedPathPrefixes, got %d: %v", len(c.AllowedPathPrefixes), c.AllowedPathPrefixes)
	}

	prefixSet := make(map[string]bool)
	for _, p := range c.AllowedPathPrefixes {
		prefixSet[p] = true
	}
	if !prefixSet["pkg/engine/orchestrator.go"] {
		t.Error("expected pkg/engine/orchestrator.go in AllowedPathPrefixes")
	}
	if !prefixSet["cmd/saw/main.go"] {
		t.Error("expected cmd/saw/main.go in AllowedPathPrefixes")
	}

	if !c.TrackCommits {
		t.Error("expected TrackCommits=true")
	}
}

func TestBuildIntegratorConstraints_NilManifest(t *testing.T) {
	c := buildIntegratorConstraints(nil, []protocol.IntegrationConnector{
		{File: "foo.go", Reason: "test"},
	})
	if c != nil {
		t.Errorf("expected nil constraints for nil manifest, got %+v", c)
	}
}

func TestBuildIntegratorConstraints_BadPath(t *testing.T) {
	res := BuildIntegratorConstraints(context.Background(), "/nonexistent/path/IMPL.yaml")
	if res.IsSuccess() {
		t.Error("expected failure for nonexistent path")
	}
}

// TestBuildIntegratorConstraints_UsesManifestField verifies that
// BuildIntegratorConstraints uses manifest.IntegrationConnectors directly
// (L4: no raw YAML re-read fallback).
func TestBuildIntegratorConstraints_UsesManifestField(t *testing.T) {
	tmpDir := t.TempDir()
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Write IMPL with integration_connectors in the typed field
	yamlContent := `title: Test L4
feature_slug: test-l4
verdict: SUITABLE
test_command: "go test ./..."
waves:
  - number: 1
    agents:
      - id: A
        task: Implement foo
integration_connectors:
  - file: pkg/wire/connector.go
    reason: wire up new function
`
	if err := os.WriteFile(implPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	res := BuildIntegratorConstraints(context.Background(), implPath)
	if !res.IsSuccess() {
		t.Fatalf("BuildIntegratorConstraints failed: %v", res.Errors)
	}
	c := res.GetData()
	if c == nil {
		t.Fatal("expected non-nil constraints")
	}
	if len(c.AllowedPathPrefixes) != 1 {
		t.Fatalf("expected 1 AllowedPathPrefixes, got %d: %v", len(c.AllowedPathPrefixes), c.AllowedPathPrefixes)
	}
	if c.AllowedPathPrefixes[0] != "pkg/wire/connector.go" {
		t.Errorf("expected pkg/wire/connector.go, got %s", c.AllowedPathPrefixes[0])
	}
}
