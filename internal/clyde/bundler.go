package clyde

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type manifestFile struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	ChunkCount int    `json:"chunk_count"`
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
	Schema        string         `json:"schema"`
	ClydeVersion  string         `json:"clyde_version"`
	CreatedAt     string         `json:"created_at"`
	Repo          string         `json:"repo"`
	RepoName      string         `json:"repo_name"`
	HeuristicOnly bool           `json:"heuristic_only"`
	FileCount     int            `json:"file_count"`
	ChunkCount    int            `json:"chunk_count"`
	TotalBytes    int64          `json:"total_bytes"`
	Book          *manifestBook  `json:"book"`
	SecretScan    *manifestScan  `json:"secret_scan,omitempty"`
	Files         []manifestFile `json:"files"`
	Skips         []manifestSkip `json:"skips"`
	BundleSHA256  string         `json:"bundle_sha256,omitempty"`
}

type manifestScan struct {
	Required  bool   `json:"required"`
	Command   string `json:"command,omitempty"`
	Completed bool   `json:"completed"`
}

type WriteBundleOptions struct {
	Force             bool
	RequireSecretScan bool
	SecretScanCommand string
}

type Bundle struct {
	Manifest Manifest
	Chunks   []ChunkRecord
	Digest   string
}

func MakeChunks(result ScanResult, maxChunkChars int, bookTitle string) []ChunkRecord {
	if maxChunkChars <= 0 {
		maxChunkChars = DefaultConfig().MaxChunkChars
	}
	var chunks []ChunkRecord
	for _, file := range result.Files {
		pieces := splitText(file.Text, maxChunkChars)
		for i, piece := range pieces {
			header := chunkHeader(result.Repo, bookTitle, file.Rel, file.SHA256, i+1, len(pieces))
			text := header + piece
			chunks = append(chunks, ChunkRecord{
				Path:        file.Rel,
				SHA256:      file.SHA256,
				ChunkIndex:  i + 1,
				ChunkTotal:  len(pieces),
				ChunkSHA256: sha256HexString(text),
				Text:        text,
			})
		}
	}
	return chunks
}

func WriteBundle(result ScanResult, outDir string, maxChunkChars int, bookTitle, bookSlug string) (Manifest, error) {
	return WriteBundleWithOptions(result, outDir, maxChunkChars, bookTitle, bookSlug, WriteBundleOptions{})
}

func WriteBundleWithOptions(result ScanResult, outDir string, maxChunkChars int, bookTitle, bookSlug string, opts WriteBundleOptions) (Manifest, error) {
	if info, err := os.Lstat(outDir); err == nil && !info.IsDir() {
		return Manifest{}, errf("out dir must be a directory, not a file: %s", outDir)
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("create bundle output directory %s: %w", outDir, err)
	}
	if err := os.Chmod(outDir, 0o700); err != nil {
		return Manifest{}, fmt.Errorf("secure bundle output directory %s: %w", outDir, err)
	}
	chunks := MakeChunks(result, maxChunkChars, bookTitle)
	manifest := Manifest{
		Schema:        "clyde.bundle.v1",
		ClydeVersion:  productVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Repo:          result.Repo,
		RepoName:      filepath.Base(result.Repo),
		HeuristicOnly: true,
		FileCount:     len(result.Files),
		ChunkCount:    len(chunks),
		TotalBytes:    result.TotalBytes(),
		Files:         make([]manifestFile, 0, len(result.Files)),
		Skips:         make([]manifestSkip, 0, len(result.Skips)),
	}
	if bookTitle != "" || bookSlug != "" {
		manifest.Book = &manifestBook{Title: bookTitle, Slug: bookSlug}
	}
	if opts.RequireSecretScan || strings.TrimSpace(opts.SecretScanCommand) != "" {
		manifest.SecretScan = &manifestScan{
			Required:  opts.RequireSecretScan,
			Command:   strings.TrimSpace(opts.SecretScanCommand),
			Completed: true,
		}
	}
	chunksByFile := make(map[string]int)
	for _, chunk := range chunks {
		chunksByFile[chunk.Path]++
	}
	for _, file := range result.Files {
		manifest.Files = append(manifest.Files, manifestFile{Path: file.Rel, Size: file.Size, SHA256: file.SHA256, ChunkCount: chunksByFile[file.Rel]})
	}
	for _, skip := range result.Skips {
		manifest.Skips = append(manifest.Skips, manifestSkip{Path: skip.Path, Reason: skip.Reason})
	}
	chunkData, err := encodeChunks(chunks)
	if err != nil {
		return Manifest{}, err
	}
	digest, data, err := bundleDigestAndManifest(manifest, chunkData)
	if err != nil {
		return Manifest{}, fmt.Errorf("encode bundle manifest: %w", err)
	}
	manifest.BundleSHA256 = digest
	data, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("encode bundle manifest: %w", err)
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	chunksPath := filepath.Join(outDir, "chunks.jsonl")
	if err := refuseUnsafeBundleTarget(manifestPath, opts.Force); err != nil {
		return Manifest{}, err
	}
	if err := refuseUnsafeBundleTarget(chunksPath, opts.Force); err != nil {
		return Manifest{}, err
	}
	if err := writePrivateAtomic(manifestPath, append(data, '\n')); err != nil {
		return Manifest{}, fmt.Errorf("write bundle manifest %s: %w", manifestPath, err)
	}
	if err := writePrivateAtomic(chunksPath, chunkData); err != nil {
		_ = os.Remove(manifestPath)
		return Manifest{}, fmt.Errorf("write bundle chunks %s: %w", chunksPath, err)
	}
	return manifest, nil
}

