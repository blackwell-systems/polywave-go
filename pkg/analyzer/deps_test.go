package analyzer

import (
	"context"
	"testing"
)

func TestAnalyzeDeps_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling AnalyzeDeps

	_, err := AnalyzeDeps(ctx, "/some/repo", []string{"file.go"})
	if err == nil {
		t.Fatal("AnalyzeDeps() expected error for cancelled context, got nil")
	}
}
