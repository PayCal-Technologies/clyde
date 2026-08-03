package clyde

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
