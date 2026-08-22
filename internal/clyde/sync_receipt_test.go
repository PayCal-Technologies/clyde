package clyde

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSyncReceiptResumeMatching(t *testing.T) {
	opts := SyncOptions{NotebookID: "nb", Backend: "mcp", BundleDigest: "sha256:abc", TitlePrefix: "Clyde - "}
	receipt := newSyncReceipt(opts)
	chunk := ChunkRecord{Path: "app.go", SHA256: "file", ChunkIndex: 1, ChunkTotal: 1, ChunkSHA256: "chunk"}
	title := "Clyde - app.go [1/1]"
	receipt.recordChunk(chunk, title, "src-1", "uploaded", nil)

	if !receipt.canResume(opts) {
		t.Fatalf("expected receipt to match")
	}
	if !receipt.uploaded(chunk, title) {
		t.Fatalf("expected chunk to be resumable")
	}
	opts.NotebookID = "other"
	if receipt.canResume(opts) {
		t.Fatalf("expected destination mismatch")
	}
}

func TestSaveSyncReceiptUsesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := newSyncReceipt(SyncOptions{NotebookID: "nb", Backend: "mcp"})
	if err := saveSyncReceipt(path, receipt); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadSyncReceipt(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Destination != "notebook_id:nb" {
		t.Fatalf("unexpected receipt: %#v", loaded)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode=%#o want 0600", info.Mode().Perm())
		}
	}
}

func TestSourceIDExtraction(t *testing.T) {
	data := []byte(`{"result":{"source":{"id":"src-1"}}}`)
	if got := sourceIDFromJSON(data); got != "src-1" {
		t.Fatalf("unexpected source id: %q", got)
	}
}
