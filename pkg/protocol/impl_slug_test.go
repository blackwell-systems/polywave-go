package protocol

import (
	"testing"
)

func TestExtractIMPLSlug_FeatureSlug(t *testing.T) {
	manifest := &IMPLManifest{FeatureSlug: "my-feature"}
	got := ExtractIMPLSlug("/some/path/IMPL-other.yaml", manifest)
	if got != "my-feature" {
		t.Errorf("expected %q, got %q", "my-feature", got)
	}
}

func TestExtractIMPLSlug_PathFallback(t *testing.T) {
	manifest := &IMPLManifest{FeatureSlug: ""}
	got := ExtractIMPLSlug("/some/path/IMPL-foo.yaml", manifest)
	if got != "foo" {
		t.Errorf("expected %q, got %q", "foo", got)
	}
}

func TestExtractIMPLSlug_NilManifest(t *testing.T) {
	got := ExtractIMPLSlug("/some/path/IMPL-bar.yaml", nil)
	if got != "bar" {
		t.Errorf("expected %q, got %q", "bar", got)
	}
}

func TestExtractIMPLSlug_EmptyBase(t *testing.T) {
	// If stripping prefix/suffix yields empty string, fall back to implPath
	got := ExtractIMPLSlug("IMPL-.yaml", nil)
	// base = TrimSuffix(TrimPrefix("IMPL-.yaml", "IMPL-"), ".yaml") = ""
	// fallback to implPath
	if got != "IMPL-.yaml" {
		t.Errorf("expected %q, got %q", "IMPL-.yaml", got)
	}
}
