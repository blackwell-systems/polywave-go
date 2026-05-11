package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// resolveSiblingCaseInsensitive looks for a directory in parentDir whose name
// matches name case-insensitively. Returns the canonical path (preserving the
// actual case on disk) and true if found. This handles the common case where
// an IMPL doc uses "lsp-mcp-go" but the directory is "LSP-MCP-GO".
func resolveSiblingCaseInsensitive(parentDir, name string) (string, bool) {
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(parentDir, e.Name()), true
		}
	}
	return "", false
}

// configLookupCI performs case-insensitive lookup in a config repo map.
// Returns the path and true if found.
func configLookupCI(lookup map[string]string, name string) (string, bool) {
	if p, ok := lookup[name]; ok {
		return p, true
	}
	for k, v := range lookup {
		if strings.EqualFold(k, name) {
			return v, true
		}
	}
	return "", false
}

// RepoEntry maps a repository name to its absolute path on disk.
// Used by cmd/polywave and web app to pass configured repository locations.
type RepoEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	BuildCommand string `json:"build_command,omitempty"` // E21B: per-repo build gate (e.g. "go build ./...")
	TestCommand  string `json:"test_command,omitempty"`  // E21B: per-repo test gate (e.g. "go test ./...")
}

// ValidateRepoData holds the data returned by a successful ValidateRepoMatch call.
type ValidateRepoData struct {
	MatchedRepo string
	Valid       bool
	Mismatches  []string
}

// ResolveTargetRepos determines which repositories an IMPL manifest targets.
// It scans file_ownership for distinct Repo values, falls back to the manifest's
// Repository field, and ultimately to fallbackRepoPath. Returns a map of
// repo name -> absolute path.
func ResolveTargetRepos(manifest *IMPLManifest, fallbackRepoPath string, configRepos []RepoEntry) (map[string]string, error) {
	// Build a lookup from configRepos
	configLookup := make(map[string]string, len(configRepos))
	for _, r := range configRepos {
		configLookup[r.Name] = r.Path
	}

	// Collect distinct repo names from file_ownership
	repoNames := make(map[string]struct{})
	for _, fo := range manifest.FileOwnership {
		if fo.Repo != "" {
			repoNames[fo.Repo] = struct{}{}
		}
	}

	resolved := make(map[string]string)

	if len(repoNames) > 0 {
		// Resolve each referenced repo name
		for name := range repoNames {
			if p, ok := configLookupCI(configLookup, name); ok {
				resolved[name] = p
				continue
			}
			// Try sibling directory resolution (case-insensitive)
			if fallbackRepoPath != "" {
				if siblingPath, found := resolveSiblingCaseInsensitive(filepath.Dir(fallbackRepoPath), name); found {
					resolved[name] = siblingPath
					continue
				}
			}
			return nil, fmt.Errorf("repo '%s' referenced in file_ownership but not found in configRepos and no sibling directory '%s' exists", name, name)
		}
		return resolved, nil
	}

	// No repo: fields in file_ownership; check manifest.Repository
	if manifest.Repository != "" {
		name := filepath.Base(manifest.Repository)
		resolved[name] = manifest.Repository
		return resolved, nil
	}

	// Fall back to fallbackRepoPath
	if fallbackRepoPath != "" {
		name := filepath.Base(fallbackRepoPath)
		resolved[name] = fallbackRepoPath
		return resolved, nil
	}

	return nil, fmt.Errorf("no repos can be resolved: no repo: fields in file_ownership, no repository in manifest, and no fallback path provided")
}

// ValidateRepoMatch checks that the resolved target repos match the repository
// where worktrees would be created. Returns a Result with ValidateRepoData on
// success, Partial if mismatches are detected, or Fatal on critical repo mismatch.
func ValidateRepoMatch(manifest *IMPLManifest, worktreeRepoPath string, configRepos []RepoEntry) result.Result[ValidateRepoData] {
	repos, err := ResolveTargetRepos(manifest, worktreeRepoPath, configRepos)
	if err != nil {
		return result.NewFailure[ValidateRepoData]([]result.PolywaveError{
			result.NewFatal("REPO_MISMATCH", err.Error()),
		})
	}

	worktreeBasename := filepath.Base(worktreeRepoPath)

	// Check if worktreeRepoPath matches one of the target repos
	for name, repoPath := range repos {
		if name == worktreeBasename || repoPath == worktreeRepoPath {
			return result.NewSuccess(ValidateRepoData{
				MatchedRepo: name,
				Valid:       true,
				Mismatches:  nil,
			})
		}
	}

	// Single-repo case: no repo: fields and all files could be in worktreeRepoPath
	hasRepoField := false
	for _, fo := range manifest.FileOwnership {
		if fo.Repo != "" {
			hasRepoField = true
			break
		}
	}

	if !hasRepoField {
		// No repo fields at all - this is a single-repo IMPL,
		// trust that files belong to the current repo
		return result.NewSuccess(ValidateRepoData{
			MatchedRepo: worktreeBasename,
			Valid:       true,
			Mismatches:  nil,
		})
	}

	// Collect mismatches for reporting
	var mismatches []string
	for name, repoPath := range repos {
		mismatches = append(mismatches,
			fmt.Sprintf("IMPL targets repo '%s' at %s but worktrees would be created in %s", name, repoPath, worktreeRepoPath))
	}

	if len(mismatches) > 0 {
		// Fatal: critical repo mismatch — worktrees would be created in wrong repo
		data := ValidateRepoData{
			Valid:      false,
			Mismatches: mismatches,
		}
		var sawErrs []result.PolywaveError
		for _, m := range mismatches {
			sawErrs = append(sawErrs, result.NewFatal("REPO_MISMATCH",
				m+" - aborting. Set repo: field in file_ownership or configure repos in polywave.config.json"))
		}
		return result.NewPartial(data, sawErrs)
	}

	return result.NewSuccess(ValidateRepoData{
		MatchedRepo: worktreeBasename,
		Valid:       true,
		Mismatches:  nil,
	})
}
