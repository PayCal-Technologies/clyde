package clyde

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	minChunkBodyBytes  = 64
	maxGeneratedChunks = 200000
)

func MakeChunks(result ScanResult, maxChunkChars int, bookTitle string) []ChunkRecord {
	chunks, _ := MakeChunksWithLimit(result, maxChunkChars, bookTitle, 0)
	return chunks
}

func MakeChunksWithLimit(result ScanResult, maxChunkChars int, bookTitle string, maxChunks int) ([]ChunkRecord, error) {
	if maxChunkChars <= 0 {
		maxChunkChars = DefaultConfig().MaxChunkChars
	}
	if maxChunks <= 0 {
		maxChunks = maxGeneratedChunks
	}
	var chunks []ChunkRecord
	for _, file := range result.Files {
		total := splitTextCount(file.Text, maxChunkChars)
		if len(chunks)+total > maxChunks {
			return nil, errf("generated chunk limit exceeded; maximum is %d", maxChunks)
		}
		index := 0
		forEachSplitText(file.Text, maxChunkChars, func(piece string) {
			index++
			header := chunkHeader(result.Repo, bookTitle, file.Rel, file.SHA256, index, total)
			text := header + piece
			chunks = append(chunks, ChunkRecord{
				Path:        file.Rel,
				SHA256:      file.SHA256,
				ChunkIndex:  index,
				ChunkTotal:  total,
				ChunkSHA256: sha256HexString(text),
				Text:        text,
			})
		})
	}
	return chunks, nil
}

func chunkHeader(repo, bookTitle, path, sha256 string, index, total int) string {
	var b strings.Builder
	b.Grow(len(repo) + len(bookTitle) + len(path) + len(sha256) + 64)
	b.WriteString("Repository: ")
	b.WriteString(filepath.Base(repo))
	b.WriteByte('\n')
	if bookTitle != "" {
		b.WriteString("Book: ")
		b.WriteString(bookTitle)
		b.WriteByte('\n')
	}
	b.WriteString("Path: ")
	b.WriteString(path)
	b.WriteString("\nSHA-256: ")
	b.WriteString(sha256)
	b.WriteString("\nChunk: ")
	b.WriteString(itoa(int64(index)))
	b.WriteByte('/')
	b.WriteString(itoa(int64(total)))
	b.WriteString("\n\n")
	return b.String()
}

func splitText(text string, maxChunkChars int) []string {
	var chunks []string
	forEachSplitText(text, maxChunkChars, func(piece string) {
		chunks = append(chunks, piece)
	})
	return chunks
}

func splitTextCount(text string, maxChunkChars int) int {
	count := 0
	forEachSplitText(text, maxChunkChars, func(string) {
		count++
	})
	return count
}

func forEachSplitText(text string, maxChunkChars int, yield func(string)) {
	if maxChunkChars <= 0 {
		maxChunkChars = DefaultConfig().MaxChunkChars
	}
	if len(text) <= maxChunkChars {
		yield(text)
		return
	}
	var current strings.Builder
	current.Grow(maxChunkChars)
	for len(text) > 0 {
		next := text
		if idx := stringsIndexByteLimit(text, '\n', maxChunkChars); idx >= 0 {
			next = text[:idx+1]
		} else if len(text) > maxChunkChars {
			next = text[:safeUTF8Cut(text, maxChunkChars)]
		}
		if current.Len()+len(next) > maxChunkChars && current.Len() > 0 {
			yield(current.String())
			current.Reset()
			current.Grow(maxChunkChars)
		}
		if len(next) > maxChunkChars {
			yield(next)
		} else {
			current.WriteString(next)
		}
		text = text[len(next):]
	}
	if current.Len() > 0 {
		yield(current.String())
	}
}

func safeUTF8Cut(text string, limit int) int {
	if limit >= len(text) {
		return len(text)
	}
	if limit <= 0 {
		return 0
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	if limit == 0 {
		_, size := utf8.DecodeRuneInString(text)
		return size
	}
	return limit
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
