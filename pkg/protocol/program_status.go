package protocol

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

// GetProgramStatus computes a comprehensive status report from a PROGRAM manifest.
// It determines the current tier, builds tier-level IMPL statuses, contract freeze states,
// and updates completion tracking.
//
// The repoPath is used to optionally cross-reference IMPL docs on disk for real-time
// status updates. If an IMPL doc cannot be found or read, the status from the manifest
// is used as a fallback.
//
// Returns a result.Result[*ProgramStatusData] containing all computed status information.
// codeProgramStatusFailed is a placeholder for result.CodeProgramStatusFailed (N017),
// which is added by Agent C in parallel. Once Agent C's changes are merged,
// replace this with result.CodeProgramStatusFailed.
const codeProgramStatusFailed = "N017_PROGRAM_STATUS_FAILED"

func GetProgramStatus(manifest *PROGRAMManifest, repoPath string) result.Result[*ProgramStatusData] {
	if manifest == nil {
		return result.NewFailure[*ProgramStatusData]([]result.SAWError{{
			Code: codeProgramStatusFailed, Message: "manifest cannot be nil", Severity: "fatal",
		}})
	}

	data := &ProgramStatusData{
		ProgramSlug: manifest.ProgramSlug,
		Title:       manifest.Title,
		State:       manifest.State,
		Completion:  manifest.Completion, // Start with manifest values
	}

	// Build a map of IMPL slug -> status from manifest
	implStatusMap := make(map[string]string)
	for _, impl := range manifest.Impls {
		implStatusMap[impl.Slug] = impl.Status
	}

	// Try to get real-time status from IMPL docs on disk
	// This gracefully handles missing files (they may not be scouted yet)
	implStatusMap = enrichIMPLStatusesFromDisk(implStatusMap, repoPath)

	// Build tier statuses
	tierStatuses := make([]TierStatusDetail, 0, len(manifest.Tiers))
	tiersComplete := 0
	implsComplete := 0

	for _, tier := range manifest.Tiers {
		tierDetail := TierStatusDetail{
			Number:       tier.Number,
			Description:  tier.Description,
			ImplStatuses: make([]ImplTierStatus, 0, len(tier.Impls)),
		}

		allComplete := true
		for _, implSlug := range tier.Impls {
			status, exists := implStatusMap[implSlug]
			if !exists {
				status = "pending" // Default if not found
			}

			tierDetail.ImplStatuses = append(tierDetail.ImplStatuses, ImplTierStatus{
				Slug:   implSlug,
				Status: status,
			})

			if status != "complete" {
				allComplete = false
			} else {
				implsComplete++
			}
		}

		tierDetail.Complete = allComplete
		if allComplete {
			tiersComplete++
		}

		tierStatuses = append(tierStatuses, tierDetail)
	}

	data.TierStatuses = tierStatuses

	// Update completion counts from actual statuses
	data.Completion.TiersComplete = tiersComplete
	data.Completion.ImplsComplete = implsComplete

	// Determine current tier: the lowest-numbered tier with at least one incomplete IMPL
	data.CurrentTier = data.Completion.TiersTotal
	for _, tierDetail := range tierStatuses {
		if !tierDetail.Complete {
			data.CurrentTier = tierDetail.Number
			break
		}
	}

	// Build contract statuses
	data.ContractStatuses = buildContractStatuses(manifest, tierStatuses)

	return result.NewSuccess(data)
}

// enrichIMPLStatusesFromDisk attempts to read IMPL docs from disk to get real-time status.
// Falls back to the provided statusMap if files cannot be read.
func enrichIMPLStatusesFromDisk(statusMap map[string]string, repoPath string) map[string]string {
	enriched := make(map[string]string)
	for slug, fallbackStatus := range statusMap {
		enriched[slug] = getIMPLStatusFromDisk(slug, repoPath, fallbackStatus)
	}
	return enriched
}

// getIMPLStatusFromDisk tries to read an IMPL doc from disk and extract its state.
// Returns fallbackStatus if the file cannot be found or read.
func getIMPLStatusFromDisk(implSlug, repoPath, fallbackStatus string) string {
	// Try common IMPL doc locations: docs/IMPL/<state>/IMPL-<slug>.yaml
	// States to try: complete, pending, in-progress
	states := []string{"complete", "in-progress", "pending", "blocked", "not-suitable"}

	for _, state := range states {
		path := filepath.Join(repoPath, "docs", "IMPL", state, fmt.Sprintf("IMPL-%s.yaml", implSlug))
		if status := tryReadIMPLState(path); status != "" {
			return status
		}
	}

	// Also try root docs/IMPL/ directory
	path := IMPLPath(repoPath, implSlug)
	if status := tryReadIMPLState(path); status != "" {
		return status
	}

	// Fall back to manifest status
	return fallbackStatus
}

// tryReadIMPLState attempts to read an IMPL doc and extract its state field.
// Returns empty string if file doesn't exist or cannot be parsed.
func tryReadIMPLState(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// Parse just enough to get the state field
	var doc struct {
		State string `yaml:"state"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return ""
	}

	return mapIMPLStateToStatus(doc.State)
}

// mapIMPLStateToStatus maps IMPL doc state strings to status strings.
// Delegates to IMPLStateToStatus for canonical behavior.
func mapIMPLStateToStatus(state string) string {
	return IMPLStateToStatus(ProtocolState(state))
}

// buildContractStatuses determines freeze status for each program contract.
// A contract is frozen if its FreezeAt references an IMPL in a completed tier.
func buildContractStatuses(manifest *PROGRAMManifest, tierStatuses []TierStatusDetail) []ContractStatus {
	statuses := make([]ContractStatus, 0, len(manifest.ProgramContracts))

	// Build a map of IMPL slug -> tier number
	implToTier := make(map[string]int)
	for _, tier := range manifest.Tiers {
		for _, implSlug := range tier.Impls {
			implToTier[implSlug] = tier.Number
		}
	}

	// Build a map of tier number -> complete status
	tierComplete := make(map[int]bool)
	for _, tierDetail := range tierStatuses {
		tierComplete[tierDetail.Number] = tierDetail.Complete
	}

	for _, contract := range manifest.ProgramContracts {
		status := ContractStatus{
			Name:     contract.Name,
			Location: contract.Location,
			FreezeAt: contract.FreezeAt,
			Frozen:   false,
		}

		// Determine if the contract is frozen
		if contract.FreezeAt != "" {
			// FreezeAt references an IMPL slug
			tierNum, exists := implToTier[contract.FreezeAt]
			if exists && tierComplete[tierNum] {
				status.Frozen = true
				status.FrozenAtTier = tierNum
			}
		}

		statuses = append(statuses, status)
	}

	return statuses
}
