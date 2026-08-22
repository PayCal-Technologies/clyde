package clyde

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOllamaListModelsAndGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "qwen2.5-coder:7b", "size": 1234}},
			})
		case "/api/generate":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["model"] != "qwen2.5-coder:7b" {
				t.Fatalf("unexpected model: %#v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"response": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, time.Second)
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Name != "qwen2.5-coder:7b" {
		t.Fatalf("unexpected models: %#v", models)
	}
	got, err := client.Generate(context.Background(), "qwen2.5-coder:7b", "hello", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestOllamaListModelsSanitizesModelMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{
				{"name": "  qwen2.5-coder:7b  ", "size": -10},
				{"name": "   ", "size": 1234},
			},
		})
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, time.Second)
	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Name != "qwen2.5-coder:7b" || models[0].Size != 0 {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestOllamaGenerateSendsNumCtx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		options, _ := body["options"].(map[string]any)
		if options["num_ctx"].(float64) != 32768 {
			t.Fatalf("missing num_ctx option: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "ok"})
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, time.Second)
	got, err := client.GenerateWithOptions(context.Background(), GenerateOptions{
		Model:  "qwen2.5-coder:7b",
		Prompt: "hello",
		NumCtx: 32768,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestOllamaGenerateRejectsHardenedInputs(t *testing.T) {
	client := NewOllamaClient("http://127.0.0.1:1", time.Second)
	if _, err := client.GenerateWithOptions(context.Background(), GenerateOptions{Model: "   ", Prompt: "hello"}, nil); err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("unexpected blank model error: %v", err)
	}
	if _, err := client.GenerateWithOptions(context.Background(), GenerateOptions{Model: "ok", Prompt: "hello", NumCtx: -1}, nil); err == nil || !strings.Contains(err.Error(), "num_ctx") {
		t.Fatalf("unexpected num_ctx error: %v", err)
	}
}

func TestOllamaGenerateStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "qwen2.5-coder:7b"}}})
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"response":"hel"}` + "\n" + `{"response":"lo","done":true}` + "\n"))
	}))
	defer server.Close()

	var out bytes.Buffer
	client := NewOllamaClient(server.URL, time.Second)
	got, err := client.Generate(context.Background(), "qwen2.5-coder:7b", "hello", true, &out)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" || out.String() != "hello" {
		t.Fatalf("unexpected stream got=%q out=%q", got, out.String())
	}
}

func TestOllamaGenerateRejectsOversizedStreamLine(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"response":"` + strings.Repeat("x", maxOllamaStreamLineBytes+1) + `"}` + "\n"))
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, time.Second)
	_, err := client.Generate(context.Background(), "qwen2.5-coder:7b", "hello", true, nil)

	if err == nil {
		t.Fatalf("expected oversized stream error")
	}
}

func TestOllamaReportsNon2xxAndMalformedStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			http.Error(w, "bad tags", http.StatusBadGateway)
		case "/api/generate":
			_, _ = w.Write([]byte("{bad json}\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewOllamaClient(server.URL, time.Second)
	if _, err := client.ListModels(context.Background()); err == nil || !strings.Contains(err.Error(), "bad tags") {
		t.Fatalf("unexpected list error: %v", err)
	}
	if _, err := client.Generate(context.Background(), "qwen2.5-coder:7b", "hello", true, nil); err == nil {
		t.Fatalf("expected malformed stream error")
	}
}

func TestSelectModelUsesEnv(t *testing.T) {
	t.Setenv("CLYDE_MODEL", "gemma3:4b")
	cfg := DefaultConfig()
	applyConfigEnv(&cfg)
	model, err := SelectModel("", cfg, []LocalModel{{Name: "qwen2.5-coder:7b"}, {Name: "gemma3:4b"}})
	if err != nil {
		t.Fatal(err)
	}
	if model != "gemma3:4b" {
		t.Fatalf("unexpected model: %s", model)
	}
}

func TestNewOllamaClientUsesEnvURL(t *testing.T) {
	t.Setenv("CLYDE_OLLAMA_URL", "http://localhost:9999")
	cfg := DefaultConfig()
	applyConfigEnv(&cfg)
	client := NewOllamaClient(cfg.OllamaURL, time.Second)
	if client.BaseURL != "http://localhost:9999" {
		t.Fatalf("unexpected URL: %s", client.BaseURL)
	}
}

func TestCLIAskAndAgentUseOllama(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir+"/main.go", "package main\n")
	var prompts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "qwen2.5-coder:7b", "size": 1234}},
			})
		case "/api/generate":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			prompts = append(prompts, body["prompt"].(string))
			_ = json.NewEncoder(w).Encode(map[string]any{"response": "feedback"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out, errOut bytes.Buffer
	status := MainWithInput([]string{"ask", "--ollama-url", server.URL, "--model", "qwen2.5-coder:7b", "--no-stream", "hello"}, strings.NewReader(""), &out, &errOut)
	if status != 0 || !strings.Contains(out.String(), "feedback") {
		t.Fatalf("ask status=%d out=%q err=%q", status, out.String(), errOut.String())
	}
	if len(prompts) != 1 || prompts[0] != "hello" {
		t.Fatalf("ask prompt not passed through: %#v", prompts)
	}

	out.Reset()
	errOut.Reset()
	status = MainWithInput([]string{"agent", dir, "--ollama-url", server.URL, "--allow-remote-ollama", "--model", "qwen2.5-coder:7b", "--no-stream", "review"}, strings.NewReader(""), &out, &errOut)
	if status != 0 || !strings.Contains(out.String(), "Clyde agent using local model: qwen2.5-coder:7b") {
		t.Fatalf("agent status=%d out=%q err=%q", status, out.String(), errOut.String())
	}
	if len(prompts) != 2 || !strings.Contains(prompts[1], "package main") {
		t.Fatalf("agent prompt missing source context: %#v", prompts)
	}
}

func TestCLIModelsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"models": []map[string]any{{"name": "qwen2.5-coder:7b", "size": 1234}},
		})
	}))
	defer server.Close()

	var out, errOut bytes.Buffer
	status := Main([]string{"models", "--ollama-url", server.URL, "--json"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), `"selected": "qwen2.5-coder:7b"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestAgentRejectsRemoteOllamaURLByDefault(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir+"/main.go", "package main\n")
	var out, errOut bytes.Buffer
	status := MainWithInput([]string{"agent", dir, "--ollama-url", "https://example.com", "review"}, strings.NewReader(""), &out, &errOut)
	if status != 1 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(errOut.String(), "refuses to send scanned source") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}
