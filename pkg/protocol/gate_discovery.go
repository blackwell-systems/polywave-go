package protocol

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverLintGate searches for an active IMPL doc under repoDir/docs/IMPL/
// and extracts the first gate of type "lint". If no IMPL doc is found or
// none defines a lint gate, it falls back to saw.config.json's lint_command
// field. Returns ("", nil) if nothing is configured (silent pass).
func DiscoverLintGate(repoDir string) (string, error) {
	// Step 1: Check active IMPL docs for a lint gate.
	implDir := IMPLDir(repoDir)
	entries, err := os.ReadDir(implDir)
	if err != nil {
		// If the directory doesn't exist, that's not an error — just no IMPL docs.
		if os.IsNotExist(err) {
			return discoverFromConfig(repoDir)
		}
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "IMPL-") || !strings.HasSuffix(name, ".yaml") {
			continue
		}

		manifest, err := Load(context.TODO(), filepath.Join(implDir, name))
		if err != nil {
			// Malformed YAML is non-fatal: skip and continue.
			continue
		}

		// Only consider active manifests (not COMPLETE or NOT_SUITABLE).
		if manifest.State == StateComplete || manifest.State == StateNotSuitable {
			continue
		}

		// Skip IMPLs that don't target this repo (cross-repo validation).
		if !manifest.TargetsRepo(repoDir) {
			continue
		}

		if manifest.QualityGates == nil {
			continue
		}

		repoName := filepath.Base(repoDir)
		for _, gate := range manifest.QualityGates.Gates {
			if gate.Type == "lint" {
				// Skip gates scoped to a different repo.
				if gate.Repo != "" && gate.Repo != repoName {
					continue
				}
				return gate.Command, nil
			}
		}
	}

	// Step 2: Fall back to saw.config.json.
	return discoverFromConfig(repoDir)
}

// sawConfig represents the minimal structure of saw.config.json relevant
// to gate discovery.
type sawConfig struct {
	LintCommand  string `json:"lint_command"`
	BuildCommand string `json:"build_command"`
}

// discoverFromConfig reads repoDir/saw.config.json and returns the
// lint_command field if present and non-empty.
func discoverFromConfig(repoDir string) (string, error) {
	configPath := filepath.Join(repoDir, "saw.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var cfg sawConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Malformed config is non-fatal.
		return "", nil
	}

	return cfg.LintCommand, nil
}

// discoverBuildFromConfig reads repoDir/saw.config.json and returns the
// build_command field if present and non-empty.
func discoverBuildFromConfig(repoDir string) (string, error) {
	configPath := filepath.Join(repoDir, "saw.config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var cfg sawConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		// Malformed config is non-fatal.
		return "", nil
	}

	return cfg.BuildCommand, nil
}

// DiscoverBuildGate searches for an active IMPL doc under repoDir/docs/IMPL/
// and extracts the first gate of type "build". Falls back to saw.config.json
// build_command field. Returns ("", nil) if nothing is configured.
func DiscoverBuildGate(repoDir string) (string, error) {
	// Step 1: Check active IMPL docs for a build gate.
	implDir := IMPLDir(repoDir)
	entries, err := os.ReadDir(implDir)
	if err != nil {
		// If the directory doesn't exist, that's not an error — just no IMPL docs.
		if os.IsNotExist(err) {
			return discoverBuildFromConfig(repoDir)
		}
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "IMPL-") || !strings.HasSuffix(name, ".yaml") {
			continue
		}

		manifest, err := Load(context.TODO(), filepath.Join(implDir, name))
		if err != nil {
			// Malformed YAML is non-fatal: skip and continue.
			continue
		}

		// Only consider active manifests (not COMPLETE or NOT_SUITABLE).
		if manifest.State == StateComplete || manifest.State == StateNotSuitable {
			continue
		}

		// Skip IMPLs that don't target this repo (cross-repo validation).
		if !manifest.TargetsRepo(repoDir) {
			continue
		}

		if manifest.QualityGates == nil {
			continue
		}

		repoName := filepath.Base(repoDir)
		for _, gate := range manifest.QualityGates.Gates {
			if gate.Type == "build" {
				// Skip gates scoped to a different repo.
				if gate.Repo != "" && gate.Repo != repoName {
					continue
				}
				return gate.Command, nil
			}
		}
	}

	// Step 2: Fall back to saw.config.json.
	return discoverBuildFromConfig(repoDir)
}
