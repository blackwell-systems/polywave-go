package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/config"
	"github.com/spf13/cobra"
)

// DetectedProject holds language detection results for a repository.
type DetectedProject struct {
	Name     string
	Language string
	Build    string
	Test     string
	Detected bool
}

// newInitCmd returns the cobra.Command for "sawtools init".
func newInitCmd() *cobra.Command {
	var repoFlag string
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a project for SAW (zero-config)",
		Long:  "Detects project language, generates saw.config.json, and verifies prerequisites.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Resolve the target directory.
			repoDir := repoFlag
			if repoDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("init: cannot determine working directory: %w", err)
				}
				repoDir = cwd
			}

			// Make absolute.
			absDir, err := filepath.Abs(repoDir)
			if err != nil {
				return fmt.Errorf("init: cannot resolve path %q: %w", repoDir, err)
			}
			repoDir = absDir

			// 2. Check if saw.config.json already exists.
			configPath := filepath.Join(repoDir, "saw.config.json")
			if _, err := os.Stat(configPath); err == nil && !forceFlag {
				return fmt.Errorf("saw.config.json already exists at %s. Use --force to overwrite.", repoDir)
			}

			// 3. Detect project.
			project := detectProject(repoDir)

			// 4. Generate config.
			cfg := generateConfig(repoDir, project)

			// 5. Write saw.config.json via config.Save, then add build/test keys.
			saveResult := config.Save(repoDir, cfg)
			if !saveResult.IsSuccess() {
				return fmt.Errorf("init: failed to write config: %v", saveResult)
			}

			// Add build/test top-level keys with "detected" markers.
			if err := addBuildTestKeys(configPath, project); err != nil {
				return fmt.Errorf("init: failed to add build/test keys: %w", err)
			}

			// 6. Run install checks.
			result := runInstallChecks()
			printHumanOutput(cmd, result)

			if result.Verdict == "FAIL" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nsawtools not found. Install it:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  go install github.com/blackwell-systems/scout-and-wave-go/cmd/sawtools@latest\n\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Or build from source:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  git clone https://github.com/blackwell-systems/scout-and-wave-go.git\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  cd scout-and-wave-go && go build -o ~/.local/bin/sawtools ./cmd/sawtools\n")
			}

			// 7. Print next steps.
			fmt.Fprintf(cmd.OutOrStdout(), "\nSAW initialized for %s (%s).\n", project.Name, project.Language)
			fmt.Fprintf(cmd.OutOrStdout(), "\nQuick start:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  saw plan \"describe your feature\"     Create an implementation plan\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  saw serve                            Open the web dashboard\n")
			fmt.Fprintf(cmd.OutOrStdout(), "\nLearn more:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  saw help                             All commands\n")

			return nil
		},
	}

	cmd.Flags().StringVar(&repoFlag, "repo", "", "Directory to initialize (default: current working directory)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Overwrite existing saw.config.json")

	return cmd
}

// detectProject scans repoDir for known marker files and returns detection results.
func detectProject(repoDir string) DetectedProject {
	name := filepath.Base(repoDir)

	// Priority-ordered marker file checks.
	markers := []struct {
		file     string
		language string
		build    string
		test     string
	}{
		{"go.mod", "go", "go build ./...", "go test ./..."},
		{"Cargo.toml", "rust", "cargo build", "cargo test"},
		{"package.json", "node", "npm run build", "npm test"},
		{"pyproject.toml", "python", "", "pytest"},
		{"requirements.txt", "python", "", "pytest"},
		{"Gemfile", "ruby", "", "bundle exec rspec"},
	}

	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(repoDir, m.file)); err == nil {
			return DetectedProject{
				Name:     name,
				Language: m.language,
				Build:    m.build,
				Test:     m.test,
				Detected: m.build != "" || m.test != "",
			}
		}
	}

	// Makefile fallback (only when no language marker found).
	if _, err := os.Stat(filepath.Join(repoDir, "Makefile")); err == nil {
		return DetectedProject{
			Name:     name,
			Language: "makefile",
			Build:    "make",
			Test:     "make test",
			Detected: true,
		}
	}

	return DetectedProject{
		Name:     name,
		Language: "unknown",
		Build:    "",
		Test:     "",
		Detected: false,
	}
}

// generateConfig creates a SAWConfig from a DetectedProject.
func generateConfig(repoDir string, project DetectedProject) *config.SAWConfig {
	return &config.SAWConfig{
		Repos: []config.RepoEntry{
			{Name: project.Name, Path: repoDir},
		},
		Agent: config.AgentConfig{
			ScoutModel: "claude-sonnet-4-6",
			WaveModel:  "claude-sonnet-4-6",
		},
	}
}

// addBuildTestKeys reads the config file, adds build/test top-level keys
// with "detected" markers, and writes back atomically.
func addBuildTestKeys(configPath string, project DetectedProject) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}

	type buildTestEntry struct {
		Command  string `json:"command"`
		Detected bool   `json:"detected"`
	}

	if project.Build != "" {
		b, _ := json.Marshal(buildTestEntry{Command: project.Build, Detected: true})
		raw["build"] = json.RawMessage(b)
	}

	if project.Test != "" {
		t, _ := json.Marshal(buildTestEntry{Command: project.Test, Detected: true})
		raw["test"] = json.RawMessage(t)
	}

	output, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	output = append(output, '\n')

	// Atomic write.
	dir := filepath.Dir(configPath)
	tmpFile, err := os.CreateTemp(dir, ".saw-config-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(output); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename temp: %w", err)
	}

	return nil
}
