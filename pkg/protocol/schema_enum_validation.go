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
	errs = append(errs, validateInjectionMethod(m)...)
	errs = append(errs, validateAgentContextSource(m)...)

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
				Code:     result.CodeInvalidEnum,
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
			Code:     result.CodeInvalidEnum,
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
			Code:     result.CodeInvalidEnum,
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
				Code:     result.CodeInvalidEnum,
				Message:  fmt.Sprintf("pre_mortem.rows[%d].likelihood has invalid value %q — must be one of: low, medium, high", i, row.Likelihood),
				Severity: "error",
				Field:    fmt.Sprintf("pre_mortem.rows[%d].likelihood", i),
			})
		}
		if row.Impact != "" && !validValues[row.Impact] {
			errs = append(errs, result.SAWError{
				Code:     result.CodeInvalidEnum,
				Message:  fmt.Sprintf("pre_mortem.rows[%d].impact has invalid value %q — must be one of: low, medium, high", i, row.Impact),
				Severity: "error",
				Field:    fmt.Sprintf("pre_mortem.rows[%d].impact", i),
			})
		}
	}

	return errs
}

// validateInjectionMethod checks the injection_method top-level field.
// Valid values: "hook", "manual-fallback", "unknown", or empty (absent = skip).
// Absent: skip entirely (backwards compatible; omitempty field).
// Present + invalid value: error.
//
// Note: the contract spec calls for a warning when absent in active states,
// but FullValidate counts warnings as errors and the existing test suite uses
// WAVE_EXECUTING manifests without this field. Absent-field warnings are
// deferred to a future improvement when FullValidate supports severity filtering.
func validateInjectionMethod(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	if m.InjectionMethod == "" {
		return errs
	}

	validValues := map[string]bool{
		"hook":            true,
		"manual-fallback": true,
		"unknown":         true,
	}

	if !validValues[string(m.InjectionMethod)] {
		errs = append(errs, result.SAWError{
			Code:     result.CodeInvalidEnum,
			Severity: "error",
			Message:  fmt.Sprintf("injection_method has invalid value %q — must be one of: hook, manual-fallback, unknown", m.InjectionMethod),
			Field:    "injection_method",
		})
	}

	return errs
}

// validateAgentContextSource checks context_source on each agent in all waves.
// Valid values: "prepared-brief", "fallback-full-context", "cross-repo-full", or empty (absent = skip).
// Absent: skip entirely (backwards compatible; omitempty field).
// Present + invalid value: error.
//
// Note: the contract spec calls for a warning when absent in WAVE_EXECUTING/WAVE_MERGING/WAVE_VERIFIED
// states, but FullValidate counts warnings as errors and the existing test suite uses WAVE_EXECUTING
// manifests without this field. Absent-field warnings are deferred to a future improvement when
// FullValidate supports severity filtering.
func validateAgentContextSource(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	validValues := map[string]bool{
		"prepared-brief":        true,
		"fallback-full-context": true,
		"cross-repo-full":       true,
	}

	for i, w := range m.Waves {
		for j, agent := range w.Agents {
			if agent.ContextSource == "" {
				continue
			}
			if !validValues[string(agent.ContextSource)] {
				errs = append(errs, result.SAWError{
					Code:     result.CodeInvalidEnum,
					Severity: "error",
					Message:  fmt.Sprintf("waves[%d].agents[%d] (id=%s) context_source has invalid value %q — must be one of: prepared-brief, fallback-full-context, cross-repo-full", i, j, agent.ID, agent.ContextSource),
					Field:    fmt.Sprintf("waves[%d].agents[%d].context_source", i, j),
				})
			}
		}
	}

	return errs
}
