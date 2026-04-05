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

// LoadConfig loads autonomy configuration from saw.config.json in repoPath.
// Returns Result[LoadConfigData] with success when file is found and parsed,
// or with WasDefault=true when file is missing or autonomy section is absent.
// Returns fatal result on JSON parse errors. Also returns fatal if the
// `level` field is set to an unrecognized value.
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

	if cfg.Level != "" {
		if r := ParseLevel(string(cfg.Level)); r.IsFatal() {
			return result.NewFailure[LoadConfigData](r.Errors)
		}
	}

	return result.NewSuccess(LoadConfigData{
		Config:     cfg,
		ConfigPath: path,
		WasDefault: false,
	})
}
