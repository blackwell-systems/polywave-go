// Package config provides the canonical SAWConfig type and saw.config.json
// loader/saver for use by both the engine and web app.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

const configFileName = "saw.config.json"

// RepoEntry represents a single repository tracked by SAW.
type RepoEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// RepoConfig is kept for backward-compat JSON deserialization of old configs.
type RepoConfig struct {
	Path string `json:"path"`
}

// AutonomyConfig holds autonomy-level settings.
type AutonomyConfig struct {
	Level          string `json:"level,omitempty"`
	MaxAutoRetries int    `json:"max_auto_retries,omitempty"`
	MaxQueueDepth  int    `json:"max_queue_depth,omitempty"`
}

// AgentConfig holds per-role model overrides.
type AgentConfig struct {
	ScoutModel       string `json:"scout_model,omitempty"`
	WaveModel        string `json:"wave_model,omitempty"`
	ChatModel        string `json:"chat_model,omitempty"`
	IntegrationModel string `json:"integration_model,omitempty"`
	ScaffoldModel    string `json:"scaffold_model,omitempty"`
	PlannerModel     string `json:"planner_model,omitempty"`
	CriticModel      string `json:"critic_model,omitempty"`
	ReviewModel      string `json:"review_model,omitempty"`
}

// QualityConfig holds quality gate settings.
type QualityConfig struct {
	RequireTests   bool          `json:"require_tests,omitempty"`
	RequireLint    bool          `json:"require_lint,omitempty"`
	BlockOnFailure bool          `json:"block_on_failure,omitempty"`
	CodeReview     CodeReviewCfg `json:"code_review,omitempty"`
}

// CodeReviewCfg holds settings for the AI code review post-merge gate.
type CodeReviewCfg struct {
	Enabled   bool   `json:"enabled,omitempty"`
	Blocking  bool   `json:"blocking,omitempty"`
	Model     string `json:"model,omitempty"`
	Threshold int    `json:"threshold,omitempty"`
}

// AppearConfig holds UI appearance settings.
type AppearConfig struct {
	Theme               string   `json:"theme,omitempty"`
	ColorTheme          string   `json:"color_theme,omitempty"`
	ColorThemeDark      string   `json:"color_theme_dark,omitempty"`
	ColorThemeLight     string   `json:"color_theme_light,omitempty"`
	FavoriteThemesDark  []string `json:"favorite_themes_dark,omitempty"`
	FavoriteThemesLight []string `json:"favorite_themes_light,omitempty"`
}

// ProvidersConfig holds credential configuration for all supported LLM providers.
type ProvidersConfig struct {
	Anthropic AnthropicProviderConfig `json:"anthropic,omitempty"`
	OpenAI    OpenAIProviderConfig    `json:"openai,omitempty"`
	Bedrock   BedrockProviderConfig   `json:"bedrock,omitempty"`
}

// AnthropicProviderConfig holds Anthropic API credentials.
type AnthropicProviderConfig struct {
	APIKey string `json:"api_key,omitempty"`
}

// OpenAIProviderConfig holds OpenAI API credentials.
type OpenAIProviderConfig struct {
	APIKey string `json:"api_key,omitempty"`
}

// BedrockProviderConfig holds AWS Bedrock credentials.
type BedrockProviderConfig struct {
	Region          string `json:"region,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	SessionToken    string `json:"session_token,omitempty"`
	Profile         string `json:"profile,omitempty"`
}

// SAWConfig is the canonical shape of saw.config.json.
type SAWConfig struct {
	Repos     []RepoEntry    `json:"repos,omitempty"`
	Repo      RepoConfig     `json:"repo,omitempty"`      // legacy, read-only for migration
	Autonomy  AutonomyConfig `json:"autonomy,omitempty"`
	Agent     AgentConfig    `json:"agent,omitempty"`
	Quality   QualityConfig  `json:"quality,omitempty"`
	Appear    AppearConfig   `json:"appearance,omitempty"`
	Providers ProvidersConfig `json:"providers,omitempty"`
}

// Load reads saw.config.json from repoPath and returns a Result wrapping the
// parsed config. Returns CodeConfigNotFound if the file does not exist, or
// CodeConfigParseFailed if the JSON is invalid.
func Load(repoPath string) result.Result[*SAWConfig] {
	path := filepath.Join(repoPath, configFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result.NewFailure[*SAWConfig]([]result.SAWError{{
				Code:     result.CodeConfigNotFound,
				Message:  "saw.config.json not found at " + path,
				Severity: "warning",
			}})
		}
		return result.NewFailure[*SAWConfig]([]result.SAWError{{
			Code:     result.CodeConfigParseFailed,
			Message:  "failed to read config: " + err.Error(),
			Severity: "fatal",
		}})
	}
	var cfg SAWConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return result.NewFailure[*SAWConfig]([]result.SAWError{{
			Code:     result.CodeConfigParseFailed,
			Message:  "failed to parse config: " + err.Error(),
			Severity: "fatal",
		}})
	}
	return result.NewSuccess(&cfg)
}

// LoadOrDefault reads saw.config.json from repoPath and returns a pointer to
// the parsed config. If the file does not exist or cannot be parsed, a default
// config (empty, non-nil) is returned. Never returns nil.
func LoadOrDefault(repoPath string) *SAWConfig {
	r := Load(repoPath)
	if r.IsSuccess() {
		return r.GetData()
	}
	return &SAWConfig{}
}

// Save writes cfg to saw.config.json in repoPath, preserving any top-level
// keys not present in SAWConfig. Returns a Result indicating success/failure.
func Save(repoPath string, cfg *SAWConfig) result.Result[struct{}] {
	path := filepath.Join(repoPath, configFileName)

	// Preserve unknown top-level keys from existing file.
	raw := make(map[string]json.RawMessage)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &raw)
	} else if !errors.Is(err, os.ErrNotExist) {
		return result.NewFailure[struct{}]([]result.SAWError{{
			Code:     result.CodeConfigSaveFailed,
			Message:  "failed to read existing config: " + err.Error(),
			Severity: "fatal",
		}})
	}

	// Marshal each known field and store.
	sections := map[string]any{
		"repos":      cfg.Repos,
		"repo":       cfg.Repo,
		"autonomy":   cfg.Autonomy,
		"agent":      cfg.Agent,
		"quality":    cfg.Quality,
		"appearance": cfg.Appear,
		"providers":  cfg.Providers,
	}
	for key, val := range sections {
		b, err := json.Marshal(val)
		if err != nil {
			return result.NewFailure[struct{}]([]result.SAWError{{
				Code:     result.CodeConfigSaveFailed,
				Message:  "failed to marshal " + key + ": " + err.Error(),
				Severity: "fatal",
			}})
		}
		raw[key] = b
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return result.NewFailure[struct{}]([]result.SAWError{{
			Code:     result.CodeConfigSaveFailed,
			Message:  "failed to marshal config: " + err.Error(),
			Severity: "fatal",
		}})
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		return result.NewFailure[struct{}]([]result.SAWError{{
			Code:     result.CodeConfigSaveFailed,
			Message:  "failed to write config: " + err.Error(),
			Severity: "fatal",
		}})
	}
	return result.NewSuccess(struct{}{})
}
