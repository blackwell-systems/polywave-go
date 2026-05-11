package engine

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/config"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// InitOpts holds options for RunInit.
type InitOpts struct {
	RepoDir string       // absolute path to target directory (required)
	Force   bool         // overwrite existing polywave.config.json
	Logger  *slog.Logger // optional
}

// InitResult is the structured result for polywave-tools init.
// The cmd file uses this to print install check output and next-step messages.
type InitResult struct {
	ConfigPath    string        `json:"config_path"`
	Language      string        `json:"language"`
	ProjectName   string        `json:"project_name"`
	InstallResult InstallResult `json:"install_result"`
	AlreadyExists bool          `json:"already_exists,omitempty"`
}

// detectedProject holds language detection results for a repository.
type detectedProject struct {
	Name     string
	Language string
	Build    string
	Test     string
	Detected bool
}

// RunInit detects project language, generates polywave.config.json, runs install
// checks, and returns a structured InitResult. The cmd file handles --force
// validation and printing next-step messages. Calls RunVerifyInstall
// internally (both defined in pkg/engine/).
func RunInit(opts InitOpts) result.Result[InitResult] {
	// 1. Resolve absDir from opts.RepoDir via filepath.Abs
	absDir, err := filepath.Abs(opts.RepoDir)
	if err != nil {
		return result.NewFailure[InitResult]([]result.PolywaveError{
			result.NewFatal("N091_ENGINE_INIT_FAILED",
				"init: cannot resolve path \""+opts.RepoDir+"\": "+err.Error()).
				WithCause(err),
		})
	}

	// 2. Compute configPath
	configPath := filepath.Join(absDir, "polywave.config.json")

	// 3. Check if polywave.config.json exists; if so and !opts.Force: return partial with AlreadyExists
	if _, err := os.Stat(configPath); err == nil && !opts.Force {
		return result.NewPartial(InitResult{AlreadyExists: true}, []result.PolywaveError{
			result.NewError("N092_ENGINE_ALREADY_INITIALIZED",
				"polywave.config.json already exists at "+absDir+". Use --force to overwrite."),
		})
	}

	// 4. Detect project
	project := detectProjectInternal(absDir)

	// 5. Generate config
	cfg := generateConfigInternal(absDir, project)

	// 6. Save config; return error on failure
	saveResult := config.Save(absDir, cfg)
	if !saveResult.IsSuccess() {
		msg := "init: failed to write config"
		if len(saveResult.Errors) > 0 {
			msg += ": " + saveResult.Errors[0].Message
		}
		return result.NewFailure[InitResult]([]result.PolywaveError{
			result.NewFatal("N091_ENGINE_INIT_FAILED", msg),
		})
	}

	// 7. Add build/test top-level keys; return error on failure
	if err := addBuildTestKeysInternal(configPath, project); err != nil {
		return result.NewFailure[InitResult]([]result.PolywaveError{
			result.NewFatal("N091_ENGINE_INIT_FAILED",
				"init: failed to add build/test keys: "+err.Error()).
				WithCause(err),
		})
	}

	// 8. Run verify install
	installResult := RunVerifyInstall(VerifyInstallOpts{RepoPath: absDir})

	// 9. Return InitResult
	return result.NewSuccess(InitResult{
		ConfigPath:    configPath,
		Language:      project.Language,
		ProjectName:   project.Name,
		InstallResult: installResult,
	})
}

// detectProjectInternal scans repoDir for known marker files and returns detection results.
func detectProjectInternal(repoDir string) detectedProject {
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
			return detectedProject{
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
		return detectedProject{
			Name:     name,
			Language: "makefile",
			Build:    "make",
			Test:     "make test",
			Detected: true,
		}
	}

	return detectedProject{
		Name:     name,
		Language: "unknown",
		Build:    "",
		Test:     "",
		Detected: false,
	}
}

// generateConfigInternal creates a PolywaveConfig from a detectedProject.
func generateConfigInternal(repoDir string, project detectedProject) *config.PolywaveConfig {
	return &config.PolywaveConfig{
		Repos: []config.RepoEntry{
			{Name: project.Name, Path: repoDir},
		},
		Agent: config.AgentConfig{
			ScoutModel: "claude-sonnet-4-6",
			WaveModel:  "claude-sonnet-4-6",
		},
	}
}

// addBuildTestKeysInternal reads the config file, adds build/test top-level keys
// with "detected" markers, and writes back atomically.
func addBuildTestKeysInternal(configPath string, project detectedProject) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
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
		return err
	}
	output = append(output, '\n')

	// Atomic write.
	dir := filepath.Dir(configPath)
	tmpFile, err := os.CreateTemp(dir, ".saw-config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(output); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}
