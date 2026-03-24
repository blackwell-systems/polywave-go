package protocol

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// validateAllEnums validates ALL enum fields in the manifest, complementing
// existing enum validators in enumvalidation.go (which cover DC02, DC03, DC06).
// This covers: FileOwnership.Action, QualityGates.Level, ScaffoldFile.Status,
// PreMortemRow.Likelihood, PreMortemRow.Impact.
// Empty values are allowed for backward compatibility.
func validateAllEnums(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	errs = append(errs, validateFileOwnershipActions(m)...)
	errs = append(errs, validateQualityGatesLevel(m)...)
	errs = append(errs, validateScaffoldStatuses(m)...)
	errs = append(errs, validatePreMortemRowEnums(m)...)

	return errs
}

// validateFileOwnershipActions checks FileOwnership.Action values.
// Valid: "new", "modify", "delete", or empty.
func validateFileOwnershipActions(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	validActions := map[string]bool{
		"new":    true,
		"modify": true,
		"delete": true,
	}

	for i, fo := range m.FileOwnership {
		if fo.Action == "" {
			continue
		}
		if !validActions[fo.Action] {
			errs = append(errs, result.SAWError{
				Code:     SV01InvalidEnum,
				Message:  fmt.Sprintf("file_ownership[%d].action has invalid value %q — must be one of: new, modify, delete", i, fo.Action),
				Severity: "error",
				Field:    fmt.Sprintf("file_ownership[%d].action", i),
			})
		}
	}

	return errs
}

// validateQualityGatesLevel checks QualityGates.Level value.
// Valid: "quick", "standard", "full", or empty.
func validateQualityGatesLevel(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	if m.QualityGates == nil {
		return errs
	}

	if m.QualityGates.Level == "" {
		return errs
	}

	validLevels := map[string]bool{
		"quick":    true,
		"standard": true,
		"full":     true,
	}

	if !validLevels[m.QualityGates.Level] {
		errs = append(errs, result.SAWError{
			Code:     SV01InvalidEnum,
			Message:  fmt.Sprintf("quality_gates.level has invalid value %q — must be one of: quick, standard, full", m.QualityGates.Level),
			Severity: "error",
			Field:    "quality_gates.level",
		})
	}

	return errs
}

// validateScaffoldStatuses checks ScaffoldFile.Status values.
// Valid: "pending", "committed", strings starting with "committed" (e.g. "committed (abc123)"),
// strings starting with "FAILED", or empty.
func validateScaffoldStatuses(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	for i, sf := range m.Scaffolds {
		if sf.Status == "" {
			continue
		}
		if sf.Status == "pending" {
			continue
		}
		if strings.HasPrefix(sf.Status, "committed") {
			continue
		}
		if strings.HasPrefix(sf.Status, "FAILED") {
			continue
		}
		errs = append(errs, result.SAWError{
			Code:     SV01InvalidEnum,
			Message:  fmt.Sprintf("scaffolds[%d].status has invalid value %q — must be one of: pending, committed, or start with \"committed\" or \"FAILED\"", i, sf.Status),
			Severity: "error",
			Field:    fmt.Sprintf("scaffolds[%d].status", i),
		})
	}

	return errs
}

// validatePreMortemRowEnums checks PreMortemRow.Likelihood and Impact values.
// Valid: "low", "medium", "high", or empty.
func validatePreMortemRowEnums(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	if m.PreMortem == nil {
		return errs
	}

	validValues := map[string]bool{
		"low":    true,
		"medium": true,
		"high":   true,
	}

	for i, row := range m.PreMortem.Rows {
		if row.Likelihood != "" && !validValues[row.Likelihood] {
			errs = append(errs, result.SAWError{
				Code:     SV01InvalidEnum,
				Message:  fmt.Sprintf("pre_mortem.rows[%d].likelihood has invalid value %q — must be one of: low, medium, high", i, row.Likelihood),
				Severity: "error",
				Field:    fmt.Sprintf("pre_mortem.rows[%d].likelihood", i),
			})
		}
		if row.Impact != "" && !validValues[row.Impact] {
			errs = append(errs, result.SAWError{
				Code:     SV01InvalidEnum,
				Message:  fmt.Sprintf("pre_mortem.rows[%d].impact has invalid value %q — must be one of: low, medium, high", i, row.Impact),
				Severity: "error",
				Field:    fmt.Sprintf("pre_mortem.rows[%d].impact", i),
			})
		}
	}

	return errs
}
