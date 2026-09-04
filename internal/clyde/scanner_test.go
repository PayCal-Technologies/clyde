package clyde

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	if len(result.Skips) != 0 {
		t.Fatalf("unexpected skips: %#v", result.Skips)
	}
}

func TestScanRepoExcludesNestedFolders(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")
	if err := os.MkdirAll(filepath.Join(dir, "packages", "api", "generated"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "packages", "web", "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "packages", "api", "generated", "client.go"), "package generated\n")
	mustWrite(t, filepath.Join(dir, "packages", "web", "dist", "bundle.js"), "console.log('built')\n")

	result, err := ScanRepoWithOptions(dir, ScanOptions{
		ExcludeFolders: []string{"generated"},
		MaxFileBytes:   250000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Files) != 1 || result.Files[0].Rel != "app.go" {
		t.Fatalf("unexpected files: %#v", result.Files)
	}
	if len(result.Skips) != 0 {
		t.Fatalf("filesystem discovery should prune excluded folders before skips, got %#v", result.Skips)
	}
}

func TestScanRepoRejectsInvalidExcludeFolder(t *testing.T) {
	dir := t.TempDir()

	_, err := ScanRepoWithOptions(dir, ScanOptions{
		ExcludeFolders: []string{"../outside"},
		MaxFileBytes:   250000,
	})

	if err == nil || !strings.Contains(err.Error(), "exclude folder") {
		t.Fatalf("unexpected error: %v", err)
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

func TestScanRepoAllowsExplicitFilesystemFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")
	t.Setenv("PATH", "")

	result, err := ScanRepoWithOptions(dir, ScanOptions{
		MaxFileBytes:            250000,
		AllowFilesystemFallback: true,
	})

	if err != nil {
		t.Fatal(err)
	}
	if result.Discovery.Method != "filesystem-fallback" || result.Discovery.GitExclusionsUsed {
		t.Fatalf("unexpected discovery: %#v", result.Discovery)
	}
	if result.Discovery.GitError == "" {
		t.Fatalf("expected recorded Git error: %#v", result.Discovery)
	}
	if len(result.Files) != 1 || result.Files[0].Rel != "app.go" {
		t.Fatalf("unexpected files: %#v", result.Files)
	}
}

func TestFilesystemCandidatePathsReportsTruncation(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "a\n")
	mustWrite(t, filepath.Join(dir, "b.txt"), "b\n")

	paths, truncated, err := filesystemCandidatePathsLimit(dir, 1, defaultExcludeFolders)

	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Fatal("expected filesystem discovery truncation")
	}
	if len(paths) != 1 {
		t.Fatalf("expected one collected path, got %d: %#v", len(paths), paths)
	}
}

func TestScanRepoUsesGitDiscoveryFromSubdirectory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	runTestGit(t, dir, "init")
	mustWrite(t, filepath.Join(dir, ".gitignore"), "component/ignored.txt\n")
	if err := os.MkdirAll(filepath.Join(dir, "component"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "component", "app.go"), "package main\n")
	mustWrite(t, filepath.Join(dir, "component", "ignored.txt"), "ignored\n")
	mustWrite(t, filepath.Join(dir, "outside.go"), "package outside\n")

	result, err := ScanRepo(filepath.Join(dir, "component"), nil, nil, 250000)

	if err != nil {
		t.Fatal(err)
	}
	if result.Discovery.Method != "git" || !result.Discovery.GitExclusionsUsed {
		t.Fatalf("expected git discovery from subdirectory, got %#v", result.Discovery)
	}
	if len(result.Files) != 1 || result.Files[0].Rel != "app.go" {
		t.Fatalf("unexpected files: %#v", result.Files)
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
	if runtime.GOOS == "windows" {
		t.Skip("file replacement identity checks are not deterministic on Windows runners")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	replacement := filepath.Join(dir, "replacement.txt")
	mustWrite(t, path, "first\n")
	stat, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, replacement, "second\n")
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}

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

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}
