package engine

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TierLoopEvent represents an observable event during tier execution.
// Shared between tier loop orchestration and parallel scout launching.
type TierLoopEvent struct {
	Type   string // "tier_started", "scout_launched", "scout_completed", "scout_failed", "impl_complete", "tier_gate", "contracts_frozen", "tier_advanced", "replan_triggered"
	Tier   int
	Detail string
}

// ParallelScoutOpts configures parallel Scout launching (E31).
type ParallelScoutOpts struct {
	ManifestPath string
	RepoPath     string
	TierNumber   int
	Slugs        []string
	Model        string
	OnEvent      func(TierLoopEvent)
}

// ParallelScoutResult collects results from parallel Scout execution.
type ParallelScoutResult struct {
	Completed []string
	Failed    []string
	Errors    map[string]string
}

// runScoutFunc is the function used to launch a single Scout agent.
// It defaults to RunScout but can be overridden in tests.
var runScoutFunc = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
	return RunScout(ctx, opts, onChunk)
}

// LaunchParallelScouts launches N Scout agents in parallel for all pending
// IMPLs in a tier (E31). Each Scout receives the feature description from
// the PROGRAM manifest and the manifest path for frozen contract awareness.
func LaunchParallelScouts(ctx context.Context, opts ParallelScoutOpts) (*ParallelScoutResult, error) {
	if len(opts.Slugs) == 0 {
		return &ParallelScoutResult{
			Errors: make(map[string]string),
		}, nil
	}

	// Parse manifest to get IMPL titles
	manifest, err := protocol.ParseProgramManifest(opts.ManifestPath)
	if err != nil {
		return nil, fmt.Errorf("LaunchParallelScouts: failed to parse manifest: %w", err)
	}

	// Build slug -> title map
	titleMap := make(map[string]string)
	for _, impl := range manifest.Impls {
		titleMap[impl.Slug] = impl.Title
	}

	var (
		mu        sync.Mutex
		completed []string
		failed    []string
		errMap    = make(map[string]string)
		wg        sync.WaitGroup
	)

	for _, slug := range opts.Slugs {
		slug := slug // capture loop variable
		title := titleMap[slug]
		if title == "" {
			title = slug // fallback to slug if no title found
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			// Emit scout_launched event
			if opts.OnEvent != nil {
				opts.OnEvent(TierLoopEvent{
					Type:   "scout_launched",
					Tier:   opts.TierNumber,
					Detail: fmt.Sprintf("Launching Scout for %s", slug),
				})
			}

			implOutPath := filepath.Join(opts.RepoPath, "docs", "IMPL", fmt.Sprintf("IMPL-%s.yaml", slug))

			scoutOpts := RunScoutOpts{
				Feature:             title,
				RepoPath:            opts.RepoPath,
				IMPLOutPath:         implOutPath,
				ProgramManifestPath: opts.ManifestPath,
				ScoutModel:          opts.Model,
			}

			scoutErr := runScoutFunc(ctx, scoutOpts, func(chunk string) {
				// Discard Scout output chunks during parallel execution
			})

			mu.Lock()
			defer mu.Unlock()

			if scoutErr != nil {
				failed = append(failed, slug)
				errMap[slug] = scoutErr.Error()

				if opts.OnEvent != nil {
					opts.OnEvent(TierLoopEvent{
						Type:   "scout_failed",
						Tier:   opts.TierNumber,
						Detail: fmt.Sprintf("Scout failed for %s: %s", slug, scoutErr.Error()),
					})
				}
			} else {
				completed = append(completed, slug)

				if opts.OnEvent != nil {
					opts.OnEvent(TierLoopEvent{
						Type:   "scout_completed",
						Tier:   opts.TierNumber,
						Detail: fmt.Sprintf("Scout completed for %s", slug),
					})
				}
			}
		}()
	}

	// Wait for all goroutines, respecting context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All scouts finished
	case <-ctx.Done():
		// Context cancelled; wait for goroutines to finish (they should
		// exit because RunScout respects context cancellation)
		<-done
	}

	return &ParallelScoutResult{
		Completed: completed,
		Failed:    failed,
		Errors:    errMap,
	}, nil
}
