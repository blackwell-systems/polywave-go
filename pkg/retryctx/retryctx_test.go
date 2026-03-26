package retryctx_test

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/retryctx"
)

// TestRetryctxShim_Compilation verifies the shim package compiles and re-exports
// the expected symbols from pkg/retry.
func TestRetryctxShim_Compilation(t *testing.T) {
	// ErrorClass constants must be accessible
	_ = retryctx.ErrorClassImport
	_ = retryctx.ErrorClassType
	_ = retryctx.ErrorClassTest
	_ = retryctx.ErrorClassBuild
	_ = retryctx.ErrorClassLint
	_ = retryctx.ErrorClassUnknown

	// ClassifyError must be callable and return expected type
	class := retryctx.ClassifyError("undefined: Foo")
	if class != retryctx.ErrorClassType {
		t.Errorf("ClassifyError returned %v, want ErrorClassType", class)
	}

	// SuggestFixes must return non-empty for known classes
	fixes := retryctx.SuggestFixes(retryctx.ErrorClassType)
	if len(fixes) == 0 {
		t.Error("SuggestFixes returned empty for ErrorClassType")
	}
}
