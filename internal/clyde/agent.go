package clyde

import (
	"strings"
)

type AgentPromptOptions struct {
	Task            string
	MaxContextChars int
}

func BuildAgentPrompt(result ScanResult, chunks []ChunkRecord, opts AgentPromptOptions) string {
	task := strings.TrimSpace(opts.Task)
	if task == "" {
		task = "Review this repository context and give concise engineering guidance, risks, and next steps."
	}
	maxChars := opts.MaxContextChars
	if maxChars <= 0 {
		maxChars = 24000
	}
	var b strings.Builder
	b.WriteString("You are Clyde, a local coding feedback agent running through Ollama.\n")
	b.WriteString("Give direct, practical engineering feedback. Prioritize correctness, missing tests, and operational risks.\n\n")
	b.WriteString("User task:\n")
	b.WriteString(task)
	b.WriteString("\n\nRepository:\n")
	b.WriteString(result.Repo)
	b.WriteString("\n\nIncluded files:\n")
	for _, file := range result.Files {
		b.WriteString("- ")
		b.WriteString(file.Rel)
		b.WriteString(" (")
		b.WriteString(formatBytes(file.Size))
		b.WriteString(")\n")
	}
	if len(result.Skips) > 0 {
		b.WriteString("\nSkipped file count: ")
		b.WriteString(itoa(int64(len(result.Skips))))
		b.WriteString("\n")
	}
	b.WriteString("\nContext chunks:\n")
	remaining := maxChars - b.Len()
	for _, chunk := range chunks {
		if remaining <= 0 {
			break
		}
		text := chunk.Text
		if len(text) > remaining {
			text = text[:remaining]
		}
		b.WriteString("\n--- ")
		b.WriteString(chunk.Path)
		b.WriteString(" [")
		b.WriteString(itoa(int64(chunk.ChunkIndex)))
		b.WriteString("/")
		b.WriteString(itoa(int64(chunk.ChunkTotal)))
		b.WriteString("] ---\n")
		b.WriteString(text)
		remaining = maxChars - b.Len()
	}
	if remaining <= 0 {
		b.WriteString("\n\n[Context truncated to fit Clyde's local prompt budget.]\n")
	}
	return b.String()
}
