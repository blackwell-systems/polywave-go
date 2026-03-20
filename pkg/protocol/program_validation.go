package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// kebabCaseRegex validates kebab-case slugs (lowercase letters, digits, hyphens only)
var kebabCaseRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateProgram validates a PROGRAMManifest against the schema rules including
// P1 (IMPL independence within tier) and tier ordering correctness.
// Returns a slice of ValidationErrors (empty if valid).
func ValidateProgram(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	errs = append(errs, validateProgramRequiredFields(manifest)...)
	errs = append(errs, validateProgramState(manifest)...)
	errs = append(errs, validateIMPLStatuses(manifest)...)
	errs = append(errs, validateP1Independence(manifest)...)
	errs = append(errs, validateTierIMPLConsistency(manifest)...)
	errs = append(errs, validateDependencyValidity(manifest)...)
	errs = append(errs, validateTierOrdering(manifest)...)
	errs = append(errs, validateProgramContractConsumers(manifest)...)
	errs = append(errs, validateSlugFormats(manifest)...)
	errs = append(errs, validateCompletionBounds(manifest)...)

	return errs
}

// validateProgramRequiredFields checks that title, program_slug, and state are non-empty.
func validateProgramRequiredFields(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(manifest.Title) == "" {
		errs = append(errs, ValidationError{
			Code:    "MISSING_FIELD",
			Message: "title is required",
			Field:   "title",
		})
	}

	if strings.TrimSpace(manifest.ProgramSlug) == "" {
		errs = append(errs, ValidationError{
			Code:    "MISSING_FIELD",
			Message: "program_slug is required",
			Field:   "program_slug",
		})
	}

	if strings.TrimSpace(string(manifest.State)) == "" {
		errs = append(errs, ValidationError{
			Code:    "MISSING_FIELD",
			Message: "state is required",
			Field:   "state",
		})
	}

	return errs
}

// validateProgramState checks that state is a valid ProgramState constant.
func validateProgramState(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	if strings.TrimSpace(string(manifest.State)) == "" {
		// Already caught by required field check
		return errs
	}

	validStates := map[ProgramState]bool{
		ProgramStatePlanning:      true,
		ProgramStateValidating:    true,
		ProgramStateReviewed:      true,
		ProgramStateScaffold:      true,
		ProgramStateTierExecuting: true,
		ProgramStateTierVerified:  true,
		ProgramStateComplete:      true,
		ProgramStateBlocked:       true,
		ProgramStateNotSuitable:   true,
	}

	if !validStates[manifest.State] {
		errs = append(errs, ValidationError{
			Code:    "INVALID_STATE",
			Message: fmt.Sprintf("state %q is invalid — must be one of: PLANNING, VALIDATING, REVIEWED, SCAFFOLD, TIER_EXECUTING, TIER_VERIFIED, COMPLETE, BLOCKED, NOT_SUITABLE", manifest.State),
			Field:   "state",
		})
	}

	return errs
}

// validateIMPLStatuses checks that each IMPL status is valid.
func validateIMPLStatuses(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	validStatuses := map[string]bool{
		"pending":   true,
		"scouting":  true,
		"reviewed":  true,
		"executing": true,
		"complete":  true,
	}

	for i, impl := range manifest.Impls {
		if !validStatuses[impl.Status] {
			errs = append(errs, ValidationError{
				Code:    "INVALID_STATUS",
				Message: fmt.Sprintf("IMPL %q has invalid status %q — must be one of: pending, scouting, reviewed, executing, complete", impl.Slug, impl.Status),
				Field:   fmt.Sprintf("impls[%d].status", i),
			})
		}
	}

	return errs
}

// validateP1Independence checks that no IMPL in a tier depends on another IMPL in the same tier.
func validateP1Independence(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	// Build map of IMPL slug -> tier number
	implTier := make(map[string]int)
	for _, impl := range manifest.Impls {
		implTier[impl.Slug] = impl.Tier
	}

	// Check each IMPL's dependencies
	for _, impl := range manifest.Impls {
		for _, dep := range impl.DependsOn {
			depTier, exists := implTier[dep]
			if exists && depTier == impl.Tier {
				errs = append(errs, ValidationError{
					Code:    "P1_VIOLATION",
					Message: fmt.Sprintf("IMPL %q (tier %d) depends on %q (tier %d) — IMPLs within the same tier must be independent", impl.Slug, impl.Tier, dep, depTier),
					Field:   "impls",
				})
			}
		}
	}

	return errs
}

