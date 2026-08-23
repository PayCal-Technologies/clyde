package clyde

import (
	"os"
	"path/filepath"
	"strings"
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

func TestScanRepoExcludesClydeOutputByDefault(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")
	if err := os.MkdirAll(filepath.Join(dir, ".clyde", "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, ".clyde", "out", "chunks.jsonl"), "prior source\n")

	result, err := ScanRepo(dir, nil, nil, 250000)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Rel != "app.go" {
		t.Fatalf("unexpected files: %#v", result.Files)
	}
	if len(result.Skips) != 1 || result.Skips[0].Path != ".clyde/out/chunks.jsonl" || result.Skips[0].Reason != "excluded by glob" {
		t.Fatalf("unexpected skips: %#v", result.Skips)
	}
}

func TestScanRepoSkipsInvalidUTF8(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.txt"), []byte{0xff, 0xfe, '\n'}, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanRepo(dir, nil, nil, 250000)

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 0 {
		t.Fatalf("expected no files, got %#v", result.Files)
	}
	if len(result.Skips) != 1 || result.Skips[0].Reason != "invalid UTF-8" {
		t.Fatalf("unexpected skips: %#v", result.Skips)
	}
}

func TestScanRepoSkipsSuspiciousPathString(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "ok.txt"), "ok\n")
	mustWrite(t, filepath.Join(dir, "evil\u202egpj.txt"), "spoof\n")

	result, err := ScanRepo(dir, nil, nil, 250000)

	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Rel != "ok.txt" {
		t.Fatalf("unexpected files: %#v", result.Files)
	}
	if len(result.Skips) != 1 || !strings.Contains(result.Skips[0].Reason, "invalid path string") {
		t.Fatalf("unexpected skips: %#v", result.Skips)
	}
}

func TestScanRepoRejectsInvalidGlob(t *testing.T) {
	dir := t.TempDir()

	_, err := ScanRepo(dir, []string{"["}, nil, 250000)

	if err == nil || !strings.Contains(err.Error(), "invalid glob pattern") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScanRepoFailsClosedWhenGitDiscoveryFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "ignored-secret.txt"), "private\n")
	t.Setenv("PATH", "")

	_, err := ScanRepo(dir, nil, nil, 250000)

	if err == nil || !strings.Contains(err.Error(), "git-aware file discovery failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScanRepoRejectsFilePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.go")
	mustWrite(t, path, "package main\n")

	_, err := ScanRepo(path, nil, nil, 250000)

	if err == nil || !strings.Contains(err.Error(), "repo path is not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScanRepoRejectsInvalidMaxFileBytes(t *testing.T) {
	dir := t.TempDir()

	_, err := ScanRepo(dir, nil, nil, 0)

	if err == nil || !strings.Contains(err.Error(), "maxFileBytes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScanRepoSkipsSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	mustWrite(t, outside, "do not read\n")
	link := filepath.Join(dir, "outside.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	result, err := ScanRepo(dir, nil, nil, 250000)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 0 {
		t.Fatalf("expected no scanned files, got %#v", result.Files)
	}
	if len(result.Skips) != 1 || result.Skips[0].Reason != "symbolic link" {
		t.Fatalf("unexpected skips: %#v", result.Skips)
	}
}

func TestReadScannedFileRejectsReplacedPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	mustWrite(t, path, "first\n")
	stat, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, "second\n")

	_, _, err = readScannedFile(path, stat, 250000)

	if err == nil || !strings.Contains(err.Error(), "file changed during scan") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadScannedFileRejectsOversizedOpenedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.txt")
	mustWrite(t, path, "abcdef\n")
	stat, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = readScannedFile(path, stat, 3)

	if err == nil || !strings.Contains(err.Error(), "larger than 3 bytes") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScanRepoIncludeCanMatchNoFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")

	result, err := ScanRepo(dir, []string{"*.md"}, nil, 250000)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 0 {
		t.Fatalf("expected no files, got %#v", result.Files)
	}
	if len(result.Skips) != 1 || result.Skips[0].Reason != "not matched by include globs" {
		t.Fatalf("unexpected skips: %#v", result.Skips)
	}
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
