package builddiag

import (
	"testing"
)

func TestPythonPatterns_ModuleNotFound(t *testing.T) {
	errorLog := `Traceback (most recent call last):
  File "app.py", line 2, in <module>
    import requests
ModuleNotFoundError: No module named 'requests'`

	diag := DiagnoseError(errorLog, "python")

	if diag.Pattern != "module_not_found" {
		t.Errorf("Expected pattern 'module_not_found', got '%s'", diag.Pattern)
	}
	if diag.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", diag.Confidence)
	}
	if !diag.AutoFixable {
		t.Error("Expected AutoFixable to be true")
	}
	if diag.Fix != "pip install <module-name>" {
		t.Errorf("Expected pip install fix, got '%s'", diag.Fix)
	}
}

func TestPythonPatterns_NameNotDefined(t *testing.T) {
	errorLog := `Traceback (most recent call last):
  File "script.py", line 10, in <module>
    print(result)
NameError: name 'result' is not defined`

	diag := DiagnoseError(errorLog, "python")

	if diag.Pattern != "name_not_defined" {
		t.Errorf("Expected pattern 'name_not_defined', got '%s'", diag.Pattern)
	}
	if diag.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", diag.Confidence)
	}
	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
}

func TestPythonPatterns_SyntaxError(t *testing.T) {
	errorLog := `  File "test.py", line 5
    if x == 5
             ^
SyntaxError: invalid syntax`

	diag := DiagnoseError(errorLog, "python")

	if diag.Pattern != "syntax_error" {
		t.Errorf("Expected pattern 'syntax_error', got '%s'", diag.Pattern)
	}
	if diag.Confidence != 0.80 {
		t.Errorf("Expected confidence 0.80, got %f", diag.Confidence)
	}
	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
}

func TestPythonPatterns_ImportError(t *testing.T) {
	errorLog := `Traceback (most recent call last):
  File "main.py", line 1, in <module>
    from mymodule import nonexistent_function
ImportError: cannot import name 'nonexistent_function' from 'mymodule'`

	diag := DiagnoseError(errorLog, "python")

	if diag.Pattern != "import_error" {
		t.Errorf("Expected pattern 'import_error', got '%s'", diag.Pattern)
	}
	if diag.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", diag.Confidence)
	}
	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
}

func TestPythonPatterns_IndentationError(t *testing.T) {
	errorLog := `  File "code.py", line 8
    print("hello")
        ^
IndentationError: expected an indented block`

	diag := DiagnoseError(errorLog, "python")

	if diag.Pattern != "indentation_error" {
		t.Errorf("Expected pattern 'indentation_error', got '%s'", diag.Pattern)
	}
	if diag.Confidence != 0.90 {
		t.Errorf("Expected confidence 0.90, got %f", diag.Confidence)
	}
	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
}

func TestPythonPatterns_TypeError(t *testing.T) {
	errorLog := `Traceback (most recent call last):
  File "app.py", line 15, in calculate
    return x + y
TypeError: unsupported operand type(s) for +: 'int' and 'str'`

	diag := DiagnoseError(errorLog, "python")

	if diag.Pattern != "type_error" {
		t.Errorf("Expected pattern 'type_error', got '%s'", diag.Pattern)
	}
	if diag.Confidence != 0.75 {
		t.Errorf("Expected confidence 0.75, got %f", diag.Confidence)
	}
	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
}

func TestPythonPatterns_ConfidenceLevels(t *testing.T) {
	patterns := catalogs["python"]
	if len(patterns) == 0 {
		t.Fatal("no Python patterns registered")
	}

	for i := 1; i < len(patterns); i++ {
		if patterns[i].Confidence > patterns[i-1].Confidence {
			t.Errorf("python patterns not sorted by confidence: index %d (%f) > index %d (%f)",
				i, patterns[i].Confidence, i-1, patterns[i-1].Confidence)
		}
	}
}

func TestPythonPatterns_MultipleAliases(t *testing.T) {
	errorLog := `ModuleNotFoundError: No module named 'numpy'`

	// Test "python" alias
	diag1 := DiagnoseError(errorLog, "python")
	if diag1.Pattern != "module_not_found" {
		t.Errorf("Expected 'python' alias to work, got pattern '%s'", diag1.Pattern)
	}

	// Test "py" alias
	diag2 := DiagnoseError(errorLog, "py")
	if diag2.Pattern != "module_not_found" {
		t.Errorf("Expected 'py' alias to work, got pattern '%s'", diag2.Pattern)
	}

	// Verify both aliases return identical results
	if diag1.Pattern != diag2.Pattern {
		t.Error("Expected 'python' and 'py' aliases to return same pattern")
	}
	if diag1.Confidence != diag2.Confidence {
		t.Error("Expected 'python' and 'py' aliases to return same confidence")
	}
}