// validateTierIMPLConsistency checks that:
// 1. Every IMPL slug appears in exactly one tier
// 2. Every tier references only IMPLs defined in the impls section
func validateTierIMPLConsistency(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	// Build set of defined IMPL slugs
	definedImpls := make(map[string]bool)
	for _, impl := range manifest.Impls {
		definedImpls[impl.Slug] = true
	}

	// Track which IMPLs appear in which tiers
	implTierCount := make(map[string]int)
	implTierNumbers := make(map[string][]int)

	for _, tier := range manifest.Tiers {
		for _, implSlug := range tier.Impls {
			// Check if IMPL is defined
			if !definedImpls[implSlug] {
				errs = append(errs, ValidationError{
					Code:    "TIER_MISMATCH",
					Message: fmt.Sprintf("tier %d references IMPL %q which is not defined in impls section", tier.Number, implSlug),
					Field:   fmt.Sprintf("tiers[%d].impls", tier.Number-1),
				})
			}
			implTierCount[implSlug]++
			implTierNumbers[implSlug] = append(implTierNumbers[implSlug], tier.Number)
		}
	}

	// Check that every IMPL appears in exactly one tier
	for implSlug := range definedImpls {
		count := implTierCount[implSlug]
		if count == 0 {
			errs = append(errs, ValidationError{
				Code:    "TIER_MISMATCH",
				Message: fmt.Sprintf("IMPL %q is not assigned to any tier", implSlug),
				Field:   "tiers",
			})
		} else if count > 1 {
			errs = append(errs, ValidationError{
				Code:    "TIER_MISMATCH",
				Message: fmt.Sprintf("IMPL %q appears in multiple tiers: %v", implSlug, implTierNumbers[implSlug]),
				Field:   "tiers",
			})
		}
	}

	return errs
}

// validateDependencyValidity checks that all depends_on references point to existing IMPL slugs.
func validateDependencyValidity(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	// Build set of defined IMPL slugs
	definedImpls := make(map[string]bool)
	for _, impl := range manifest.Impls {
		definedImpls[impl.Slug] = true
	}

	// Check all dependencies
	for i, impl := range manifest.Impls {
		for _, dep := range impl.DependsOn {
			if !definedImpls[dep] {
				errs = append(errs, ValidationError{
					Code:    "INVALID_DEPENDENCY",
					Message: fmt.Sprintf("IMPL %q depends on %q which does not exist", impl.Slug, dep),
					Field:   fmt.Sprintf("impls[%d].depends_on", i),
				})
			}
		}
	}

	return errs
}

// validateTierOrdering checks that if IMPL-A depends on IMPL-B, A's tier is strictly greater than B's tier.
func validateTierOrdering(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	// Build map of IMPL slug -> tier number
	implTier := make(map[string]int)
	for _, impl := range manifest.Impls {
		implTier[impl.Slug] = impl.Tier
	}

	// Check each IMPL's dependencies
	for _, impl := range manifest.Impls {
		for _, dep := range impl.DependsOn {
			depTier, exists := implTier[dep]
			if exists && impl.Tier <= depTier {
				errs = append(errs, ValidationError{
					Code:    "TIER_ORDER_VIOLATION",
					Message: fmt.Sprintf("IMPL %q (tier %d) depends on %q (tier %d) — dependent IMPLs must be in strictly later tiers", impl.Slug, impl.Tier, dep, depTier),
					Field:   "impls",
				})
			}
		}
	}

	return errs
}

// validateProgramContractConsumers checks that all consumer.Impl references point to existing IMPL slugs.
func validateProgramContractConsumers(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	// Build set of defined IMPL slugs
	definedImpls := make(map[string]bool)
	for _, impl := range manifest.Impls {
		definedImpls[impl.Slug] = true
	}

	// Check all contract consumers
	for i, contract := range manifest.ProgramContracts {
		for j, consumer := range contract.Consumers {
			if !definedImpls[consumer.Impl] {
				errs = append(errs, ValidationError{
					Code:    "INVALID_CONSUMER",
					Message: fmt.Sprintf("program contract %q references consumer IMPL %q which does not exist", contract.Name, consumer.Impl),
					Field:   fmt.Sprintf("program_contracts[%d].consumers[%d].impl", i, j),
				})
			}
		}
	}

	return errs
}

