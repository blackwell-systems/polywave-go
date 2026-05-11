// Package config provides unified configuration loading and saving for
// Polywave. It replaces the scattered config handling previously split
// across backend.PolywaveProviders, autonomy.Config, and the web app's
// service.PolywaveConfig with a single PolywaveConfig type and Load/Save API.
package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/autonomy"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// PolywaveConfig is the unified configuration type for all Polywave operations.
// It is the superset of fields from backend.PolywaveProviders, autonomy.Config,
// and the web application's service.PolywaveConfig.
type PolywaveConfig struct {
	Providers     ProvidersConfig `json:"providers,omitempty"`
	Autonomy      AutonomyConfig  `json:"autonomy,omitempty"`
	Repos         []RepoEntry     `json:"repos,omitempty"`
	Agent         AgentConfig     `json:"agent,omitempty"`
	Quality       QualityConfig   `json:"quality,omitempty"`
	Appear        AppearConfig    `json:"appearance,omitempty"`
	Notifications any             `json:"notifications,omitempty"`
	CodeReview    any             `json:"code_review,omitempty"`
}

// ProvidersConfig holds API credentials for supported LLM providers.
type ProvidersConfig struct {
	Anthropic AnthropicProvider `json:"anthropic,omitempty"`
	Bedrock   BedrockProvider   `json:"bedrock,omitempty"`
	OpenAI    OpenAIProvider    `json:"openai,omitempty"`
}

// AnthropicProvider holds Anthropic API configuration.
type AnthropicProvider struct {
	APIKey string `json:"api_key,omitempty"`
}

// BedrockProvider holds AWS Bedrock configuration.
type BedrockProvider struct {
	Region         string `json:"region,omitempty"`
	AccessKeyID    string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	SessionToken   string `json:"session_token,omitempty"`
	Profile        string `json:"profile,omitempty"`
}

// OpenAIProvider holds OpenAI API configuration.
type OpenAIProvider struct {
	APIKey string `json:"api_key,omitempty"`
}

// AutonomyConfig is a type alias for autonomy.Config.
// Using an alias (not embedding) means config.AutonomyConfig and autonomy.Config
// are the same type — no conversion needed at call sites.
type AutonomyConfig = autonomy.Config

// RepoEntry identifies a repository by name and filesystem path.
type RepoEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// AgentConfig specifies which model to use for each agent role.
type AgentConfig struct {
	ScoutModel       string `json:"scout_model,omitempty"`
	WaveModel        string `json:"wave_model,omitempty"`
	ChatModel        string `json:"chat_model,omitempty"`
	ScaffoldModel    string `json:"scaffold_model,omitempty"`
	IntegrationModel string `json:"integration_model,omitempty"`
	PlannerModel     string `json:"planner_model,omitempty"`
	ReviewModel      string `json:"review_model,omitempty"`
}

// QualityConfig controls quality gate enforcement.
type QualityConfig struct {
	RequireTests   bool          `json:"require_tests,omitempty"`
	RequireLint    bool          `json:"require_lint,omitempty"`
	BlockOnFailure bool          `json:"block_on_failure,omitempty"`
	CodeReview     CodeReviewCfg `json:"code_review,omitempty"`
}

// CodeReviewCfg controls automated code review settings.
type CodeReviewCfg struct {
	Enabled   bool   `json:"enabled,omitempty"`
	Blocking  bool   `json:"blocking,omitempty"`
	Model     string `json:"model,omitempty"`
	Threshold int    `json:"threshold,omitempty"`
}

// AppearConfig controls UI appearance settings.
type AppearConfig struct {
	Theme               string   `json:"theme,omitempty"`
	Contrast            string   `json:"contrast,omitempty"`
	ColorTheme          string   `json:"color_theme,omitempty"`
	ColorThemeDark      string   `json:"color_theme_dark,omitempty"`
	ColorThemeLight     string   `json:"color_theme_light,omitempty"`
	FavoriteThemesDark  []string `json:"favorite_themes_dark,omitempty"`
	FavoriteThemesLight []string `json:"favorite_themes_light,omitempty"`
}

const configFileName = "polywave.config.json"

// maxWalkDepth is the maximum number of parent directories to traverse.
const maxWalkDepth = 10

