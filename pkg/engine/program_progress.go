package engine

import (
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// UpdateProgramIMPLStatus updates the status of a single IMPL in the PROGRAM
// manifest and recalculates completion counters (E32).
func UpdateProgramIMPLStatus(manifestPath string, implSlug string, newStatus string) error {
	manifest, err := protocol.ParseProgramManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("UpdateProgramIMPLStatus: failed to parse manifest: %w", err)
	}

	// Find and update the IMPL entry
	found := false
	for i := range manifest.Impls {
		if manifest.Impls[i].Slug == implSlug {
			manifest.Impls[i].Status = newStatus
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("UpdateProgramIMPLStatus: IMPL slug %q not found in manifest", implSlug)
	}

	// Recalculate completion counters
	recalculateCompletion(manifest)

	// Write back
	return writeManifest(manifestPath, manifest)
}

// SyncProgramStatusFromDisk reads IMPL docs from disk and updates the PROGRAM
// manifest status fields to match their on-disk state. It recalculates
// completion counters after syncing (E32).
func SyncProgramStatusFromDisk(manifestPath string, repoPath string) error {
	manifest, err := protocol.ParseProgramManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("SyncProgramStatusFromDisk: failed to parse manifest: %w", err)
	}

	// Use GetProgramStatus to get enriched statuses from disk
	statusResult, err := protocol.GetProgramStatus(manifest, repoPath)
	if err != nil {
		return fmt.Errorf("SyncProgramStatusFromDisk: failed to get program status: %w", err)
	}

	// Build a map of slug -> disk status from the tier statuses
	diskStatus := make(map[string]string)
	for _, tier := range statusResult.TierStatuses {
		for _, implStatus := range tier.ImplStatuses {
			diskStatus[implStatus.Slug] = implStatus.Status
		}
	}

	// Update manifest IMPL statuses from disk
	for i := range manifest.Impls {
		if status, ok := diskStatus[manifest.Impls[i].Slug]; ok {
			manifest.Impls[i].Status = status
		}
	}

	// Recalculate completion counters
	recalculateCompletion(manifest)

	// Write back
	return writeManifest(manifestPath, manifest)
}

// recalculateCompletion updates the completion counters in the manifest
// based on current IMPL statuses.
func recalculateCompletion(manifest *protocol.PROGRAMManifest) {
	// Build slug -> status map
	statusMap := make(map[string]string)
	for _, impl := range manifest.Impls {
		statusMap[impl.Slug] = impl.Status
	}

	implsComplete := 0
	for _, impl := range manifest.Impls {
		if impl.Status == "complete" {
			implsComplete++
		}
	}

	tiersComplete := 0
	for _, tier := range manifest.Tiers {
		allComplete := true
		for _, slug := range tier.Impls {
			if statusMap[slug] != "complete" {
				allComplete = false
				break
			}
		}
		if allComplete && len(tier.Impls) > 0 {
			tiersComplete++
		}
	}

	manifest.Completion.ImplsComplete = implsComplete
	manifest.Completion.TiersComplete = tiersComplete
	manifest.Completion.ImplsTotal = len(manifest.Impls)
	manifest.Completion.TiersTotal = len(manifest.Tiers)
}

// writeManifest marshals the manifest to YAML and writes it to disk.
func writeManifest(path string, manifest *protocol.PROGRAMManifest) error {
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("writeManifest: failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writeManifest: failed to write manifest: %w", err)
	}
	return nil
}