// validateSlugFormats checks that program_slug and IMPL slugs are kebab-case.
func validateSlugFormats(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	// Validate program_slug
	if manifest.ProgramSlug != "" && !kebabCaseRegex.MatchString(manifest.ProgramSlug) {
		errs = append(errs, ValidationError{
			Code:    "INVALID_SLUG_FORMAT",
			Message: fmt.Sprintf("program_slug %q is not kebab-case (lowercase letters, digits, hyphens only)", manifest.ProgramSlug),
			Field:   "program_slug",
		})
	}

	// Validate IMPL slugs
	for i, impl := range manifest.Impls {
		if impl.Slug != "" && !kebabCaseRegex.MatchString(impl.Slug) {
			errs = append(errs, ValidationError{
				Code:    "INVALID_SLUG_FORMAT",
				Message: fmt.Sprintf("IMPL slug %q is not kebab-case (lowercase letters, digits, hyphens only)", impl.Slug),
				Field:   fmt.Sprintf("impls[%d].slug", i),
			})
		}
	}

	return errs
}

// ValidateP1FileDisjointness checks that IMPLs in the same tier do not have
// overlapping file_ownership entries (P1 invariant). Returns ValidationError
// with Code "P1_FILE_OVERLAP" if the same file appears in multiple IMPLs.
func ValidateP1FileDisjointness(tier int, impls []*IMPLManifest) []ValidationError {
	var errs []ValidationError
	fileToIMPL := make(map[string]string) // file path -> IMPL slug

	for _, impl := range impls {
		for _, fo := range impl.FileOwnership {
			if existingIMPL, exists := fileToIMPL[fo.File]; exists {
				errs = append(errs, ValidationError{
					Code:    "P1_FILE_OVERLAP",
					Message: fmt.Sprintf("File %s owned by both %s and %s in tier %d (violates P1 disjoint ownership)",
						fo.File, existingIMPL, impl.FeatureSlug, tier),
					Field: "file_ownership",
				})
			} else {
				fileToIMPL[fo.File] = impl.FeatureSlug
			}
		}
	}
	return errs
}

// ValidateProgramImportMode performs extended validation for import mode.
// It checks that referenced IMPL docs exist on disk, have valid states
// (reviewed/complete), and don't violate P1 (file_ownership disjointness
// within a tier) or P2 (frozen contract redefinition).
func ValidateProgramImportMode(manifest *PROGRAMManifest, repoPath string) []ValidationError {
	var errs []ValidationError

	// Collect states that qualify as "reviewed or later" for import-mode validation.
	reviewedOrLater := map[ProtocolState]bool{
		StateReviewed:        true,
		StateScaffoldPending: true,
		StateWavePending:     true,
		StateWaveExecuting:   true,
		StateWaveMerging:     true,
		StateWaveVerified:    true,
		StateComplete:        true,
	}

	// Build tier -> []slug mapping for P1 file overlap checks.
	tierIMPLs := make(map[int][]string)
	for _, impl := range manifest.Impls {
		tierIMPLs[impl.Tier] = append(tierIMPLs[impl.Tier], impl.Slug)
	}

	// Build set of frozen contract names (freeze_at references a completed tier).
	frozenContracts := make(map[string]bool)
	completedTiers := make(map[string]bool)
	for _, tier := range manifest.Tiers {
		// A tier is "completed" if all its IMPLs have status "complete".
		allComplete := true
		for _, slug := range tier.Impls {
			for _, impl := range manifest.Impls {
				if impl.Slug == slug && impl.Status != "complete" {
					allComplete = false
					break
				}
			}
			if !allComplete {
				break
			}
		}
		if allComplete && len(tier.Impls) > 0 {
			completedTiers[fmt.Sprintf("tier-%d", tier.Number)] = true
		}
	}
	for _, contract := range manifest.ProgramContracts {
		if contract.FreezeAt != "" && completedTiers[contract.FreezeAt] {
			frozenContracts[contract.Name] = true
		}
	}

	// Collect IMPL docs per tier for P1 file ownership validation.
	// tier -> []*IMPLManifest
	tierIMPLDocs := make(map[int][]*IMPLManifest)

	for _, impl := range manifest.Impls {
		if impl.Status != "reviewed" && impl.Status != "complete" {
			continue
		}

		// Check 1: IMPL file exists on disk.
		implPath := filepath.Join(repoPath, "docs", "IMPL", fmt.Sprintf("IMPL-%s.yaml", impl.Slug))
		completePath := filepath.Join(repoPath, "docs", "IMPL", "complete", fmt.Sprintf("IMPL-%s.yaml", impl.Slug))

		var resolvedPath string
		if _, err := os.Stat(implPath); err == nil {
			resolvedPath = implPath
		} else if _, err := os.Stat(completePath); err == nil {
			resolvedPath = completePath
		} else {
			errs = append(errs, ValidationError{
				Code:    "IMPL_FILE_MISSING",
				Message: fmt.Sprintf("IMPL %q has status %q but IMPL-%s.yaml not found in docs/IMPL/ or docs/IMPL/complete/", impl.Slug, impl.Status, impl.Slug),
				Field:   "impls",
			})
			continue
		}

		// Check 2: Parse IMPL doc and verify state consistency.
		implDoc, err := Load(resolvedPath)
		if err != nil {
			errs = append(errs, ValidationError{
				Code:    "IMPL_FILE_MISSING",
				Message: fmt.Sprintf("IMPL %q: failed to parse %s: %v", impl.Slug, resolvedPath, err),
				Field:   "impls",
			})
			continue
		}

		if impl.Status == "reviewed" && !reviewedOrLater[implDoc.State] {
			errs = append(errs, ValidationError{
				Code:    "IMPL_STATE_MISMATCH",
				Message: fmt.Sprintf("IMPL %q has program status %q but IMPL doc state is %q (expected REVIEWED or later)", impl.Slug, impl.Status, implDoc.State),
				Field:   "impls",
			})
		}
		if impl.Status == "complete" && implDoc.State != StateComplete {
			errs = append(errs, ValidationError{
				Code:    "IMPL_STATE_MISMATCH",
				Message: fmt.Sprintf("IMPL %q has program status %q but IMPL doc state is %q (expected COMPLETE)", impl.Slug, impl.Status, implDoc.State),
				Field:   "impls",
			})
		}

		// Collect IMPL doc for tier-level P1 validation.
		tierIMPLDocs[impl.Tier] = append(tierIMPLDocs[impl.Tier], implDoc)

		// Check P2: frozen contract redefinition.
		for _, ic := range implDoc.InterfaceContracts {
			if frozenContracts[ic.Name] {
				errs = append(errs, ValidationError{
					Code:    "P2_CONTRACT_REDEFINITION",
					Message: fmt.Sprintf("IMPL %q redefines frozen program contract %q", impl.Slug, ic.Name),
					Field:   "interface_contracts",
				})
			}
		}
	}

	// Run P1 file disjointness check for each tier.
	for tier, implsInTier := range tierIMPLDocs {
		p1Errs := ValidateP1FileDisjointness(tier, implsInTier)
		errs = append(errs, p1Errs...)
	}

	return errs
}

