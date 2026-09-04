package clyde

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
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

func TestSyncReceiptPendingChunkIsUnresolved(t *testing.T) {
	receipt := newSyncReceipt(SyncOptions{NotebookID: "nb", Backend: "mcp"})
	chunk := ChunkRecord{Path: "app.go", SHA256: "file", ChunkIndex: 1, ChunkTotal: 1, ChunkSHA256: "chunk"}
	title := "app.go [1/1]"
	receipt.recordChunk(chunk, title, "", "pending", nil)

	status, ok := receipt.unresolved(chunk, title)

	if !ok || status != "pending" {
		t.Fatalf("expected unresolved pending chunk, got status=%q ok=%v", status, ok)
	}
	if receipt.uploaded(chunk, title) {
		t.Fatal("pending chunk must not be treated as uploaded")
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

func TestSyncReceiptRecordsBackendIdentity(t *testing.T) {
	opts := SyncOptions{NotebookID: "nb", Backend: "mcp", MCPCommand: defaultMCPCommand}
	receipt := newSyncReceipt(opts)

	if len(receipt.BackendIdentity.Command) != len(defaultMCPCommand) {
		t.Fatalf("missing backend command identity: %#v", receipt.BackendIdentity)
	}
	if receipt.BackendIdentity.Package != "notebooklm-mcp@2.0.0" {
		t.Fatalf("unexpected backend package: %#v", receipt.BackendIdentity)
	}
	if receipt.BackendIdentity.Runtime == "" {
		t.Fatalf("missing runtime identity: %#v", receipt.BackendIdentity)
	}
	if !slices.Contains(receipt.BackendIdentity.EnvContract, "NOTEBOOKLM_TRANSPORT=stdio") {
		t.Fatalf("missing MCP environment contract: %#v", receipt.BackendIdentity)
	}
}

func TestSyncReceiptResumeBindsBackendCommandWhenRecorded(t *testing.T) {
	opts := SyncOptions{NotebookID: "nb", Backend: "mcp", MCPCommand: []string{"npx", "-y", "notebooklm-mcp@2.0.0"}}
	receipt := newSyncReceipt(opts)
	opts.MCPCommand = []string{"npx", "-y", "notebooklm-mcp@2.1.0"}

	if receipt.canResume(opts) {
		t.Fatal("expected backend command mismatch")
	}
}

func TestSyncReceiptResumeBindsRecordedBackendDigest(t *testing.T) {
	opts := SyncOptions{NotebookID: "nb", Backend: "mcp", MCPCommand: []string{"npx", "-y", "notebooklm-mcp@2.0.0"}}
	receipt := newSyncReceipt(opts)
	receipt.BackendIdentity.ExecutableSHA256 = "sha256:other"

	if receipt.canResume(opts) {
		t.Fatal("expected backend executable digest mismatch")
	}
}

func TestDeleteExistingNLMSourcesSkipsCompletedReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := newSyncReceipt(SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true, ReceiptPath: path})
	receipt.recordDeletionPhase("completed")
	opts := SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true, ReceiptPath: path, RequestTimeout: 10 * time.Second}

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

func TestRecordUploadedSyncReceiptChunkMarksAmbiguousAfterUploadedSaveFailure(t *testing.T) {
	dir := t.TempDir()
	opts := SyncOptions{NotebookID: "nb", Backend: "mcp", ReceiptPath: dir}
	receipt := newSyncReceipt(opts)
	chunk := ChunkRecord{Path: "app.go", SHA256: "file", ChunkIndex: 1, ChunkTotal: 1, ChunkSHA256: "chunk"}
	title := "app.go [1/1]"

	err := recordUploadedSyncReceiptChunk(opts, &receipt, chunk, title, "src-1")

	if err == nil || !strings.Contains(err.Error(), "remote upload succeeded") {
		t.Fatalf("unexpected error: %v", err)
	}
	if status, ok := receipt.chunkStatus(chunk, title); !ok || status != "ambiguous" {
		t.Fatalf("expected in-memory ambiguous status, got status=%q ok=%v receipt=%#v", status, ok, receipt.Chunks)
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

func TestPrepareSyncReceiptResumeRequiresExistingReceipt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	opts := SyncOptions{NotebookID: "nb", Backend: "mcp", ReceiptPath: path, Resume: true}

	_, err := prepareSyncReceipt(opts)

	if err == nil || !strings.Contains(err.Error(), "--resume requires an existing sync receipt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncChunksRequiresReceiptForDestructiveDelete(t *testing.T) {
	_, err := SyncChunks(t.Context(), nil, SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true})

	if err == nil || !strings.Contains(err.Error(), "--delete-existing-sources requires --receipt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteExistingNLMSourcesUsesPlannedInventoryOnResume(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "nlm.log")
	t.Setenv("CLYDE_NLM_HELPER", "1")
	t.Setenv("CLYDE_NLM_HELPER_LOG", logPath)
	t.Setenv("CLYDE_NLM_HELPER_LIST", `[{"id":"src-a","title":"A"},{"id":"src-b","title":"B"},{"id":"src-c","title":"new"}]`)
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := newSyncReceipt(SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true, ReceiptPath: path})
	receipt.recordDeletionPhase("planned")
	receipt.recordDeletions([]map[string]any{
		{"id": "src-a", "title": "A"},
		{"id": "src-b", "title": "B"},
	}, "planned")
	opts := SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true, ReceiptPath: path, RequestTimeout: 10 * time.Second}

	if err := deleteExistingNLMSources(t.Context(), []string{os.Args[0], "-test.run=TestNLMHelperProcess", "--"}, "nb", opts, &receipt); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(data))
	if !slices.Contains(fields, "src-a") || !slices.Contains(fields, "src-b") {
		t.Fatalf("expected planned IDs in delete command: %q", data)
	}
	if slices.Contains(fields, "src-c") {
		t.Fatalf("unexpected fresh listing delete target: %q", data)
	}
}

