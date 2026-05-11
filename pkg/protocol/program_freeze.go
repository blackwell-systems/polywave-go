package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// FreezeContracts identifies and freezes program contracts at a tier boundary.
// A contract freezes if its FreezeAt field references an IMPL in the completing tier.
// For each contract to freeze, verifies that:
//   - The file at contract.Location exists in repoPath
//   - The file is committed to HEAD (git status --porcelain returns empty)
//
// Returns a result.Result[*FreezeContractsData] with:
//   - ContractsFrozen: contracts successfully frozen (file exists and committed)
//   - ContractsSkipped: contracts whose FreezeAt does not match this tier
//   - Errors: contracts where file is missing or uncommitted
//   - Success: true only if all matching contracts are successfully frozen
func FreezeContracts(manifest *PROGRAMManifest, tierNumber int, repoPath string) result.Result[*FreezeContractsData] {
	data := &FreezeContractsData{
		TierNumber:       tierNumber,
		ContractsFrozen:  []FrozenContract{},
		ContractsSkipped: []string{},
		Success:          false,
		Errors:           []string{},
	}

	// Find the tier by number
	var tier *ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == tierNumber {
			tier = &manifest.Tiers[i]
			break
		}
	}

	if tier == nil {
		return result.NewFailure[*FreezeContractsData]([]result.PolywaveError{{
			Code: result.CodeFreezeError, Message: fmt.Sprintf("tier %d not found in manifest", tierNumber), Severity: "fatal",
		}})
	}

	// Build a set of IMPL slugs in this tier
	tierImplSlugs := make(map[string]bool)
	for _, implSlug := range tier.Impls {
		tierImplSlugs[implSlug] = true
	}

	// Process each contract
	for _, contract := range manifest.ProgramContracts {
		// Skip contracts with empty FreezeAt
		if strings.TrimSpace(contract.FreezeAt) == "" {
			data.ContractsSkipped = append(data.ContractsSkipped, contract.Name)
			continue
		}

		// Check if this contract should freeze at this tier
		// Match by checking if any IMPL slug in the tier appears as a whole word
		// in the contract's FreezeAt string
		shouldFreeze := false
		for slug := range tierImplSlugs {
			if matchesSlugInFreezeAt(contract.FreezeAt, slug) {
				shouldFreeze = true
				break
			}
		}

		if !shouldFreeze {
			data.ContractsSkipped = append(data.ContractsSkipped, contract.Name)
			continue
		}

		// Contract should freeze — verify file exists and is committed
		frozen := FrozenContract{
			Name:       contract.Name,
			Location:   contract.Location,
			FreezeAt:   contract.FreezeAt,
			FileExists: false,
			Committed:  false,
		}

		// Check if file exists
		fullPath := filepath.Join(repoPath, contract.Location)
		if _, err := os.Stat(fullPath); err == nil {
			frozen.FileExists = true
		}

		// Check if file is committed (git status --porcelain returns empty)
		if frozen.FileExists {
			out, err := git.StatusPorcelainFile(repoPath, contract.Location)
			if err == nil && out == "" {
				frozen.Committed = true
			}
		}

		// Record result
		if frozen.FileExists && frozen.Committed {
			data.ContractsFrozen = append(data.ContractsFrozen, frozen)
		} else {
			data.ContractsFrozen = append(data.ContractsFrozen, frozen)
			if !frozen.FileExists {
				data.Errors = append(data.Errors, fmt.Sprintf("contract %s: file not found at %s", contract.Name, contract.Location))
			} else if !frozen.Committed {
				data.Errors = append(data.Errors, fmt.Sprintf("contract %s: file not committed at %s", contract.Name, contract.Location))
			}
		}
	}

	// Success is true only if all matching contracts are successfully frozen
	data.Success = len(data.Errors) == 0

	if !data.Success {
		return result.NewPartial(data, []result.PolywaveError{{
			Code: result.CodeFreezeError, Message: fmt.Sprintf("%d freeze error(s)", len(data.Errors)), Severity: "error",
		}})
	}
	return result.NewSuccess(data)
}

// matchesSlugInFreezeAt checks if the given IMPL slug appears as a whole word
// in the freezeAt string. For example:
//   - "auth" matches "IMPL-auth completion" → true
//   - "auth" matches "authorization" → false
//   - "auth" matches "after auth done" → true
func matchesSlugInFreezeAt(freezeAt, slug string) bool {
	// Use word boundary matching: \b ensures we match whole words only
	// Quote the slug to escape any regex special characters
	pattern := `\b` + regexp.QuoteMeta(slug) + `\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(freezeAt)
}
