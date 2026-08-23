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

func TestSyncReceiptResumeBindsDeleteExistingSources(t *testing.T) {
	opts := SyncOptions{NotebookID: "nb", Backend: "nlm", BundleDigest: "sha256:abc", DeleteExistingSources: true}
	receipt := newSyncReceipt(opts)
	if !receipt.canResume(opts) {
		t.Fatalf("expected receipt to match")
	}
	opts.DeleteExistingSources = false
	if receipt.canResume(opts) {
		t.Fatal("expected deletion setting mismatch")
	}
}

func TestDeleteExistingNLMSourcesSkipsCompletedReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := newSyncReceipt(SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true, ReceiptPath: path})
	receipt.recordDeletionPhase("completed")
	opts := SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true, ReceiptPath: path}

	if err := deleteExistingNLMSources(t.Context(), []string{"definitely-not-a-real-nlm-command"}, "nb", opts, &receipt); err != nil {
		t.Fatal(err)
	}
}

func TestRecordSyncReceiptChunkReturnsPersistenceError(t *testing.T) {
	dir := t.TempDir()
	receipt := newSyncReceipt(SyncOptions{NotebookID: "nb", Backend: "mcp"})
	chunk := ChunkRecord{Path: "app.go", SHA256: "file", ChunkIndex: 1, ChunkTotal: 1, ChunkSHA256: "chunk"}
	opts := SyncOptions{NotebookID: "nb", Backend: "mcp", ReceiptPath: dir}

	err := recordSyncReceiptChunk(opts, &receipt, chunk, "app.go [1/1]", "src", "uploaded", nil)

	if err == nil {
		t.Fatal("expected receipt persistence error")
	}
}

func TestPrepareSyncReceiptRefusesOverwriteWithoutResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	opts := SyncOptions{NotebookID: "nb", Backend: "mcp", ReceiptPath: path}
	if _, err := prepareSyncReceipt(opts); err != nil {
		t.Fatal(err)
	}

	_, err := prepareSyncReceipt(opts)

	if err == nil {
		t.Fatal("expected existing receipt error")
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
