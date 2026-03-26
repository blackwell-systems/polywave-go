package retry_test

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/retry"
)

// ---------------------------------------------------------------------------
// ClassifyError tests
// ---------------------------------------------------------------------------

func TestClassifyError_ImportError(t *testing.T) {
	cases := []string{
		"could not import github.com/foo/bar",
		"cannot find package \"foo\" in any of",
		"no required module provides github.com/foo",
	}
	for _, c := range cases {
		got := retry.ClassifyError(c)
		if got != retry.ErrorClassImport {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retry.ErrorClassImport)
		}
	}
}

func TestClassifyError_TypeError(t *testing.T) {
	cases := []string{
		"cannot use x as type string",
		"undefined: MyFunc",
		"has no field or method Foo",
		"not enough arguments in call to bar",
		"too many arguments in call to baz",
	}
	for _, c := range cases {
		got := retry.ClassifyError(c)
		if got != retry.ErrorClassType {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retry.ErrorClassType)
		}
	}
}

func TestClassifyError_TestFailure(t *testing.T) {
	cases := []string{
		"--- FAIL: TestMyFunc (0.01s)",
		"FAIL\tgithub.com/foo/bar",
		"panic: test timed out after 30s",
	}
	for _, c := range cases {
		got := retry.ClassifyError(c)
		if got != retry.ErrorClassTest {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retry.ErrorClassTest)
		}
	}
}

func TestClassifyError_BuildError(t *testing.T) {
	cases := []string{
		"cannot find module providing package foo",
		"build constraints exclude all Go files",
		"syntax error: unexpected token",
	}
	for _, c := range cases {
		got := retry.ClassifyError(c)
		if got != retry.ErrorClassBuild {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retry.ErrorClassBuild)
		}
	}
}

func TestClassifyError_LintError(t *testing.T) {
	cases := []string{
		"go vet: suspicious usage",
		"should have comment on exported function Foo",
		"exported type Bar should have comment",
	}
	for _, c := range cases {
		got := retry.ClassifyError(c)
		if got != retry.ErrorClassLint {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retry.ErrorClassLint)
		}
	}
}

func TestClassifyError_Unknown(t *testing.T) {
	cases := []string{
		"",
		"everything is fine",
		"no recognisable pattern here",
	}
	for _, c := range cases {
		got := retry.ClassifyError(c)
		if got != retry.ErrorClassUnknown {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retry.ErrorClassUnknown)
		}
	}
}

// ---------------------------------------------------------------------------
// SuggestFixes tests
// ---------------------------------------------------------------------------

func TestSuggestFixes_AllClasses(t *testing.T) {
	classes := []retry.ErrorClass{
		retry.ErrorClassImport,
		retry.ErrorClassType,
		retry.ErrorClassTest,
		retry.ErrorClassBuild,
		retry.ErrorClassLint,
	}
	for _, class := range classes {
		fixes := retry.SuggestFixes(class)
		if len(fixes) == 0 {
			t.Errorf("SuggestFixes(%v) returned no suggestions", class)
		}
		for _, fix := range fixes {
			if fix == "" {
				t.Errorf("SuggestFixes(%v) returned an empty suggestion string", class)
			}
		}
	}

	// Unknown class should return an empty (but non-nil) slice.
	unknownFixes := retry.SuggestFixes(retry.ErrorClassUnknown)
	if unknownFixes == nil {
		t.Error("SuggestFixes(ErrorClassUnknown) returned nil; want empty slice")
	}
}
