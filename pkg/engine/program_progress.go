package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// UpdateProgData holds data returned by UpdateProgramIMPLStatus.
type UpdateProgData struct {
	ManifestPath   string `json:"manifest_path"`
	ImplSlug       string `json:"impl_slug"`
	NewStatus      string `json:"new_status"`
	ImplsComplete  int    `json:"impls_complete"`
	TiersComplete  int    `json:"tiers_complete"`
}

// SyncData holds data returned by SyncProgramStatusFromDisk.
type SyncData struct {
	ManifestPath  string            `json:"manifest_path"`
	StatusUpdates map[string]string `json:"status_updates"`
	ImplsComplete int               `json:"impls_complete"`
	TiersComplete int               `json:"tiers_complete"`
}

// WriteManifestData holds data returned by writeManifest.
type WriteManifestData struct {
	Path string `json:"path"`
}

// UpdateProgramIMPLStatus updates the status of a single IMPL in the PROGRAM
// manifest and recalculates completion counters (E32).
func UpdateProgramIMPLStatus(manifestPath string, implSlug string, newStatus string) result.Result[UpdateProgData] {
	manifest, err := protocol.ParseProgramManifest(manifestPath)
	if err != nil {
		return result.NewFailure[UpdateProgData]([]result.SAWError{
			result.NewFatal("ENGINE_UPDATE_PROG_PARSE_FAILED",
				fmt.Sprintf("UpdateProgramIMPLStatus: failed to parse manifest: %v", err)).
				WithContext("manifest_path", manifestPath),
		})
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
		return result.NewFailure[UpdateProgData]([]result.SAWError{
			result.NewFatal("ENGINE_UPDATE_PROG_SLUG_NOT_FOUND",
				fmt.Sprintf("UpdateProgramIMPLStatus: IMPL slug %q not found in manifest", implSlug)).
				WithContext("impl_slug", implSlug).
				WithContext("manifest_path", manifestPath),
		})
	}

	// Recalculate completion counters
	recalculateCompletion(manifest)

	// Write back
	writeRes := writeManifest(manifestPath, manifest)
	if writeRes.IsFatal() {
		return result.NewFailure[UpdateProgData](writeRes.Errors)
	}

	return result.NewSuccess(UpdateProgData{
		ManifestPath:  manifestPath,
		ImplSlug:      implSlug,
		NewStatus:     newStatus,
		ImplsComplete: manifest.Completion.ImplsComplete,
		TiersComplete: manifest.Completion.TiersComplete,
	})
}

// SyncProgramStatusFromDisk reads IMPL docs from disk and updates the PROGRAM
// manifest status fields to match their on-disk state. It recalculates
// completion counters after syncing (E32).
func SyncProgramStatusFromDisk(manifestPath string, repoPath string) result.Result[SyncData] {
	manifest, err := protocol.ParseProgramManifest(manifestPath)
	if err != nil {
		return result.NewFailure[SyncData]([]result.SAWError{
			result.NewFatal("ENGINE_SYNC_PARSE_FAILED",
				fmt.Sprintf("SyncProgramStatusFromDisk: failed to parse manifest: %v", err)).
				WithContext("manifest_path", manifestPath),
		})
	}

	// Use GetProgramStatus to get enriched statuses from disk
	statusRes := protocol.GetProgramStatus(manifest, repoPath)
	if statusRes.IsFatal() {
		return result.NewFailure[SyncData]([]result.SAWError{
			result.NewFatal("ENGINE_SYNC_STATUS_FAILED",
				fmt.Sprintf("SyncProgramStatusFromDisk: failed to get program status: %s", statusRes.Errors[0].Message)).
				WithContext("manifest_path", manifestPath),
		})
	}
	statusResult := statusRes.GetData()

	// Build a map of slug -> disk status from the tier statuses
	diskStatus := make(map[string]string)
	for _, tier := range statusResult.TierStatuses {
		for _, implStatus := range tier.ImplStatuses {
			diskStatus[implStatus.Slug] = implStatus.Status
		}
	}

	// Track status updates
	updates := make(map[string]string)

	// Update manifest IMPL statuses from disk
	for i := range manifest.Impls {
		if status, ok := diskStatus[manifest.Impls[i].Slug]; ok {
			if manifest.Impls[i].Status != status {
				updates[manifest.Impls[i].Slug] = status
			}
			manifest.Impls[i].Status = status
		}
	}

	// Recalculate completion counters
	recalculateCompletion(manifest)

	// Write back
	writeRes := writeManifest(manifestPath, manifest)
	if writeRes.IsFatal() {
		return result.NewFailure[SyncData](writeRes.Errors)
	}

	return result.NewSuccess(SyncData{
		ManifestPath:  manifestPath,
		StatusUpdates: updates,
		ImplsComplete: manifest.Completion.ImplsComplete,
		TiersComplete: manifest.Completion.TiersComplete,
	})
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
func writeManifest(path string, manifest *protocol.PROGRAMManifest) result.Result[WriteManifestData] {
	if err := protocol.SaveYAML(context.TODO(), path, manifest); err != nil {
		return result.NewFailure[WriteManifestData]([]result.SAWError{
			result.NewFatal("ENGINE_WRITE_MANIFEST_FAILED",
				fmt.Sprintf("writeManifest: %v", err)).
				WithContext("path", path),
		})
	}
	return result.NewSuccess(WriteManifestData{Path: path})
}
