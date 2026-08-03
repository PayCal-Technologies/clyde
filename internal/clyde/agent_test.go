package clyde

import (
	"strings"
	"testing"
)

func TestBuildAgentPromptIncludesTaskAndContext(t *testing.T) {
	result := ScanResult{
		Repo: "/tmp/example",
		Files: []FileRecord{{
			Rel:  "main.go",
			Size: 12,
		}},
	}
	chunks := []ChunkRecord{{Path: "main.go", ChunkIndex: 1, ChunkTotal: 1, Text: "package main\n"}}

	prompt := BuildAgentPrompt(result, chunks, AgentPromptOptions{Task: "Find risks", MaxContextChars: 1000})

	if !strings.Contains(prompt, "Find risks") {
		t.Fatalf("missing task: %s", prompt)
	}
	if !strings.Contains(prompt, "package main") {
		t.Fatalf("missing context: %s", prompt)
	}
}

func TestSelectModelPrefersQwen14B(t *testing.T) {
	model, err := SelectModel("", []LocalModel{
		{Name: "gemma3:4b"},
		{Name: "qwen2.5-coder:14b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if model != "qwen2.5-coder:14b" {
		t.Fatalf("unexpected model: %s", model)
	}
}
