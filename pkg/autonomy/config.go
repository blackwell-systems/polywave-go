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
// Returns Result[LoadConfigData] with success when file is found and parsed,
// or with WasDefault=true when file is missing or autonomy section is absent.
// Returns fatal result on JSON parse errors.
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
			result.NewFatal("CONFIG_LOAD_FAILED", fmt.Sprintf("failed to read %s: %v", path, err)).WithCause(err),
		})
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return result.NewFailure[LoadConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_LOAD_FAILED", fmt.Sprintf("invalid JSON in %s: %v", path, err)).WithCause(err),
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
			result.NewFatal("CONFIG_LOAD_FAILED", fmt.Sprintf("failed to parse autonomy config in %s: %v", path, err)).WithCause(err),
		})
	}

	return result.NewSuccess(LoadConfigData{
		Config:     cfg,
		ConfigPath: path,
		WasDefault: false,
	})
}

// SaveConfig writes cfg to the autonomy section of saw.config.json in repoPath.
// If the file exists, other top-level keys (providers, repos, agent, etc.) are
// preserved. If the file does not exist, it is created with only the autonomy key.
// Returns Result[SaveConfigData] with ConfigPath and BytesWritten on success.
func SaveConfig(repoPath string, cfg Config) result.Result[SaveConfigData] {
	path := filepath.Join(repoPath, configFileName)

	// Start with an empty map; populate from existing file if present.
	raw := make(map[string]any)

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result.NewFailure[SaveConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("failed to read existing %s: %v", path, err)).WithCause(err),
		})
	}
	if err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return result.NewFailure[SaveConfigData]([]result.SAWError{
				result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("invalid JSON in existing %s: %v", path, err)).WithCause(err),
			})
		}
	}

	// Marshal cfg into a json.RawMessage and store under "autonomy".
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return result.NewFailure[SaveConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("failed to marshal config for %s: %v", path, err)).WithCause(err),
		})
	}
	raw["autonomy"] = json.RawMessage(cfgBytes)

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return result.NewFailure[SaveConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("failed to marshal %s to JSON: %v", path, err)).WithCause(err),
		})
	}

	if err := os.WriteFile(path, out, 0600); err != nil {
		return result.NewFailure[SaveConfigData]([]result.SAWError{
			result.NewFatal("CONFIG_SAVE_FAILED", fmt.Sprintf("failed to write %s: %v", path, err)).WithCause(err),
		})
	}

	return result.NewSuccess(SaveConfigData{
		ConfigPath:   path,
		BytesWritten: len(out),
	})
}
