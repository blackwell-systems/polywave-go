package autonomy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const configFileName = "saw.config.json"

// LoadConfig loads autonomy configuration from saw.config.json in repoPath.
// If the file doesn't exist, DefaultConfig() is returned with no error.
// If the file exists but contains invalid JSON, an error is returned.
// If the autonomy section is missing, DefaultConfig() is returned.
func LoadConfig(repoPath string) (Config, error) {
	path := filepath.Join(repoPath, configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return DefaultConfig(), err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Config{}, err
	}

	autonomyRaw, ok := raw["autonomy"]
	if !ok {
		return DefaultConfig(), nil
	}

	var cfg Config
	if err := json.Unmarshal(autonomyRaw, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// SaveConfig writes cfg back to saw.config.json in repoPath.
// It reads the existing file first to preserve other top-level keys,
// then updates (or creates) the "autonomy" key.
func SaveConfig(repoPath string, cfg Config) error {
	path := filepath.Join(repoPath, configFileName)

	// Start with an empty map; populate from existing file if present.
	raw := make(map[string]any)

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	}

	// Marshal cfg into a json.RawMessage and store under "autonomy".
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	raw["autonomy"] = json.RawMessage(cfgBytes)

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, out, 0644)
}