// PartitionIMPLsByStatus splits a tier's IMPLs into two groups:
//   - needsScout: IMPLs with status "pending" or "scouting"
//   - preExisting: IMPLs with status "reviewed" or "complete"
func PartitionIMPLsByStatus(manifest *PROGRAMManifest, tierNumber int) (needsScout []ProgramIMPL, preExisting []ProgramIMPL) {
	// Build slug -> ProgramIMPL map.
	implMap := make(map[string]ProgramIMPL)
	for _, impl := range manifest.Impls {
		implMap[impl.Slug] = impl
	}

	// Find the tier and partition its IMPLs.
	for _, tier := range manifest.Tiers {
		if tier.Number != tierNumber {
			continue
		}
		for _, slug := range tier.Impls {
			impl, ok := implMap[slug]
			if !ok {
				continue
			}
			switch impl.Status {
			case "pending", "scouting":
				needsScout = append(needsScout, impl)
			case "reviewed", "complete":
				preExisting = append(preExisting, impl)
			default:
				// Other statuses (e.g. "executing") go to needsScout as a safe default.
				needsScout = append(needsScout, impl)
			}
		}
		break
	}
	return needsScout, preExisting
}

// validateCompletionBounds checks that completion counts don't exceed totals.
func validateCompletionBounds(manifest *PROGRAMManifest) []ValidationError {
	var errs []ValidationError

	if manifest.Completion.TiersComplete > manifest.Completion.TiersTotal {
		errs = append(errs, ValidationError{
			Code:    "COMPLETION_BOUNDS",
			Message: fmt.Sprintf("tiers_complete (%d) exceeds tiers_total (%d)", manifest.Completion.TiersComplete, manifest.Completion.TiersTotal),
			Field:   "completion.tiers_complete",
		})
	}

	if manifest.Completion.ImplsComplete > manifest.Completion.ImplsTotal {
		errs = append(errs, ValidationError{
			Code:    "COMPLETION_BOUNDS",
			Message: fmt.Sprintf("impls_complete (%d) exceeds impls_total (%d)", manifest.Completion.ImplsComplete, manifest.Completion.ImplsTotal),
			Field:   "completion.impls_complete",
		})
	}

	if manifest.Completion.ImplsTotal != len(manifest.Impls) {
		errs = append(errs, ValidationError{
			Code:    "IMPLS_TOTAL_MISMATCH",
			Message: fmt.Sprintf("impls_total (%d) must equal the number of impls entries (%d)", manifest.Completion.ImplsTotal, len(manifest.Impls)),
			Field:   "completion.impls_total",
		})
	}

	return errs
}
