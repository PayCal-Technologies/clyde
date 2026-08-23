package clyde

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	chunks, err := MakeChunksWithLimit(result, maxChunkChars, bookTitle, maxGeneratedChunks)
	if err != nil {
		return Manifest{}, err
	}
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
	manifestData, err := readFileLimited(manifestPath, maxBundleManifestBytes)
	if err != nil {
		return Bundle{}, fmt.Errorf("read bundle manifest %s: %w", manifestPath, err)
	}
	chunksData, err := readFileLimited(chunksPath, maxBundleChunksBytes)
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
