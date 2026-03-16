package builddiag

func init() {
	RegisterPatterns("go", []ErrorPattern{
		{
			Name:        "missing_package",
			Regex:       `cannot find package|package .* is not in GOROOT`,
			Fix:         "go mod tidy && go build ./...",
			Rationale:   "go.sum is stale or missing dependency",
			AutoFixable: true,
			Confidence:  0.95,
		},
		{
			Name:        "missing_go_sum_entry",
			Regex:       `missing go.sum entry`,
			Fix:         "go mod tidy",
			Rationale:   "go.sum does not have checksums for all dependencies",
			AutoFixable: true,
			Confidence:  0.95,
		},
		{
			Name:        "import_cycle",
			Regex:       `import cycle not allowed`,
			Fix:         "Refactor to break import cycle (extract interface or move code)",
			Rationale:   "Circular dependency between packages",
			AutoFixable: false,
			Confidence:  0.95,
		},
		{
			Name:        "type_mismatch",
			Regex:       `cannot use .* \(type .*\) as type`,
			Fix:         "Check interface contract and adjust type implementation",
			Rationale:   "Type does not satisfy interface or wrong type passed",
			AutoFixable: false,
			Confidence:  0.90,
		},
		{
			Name:        "undefined_identifier",
			Regex:       `undefined: \w+`,
			Fix:         "Check imports and add missing import statement",
			Rationale:   "Symbol used without importing its package",
			AutoFixable: false,
			Confidence:  0.85,
		},
		{
			Name:        "syntax_error",
			Regex:       `syntax error:`,
			Fix:         "Fix syntax error at reported line",
			Rationale:   "Go syntax violation",
			AutoFixable: false,
			Confidence:  0.85,
		},
	})
}
