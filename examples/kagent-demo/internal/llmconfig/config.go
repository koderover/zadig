package llmconfig

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
)

type Config struct {
	Provider Provider
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
}

func Load() (Config, error) {
	return LoadFromMap(mapFromEnv())
}

func LoadFromMap(env map[string]string) (Config, error) {
	timeout := 30 * time.Second
	if raw := strings.TrimSpace(env["MODEL_TIMEOUT"]); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("parse MODEL_TIMEOUT: %w", err)
		}
		timeout = parsed
	}

	cfg := Config{
		Provider: Provider(strings.ToLower(strings.TrimSpace(env["MODEL_PROVIDER"]))),
		BaseURL:  strings.TrimRight(strings.TrimSpace(env["MODEL_BASE_URL"]), "/"),
		APIKey:   strings.TrimSpace(env["MODEL_API_KEY"]),
		Model:    strings.TrimSpace(env["MODEL_NAME"]),
		Timeout:  timeout,
	}

	if cfg.Provider == "" {
		return Config{}, fmt.Errorf("MODEL_PROVIDER is required")
	}
	switch cfg.Provider {
	case ProviderOpenAI, ProviderAnthropic:
	default:
		return Config{}, fmt.Errorf("unsupported MODEL_PROVIDER %q", cfg.Provider)
	}
	if cfg.BaseURL == "" {
		return Config{}, fmt.Errorf("MODEL_BASE_URL is required")
	}
	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("MODEL_API_KEY is required")
	}
	if cfg.Model == "" {
		return Config{}, fmt.Errorf("MODEL_NAME is required")
	}
	return cfg, nil
}

func mapFromEnv() map[string]string {
	keys := []string{
		"MODEL_PROVIDER",
		"MODEL_BASE_URL",
		"MODEL_API_KEY",
		"MODEL_NAME",
		"MODEL_TIMEOUT",
	}
	env := make(map[string]string, len(keys))
	for _, key := range keys {
		env[key] = os.Getenv(key)
	}
	return env
}
