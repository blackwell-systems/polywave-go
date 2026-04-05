package builddiag

// init registers language patterns at package init time. Tests that
// clear or replace catalog entries must not use t.Parallel().
func init() {
	patterns := []ErrorPattern{
		{
			Name:        "module_not_found",
			Regex:       `Cannot find module|Module not found`,
			Fix:         "npm install <module-name>",
			Rationale:   "Dependency not installed in node_modules",
			AutoFixable: true,
			Confidence:  0.95,
		},
		{
			Name:        "property_not_exist",
			Regex:       `Property .* does not exist on type`,
			Fix:         "Check interface contract and type definitions",
			Rationale:   "TypeScript type mismatch or missing property",
			AutoFixable: false,
			Confidence:  0.90,
		},
		{
			Name:        "type_any_implicit",
			Regex:       `Parameter .* implicitly has an 'any' type`,
			Fix:         "Add explicit type annotation to parameter",
			Rationale:   "TypeScript strict mode requires explicit types",
			AutoFixable: false,
			Confidence:  0.85,
		},
		{
			Name:        "import_path_invalid",
			Regex:       `Cannot resolve module|File .* is not a module`,
			Fix:         "Check import path and file extension (.js, .ts, .tsx)",
			Rationale:   "Import path does not match file structure",
			AutoFixable: false,
			Confidence:  0.85,
		},
		{
			Name:        "syntax_error",
			Regex:       `SyntaxError: Unexpected token`,
			Fix:         "Check syntax at reported line (missing semicolon, brace, etc.)",
			Rationale:   "JavaScript syntax violation",
			AutoFixable: false,
			Confidence:  0.80,
		},
	}

	RegisterPatterns("javascript", patterns)
	RegisterPatterns("typescript", patterns)
	RegisterPatterns("js", patterns)
	RegisterPatterns("ts", patterns)
}
