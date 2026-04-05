package builddiag

// init registers language patterns at package init time. Tests that
// clear or replace catalog entries must not use t.Parallel().
func init() {
	patterns := []ErrorPattern{
		{
			Name:        "module_not_found",
			Regex:       `ModuleNotFoundError: No module named`,
			Fix:         "pip install <module-name>",
			Rationale:   "Package not installed in Python environment",
			AutoFixable: true,
			Confidence:  0.95,
		},
		{
			Name:        "indentation_error",
			Regex:       `IndentationError: expected an indented block`,
			Fix:         "Fix indentation at reported line",
			Rationale:   "Python requires consistent indentation",
			AutoFixable: false,
			Confidence:  0.90,
		},
		{
			Name:        "name_not_defined",
			Regex:       `NameError: name .* is not defined`,
			Fix:         "Check imports and variable definitions",
			Rationale:   "Variable or function used before definition or import",
			AutoFixable: false,
			Confidence:  0.85,
		},
		{
			Name:        "import_error",
			Regex:       `ImportError: cannot import name`,
			Fix:         "Check import statement and module structure",
			Rationale:   "Imported name does not exist in module",
			AutoFixable: false,
			Confidence:  0.85,
		},
		{
			Name:        "syntax_error",
			Regex:       `SyntaxError: invalid syntax`,
			Fix:         "Fix syntax error at reported line (indentation, colons, etc.)",
			Rationale:   "Python syntax violation",
			AutoFixable: false,
			Confidence:  0.80,
		},
		{
			Name:        "type_error",
			Regex:       `TypeError:`,
			Fix:         "Check type annotations and function signatures",
			Rationale:   "Type mismatch in function call or operation",
			AutoFixable: false,
			Confidence:  0.75,
		},
	}

	RegisterPatterns("python", patterns)
	RegisterPatterns("py", patterns)
}
