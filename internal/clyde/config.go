package clyde

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const configEnv = "CLYDE_CONFIG"

const (
	maxConfigNumCtx               = 1_000_000
	maxConfigTimeoutSeconds       = 24 * 60 * 60
	maxConfigContextChars         = 20_000_000
	maxConfigFileBytes      int64 = 1 << 30
	maxConfigChunkChars           = 5_000_000
)

type Config struct {
	OllamaURL           string `json:"ollama_url"`
	Model               string `json:"model"`
	NumCtx              int    `json:"num_ctx"`
	AskTimeoutSeconds   int    `json:"ask_timeout_seconds"`
	AgentTimeoutSeconds int    `json:"agent_timeout_seconds"`
	MaxContextChars     int    `json:"max_context_chars"`
	MaxFileBytes        int64  `json:"max_file_bytes"`
	MaxChunkChars       int    `json:"max_chunk_chars"`
}

func DefaultConfig() Config {
	return Config{
		OllamaURL:           "http://127.0.0.1:11434",
		Model:               "qwen2.5-coder:7b",
		NumCtx:              8192,
		AskTimeoutSeconds:   120,
		AgentTimeoutSeconds: 180,
		MaxContextChars:     16000,
		MaxFileBytes:        250000,
		MaxChunkChars:       18000,
	}
}

func ConfigPath() (string, error) {
	if path := os.Getenv(configEnv); path != "" {
		return path, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "clyde", "config.json"), nil
}

func LoadConfig() (Config, string, error) {
	cfg := DefaultConfig()
	path, err := ConfigPath()
	if err != nil {
		return cfg, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyConfigEnv(&cfg)
			if err := ValidateConfig(cfg); err != nil {
				return cfg, path, err
			}
			return cfg, path, nil
		}
		return cfg, path, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, path, err
	}
	if err := rejectNegativeConfig(cfg); err != nil {
		return cfg, path, err
	}
	cfg = withConfigDefaults(cfg)
	applyConfigEnv(&cfg)
	if err := ValidateConfig(cfg); err != nil {
		return cfg, path, err
	}
	return cfg, path, nil
}

func WriteDefaultConfig(path string) error {
	cfg := DefaultConfig()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func ValidateConfig(cfg Config) error {
	if err := validateOllamaURL(cfg.OllamaURL); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return errf("model must not be blank")
	}
	if err := validatePositiveInt("num_ctx", cfg.NumCtx, maxConfigNumCtx); err != nil {
		return err
	}
	if err := validatePositiveInt("ask_timeout_seconds", cfg.AskTimeoutSeconds, maxConfigTimeoutSeconds); err != nil {
		return err
	}
	if err := validatePositiveInt("agent_timeout_seconds", cfg.AgentTimeoutSeconds, maxConfigTimeoutSeconds); err != nil {
		return err
	}
	if err := validatePositiveInt("max_context_chars", cfg.MaxContextChars, maxConfigContextChars); err != nil {
		return err
	}
	if err := validatePositiveInt64("max_file_bytes", cfg.MaxFileBytes, maxConfigFileBytes); err != nil {
		return err
	}
	return validatePositiveInt("max_chunk_chars", cfg.MaxChunkChars, maxConfigChunkChars)
}

func withConfigDefaults(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.OllamaURL == "" {
		cfg.OllamaURL = defaults.OllamaURL
	}
	if cfg.Model == "" {
		cfg.Model = defaults.Model
	}
	if cfg.NumCtx <= 0 {
		cfg.NumCtx = defaults.NumCtx
	}
	if cfg.AskTimeoutSeconds <= 0 {
		cfg.AskTimeoutSeconds = defaults.AskTimeoutSeconds
	}
	if cfg.AgentTimeoutSeconds <= 0 {
		cfg.AgentTimeoutSeconds = defaults.AgentTimeoutSeconds
	}
	if cfg.MaxContextChars <= 0 {
		cfg.MaxContextChars = defaults.MaxContextChars
	}
	if cfg.MaxFileBytes <= 0 {
		cfg.MaxFileBytes = defaults.MaxFileBytes
	}
	if cfg.MaxChunkChars <= 0 {
		cfg.MaxChunkChars = defaults.MaxChunkChars
	}
	return cfg
}

func applyConfigEnv(cfg *Config) {
	if value := os.Getenv("CLYDE_OLLAMA_URL"); value != "" {
		cfg.OllamaURL = value
	}
	if value := os.Getenv("CLYDE_MODEL"); value != "" {
		cfg.Model = value
	}
}

func rejectNegativeConfig(cfg Config) error {
	if cfg.NumCtx < 0 {
		return errf("num_ctx must be zero or positive")
	}
	if cfg.AskTimeoutSeconds < 0 {
		return errf("ask_timeout_seconds must be zero or positive")
	}
	if cfg.AgentTimeoutSeconds < 0 {
		return errf("agent_timeout_seconds must be zero or positive")
	}
	if cfg.MaxContextChars < 0 {
		return errf("max_context_chars must be zero or positive")
	}
	if cfg.MaxFileBytes < 0 {
		return errf("max_file_bytes must be zero or positive")
	}
	if cfg.MaxChunkChars < 0 {
		return errf("max_chunk_chars must be zero or positive")
	}
	return nil
}

func validateOllamaURL(raw string) error {
	if strings.TrimSpace(raw) != raw || raw == "" {
		return errf("ollama_url must be a valid http or https URL")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errf("ollama_url must be a valid http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errf("ollama_url must use http or https")
	}
	return nil
}

func validatePositiveInt(name string, value, max int) error {
	if value <= 0 {
		return errf("%s must be positive", name)
	}
	if value > max {
		return errf("%s is too large; maximum is %d", name, max)
	}
	return nil
}

func validatePositiveInt64(name string, value, max int64) error {
	if value <= 0 {
		return errf("%s must be positive", name)
	}
	if value > max {
		return errf("%s is too large; maximum is %d", name, max)
	}
	return nil
}
