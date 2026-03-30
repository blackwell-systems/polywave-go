// Package config provides unified configuration loading and saving for
// scout-and-wave. It replaces the scattered config handling previously split
// across backend.SAWProviders, autonomy.Config, and the web app's
// service.SAWConfig with a single SAWConfig type and Load/Save API.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// SAWConfig is the unified configuration type for all SAW operations.
// It is the superset of fields from backend.SAWProviders, autonomy.Config,
// and the web application's service.SAWConfig.
type SAWConfig struct {
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

// AutonomyConfig controls agent autonomy behavior.
type AutonomyConfig struct {
	Level          string `json:"level,omitempty"`
	MaxAutoRetries int    `json:"max_auto_retries,omitempty"`
	MaxQueueDepth  int    `json:"max_queue_depth,omitempty"`
}

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

const configFileName = "saw.config.json"

// maxWalkDepth is the maximum number of parent directories to traverse.
const maxWalkDepth = 10

// FindConfigPath walks up from startDir looking for saw.config.json.
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

// Load finds saw.config.json starting from startDir, walking up to 10
// parent directories. Parses the full config into SAWConfig.
// Returns N013_CONFIG_NOT_FOUND if no config file exists.
// Returns N014_CONFIG_INVALID if JSON parsing fails.
func Load(startDir string) result.Result[*SAWConfig] {
	path := FindConfigPath(startDir)
	if path == "" {
		return result.NewFailure[*SAWConfig]([]result.SAWError{
			result.NewError(result.CodeConfigNotFound, "no saw.config.json found walking up from "+startDir),
		})
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return result.NewFailure[*SAWConfig]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "failed to read config: "+err.Error()),
		})
	}

	var cfg SAWConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return result.NewFailure[*SAWConfig]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "invalid JSON in "+path+": "+err.Error()),
		})
	}

	// Backward compatibility: migrate legacy repo.path to repos array.
	if len(cfg.Repos) == 0 {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err == nil {
			if repoRaw, ok := raw["repo"]; ok {
				var legacy struct {
					Path string `json:"path"`
					Name string `json:"name"`
				}
				if err := json.Unmarshal(repoRaw, &legacy); err == nil && legacy.Path != "" {
					name := legacy.Name
					if name == "" {
						name = filepath.Base(legacy.Path)
					}
					cfg.Repos = []RepoEntry{{Name: name, Path: legacy.Path}}
				}
			}
		}
	}

	return result.NewSuccess(&cfg)
}

// Save atomically writes cfg to saw.config.json at repoPath.
// Uses temp-file + rename pattern. Preserves unknown top-level keys
// that may exist in the current config file.
func Save(repoPath string, cfg *SAWConfig) result.Result[bool] {
	configPath := filepath.Join(repoPath, configFileName)

	// Read existing file to preserve unknown keys.
	existing := make(map[string]json.RawMessage)
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &existing) // ignore errors; we'll overwrite
	}

	// Marshal the cfg struct to get known keys.
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return result.NewFailure[bool]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "failed to marshal config: "+err.Error()),
		})
	}
	var cfgMap map[string]json.RawMessage
	if err := json.Unmarshal(cfgBytes, &cfgMap); err != nil {
		return result.NewFailure[bool]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "failed to re-parse config: "+err.Error()),
		})
	}

	// Merge: known keys from cfg overwrite existing; unknown keys preserved.
	for k, v := range cfgMap {
		existing[k] = v
	}

	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return result.NewFailure[bool]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "failed to marshal merged config: "+err.Error()),
		})
	}
	output = append(output, '\n')

	// Atomic write: temp file + rename.
	tmpFile, err := os.CreateTemp(repoPath, ".saw-config-*.tmp")
	if err != nil {
		return result.NewFailure[bool]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "failed to create temp file: "+err.Error()),
		})
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(output); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return result.NewFailure[bool]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "failed to write temp file: "+err.Error()),
		})
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return result.NewFailure[bool]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "failed to close temp file: "+err.Error()),
		})
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return result.NewFailure[bool]([]result.SAWError{
			result.NewError(result.CodeConfigInvalid, "failed to rename temp file: "+err.Error()),
		})
	}

	return result.NewSuccess(true)
}

// LoadOrDefault is a convenience function that returns a default config
// if Load fails. The default config uses "gated" autonomy level,
// max_auto_retries 2, and max_queue_depth 10.
func LoadOrDefault(startDir string) *SAWConfig {
	r := Load(startDir)
	if r.IsSuccess() {
		return r.GetData()
	}
	return &SAWConfig{
		Autonomy: AutonomyConfig{
			Level:          "gated",
			MaxAutoRetries: 2,
			MaxQueueDepth:  10,
		},
	}
}
