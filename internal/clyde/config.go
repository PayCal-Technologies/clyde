package clyde

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const configEnv = "CLYDE_CONFIG"

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
			return cfg, path, nil
		}
		return cfg, path, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, path, err
	}
	cfg = withConfigDefaults(cfg)
	applyConfigEnv(&cfg)
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