func LoadBundle(dir string) (Bundle, error) {
	manifestPath := filepath.Join(dir, "manifest.json")
	chunksPath := filepath.Join(dir, "chunks.jsonl")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return Bundle{}, fmt.Errorf("read bundle manifest %s: %w", manifestPath, err)
	}
	chunksData, err := os.ReadFile(chunksPath)
	if err != nil {
		return Bundle{}, fmt.Errorf("read bundle chunks %s: %w", chunksPath, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return Bundle{}, fmt.Errorf("decode bundle manifest %s: %w", manifestPath, err)
	}
	chunks, err := decodeChunks(chunksData)
	if err != nil {
		return Bundle{}, fmt.Errorf("decode bundle chunks %s: %w", chunksPath, err)
	}
	bundle := Bundle{Manifest: manifest, Chunks: chunks, Digest: manifest.BundleSHA256}
	if err := VerifyBundle(bundle, chunksData); err != nil {
		return Bundle{}, err
	}
	return bundle, nil
}

func VerifyBundle(bundle Bundle, chunksData []byte) error {
	manifest := bundle.Manifest
	if manifest.Schema != "clyde.bundle.v1" {
		return errf("unsupported bundle schema: %s", manifest.Schema)
	}
	if manifest.FileCount != len(manifest.Files) {
		return errf("bundle manifest file count mismatch: manifest=%d files=%d", manifest.FileCount, len(manifest.Files))
	}
	if manifest.ChunkCount != len(bundle.Chunks) {
		return errf("bundle manifest chunk count mismatch: manifest=%d chunks=%d", manifest.ChunkCount, len(bundle.Chunks))
	}
	if chunksData == nil {
		var err error
		chunksData, err = encodeChunks(bundle.Chunks)
		if err != nil {
			return err
		}
	}
	if manifest.BundleSHA256 == "" {
		return errf("bundle manifest is missing bundle_sha256")
	}
	digest, _, err := bundleDigestAndManifest(manifest, chunksData)
	if err != nil {
		return err
	}
	if digest != manifest.BundleSHA256 {
		return errf("bundle digest mismatch: manifest=%s actual=%s", manifest.BundleSHA256, digest)
	}
	fileChunks := make(map[string]int)
	fileSHA := make(map[string]string)
	for _, file := range manifest.Files {
		fileSHA[file.Path] = file.SHA256
	}
	for _, chunk := range bundle.Chunks {
		if chunk.Path == "" || chunk.ChunkIndex <= 0 || chunk.ChunkTotal <= 0 {
			return errf("invalid chunk metadata for %q", chunk.Path)
		}
		expectedFileSHA, ok := fileSHA[chunk.Path]
		if !ok {
			return errf("bundle chunk references unknown file: %s", chunk.Path)
		}
		if chunk.SHA256 != expectedFileSHA {
			return errf("bundle chunk file digest mismatch for %s", chunk.Path)
		}
		if chunk.ChunkSHA256 == "" {
			return errf("bundle chunk is missing chunk_sha256 for %s %d/%d", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal)
		}
		if actual := sha256HexString(chunk.Text); actual != chunk.ChunkSHA256 {
			return errf("bundle chunk digest mismatch for %s %d/%d", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal)
		}
		fileChunks[chunk.Path]++
	}
	for _, file := range manifest.Files {
		if file.ChunkCount != fileChunks[file.Path] {
			return errf("bundle chunk count mismatch for %s: manifest=%d chunks=%d", file.Path, file.ChunkCount, fileChunks[file.Path])
		}
	}
	return nil
}

func encodeChunks(chunks []ChunkRecord) ([]byte, error) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	encoder := json.NewEncoder(writer)
	for _, chunk := range chunks {
		if err := encoder.Encode(chunk); err != nil {
			return nil, fmt.Errorf("encode bundle chunk %s %d/%d: %w", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal, err)
		}
	}
	if err := writer.Flush(); err != nil {
		return nil, fmt.Errorf("flush bundle chunks: %w", err)
	}
	return buf.Bytes(), nil
}

func decodeChunks(data []byte) ([]ChunkRecord, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var chunks []ChunkRecord
	for {
		var chunk ChunkRecord
		if err := decoder.Decode(&chunk); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

func bundleDigestAndManifest(manifest Manifest, chunksData []byte) (string, []byte, error) {
	manifest.BundleSHA256 = ""
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(append(append([]byte{}, data...), chunksData...))
	return "sha256:" + hex.EncodeToString(sum[:]), data, nil
}

func refuseUnsafeBundleTarget(path string, force bool) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errf("refusing to overwrite symlink bundle file: %s", path)
	}
	if !force {
		return errf("refusing to overwrite existing bundle file without --force: %s", path)
	}
	if !info.Mode().IsRegular() {
		return errf("refusing to overwrite non-regular bundle file: %s", path)
	}
	return nil
}

func writePrivateAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	ok = true
	return nil
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
	if maxChunkChars <= 0 {
		maxChunkChars = DefaultConfig().MaxChunkChars
	}
	if len(text) <= maxChunkChars {
		return []string{text}
	}
	var chunks []string
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
			chunks = append(chunks, current.String())
			current.Reset()
			current.Grow(maxChunkChars)
		}
		if len(next) > maxChunkChars {
			chunks = append(chunks, next)
		} else {
			current.WriteString(next)
		}
		text = text[len(next):]
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
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

func sha256HexString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
