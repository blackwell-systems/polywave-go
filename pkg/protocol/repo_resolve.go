package protocol

import (
	"fmt"
	"os"
	"path/filepath"
)

// RepoEntry maps a repository name to its absolute path on disk.
// Used by cmd/saw and web app to pass configured repository locations.
type RepoEntry struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	BuildCommand string `json:"build_command,omitempty"` // E21B: per-repo build gate (e.g. "go build ./...")
	TestCommand  string `json:"test_command,omitempty"`  // E21B: per-repo test gate (e.g. "go test ./...")
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

	result := make(map[string]string)

	if len(repoNames) > 0 {
		// Resolve each referenced repo name
		for name := range repoNames {
			if p, ok := configLookup[name]; ok {
				result[name] = p
				continue
			}
			// Try sibling directory resolution
			if fallbackRepoPath != "" {
				siblingPath := filepath.Join(filepath.Dir(fallbackRepoPath), name)
				if info, err := os.Stat(siblingPath); err == nil && info.IsDir() {
					result[name] = siblingPath
					continue
				}
			}
			return nil, fmt.Errorf("repo '%s' referenced in file_ownership but not found in configRepos and no sibling directory '%s' exists", name, name)
		}
		return result, nil
	}

	// No repo: fields in file_ownership; check manifest.Repository
	if manifest.Repository != "" {
		name := filepath.Base(manifest.Repository)
		result[name] = manifest.Repository
		return result, nil
	}

	// Fall back to fallbackRepoPath
	if fallbackRepoPath != "" {
		name := filepath.Base(fallbackRepoPath)
		result[name] = fallbackRepoPath
		return result, nil
	}

	return nil, fmt.Errorf("no repos can be resolved: no repo: fields in file_ownership, no repository in manifest, and no fallback path provided")
}

// ValidateRepoMatch checks that the resolved target repos match the repository
// where worktrees would be created. Returns nil if valid, an error with a
// diagnostic message if mismatched.
func ValidateRepoMatch(manifest *IMPLManifest, worktreeRepoPath string, configRepos []RepoEntry) error {
	repos, err := ResolveTargetRepos(manifest, worktreeRepoPath, configRepos)
	if err != nil {
		return err
	}

	worktreeBasename := filepath.Base(worktreeRepoPath)

	// Check if worktreeRepoPath matches one of the target repos
	for name, repoPath := range repos {
		if name == worktreeBasename || repoPath == worktreeRepoPath {
			return nil
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
		return nil
	}

	// Build a readable error with the first mismatched repo
	for name, repoPath := range repos {
		return fmt.Errorf("IMPL targets repo '%s' at %s but worktrees would be created in %s - aborting. Set repo: field in file_ownership or configure repos in saw.config.json", name, repoPath, worktreeRepoPath)
	}

	return nil
}
