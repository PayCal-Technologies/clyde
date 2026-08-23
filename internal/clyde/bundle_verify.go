package clyde

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

func sha256HexString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
