package protocol

import (
	"fmt"
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

	return errs
}
