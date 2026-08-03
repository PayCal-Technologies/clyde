package clyde

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type LocalModel struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

type OllamaClient struct {
	BaseURL string
	Client  *http.Client
}

type GenerateOptions struct {
	Model  string
	Prompt string
	Stream bool
	NumCtx int
}

func NewOllamaClient(baseURL string, timeout time.Duration) OllamaClient {
	if baseURL == "" {
		baseURL = DefaultConfig().OllamaURL
	}
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return OllamaClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Client:  &http.Client{Timeout: timeout},
	}
}

func (c OllamaClient) ListModels(ctx context.Context) ([]LocalModel, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, errf("ollama is not reachable at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, errf("ollama list models failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Models []struct {
			Name       string `json:"name"`
			Size       int64  `json:"size"`
			ModifiedAt string `json:"modified_at"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	models := make([]LocalModel, 0, len(payload.Models))
	for _, model := range payload.Models {
		models = append(models, LocalModel{Name: model.Name, Size: model.Size, ModifiedAt: model.ModifiedAt})
	}
	return models, nil
}

func (c OllamaClient) Generate(ctx context.Context, model, prompt string, stream bool, out io.Writer) (string, error) {
	return c.GenerateWithOptions(ctx, GenerateOptions{Model: model, Prompt: prompt, Stream: stream}, out)
}

func (c OllamaClient) GenerateWithOptions(ctx context.Context, opts GenerateOptions, out io.Writer) (string, error) {
	if opts.Model == "" {
		return "", errf("model must not be empty")
	}
	payload := map[string]any{
		"model":  opts.Model,
		"prompt": opts.Prompt,
		"stream": opts.Stream,
	}
	if opts.NumCtx > 0 {
		payload["options"] = map[string]any{"num_ctx": opts.NumCtx}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", errf("ollama generate failed at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", errf("ollama generate failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if !opts.Stream {
		var payload struct {
			Response string `json:"response"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return "", err
		}
		return payload.Response, nil
	}
	var full strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var chunk struct {
			Response string `json:"response"`
			Done     bool   `json:"done"`
			Error    string `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return full.String(), err
		}
		if chunk.Error != "" {
			return full.String(), errf("%s", chunk.Error)
		}
		if chunk.Response != "" {
			full.WriteString(chunk.Response)
			if out != nil {
				fmt.Fprint(out, chunk.Response)
			}
		}
		if chunk.Done {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return full.String(), err
	}
	return full.String(), nil
}

func SelectModel(explicit string, cfg Config, models []LocalModel) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if cfg.Model != "" {
		return cfg.Model, nil
	}
	if len(models) == 0 {
		return "", errf("no Ollama models found; run `ollama pull %s` or pass --model", DefaultConfig().Model)
	}
	for _, model := range models {
		if model.Name == DefaultConfig().Model {
			return model.Name, nil
		}
	}
	return models[0].Name, nil
}
