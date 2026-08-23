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
	"strings"
)

const (
	maxBundleManifestBytes = 16 * 1024 * 1024
	maxBundleChunksBytes   = 1024 * 1024 * 1024
)

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
	if err := verifyBundleStructure(manifest, bundle.Chunks); err != nil {
		return err
	}
	return nil
}

func verifyBundleStructure(manifest Manifest, chunks []ChunkRecord) error {
	totalBytes := int64(0)
	fileSHA := make(map[string]string)
	fileChunkCount := make(map[string]int)
	for _, file := range manifest.Files {
		if file.Path == "" {
			return errf("bundle manifest contains empty file path")
		}
		if err := validateRepoRelPath(file.Path); err != nil {
			return errf("bundle manifest contains invalid file path %q: %v", file.Path, err)
		}
		if _, exists := fileSHA[file.Path]; exists {
			return errf("bundle manifest contains duplicate file path: %s", file.Path)
		}
		if file.Size < 0 {
			return errf("bundle manifest contains negative file size for %s", file.Path)
		}
		fileSHA[file.Path] = file.SHA256
		fileChunkCount[file.Path] = file.ChunkCount
		totalBytes += file.Size
	}
	if totalBytes != manifest.TotalBytes {
		return errf("bundle manifest total bytes mismatch: manifest=%d files=%d", manifest.TotalBytes, totalBytes)
	}
	fileChunks := make(map[string]map[int]ChunkRecord)
	for _, chunk := range chunks {
		if chunk.Path == "" || chunk.ChunkIndex <= 0 || chunk.ChunkTotal <= 0 {
			return errf("invalid chunk metadata for %q", chunk.Path)
		}
		if err := validateRepoRelPath(chunk.Path); err != nil {
			return errf("bundle chunk contains invalid file path %q: %v", chunk.Path, err)
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
		if chunk.ChunkTotal != fileChunkCount[chunk.Path] {
			return errf("bundle chunk total mismatch for %s: manifest=%d chunk=%d", chunk.Path, fileChunkCount[chunk.Path], chunk.ChunkTotal)
		}
		if chunk.ChunkIndex > chunk.ChunkTotal {
			return errf("bundle chunk index exceeds total for %s: %d/%d", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal)
		}
		if fileChunks[chunk.Path] == nil {
			fileChunks[chunk.Path] = make(map[int]ChunkRecord)
		}
		if _, exists := fileChunks[chunk.Path][chunk.ChunkIndex]; exists {
			return errf("bundle contains duplicate chunk index for %s: %d", chunk.Path, chunk.ChunkIndex)
		}
		fileChunks[chunk.Path][chunk.ChunkIndex] = chunk
	}
	for _, file := range manifest.Files {
		chunksByIndex := fileChunks[file.Path]
		if file.ChunkCount != len(chunksByIndex) {
			return errf("bundle chunk count mismatch for %s: manifest=%d chunks=%d", file.Path, file.ChunkCount, len(chunksByIndex))
		}
		var reconstructed strings.Builder
		for i := 1; i <= file.ChunkCount; i++ {
			chunk, ok := chunksByIndex[i]
			if !ok {
				return errf("bundle missing chunk index for %s: %d", file.Path, i)
			}
			body, err := chunkBody(chunk.Text)
			if err != nil {
				return err
			}
			reconstructed.WriteString(body)
		}
		if int64(len(reconstructed.String())) != file.Size {
			return errf("bundle reconstructed size mismatch for %s: manifest=%d reconstructed=%d", file.Path, file.Size, len(reconstructed.String()))
		}
		if sha256HexString(reconstructed.String()) != file.SHA256 {
			return errf("bundle reconstructed digest mismatch for %s", file.Path)
		}
	}
	return nil
}

func chunkBody(text string) (string, error) {
	idx := strings.Index(text, "\n\n")
	if idx < 0 {
		return "", errf("bundle chunk is missing metadata header")
	}
	return text[idx+2:], nil
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
		if len(chunks) >= maxGeneratedChunks {
			return nil, errf("bundle chunk limit exceeded; maximum is %d", maxGeneratedChunks)
		}
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

func readFileLimited(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errf("file is too large; maximum is %d bytes", maxBytes)
	}
	return data, nil
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

func sha256HexString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
