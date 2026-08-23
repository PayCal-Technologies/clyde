package clyde

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

func cmdModels(args []string, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	fs.SetOutput(out)
	ollamaURL := fs.String("ollama-url", cfg.OllamaURL, "Ollama base URL")
	timeout := fs.Float64("timeout", 10, "seconds to wait for Ollama")
	jsonOut := fs.Bool("json", false, "print machine-readable JSON")
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"json": true})); err != nil {
		return err
	}
	if err := validateOllamaURL(*ollamaURL); err != nil {
		return err
	}
	if err := validatePositiveSeconds("timeout", *timeout); err != nil {
		return err
	}
	client := NewOllamaClient(*ollamaURL, time.Duration(*timeout*float64(time.Second)))
	models, err := client.ListModels(context.Background())
	if err != nil {
		return err
	}
	selected, _ := SelectModel("", cfg, models)
	if *jsonOut {
		data, err := json.MarshalIndent(map[string]any{
			"ollama_url": client.BaseURL,
			"selected":   selected,
			"models":     models,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	if len(models) == 0 {
		fmt.Fprintln(out, "No Ollama models found.")
		return nil
	}
	for _, model := range models {
		marker := " "
		if model.Name == selected {
			marker = "*"
		}
		fmt.Fprintf(out, "%s %s\t%s\n", marker, model.Name, formatBytes(model.Size))
	}
	return nil
}

func cmdAsk(args []string, stdin io.Reader, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(out)
	model := fs.String("model", cfg.Model, "Ollama model name")
	ollamaURL := fs.String("ollama-url", cfg.OllamaURL, "Ollama base URL")
	timeout := fs.Float64("timeout", float64(cfg.AskTimeoutSeconds), "seconds to wait for Ollama")
	numCtx := fs.Int("num-ctx", cfg.NumCtx, "Ollama context window tokens; 0 uses the model default")
	noStream := fs.Bool("no-stream", false, "wait for the full response before printing")
	promptFile := fs.String("prompt-file", "", "read prompt from file")
	readStdin := fs.Bool("stdin", false, "read prompt from stdin")
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"no-stream": true, "stdin": true})); err != nil {
		return err
	}
	if err := validateOllamaURL(*ollamaURL); err != nil {
		return err
	}
	if err := validatePositiveSeconds("timeout", *timeout); err != nil {
		return err
	}
	if err := validateNumCtxFlag(*numCtx); err != nil {
		return err
	}
	prompt, err := promptText(stdin, fs.Args(), *promptFile, *readStdin)
	if err != nil {
		return err
	}
	if strings.TrimSpace(prompt) == "" {
		return errf("ask requires a prompt")
	}
	client := NewOllamaClient(*ollamaURL, time.Duration(*timeout*float64(time.Second)))
	models, err := client.ListModels(context.Background())
	if err != nil {
		return err
	}
	selected, err := SelectModel(*model, cfg, models)
	if err != nil {
		return err
	}
	response, err := client.GenerateWithOptions(context.Background(), GenerateOptions{
		Model:  selected,
		Prompt: prompt,
		Stream: !*noStream,
		NumCtx: *numCtx,
	}, out)
	if err != nil {
		return err
	}
	if *noStream {
		fmt.Fprintln(out, response)
	} else {
		fmt.Fprintln(out)
	}
	return nil
}

func cmdAgent(args []string, stdin io.Reader, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	flags := scanFlags{maxFileBytes: cfg.MaxFileBytes, maxChunkChars: cfg.MaxChunkChars}
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(out)
	model := fs.String("model", cfg.Model, "Ollama model name")
	ollamaURL := fs.String("ollama-url", cfg.OllamaURL, "Ollama base URL")
	timeout := fs.Float64("timeout", float64(cfg.AgentTimeoutSeconds), "seconds to wait for Ollama")
	maxContext := fs.Int("max-context-chars", cfg.MaxContextChars, "maximum repository context characters to send to the model")
	numCtx := fs.Int("num-ctx", cfg.NumCtx, "Ollama context window tokens; 0 uses the model default")
	noStream := fs.Bool("no-stream", false, "wait for the full response before printing")
	promptFile := fs.String("prompt-file", "", "read feedback prompt from file")
	readStdin := fs.Bool("stdin", false, "read feedback prompt from stdin")
	allowRemote := fs.Bool("allow-remote-ollama", false, "allow sending scanned source context to a non-local Ollama URL")
	addScanFlags(fs, &flags)
	boolFlags := map[string]bool{"no-stream": true, "stdin": true, "allow-remote-ollama": true, "allow-filesystem-fallback": true}
	if err := fs.Parse(interspersedArgs(args, boolFlags)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errf("agent requires REPO and optional feedback prompt")
	}
	if err := validateOllamaURL(*ollamaURL); err != nil {
		return err
	}
	if err := validatePositiveSeconds("timeout", *timeout); err != nil {
		return err
	}
	if err := validatePositiveInt("max-context-chars", *maxContext, maxConfigContextChars); err != nil {
		return err
	}
	if err := validateNumCtxFlag(*numCtx); err != nil {
		return err
	}
	if !*allowRemote && !isLocalOllamaURL(*ollamaURL) {
		return errf("agent refuses to send scanned source to non-local Ollama URL; use --allow-remote-ollama to override")
	}
	if err := validateScanFlags(flags); err != nil {
		return err
	}
	task, err := promptText(stdin, fs.Args()[1:], *promptFile, *readStdin)
	if err != nil {
		return err
	}
	result, chunks, err := scanAndChunk(fs.Arg(0), flags, "")
	if err != nil {
		return err
	}
	client := NewOllamaClient(*ollamaURL, time.Duration(*timeout*float64(time.Second)))
	models, err := client.ListModels(context.Background())
	if err != nil {
		return err
	}
	selected, err := SelectModel(*model, cfg, models)
	if err != nil {
		return err
	}
	prompt := BuildAgentPrompt(result, chunks, AgentPromptOptions{
		Task:            task,
		MaxContextChars: *maxContext,
	})
	fmt.Fprintf(out, "Clyde agent using local model: %s\n", selected)
	fmt.Fprintf(out, "Included files: %d, chunks: %d, prompt chars: %d\n\n", len(result.Files), len(chunks), len(prompt))
	response, err := client.GenerateWithOptions(context.Background(), GenerateOptions{
		Model:  selected,
		Prompt: prompt,
		Stream: !*noStream,
		NumCtx: *numCtx,
	}, out)
	if err != nil {
		return err
	}
	if *noStream {
		fmt.Fprintln(out, response)
	} else {
		fmt.Fprintln(out)
	}
	return nil
}
