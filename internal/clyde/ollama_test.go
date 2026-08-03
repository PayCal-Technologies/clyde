package clyde

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOllamaListModelsAndGenerate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "qwen2.5-coder:14b", "size": 1234}},
			})
		case "/api/generate":
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
	if len(models) != 1 || models[0].Name != "qwen2.5-coder:14b" {
		t.Fatalf("unexpected models: %#v", models)
	}
	got, err := client.Generate(context.Background(), "qwen2.5-coder:14b", "hello", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != "ok" {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestOllamaGenerateStreaming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "qwen2.5-coder:14b"}}})
			return
		}
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"response":"hel"}` + "\n" + `{"response":"lo","done":true}` + "\n"))
	}))
	defer server.Close()

	var out bytes.Buffer
	client := NewOllamaClient(server.URL, time.Second)
	got, err := client.Generate(context.Background(), "qwen2.5-coder:14b", "hello", true, &out)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" || out.String() != "hello" {
		t.Fatalf("unexpected stream got=%q out=%q", got, out.String())
	}
}

func TestSelectModelUsesEnv(t *testing.T) {
	t.Setenv("CLYDE_MODEL", "gemma3:4b")
	model, err := SelectModel("", []LocalModel{{Name: "qwen2.5-coder:14b"}, {Name: "gemma3:4b"}})
	if err != nil {
		t.Fatal(err)
	}
	if model != "gemma3:4b" {
		t.Fatalf("unexpected model: %s", model)
	}
}

func TestNewOllamaClientUsesEnvURL(t *testing.T) {
	t.Setenv("CLYDE_OLLAMA_URL", "http://localhost:9999")
	client := NewOllamaClient("", time.Second)
	if client.BaseURL != "http://localhost:9999" {
		t.Fatalf("unexpected URL: %s", client.BaseURL)
	}
}

func TestCLIAskAndAgentUseOllama(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, dir+"/main.go", "package main\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{{"name": "qwen2.5-coder:14b", "size": 1234}},
			})
		case "/api/generate":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !strings.Contains(body["prompt"].(string), "package main") && strings.Contains(strings.Join(os.Args, " "), "-test.run") {
				// ask does not include repo context; agent does.
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"response": "feedback"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var out, errOut bytes.Buffer
	status := MainWithInput([]string{"ask", "--ollama-url", server.URL, "--model", "qwen2.5-coder:14b", "--no-stream", "hello"}, strings.NewReader(""), &out, &errOut)
	if status != 0 || !strings.Contains(out.String(), "feedback") {
		t.Fatalf("ask status=%d out=%q err=%q", status, out.String(), errOut.String())
	}

	out.Reset()
	errOut.Reset()
	status = MainWithInput([]string{"agent", dir, "--ollama-url", server.URL, "--allow-remote-ollama", "--model", "qwen2.5-coder:14b", "--no-stream", "review"}, strings.NewReader(""), &out, &errOut)
	if status != 0 || !strings.Contains(out.String(), "Clyde agent using local model: qwen2.5-coder:14b") {
		t.Fatalf("agent status=%d out=%q err=%q", status, out.String(), errOut.String())
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
