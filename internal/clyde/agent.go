package clyde

import (
	"sort"
	"strings"
	"unicode/utf8"
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
	if maxChars > 0 {
		grow := maxChars
		if grow > 64*1024 {
			grow = 64 * 1024
		}
		b.Grow(grow)
	}
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
	for _, chunk := range prioritizeAgentChunks(chunks) {
		if remaining <= 0 {
			break
		}
		text := chunk.Text
		if len(text) > remaining {
			text = safePrefix(text, remaining)
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

func prioritizeAgentChunks(chunks []ChunkRecord) []ChunkRecord {
	if agentChunksAlreadyPrioritized(chunks) {
		return chunks
	}
	ordered := append([]ChunkRecord(nil), chunks...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return agentPathRank(ordered[i].Path) < agentPathRank(ordered[j].Path)
	})
	return ordered
}

func agentChunksAlreadyPrioritized(chunks []ChunkRecord) bool {
	if len(chunks) < 2 {
		return true
	}
	previous := agentPathRank(chunks[0].Path)
	for _, chunk := range chunks[1:] {
		current := agentPathRank(chunk.Path)
		if current < previous {
			return false
		}
		previous = current
	}
	return true
}

func agentPathRank(path string) int {
	lower := strings.ToLower(path)
	switch {
	case lower == "readme.md":
		return 0
	case lower == "go.mod":
		return 1
	case strings.HasPrefix(lower, "cmd/"):
		return 2
	case strings.HasPrefix(lower, "internal/") && strings.HasSuffix(lower, "_test.go"):
		return 4
	case strings.HasPrefix(lower, "internal/"):
		return 3
	case strings.Contains(lower, "test"):
		return 5
	default:
		return 6
	}
}

func safePrefix(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}
