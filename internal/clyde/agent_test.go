package clyde

import (
	"strings"
	"testing"
	"unicode/utf8"
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

func TestSelectModelUsesConfiguredDefault(t *testing.T) {
	model, err := SelectModel("", DefaultConfig(), []LocalModel{
		{Name: "gemma3:4b"},
		{Name: "qwen2.5-coder:7b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if model != "qwen2.5-coder:7b" {
		t.Fatalf("unexpected model: %s", model)
	}
}

func TestBuildAgentPromptTruncatesUTF8SafelyAndPrioritizes(t *testing.T) {
	result := ScanResult{
		Repo: "/tmp/example",
		Files: []FileRecord{
			{Rel: "z.txt", Size: 1},
			{Rel: "README.md", Size: 1},
		},
	}
	chunks := []ChunkRecord{
		{Path: "z.txt", ChunkIndex: 1, ChunkTotal: 1, Text: strings.Repeat("z", 200)},
		{Path: "README.md", ChunkIndex: 1, ChunkTotal: 1, Text: "hello ☃"},
	}

	prompt := BuildAgentPrompt(result, chunks, AgentPromptOptions{Task: "Review", MaxContextChars: 260})

	if !strings.Contains(prompt, "README.md") {
		t.Fatalf("expected prioritized README context: %s", prompt)
	}
	if !strings.Contains(prompt, "[Context truncated") {
		t.Fatalf("expected truncation marker: %s", prompt)
	}
	if !utf8.ValidString(prompt) {
		t.Fatalf("prompt is not valid UTF-8")
	}
}

func TestPrioritizeAgentChunksReusesAlreadyPrioritizedSlice(t *testing.T) {
	chunks := []ChunkRecord{
		{Path: "README.md"},
		{Path: "cmd/clyde/main.go"},
		{Path: "internal/clyde/agent.go"},
	}

	prioritized := prioritizeAgentChunks(chunks)

	if &prioritized[0] != &chunks[0] {
		t.Fatalf("expected already prioritized slice to be reused")
	}
}

func TestPrioritizeAgentChunksSortsWhenNeeded(t *testing.T) {
	chunks := []ChunkRecord{
		{Path: "z.txt"},
		{Path: "README.md"},
		{Path: "internal/clyde/agent.go"},
	}

	prioritized := prioritizeAgentChunks(chunks)

	if prioritized[0].Path != "README.md" {
		t.Fatalf("expected README first, got %#v", prioritized)
	}
	if chunks[0].Path != "z.txt" {
		t.Fatalf("expected original slice to remain unchanged, got %#v", chunks)
	}
}

func TestSafePrefixEdgeCases(t *testing.T) {
	if got := safePrefix("hello", 0); got != "" {
		t.Fatalf("expected empty prefix, got %q", got)
	}
	if got := safePrefix("hello", 5); got != "hello" {
		t.Fatalf("expected full string, got %q", got)
	}
	if got := safePrefix("snowman ☃", len("snowman ")+1); got != "snowman " {
		t.Fatalf("expected valid UTF-8 boundary, got %q", got)
	}
}
