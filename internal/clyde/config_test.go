package clyde

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
  "max_chunk_chars": 456,
  "exclude_folders": ["fixtures", "generated/api"]
}
`)

	cfg, gotPath, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("unexpected path: %s", gotPath)
	}
	if cfg.Model != "gemma3:4b" || cfg.NumCtx != 4096 || cfg.MaxFileBytes != 123 || strings.Join(cfg.ExcludeFolders, ",") != "fixtures,generated/api" {
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

func TestLoadConfigRejectsWritableConfigFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not a Windows privacy boundary")
	}
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{}`)
	if err := os.Chmod(path, 0o622); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadConfig()

	if err == nil || !strings.Contains(err.Error(), "group- or world-writable") {
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
	if len(cfg.ExcludeFolders) != 0 {
		t.Fatalf("default config should not include target folders: %#v", cfg.ExcludeFolders)
	}
}

func TestLoadConfigRejectsInvalidExcludeFolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{"exclude_folders":["/tmp/build"]}`)

	_, _, err := LoadConfig()

	if err == nil || !strings.Contains(err.Error(), "exclude folder") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigCommandInitAndShow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	t.Setenv("CLYDE_CONFIG", path)

	var out, errOut bytes.Buffer
	status := Main([]string{"config", "init"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
		t.Fatalf("unexpected config dir mode: %v", info.Mode().Perm())
	}
	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected config mode: %v", info.Mode().Perm())
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
	if _, ok := payload["exclude_folders"]; ok {
		t.Fatalf("basic config should not include target folders: %s", out.String())
	}
}

func TestConfigInitRefusesExistingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	mustWrite(t, target, "{}")
	path := filepath.Join(dir, "config.json")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLYDE_CONFIG", path)

	var out, errOut bytes.Buffer
	status := Main([]string{"config", "init"}, &out, &errOut)

	if status != 1 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(errOut.String(), "config already exists") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}" {
		t.Fatalf("target was modified: %q", data)
	}
}

func TestConfigInitRejectsSymlinkParentDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "redirect")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "config-dir")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	t.Setenv("CLYDE_CONFIG", filepath.Join(link, "config.json"))

	var out, errOut bytes.Buffer
	status := Main([]string{"config", "init"}, &out, &errOut)

	if status != 1 || !strings.Contains(errOut.String(), "symlink path component") {
		t.Fatalf("expected symlink parent refusal, status=%d stderr=%s", status, errOut.String())
	}
}

func TestConfigInitRefusesDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.Symlink(filepath.Join(dir, "missing.json"), path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLYDE_CONFIG", path)

	var out, errOut bytes.Buffer
	status := Main([]string{"config", "init"}, &out, &errOut)

	if status != 1 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(errOut.String(), "config already exists") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
	if info, err := os.Lstat(path); err != nil {
		t.Fatal(err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink to remain, got %v", info.Mode())
	}
}

func TestWriteDefaultConfigRefusesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	mustWrite(t, path, "{}")

	err := WriteDefaultConfig(path)

	if err == nil || !strings.Contains(err.Error(), "config already exists") {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{}" {
		t.Fatalf("config was modified: %q", data)
	}
}
