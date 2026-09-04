package clyde

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestMakeChunksPreservesHeaders(t *testing.T) {
	result := ScanResult{
		Repo: "/tmp/example",
		Files: []FileRecord{{
			Path:   "/tmp/example/app.go",
			Rel:    "app.go",
			Size:   12,
			SHA256: "abc123",
			Text:   "package main\n",
		}},
	}

	chunks := MakeChunks(result, 100, "2026-07-21 1435 - Demo")
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Text, "Path: app.go") || !strings.Contains(chunks[0].Text, "Book: 2026-07-21 1435 - Demo") {
		t.Fatalf("missing headers: %s", chunks[0].Text)
	}
}

func TestMakeChunksDefaultsInvalidChunkSize(t *testing.T) {
	result := ScanResult{
		Repo: "/tmp/example",
		Files: []FileRecord{{
			Path:   "/tmp/example/app.go",
			Rel:    "app.go",
			Size:   12,
			SHA256: "abc123",
			Text:   "package main\n",
		}},
	}

	chunks := MakeChunks(result, 0, "")

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestSplitTextPreservesTextAcrossManySmallPieces(t *testing.T) {
	text := strings.Repeat("alpha\n", 80)

	chunks := splitText(text, 25)

	if strings.Join(chunks, "") != text {
		t.Fatalf("split text did not preserve original content")
	}
	for _, chunk := range chunks {
		if len(chunk) > 25 {
			t.Fatalf("chunk exceeded limit: %d", len(chunk))
		}
	}
}

func TestWriteBundleRecordsBookMetadata(t *testing.T) {
	dir := t.TempDir()
	manifest, err := WriteBundle(ScanResult{Repo: dir}, filepath.Join(dir, "out"), 100, "2026-07-21 1435 - Demo", "20260721-1435-demo")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Book == nil || manifest.Book.Slug != "20260721-1435-demo" {
		t.Fatalf("unexpected book metadata: %#v", manifest.Book)
	}
	data, err := os.ReadFile(filepath.Join(dir, "out", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Schema != "clyde.bundle.v1" {
		t.Fatalf("unexpected schema: %s", decoded.Schema)
	}
	if decoded.BundleSHA256 == "" || !decoded.HeuristicOnly {
		t.Fatalf("missing digest or heuristic marker: %#v", decoded)
	}
}

func TestWriteBundleRecordsDiscoveryMetadata(t *testing.T) {
	dir := t.TempDir()
	manifest, err := WriteBundle(ScanResult{
		Repo: dir,
		Discovery: ScanDiscovery{
			Method:            "git",
			GitExclusionsUsed: true,
			GitCommit:         "abc123",
			GitWorkingTree:    "dirty",
		},
	}, filepath.Join(dir, "out"), 100, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Discovery.Method != "git" || !manifest.Discovery.GitExclusionsUsed || manifest.Discovery.GitCommit != "abc123" || manifest.Discovery.GitWorkingTree != "dirty" {
		t.Fatalf("unexpected discovery metadata: %#v", manifest.Discovery)
	}
}

func TestWriteBundleWritesChunkRecords(t *testing.T) {
	dir := t.TempDir()
	text := "package main\nfunc main() {}\n"
	_, err := WriteBundle(ScanResult{
		Repo: dir,
		Files: []FileRecord{{
			Path:   filepath.Join(dir, "app.go"),
			Rel:    "app.go",
			Size:   int64(len(text)),
			SHA256: sha256HexString(text),
			Text:   text,
		}},
	}, filepath.Join(dir, "out"), 100, "", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "out", "chunks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 chunk line, got %d: %s", len(lines), string(data))
	}
	var chunk ChunkRecord
	if err := json.Unmarshal([]byte(lines[0]), &chunk); err != nil {
		t.Fatal(err)
	}
	if chunk.Path != "app.go" || chunk.ChunkTotal != 1 || !strings.Contains(chunk.Text, "Repository: ") {
		t.Fatalf("unexpected chunk: %#v", chunk)
	}
}

func TestWriteBundleRejectsFileOutputPath(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "bundle.json")
	if err := os.WriteFile(outFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := WriteBundle(ScanResult{Repo: dir}, outFile, 100, "", "")

	if err == nil || !strings.Contains(err.Error(), "out dir must be a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadBundleVerifiesDigest(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	text := "package main\n"
	manifest, err := WriteBundle(ScanResult{
		Repo: dir,
		Files: []FileRecord{{
			Path:   filepath.Join(dir, "app.go"),
			Rel:    "app.go",
			Size:   int64(len(text)),
			SHA256: sha256HexString(text),
			Text:   text,
		}},
	}, outDir, 100, "", "")
	if err != nil {
		t.Fatal(err)
	}

	bundle, err := LoadBundle(outDir)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Digest != manifest.BundleSHA256 || len(bundle.Chunks) != 1 {
		t.Fatalf("unexpected bundle: %#v", bundle)
	}

	chunkPath := filepath.Join(outDir, "chunks.jsonl")
	data, err := os.ReadFile(chunkPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chunkPath, append(data, []byte(`{"path":"extra"}`+"\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBundle(outDir); err == nil || !strings.Contains(err.Error(), "chunk count mismatch") {
		t.Fatalf("expected verification error, got %v", err)
	}
}

func TestVerifyBundleRejectsDuplicateChunkIndex(t *testing.T) {
	text := "hello world\n"
	chunk := ChunkRecord{
		Path:       "a.txt",
		SHA256:     sha256HexString(text),
		ChunkIndex: 1,
		ChunkTotal: 2,
		Text:       "Path: a.txt\n\n" + text,
	}
	chunk.ChunkSHA256 = sha256HexString(chunk.Text)
	bundle := Bundle{
		Manifest: Manifest{
			Schema:     "clyde.bundle.v1",
			FileCount:  1,
			ChunkCount: 2,
			TotalBytes: int64(len(text) * 2),
			Files: []manifestFile{{
				Path:       "a.txt",
				Size:       int64(len(text) * 2),
				SHA256:     chunk.SHA256,
				ChunkCount: 2,
			}},
			BundleSHA256: "sha256:test",
		},
		Chunks: []ChunkRecord{chunk, chunk},
	}

	err := verifyBundleStructure(bundle.Manifest, bundle.Chunks)

	if err == nil || !strings.Contains(err.Error(), "duplicate chunk index") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyBundleRejectsManifestTotalBytesMismatch(t *testing.T) {
	err := verifyBundleStructure(Manifest{
		Schema:     "clyde.bundle.v1",
		FileCount:  1,
		ChunkCount: 0,
		TotalBytes: 99,
		Files: []manifestFile{{
			Path:       "a.txt",
			Size:       0,
			SHA256:     sha256HexString(""),
			ChunkCount: 0,
		}},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), "total bytes mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyBundleRejectsSecretScanTargetMismatch(t *testing.T) {
	text := "hello\n"
	chunk := ChunkRecord{
		Path:       "a.txt",
		SHA256:     sha256HexString(text),
		ChunkIndex: 1,
		ChunkTotal: 1,
		Text:       "Path: a.txt\n\n" + text,
	}
	chunk.ChunkSHA256 = sha256HexString(chunk.Text)
	err := verifyBundleStructure(Manifest{
		Schema:     "clyde.bundle.v1",
		FileCount:  1,
		ChunkCount: 1,
		TotalBytes: int64(len(text)),
		SecretScan: &manifestScan{TargetSHA256: "sha256:wrong", Completed: true},
		Files: []manifestFile{{
			Path:       "a.txt",
			Size:       int64(len(text)),
			SHA256:     sha256HexString(text),
			ChunkCount: 1,
		}},
	}, []ChunkRecord{chunk})

	if err == nil || !strings.Contains(err.Error(), "secret scan target digest mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadBundleVerifiesSecretScanTargetDigest(t *testing.T) {
	dir := t.TempDir()
	text := "hello\n"
	result := ScanResult{
		Repo: dir,
		Files: []FileRecord{{
			Path:   filepath.Join(dir, "a.txt"),
			Rel:    "a.txt",
			Size:   int64(len(text)),
			SHA256: sha256HexString(text),
			Text:   text,
		}},
	}
	snapshotDir, targetDigest, err := writeSecretScanSnapshot(result)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(snapshotDir)
	outDir := filepath.Join(dir, "out")
	_, err = WriteBundleWithOptions(result, outDir, 100, "", "", WriteBundleOptions{
		SecretScanCommand: "scanner {repo}",
		SecretScanResult: &manifestScan{
			Command:      "scanner {repo}",
			TargetSHA256: targetDigest,
			OutputSHA256: "sha256:" + sha256HexString("clean\n"),
			Completed:    true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := LoadBundle(outDir); err != nil {
		t.Fatalf("expected secret scan target digest to verify: %v", err)
	}
}

func TestVerifyBundleRejectsInvalidManifestPathString(t *testing.T) {
	err := verifyBundleStructure(Manifest{
		Schema:     "clyde.bundle.v1",
		FileCount:  1,
		ChunkCount: 0,
		TotalBytes: 0,
		Files: []manifestFile{{
			Path:       "safe/\u202efile.go",
			Size:       0,
			SHA256:     sha256HexString(""),
			ChunkCount: 0,
		}},
	}, nil)

	if err == nil || !strings.Contains(err.Error(), "invalid file path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteBundleUsesPrivateModesAndRejectsOverwrite(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	if _, err := WriteBundle(ScanResult{Repo: dir}, outDir, 100, "", ""); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		for _, path := range []string{outDir, filepath.Join(outDir, "manifest.json"), filepath.Join(outDir, "chunks.jsonl")} {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			want := os.FileMode(0o600)
			if info.IsDir() {
				want = 0o700
			}
			if info.Mode().Perm() != want {
				t.Fatalf("%s mode=%#o want %#o", path, info.Mode().Perm(), want)
			}
		}
	}
	if _, err := WriteBundle(ScanResult{Repo: dir}, outDir, 100, "", ""); err == nil || !strings.Contains(err.Error(), "without --force") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
	if _, err := WriteBundleWithOptions(ScanResult{Repo: dir}, outDir, 100, "", "", WriteBundleOptions{Force: true}); err != nil {
		t.Fatalf("force overwrite failed: %v", err)
	}
}

func TestWriteBundleRejectsSymlinkTargets(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	if err := os.Mkdir(outDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(outDir, "chunks.jsonl")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	_, err := WriteBundleWithOptions(ScanResult{Repo: dir}, outDir, 100, "", "", WriteBundleOptions{Force: true})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink refusal, got %v", err)
	}
}

func TestWriteBundleRejectsSymlinkParentDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "redirect")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".clyde")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := WriteBundle(ScanResult{Repo: dir}, filepath.Join(link, "out"), 100, "", "")

	if err == nil || !strings.Contains(err.Error(), "symlink path component") {
		t.Fatalf("expected symlink parent refusal, got %v", err)
	}
}

func TestLoadBundleRejectsSymlinkManifest(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "out")
	if _, err := WriteBundle(ScanResult{Repo: dir}, outDir, 100, "", ""); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	target := filepath.Join(dir, "manifest-target.json")
	if err := os.Rename(manifestPath, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, manifestPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	_, err := LoadBundle(outDir)

	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink read refusal, got %v", err)
	}
}

func TestMakeChunksWithLimitRejectsTooManyChunks(t *testing.T) {
	result := ScanResult{
		Repo: "/tmp/repo",
		Files: []FileRecord{{
			Rel:    "big.txt",
			SHA256: "abc",
			Text:   strings.Repeat("x", 256),
		}},
	}

	_, err := MakeChunksWithLimit(result, 64, "", 1)

	if err == nil || !strings.Contains(err.Error(), "chunk limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSplitTextPreservesUTF8(t *testing.T) {
	text := strings.Repeat("é", 20)
	chunks := splitText(text, 5)
	if strings.Join(chunks, "") != text {
		t.Fatalf("split text did not preserve original UTF-8")
	}
	for _, chunk := range chunks {
		if !utf8.ValidString(chunk) {
			t.Fatalf("invalid UTF-8 chunk: %q", chunk)
		}
	}
}

func TestWriteBundleRecordsSkips(t *testing.T) {
	dir := t.TempDir()
	manifest, err := WriteBundle(ScanResult{
		Repo:  dir,
		Skips: []SkipRecord{{Path: "secret.env", Reason: "possible secret material"}},
	}, filepath.Join(dir, "out"), 100, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Skips) != 1 || manifest.Skips[0].Path != "secret.env" {
		t.Fatalf("unexpected skips: %#v", manifest.Skips)
	}
}
