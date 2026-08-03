package clyde

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRepoSkipsLikelySecret(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ok.go"), "package main\n")
	mustWrite(t, filepath.Join(dir, "secret.env"), "API_KEY='abcdefghijklmnopqrstuvwxyz123456'\n")

	result, err := ScanRepo(dir, nil, nil, 250000)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Rel != "ok.go" {
		t.Fatalf("unexpected files: %#v", result.Files)
	}
	if len(result.Skips) != 1 || result.Skips[0].Reason != "possible secret material" {
		t.Fatalf("unexpected skips: %#v", result.Skips)
	}
}

func TestScanRepoRespectsExclude(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")
	mustWrite(t, filepath.Join(dir, "notes.md"), "# Notes\n")

	result, err := ScanRepo(dir, nil, []string{"*.md"}, 250000)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Rel != "app.go" {
		t.Fatalf("unexpected files: %#v", result.Files)
	}
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
