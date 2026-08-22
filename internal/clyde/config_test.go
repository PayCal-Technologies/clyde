package clyde

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfigReadsClydeConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{
  "ollama_url": "http://127.0.0.1:19999",
  "model": "gemma3:4b",
  "num_ctx": 4096,
  "ask_timeout_seconds": 12,
  "agent_timeout_seconds": 34,
  "max_context_chars": 5000,
  "max_file_bytes": 123,
  "max_chunk_chars": 456
}
`)

	cfg, gotPath, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if cfg.Model != "gemma3:4b" || cfg.NumCtx != 4096 || cfg.MaxFileBytes != 123 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	t.Setenv("CLYDE_MODEL", "env-model")
	t.Setenv("CLYDE_OLLAMA_URL", "http://localhost:17777")
	mustWrite(t, path, `{"ollama_url":"http://localhost:16666","model":"file-model"}`)

	cfg, _, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Model != "env-model" || cfg.OllamaURL != "http://localhost:17777" {
		t.Fatalf("env did not override config: %#v", cfg)
	}
}

func TestLoadConfigRejectsInvalidOllamaURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{"ollama_url":"not-a-url"}`)

	_, _, err := LoadConfig()

	if err == nil || !strings.Contains(err.Error(), "ollama_url") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsInvalidEnvOllamaURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	t.Setenv("CLYDE_OLLAMA_URL", "ftp://example.com")

	_, _, err := LoadConfig()

	if err == nil || !strings.Contains(err.Error(), "ollama_url") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsBlankModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{"model":"   "}`)

	_, _, err := LoadConfig()

	if err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigTrimsStringValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{"ollama_url":" http://127.0.0.1:11434/ ","model":" qwen2.5-coder:7b "}`)

	cfg, _, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OllamaURL != "http://127.0.0.1:11434/" || cfg.Model != "qwen2.5-coder:7b" {
		t.Fatalf("unexpected config strings: %#v", cfg)
	}
}

func TestLoadConfigRejectsNULStringValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, "{\"model\":\"bad\\u0000model\"}")

	_, _, err := LoadConfig()

	if err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsNegativeLimits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{"max_file_bytes":-1}`)

	_, _, err := LoadConfig()

	if err == nil || !strings.Contains(err.Error(), "max_file_bytes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigDefaultsZeroValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{"num_ctx":0,"max_chunk_chars":0}`)

	cfg, _, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NumCtx != DefaultConfig().NumCtx || cfg.MaxChunkChars != DefaultConfig().MaxChunkChars {
		t.Fatalf("zero values did not default: %#v", cfg)
	}
}

func TestConfigCommandInitAndShow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)

	var out, errOut bytes.Buffer
	status := Main([]string{"config", "init"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	out.Reset()
	errOut.Reset()
	status = Main([]string{"config", "show"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"model": "qwen2.5-coder:7b"`) {
		t.Fatalf("unexpected config output: %s", out.String())
	}
}