// FindConfigPath walks up from startDir looking for polywave.config.json.
// Returns the absolute path if found, empty string if not.
func FindConfigPath(startDir string) string {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}
	for i := 0; i < maxWalkDepth; i++ {
		candidate := filepath.Join(dir, configFileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	return ""
}

// Load finds polywave.config.json starting from startDir, walking up to 10
// parent directories. Parses the full config into PolywaveConfig.
// Returns N013_CONFIG_NOT_FOUND if no config file exists.
// Returns N014_CONFIG_INVALID if JSON parsing fails.
func Load(startDir string) result.Result[*PolywaveConfig] {
	path := FindConfigPath(startDir)
	if path == "" {
		return result.NewFailure[*PolywaveConfig]([]result.PolywaveError{
			result.NewError(result.CodeConfigNotFound, "no polywave.config.json found walking up from "+startDir),
		})
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return result.NewFailure[*PolywaveConfig]([]result.PolywaveError{
			result.NewError(result.CodeConfigInvalid, "failed to read config: "+err.Error()),
		})
	}

	var cfg PolywaveConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return result.NewFailure[*PolywaveConfig]([]result.PolywaveError{
			result.NewError(result.CodeConfigInvalid, "invalid JSON in "+path+": "+err.Error()),
		})
	}

	// Backward compatibility: migrate legacy repo.path to repos array.
	if len(cfg.Repos) == 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			return result.NewFailure[*PolywaveConfig]([]result.PolywaveError{
				result.NewError(result.CodeConfigInvalid, "failed to re-parse config for migration: "+err.Error()),
			})
		}
		if repoRaw, ok := raw["repo"]; ok {
			var legacy struct {
				Path string `json:"path"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(repoRaw, &legacy); err != nil || legacy.Path == "" {
				if err != nil {
					slog.Warn("legacy repo migration: malformed repo entry", "path", path, "err", err)
				}
			} else {
				name := legacy.Name
				if name == "" {
					name = filepath.Base(legacy.Path)
				}
				cfg.Repos = []RepoEntry{{Name: name, Path: legacy.Path}}
			}
		}
	}

	return result.NewSuccess(&cfg)
}

// Save atomically writes cfg to polywave.config.json at repoPath.
// Uses temp-file + rename pattern. Preserves unknown top-level keys
// that may exist in the current config file.
func Save(repoPath string, cfg *PolywaveConfig) result.Result[bool] {
	configPath := filepath.Join(repoPath, configFileName)

	// Read existing file to preserve unknown keys.
	existing := make(map[string]json.RawMessage)
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &existing) // ignore errors; we'll overwrite
	}

	// Marshal the cfg struct to get known keys.
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return result.NewFailure[bool]([]result.PolywaveError{
			result.NewError(result.CodeConfigInvalid, "failed to marshal config: "+err.Error()),
		})
	}
	var cfgMap map[string]json.RawMessage
	if err := json.Unmarshal(cfgBytes, &cfgMap); err != nil {
		return result.NewFailure[bool]([]result.PolywaveError{
			result.NewError(result.CodeConfigInvalid, "failed to re-parse config: "+err.Error()),
		})
	}

	// Merge: known keys from cfg overwrite existing; unknown keys preserved.
	for k, v := range cfgMap {
		existing[k] = v
	}

	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return result.NewFailure[bool]([]result.PolywaveError{
			result.NewError(result.CodeConfigInvalid, "failed to marshal merged config: "+err.Error()),
		})
	}
	output = append(output, '\n')

	// Validate that repoPath exists before attempting CreateTemp.
	if _, err := os.Stat(repoPath); err != nil {
		return result.NewFailure[bool]([]result.PolywaveError{
			result.NewError(result.CodeConfigIOFailed, "config save: target directory does not exist: "+repoPath),
		})
	}

	// Atomic write: temp file + rename.
	tmpFile, err := os.CreateTemp(repoPath, ".polywave-config-*.tmp")
	if err != nil {
		return result.NewFailure[bool]([]result.PolywaveError{
			result.NewError(result.CodeConfigIOFailed, "failed to create temp file: "+err.Error()),
		})
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(output); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return result.NewFailure[bool]([]result.PolywaveError{
			result.NewError(result.CodeConfigIOFailed, "failed to write temp file: "+err.Error()),
		})
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return result.NewFailure[bool]([]result.PolywaveError{
			result.NewError(result.CodeConfigIOFailed, "failed to close temp file: "+err.Error()),
		})
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return result.NewFailure[bool]([]result.PolywaveError{
			result.NewError(result.CodeConfigIOFailed, "failed to rename temp file: "+err.Error()),
		})
	}

	// Chmod failure is non-fatal: the file is already atomically on disk.
	// Log and continue rather than surfacing a spurious save error.
	if err := os.Chmod(configPath, 0600); err != nil {
		slog.Warn("config: failed to set file permissions", "path", configPath, "err", err)
	}

	return result.NewSuccess(true)
}

// LoadOrDefault is a convenience function that returns a default config
// if Load fails. The default config uses "gated" autonomy level,
// max_auto_retries 2, and max_queue_depth 10.
func LoadOrDefault(startDir string) *PolywaveConfig {
	r := Load(startDir)
	if r.IsSuccess() {
		return r.GetData()
	}
	slog.Warn("LoadOrDefault: falling back to default config",
		"reason", r.Errors[0].Message,
		"startDir", startDir)
	return &PolywaveConfig{
		Autonomy: AutonomyConfig{
			Level:          "gated",
			MaxAutoRetries: 2,
			MaxQueueDepth:  10,
		},
	}
}
