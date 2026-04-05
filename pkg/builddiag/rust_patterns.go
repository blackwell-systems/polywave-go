package builddiag

// init registers language patterns at package init time. Tests that
// clear or replace catalog entries must not use t.Parallel().
func init() {
	RegisterPatterns("rust", []ErrorPattern{
		{
			Name:        "cannot_find_value",
			Regex:       `error\[E0425\]: cannot find value`,
			Fix:         "Add missing 'use' statement or check module visibility",
			Rationale:   "Symbol used without importing or not in scope",
			AutoFixable: false,
			Confidence:  0.90,
		},
		{
			Name:        "trait_bound_not_satisfied",
			Regex:       `error\[E0277\]: the trait bound .* is not satisfied`,
			Fix:         "Implement missing trait or add trait bound to generic",
			Rationale:   "Type does not implement required trait",
			AutoFixable: false,
			Confidence:  0.90,
		},
		{
			Name:        "unresolved_import",
			Regex:       `error\[E0432\]: unresolved import`,
			Fix:         "Check Cargo.toml dependencies and module path",
			Rationale:   "Import path is invalid or dependency missing",
			AutoFixable: false,
			Confidence:  0.90,
		},
		{
			Name:        "mismatched_types",
			Regex:       `error\[E0308\]: mismatched types`,
			Fix:         "Check type annotations and return types",
			Rationale:   "Expected type does not match actual type",
			AutoFixable: false,
			Confidence:  0.85,
		},
		{
			Name:        "macro_undefined",
			Regex:       `error: cannot find macro`,
			Fix:         "Add dependency providing macro to Cargo.toml",
			Rationale:   "Macro crate not in dependencies",
			AutoFixable: false,
			Confidence:  0.85,
		},
	})
}
