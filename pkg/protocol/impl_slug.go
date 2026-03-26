package protocol

import (
	"path/filepath"
	"strings"
)

// ExtractIMPLSlug returns the canonical feature slug for an IMPL document.
// Priority order:
//  1. manifest.FeatureSlug if non-empty
//  2. filepath.Base(implPath) with "IMPL-" prefix and ".yaml" suffix stripped (if non-empty result)
//  3. implPath as final fallback
//
// Agent A will provide the full implementation.
func ExtractIMPLSlug(implPath string, manifest *IMPLManifest) string {
	if manifest != nil && manifest.FeatureSlug != "" {
		return manifest.FeatureSlug
	}
	base := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(implPath), "IMPL-"), ".yaml")
	if base != "" {
		return base
	}
	return implPath
}
