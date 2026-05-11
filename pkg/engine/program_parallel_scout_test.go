package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// createTestManifest creates a temporary PROGRAM manifest for testing.
func createTestManifest(t *testing.T, manifest *protocol.PROGRAMManifest) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "PROGRAM.yaml")
	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}
	return path
}

func TestParallelScout_ThreeScouts_TwoSucceedOneFails(t *testing.T) {
	manifest := &protocol.PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-prog",
		Impls: []protocol.ProgramIMPL{
			{Slug: "feature-a", Title: "Feature A", Tier: 1, Status: "pending"},
			{Slug: "feature-b", Title: "Feature B", Tier: 1, Status: "pending"},
			{Slug: "feature-c", Title: "Feature C", Tier: 1, Status: "pending"},
		},
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"feature-a", "feature-b", "feature-c"}},
		},
		Completion: protocol.ProgramCompletion{TiersTotal: 1, ImplsTotal: 3},
	}
	manifestPath := createTestManifest(t, manifest)

	// Override runScoutFunc: feature-b fails, others succeed
	origFunc := runScoutFunc
	runScoutFunc = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
		if opts.Feature == "Feature B" {
			return fmt.Errorf("scout failed for feature-b")
		}
		return nil
	}
	defer func() { runScoutFunc = origFunc }()

	var events []TierLoopEvent
	var mu sync.Mutex

	result, err := LaunchParallelScouts(context.Background(), ParallelScoutOpts{
		ManifestPath: manifestPath,
		RepoPath:     t.TempDir(),
		TierNumber:   1,
		Slugs:        []string{"feature-a", "feature-b", "feature-c"},
		OnEvent: func(e TierLoopEvent) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, e)
		},
	})
	if err != nil {
		t.Fatalf("LaunchParallelScouts returned error: %v", err)
	}

	// Verify results
	if len(result.Completed) != 2 {
		t.Errorf("expected 2 completed, got %d: %v", len(result.Completed), result.Completed)
	}
	if len(result.Failed) != 1 {
		t.Errorf("expected 1 failed, got %d: %v", len(result.Failed), result.Failed)
	}
	if result.Failed[0] != "feature-b" {
		t.Errorf("expected feature-b to fail, got %v", result.Failed)
	}
	if _, ok := result.Errors["feature-b"]; !ok {
		t.Error("expected error entry for feature-b")
	}

	// Verify events were emitted (3 launched + 2 completed + 1 failed = 6)
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 6 {
		t.Errorf("expected 6 events, got %d", len(events))
	}

	// Count event types
	launched, completed, failed := 0, 0, 0
	for _, e := range events {
		switch e.Type {
		case "scout_launched":
			launched++
		case "scout_completed":
			completed++
		case "scout_failed":
			failed++
		}
	}
	if launched != 3 {
		t.Errorf("expected 3 scout_launched events, got %d", launched)
	}
	if completed != 2 {
		t.Errorf("expected 2 scout_completed events, got %d", completed)
	}
	if failed != 1 {
		t.Errorf("expected 1 scout_failed events, got %d", failed)
	}
}

func TestParallelScout_RespectsContextCancellation(t *testing.T) {
	manifest := &protocol.PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-prog",
		Impls: []protocol.ProgramIMPL{
			{Slug: "slow-feature", Title: "Slow Feature", Tier: 1, Status: "pending"},
		},
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"slow-feature"}},
		},
		Completion: protocol.ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
	}
	manifestPath := createTestManifest(t, manifest)

	// Override runScoutFunc to respect context cancellation
	origFunc := runScoutFunc
	runScoutFunc = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
		<-ctx.Done()
		return ctx.Err()
	}
	defer func() { runScoutFunc = origFunc }()

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately to test cancellation path
	cancel()

	result, err := LaunchParallelScouts(ctx, ParallelScoutOpts{
		ManifestPath: manifestPath,
		RepoPath:     t.TempDir(),
		TierNumber:   1,
		Slugs:        []string{"slow-feature"},
	})
	if err != nil {
		t.Fatalf("LaunchParallelScouts returned error: %v", err)
	}

	// The scout should have failed due to context cancellation
	if len(result.Failed) != 1 {
		t.Errorf("expected 1 failed (cancelled), got %d failed, %d completed", len(result.Failed), len(result.Completed))
	}
}

func TestParallelScout_EmptySlugs(t *testing.T) {
	result, err := LaunchParallelScouts(context.Background(), ParallelScoutOpts{
		ManifestPath: "/nonexistent",
		RepoPath:     t.TempDir(),
		TierNumber:   1,
		Slugs:        []string{},
	})
	if err != nil {
		t.Fatalf("expected no error for empty slugs, got: %v", err)
	}
	if len(result.Completed) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty results for empty slugs")
	}
}

func TestParallelScout_ScoutOptsCorrect(t *testing.T) {
	manifest := &protocol.PROGRAMManifest{
		Title:       "Test Program",
		ProgramSlug: "test-prog",
		Impls: []protocol.ProgramIMPL{
			{Slug: "my-feature", Title: "My Feature Title", Tier: 1, Status: "pending"},
		},
		Tiers: []protocol.ProgramTier{
			{Number: 1, Impls: []string{"my-feature"}},
		},
		Completion: protocol.ProgramCompletion{TiersTotal: 1, ImplsTotal: 1},
	}
	manifestPath := createTestManifest(t, manifest)
	repoPath := t.TempDir()

	// Capture the opts passed to RunScout
	var capturedOpts RunScoutOpts
	origFunc := runScoutFunc
	runScoutFunc = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
		capturedOpts = opts
		return nil
	}
	defer func() { runScoutFunc = origFunc }()

	_, err := LaunchParallelScouts(context.Background(), ParallelScoutOpts{
		ManifestPath: manifestPath,
		RepoPath:     repoPath,
		TierNumber:   1,
		Slugs:        []string{"my-feature"},
		Model:        "claude-sonnet-4-20250514",
	})
	if err != nil {
		t.Fatalf("LaunchParallelScouts returned error: %v", err)
	}

	// Verify Scout opts
	if capturedOpts.Feature != "My Feature Title" {
		t.Errorf("expected Feature='My Feature Title', got %q", capturedOpts.Feature)
	}
	if capturedOpts.RepoPath != repoPath {
		t.Errorf("expected RepoPath=%q, got %q", repoPath, capturedOpts.RepoPath)
	}
	expectedIMPLOut := filepath.Join(repoPath, "docs", "IMPL", "IMPL-my-feature.yaml")
	if capturedOpts.IMPLOutPath != expectedIMPLOut {
		t.Errorf("expected IMPLOutPath=%q, got %q", expectedIMPLOut, capturedOpts.IMPLOutPath)
	}
	if capturedOpts.ProgramManifestPath != manifestPath {
		t.Errorf("expected ProgramManifestPath=%q, got %q", manifestPath, capturedOpts.ProgramManifestPath)
	}
	if capturedOpts.ScoutModel != "claude-sonnet-4-20250514" {
		t.Errorf("expected ScoutModel='claude-sonnet-4-20250514', got %q", capturedOpts.ScoutModel)
	}
}
