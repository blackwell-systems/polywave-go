package backend

import (
	"encoding/json"
	"os"

	"github.com/blackwell-systems/polywave-go/pkg/config"
)

// PolywaveProviders mirrors the providers section of polywave.config.json.
type PolywaveProviders struct {
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

// LoadProvidersFromConfig reads polywave.config.json from dir or its parents
// and returns the providers section. Returns zero value if not found.
func LoadProvidersFromConfig(dir string) PolywaveProviders {
	var providers PolywaveProviders
	path := config.FindConfigPath(dir)
	if path == "" {
		return providers
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return providers
	}
	var cfg struct {
		Providers PolywaveProviders `json:"providers"`
	}
	if json.Unmarshal(data, &cfg) == nil {
		providers = cfg.Providers
	}
	return providers
}

