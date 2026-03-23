package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SAWProviders mirrors the providers section of saw.config.json.
type SAWProviders struct {
	Anthropic struct {
		APIKey string `json:"api_key"`
	} `json:"anthropic"`
	Bedrock struct {
		Region         string `json:"region"`
		AccessKeyID    string `json:"access_key_id"`
		SecretAccessKey string `json:"secret_access_key"`
		SessionToken   string `json:"session_token"`
		Profile        string `json:"profile"`
	} `json:"bedrock"`
	OpenAI struct {
		APIKey string `json:"api_key"`
	} `json:"openai"`
}

// LoadProvidersFromConfig reads saw.config.json from dir or its parents
// and returns the providers section. Returns zero value if not found.
func LoadProvidersFromConfig(dir string) SAWProviders {
	var providers SAWProviders
	path := findConfigFile(dir)
	if path == "" {
		return providers
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return providers
	}
	var cfg struct {
		Providers SAWProviders `json:"providers"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		providers = cfg.Providers
	}
	return providers
}

// findConfigFile walks up from dir looking for saw.config.json.
func findConfigFile(dir string) string {
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, "saw.config.json")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}