func TestDeleteExistingNLMSourcesReconcilesMissingPlannedIDs(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "nlm.log")
	t.Setenv("CLYDE_NLM_HELPER", "1")
	t.Setenv("CLYDE_NLM_HELPER_LOG", logPath)
	t.Setenv("CLYDE_NLM_HELPER_LIST", `[{"id":"src-b","title":"B"}]`)
	path := filepath.Join(t.TempDir(), "receipt.json")
	receipt := newSyncReceipt(SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true, ReceiptPath: path})
	receipt.recordDeletionPhase("planned")
	receipt.recordDeletions([]map[string]any{
		{"id": "src-a", "title": "A"},
		{"id": "src-b", "title": "B"},
	}, "planned")
	opts := SyncOptions{NotebookID: "nb", Backend: "nlm", DeleteExistingSources: true, ReceiptPath: path, RequestTimeout: 10 * time.Second}

	if err := deleteExistingNLMSources(t.Context(), []string{os.Args[0], "-test.run=TestNLMHelperProcess", "--"}, "nb", opts, &receipt); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(data))
	if slices.Contains(fields, "src-a") {
		t.Fatalf("missing planned ID should not be retried: %q", data)
	}
	if !slices.Contains(fields, "src-b") {
		t.Fatalf("remaining planned ID should be retried: %q", data)
	}
	for _, deletion := range receipt.Deletions {
		if deletion.SourceID == "src-a" && deletion.Status != "deleted" {
			t.Fatalf("missing source should be marked deleted: %#v", receipt.Deletions)
		}
	}
}

func TestNLMHelperProcess(t *testing.T) {
	if os.Getenv("CLYDE_NLM_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) > 0 {
		args = args[1:]
	}
	logPath := os.Getenv("CLYDE_NLM_HELPER_LOG")
	if len(args) >= 2 && args[0] == "login" && args[1] == "--check" {
		_ = appendTestLog(logPath, "login-checked\n")
		os.Exit(0)
	}
	if len(args) >= 2 && args[0] == "source" && args[1] == "list" {
		_ = appendTestLog(logPath, "list-called\n")
		payload := os.Getenv("CLYDE_NLM_HELPER_LIST")
		if payload == "" {
			payload = `[{"id":"src-c","title":"new"}]`
		}
		_, _ = os.Stdout.WriteString(payload)
		os.Exit(0)
	}
	if len(args) >= 2 && args[0] == "source" && args[1] == "delete" {
		_ = appendTestLog(logPath, strings.Join(args, " ")+"\n")
		_, _ = os.Stdout.WriteString(`{}`)
		os.Exit(0)
	}
	os.Exit(2)
}

func appendTestLog(path, text string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(text)
	return err
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

func TestSaveSyncReceiptRejectsSymlinkParentDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "redirect")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".clyde")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	err := saveSyncReceipt(filepath.Join(link, "receipt.json"), newSyncReceipt(SyncOptions{NotebookID: "nb", Backend: "mcp"}))

	if err == nil || !strings.Contains(err.Error(), "symlink path component") {
		t.Fatalf("expected symlink parent refusal, got %v", err)
	}
}

func TestSourceIDExtraction(t *testing.T) {
	data := []byte(`{"result":{"source":{"id":"src-1"}}}`)
	if got := sourceIDFromJSON(data); got != "src-1" {
		t.Fatalf("unexpected source id: %q", got)
	}
}
