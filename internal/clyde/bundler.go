package clyde

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type manifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type manifestSkip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type manifestBook struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type Manifest struct {
	Schema     string         `json:"schema"`
	CreatedAt  string         `json:"created_at"`
	Repo       string         `json:"repo"`
	RepoName   string         `json:"repo_name"`
	FileCount  int            `json:"file_count"`
	ChunkCount int            `json:"chunk_count"`
	TotalBytes int64          `json:"total_bytes"`
	Book       *manifestBook  `json:"book"`
	Files      []manifestFile `json:"files"`
	Skips      []manifestSkip `json:"skips"`
}

func MakeChunks(result ScanResult, maxChunkChars int, bookTitle string) []ChunkRecord {
	if maxChunkChars <= 0 {
		maxChunkChars = DefaultConfig().MaxChunkChars
	}
	var chunks []ChunkRecord
	for _, file := range result.Files {
		pieces := splitText(file.Text, maxChunkChars)
		for i, piece := range pieces {
			header := "Repository: " + filepath.Base(result.Repo) + "\n"
			if bookTitle != "" {
				header += "Book: " + bookTitle + "\n"
			}
			header += "Path: " + file.Rel + "\nSHA-256: " + file.SHA256 + "\nChunk: " + itoa(int64(i+1)) + "/" + itoa(int64(len(pieces))) + "\n\n"
			chunks = append(chunks, ChunkRecord{
				Path:       file.Rel,
				SHA256:     file.SHA256,
				ChunkIndex: i + 1,
				ChunkTotal: len(pieces),
				Text:       header + piece,
			})
		}
	}
	return chunks
}

func WriteBundle(result ScanResult, outDir string, maxChunkChars int, bookTitle, bookSlug string) (Manifest, error) {
	if info, err := os.Stat(outDir); err == nil && !info.IsDir() {
		return Manifest{}, errf("out dir must be a directory, not a file: %s", outDir)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Manifest{}, err
	}
	chunks := MakeChunks(result, maxChunkChars, bookTitle)
	manifest := Manifest{
		Schema:     "clyde.bundle.v1",
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		Repo:       result.Repo,
		RepoName:   filepath.Base(result.Repo),
		FileCount:  len(result.Files),
		ChunkCount: len(chunks),
		TotalBytes: result.TotalBytes(),
	}
	if bookTitle != "" || bookSlug != "" {
		manifest.Book = &manifestBook{Title: bookTitle, Slug: bookSlug}
	}
	for _, file := range result.Files {
		manifest.Files = append(manifest.Files, manifestFile{Path: file.Rel, Size: file.Size, SHA256: file.SHA256})
	}
	for _, skip := range result.Skips {
		manifest.Skips = append(manifest.Skips, manifestSkip{Path: skip.Path, Reason: skip.Reason})
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), append(data, '\n'), 0o644); err != nil {
		return Manifest{}, err
	}
	file, err := os.Create(filepath.Join(outDir, "chunks.jsonl"))
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	for _, chunk := range chunks {
		if err := encoder.Encode(chunk); err != nil {
			return Manifest{}, err
		}
	}
	return manifest, writer.Flush()
}

func splitText(text string, maxChunkChars int) []string {
	if maxChunkChars <= 0 {
		maxChunkChars = DefaultConfig().MaxChunkChars
	}
	if len(text) <= maxChunkChars {
		return []string{text}
	}
	var chunks []string
	var current string
	for len(text) > 0 {
		next := text
		if idx := stringsIndexByteLimit(text, '\n', maxChunkChars); idx >= 0 {
			next = text[:idx+1]
		} else if len(text) > maxChunkChars {
			next = text[:maxChunkChars]
		}
		if len(current)+len(next) > maxChunkChars && current != "" {
			chunks = append(chunks, current)
			current = ""
		}
		if len(next) > maxChunkChars {
			chunks = append(chunks, next)
		} else {
			current += next
		}
		text = text[len(next):]
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	return chunks
}

func stringsIndexByteLimit(value string, needle byte, limit int) int {
	max := limit
	if len(value) < max {
		max = len(value)
	}
	for i := max - 1; i >= 0; i-- {
		if value[i] == needle {
			return i
		}
	}
	return -1
}
