package autonomy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

const configFileName = "saw.config.json"

// LoadConfigData holds the result data for a successful LoadConfig call.
type LoadConfigData struct {
	Config     Config
	ConfigPath string
	WasDefault bool // true if file not found or autonomy section missing, DefaultConfig() returned
}

// SaveConfigData holds the result data for a successful SaveConfig call.
type SaveConfigData struct {
	ConfigPath   string
	BytesWritten int
}

// LoadConfig loads autonomy configuration from saw.config.json in repoPath.
// If the file doesn't exist, DefaultConfig() is returned with WasDefault=true.
// If the file exists but contains invalid JSON, a Fatal result is returned.
// If the autonomy section is missing, DefaultConfig() is returned with WasDefault=true.
func LoadConfig(repoPath string) result.Result[LoadConfigData] {
	path := filepath.Join(repoPath, configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result.NewSuccess(LoadConfigData{
				Config:     DefaultConfig(),
				ConfigPath: path,
				WasDefault: true,
			})
		}
		return result.NewFailure[LoadConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_LOAD_FAILED", fmt.Sprintf("failed to read config file: %v", err)).WithCause(err),
		})
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return result.NewFailure[LoadConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_LOAD_FAILED", fmt.Sprintf("invalid JSON in config file: %v", err)).WithCause(err),
		})
	}

	autonomyRaw, ok := raw["autonomy"]
	if !ok {
		return result.NewSuccess(LoadConfigData{
			Config:     DefaultConfig(),
			ConfigPath: path,
			WasDefault: true,
		})
	}

	var cfg Config
	if err := json.Unmarshal(autonomyRaw, &cfg); err != nil {
		return result.NewFailure[LoadConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_LOAD_FAILED", fmt.Sprintf("failed to parse autonomy config: %v", err)).WithCause(err),
		})
	}

	return result.NewSuccess(LoadConfigData{
		Config:     cfg,
		ConfigPath: path,
		WasDefault: false,
	})
}

// SaveConfig writes cfg back to saw.config.json in repoPath.
// It reads the existing file first to preserve other top-level keys,
// then updates (or creates) the "autonomy" key. Non-autonomy keys
// (providers, repos, agent, etc.) are preserved unchanged.
func SaveConfig(repoPath string, cfg Config) result.Result[SaveConfigData] {
	path := filepath.Join(repoPath, configFileName)

	// Start with an empty map; populate from existing file if present.
	raw := make(map[string]any)

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result.NewFailure[SaveConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("failed to read existing config file: %v", err)).WithCause(err),
		})
	}
	if err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return result.NewFailure[SaveConfigData]([]result.SAWError{
				result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("invalid JSON in existing config file: %v", err)).WithCause(err),
			})
		}
	}

	// Marshal cfg into a json.RawMessage and store under "autonomy".
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return result.NewFailure[SaveConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("failed to marshal config: %v", err)).WithCause(err),
		})
	}
	raw["autonomy"] = json.RawMessage(cfgBytes)

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return result.NewFailure[SaveConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("failed to marshal config to JSON: %v", err)).WithCause(err),
		})
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return result.NewFailure[SaveConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("failed to write config file: %v", err)).WithCause(err),
		})
	}

	return result.NewSuccess(SaveConfigData{
		ConfigPath:   path,
		BytesWritten: len(out),
	})
}
