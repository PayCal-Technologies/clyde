package clyde

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

func RunTUI(stdin io.Reader, stdout, stderr io.Writer) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(stdin)
	client := NewOllamaClient(cfg.OllamaURL, time.Duration(cfg.AskTimeoutSeconds)*time.Second)
	models, modelErr := client.ListModels(context.Background())
	selected := ""
	if modelErr == nil {
		selected, _ = SelectModel("", cfg, models)
	}

	fmt.Fprintln(stdout, "Clyde")
	fmt.Fprintln(stdout, "Local MCP and model harness")
	for {
		fmt.Fprintln(stdout)
		if modelErr != nil {
			fmt.Fprintf(stdout, "Ollama: unavailable (%v)\n", modelErr)
		} else {
			fmt.Fprintf(stdout, "Ollama: %d model(s)\n", len(models))
			for i, model := range models {
				marker := " "
				if model.Name == selected {
					marker = "*"
				}
				fmt.Fprintf(stdout, "  %s %d. %s (%s)\n", marker, i+1, model.Name, formatBytes(model.Size))
			}
		}
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, "Commands:")
		fmt.Fprintln(stdout, "  1  Ask local model")
		fmt.Fprintln(stdout, "  2  Refresh/list models")
		fmt.Fprintln(stdout, "  3  Preview current repo")
		fmt.Fprintln(stdout, "  4  Select model")
		fmt.Fprintln(stdout, "  5  Run agent on current repo")
		fmt.Fprintln(stdout, "  q  Quit")
		fmt.Fprintln(stdout)
		fmt.Fprint(stdout, "Choice: ")
		choice, _ := reader.ReadString('\n')
		switch strings.TrimSpace(choice) {
		case "1":
			if modelErr != nil {
				return modelErr
			}
			fmt.Fprintf(stdout, "Model [%s]: ", selected)
			model, _ := reader.ReadString('\n')
			model = strings.TrimSpace(model)
			if model == "" {
				model = selected
			}
			fmt.Fprint(stdout, "Prompt: ")
			prompt, _ := reader.ReadString('\n')
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				fmt.Fprintln(stderr, "prompt must not be empty")
				continue
			}
			fmt.Fprintln(stdout)
			_, err := client.Generate(context.Background(), model, prompt, true, stdout)
			fmt.Fprintln(stdout)
			if err != nil {
				fmt.Fprintf(stderr, "ollama error: %v\n", err)
			}
		case "2":
			models, modelErr = client.ListModels(context.Background())
			if modelErr == nil {
				selected, _ = SelectModel(selected, cfg, models)
			}
		case "3":
			if err := cmdPreview([]string{".", "--show-files", "12", "--show-skips", "8"}, stdout); err != nil {
				fmt.Fprintf(stderr, "preview error: %v\n", err)
			}
		case "4":
			if modelErr != nil {
				fmt.Fprintf(stderr, "ollama error: %v\n", modelErr)
				continue
			}
			fmt.Fprint(stdout, "Model name or number: ")
			choice, _ := reader.ReadString('\n')
			choice = strings.TrimSpace(choice)
			if choice == "" {
				continue
			}
			if index := modelIndex(choice, len(models)); index >= 0 {
				selected = models[index].Name
			} else {
				selected = choice
			}
		case "5":
			if modelErr != nil {
				fmt.Fprintf(stderr, "ollama error: %v\n", modelErr)
				continue
			}
			fmt.Fprint(stdout, "Agent prompt: ")
			prompt, _ := reader.ReadString('\n')
			prompt = strings.TrimSpace(prompt)
			if prompt == "" {
				prompt = "Review this repository and give concise guidance, risks, and next steps."
			}
			if err := cmdAgent([]string{".", "--model", selected, "--ollama-url", cfg.OllamaURL, "--num-ctx", itoa(int64(cfg.NumCtx)), prompt}, strings.NewReader(""), stdout); err != nil {
				fmt.Fprintf(stderr, "agent error: %v\n", err)
			}
		case "q", "quit", "exit", "":
			return nil
		default:
			fmt.Fprintf(stderr, "unknown choice: %s\n", strings.TrimSpace(choice))
		}
	}
}

func modelIndex(value string, count int) int {
	var n int
	for _, r := range value {
		if r < '0' || r > '9' {
			return -1
		}
		n = n*10 + int(r-'0')
	}
	if n <= 0 || n > count {
		return -1
	}
	return n - 1
}
