package clyde

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewShowsIncludedFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")

	var out, errOut bytes.Buffer
	status := Main([]string{"preview", dir}, &out, &errOut)

	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), "Included files: 1") || !strings.Contains(out.String(), "app.go") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestBundleWritesManifest(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")
	outDir := filepath.Join(dir, "out")

	var out, errOut bytes.Buffer
	status := Main([]string{"bundle", dir, "--out", outDir, "--subject", "Demo Sync"}, &out, &errOut)

	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	data, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FileCount != 1 || manifest.Book == nil {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestSyncDeleteExistingSourcesRequiresNLMBackend(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")

	var out, errOut bytes.Buffer
	status := Main([]string{"sync", dir, "--notebook-id", "nb", "--approve-upload", "--delete-existing-sources"}, &out, &errOut)

	if status != 1 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(errOut.String(), "--delete-existing-sources requires --backend nlm") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestBookCommandPrintsDatedName(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"book", "Demo", "Sync"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), " - Demo Sync") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
