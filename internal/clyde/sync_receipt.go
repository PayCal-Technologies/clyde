package clyde

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type SyncReceipt struct {
	Schema       string              `json:"schema"`
	ClydeVersion string              `json:"clyde_version"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
	BundleDigest string              `json:"bundle_digest,omitempty"`
	Destination  string              `json:"destination"`
	Backend      string              `json:"backend"`
	TitlePrefix  string              `json:"title_prefix,omitempty"`
	Chunks       []SyncReceiptChunk  `json:"chunks"`
	Deletions    []SyncReceiptDelete `json:"deletions,omitempty"`
}

type SyncReceiptChunk struct {
	Path        string `json:"path"`
	SHA256      string `json:"sha256"`
	ChunkIndex  int    `json:"chunk_index"`
	ChunkTotal  int    `json:"chunk_total"`
	ChunkSHA256 string `json:"chunk_sha256"`
	Title       string `json:"title"`
	SourceID    string `json:"source_id,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	UploadedAt  string `json:"uploaded_at,omitempty"`
}

type SyncReceiptDelete struct {
	SourceID  string         `json:"source_id"`
	Title     string         `json:"title,omitempty"`
	Raw       map[string]any `json:"raw,omitempty"`
	Status    string         `json:"status"`
	DeletedAt string         `json:"deleted_at,omitempty"`
}

func newSyncReceipt(opts SyncOptions) SyncReceipt {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return SyncReceipt{
		Schema:       "clyde.sync_receipt.v1",
		ClydeVersion: productVersion,
		CreatedAt:    now,
		UpdatedAt:    now,
		BundleDigest: opts.BundleDigest,
		Destination:  syncDestination(opts),
		Backend:      opts.Backend,
		TitlePrefix:  opts.TitlePrefix,
	}
}

func loadSyncReceipt(path string) (SyncReceipt, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SyncReceipt{}, err
	}
	var receipt SyncReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return SyncReceipt{}, err
	}
	if receipt.Schema != "clyde.sync_receipt.v1" {
		return SyncReceipt{}, errf("unsupported sync receipt schema: %s", receipt.Schema)
	}
	return receipt, nil
}

func saveSyncReceipt(path string, receipt SyncReceipt) error {
	receipt.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writePrivateAtomic(path, append(data, '\n'))
}

func (r SyncReceipt) canResume(opts SyncOptions) bool {
	return r.BundleDigest == opts.BundleDigest &&
		r.Destination == syncDestination(opts) &&
		r.Backend == opts.Backend &&
		r.TitlePrefix == opts.TitlePrefix
}

func (r SyncReceipt) uploaded(chunk ChunkRecord, title string) bool {
	for _, entry := range r.Chunks {
		if entry.Status == "uploaded" &&
			entry.Path == chunk.Path &&
			entry.ChunkIndex == chunk.ChunkIndex &&
			entry.ChunkTotal == chunk.ChunkTotal &&
			entry.ChunkSHA256 == chunk.ChunkSHA256 &&
			entry.Title == title {
			return true
		}
	}
	return false
}

func (r *SyncReceipt) recordChunk(chunk ChunkRecord, title, sourceID, status string, err error) {
	entry := SyncReceiptChunk{
		Path:        chunk.Path,
		SHA256:      chunk.SHA256,
		ChunkIndex:  chunk.ChunkIndex,
		ChunkTotal:  chunk.ChunkTotal,
		ChunkSHA256: chunk.ChunkSHA256,
		Title:       title,
		SourceID:    sourceID,
		Status:      status,
	}
	if status == "uploaded" {
		entry.UploadedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err != nil {
		entry.Error = err.Error()
	}
	for i, existing := range r.Chunks {
		if existing.Path == entry.Path &&
			existing.ChunkIndex == entry.ChunkIndex &&
			existing.ChunkTotal == entry.ChunkTotal &&
			existing.ChunkSHA256 == entry.ChunkSHA256 &&
			existing.Title == entry.Title {
			r.Chunks[i] = entry
			return
		}
	}
	r.Chunks = append(r.Chunks, entry)
}

func (r *SyncReceipt) recordDeletions(sources []map[string]any, status string) {
	for _, source := range sources {
		id, _ := source["id"].(string)
		if id == "" {
			continue
		}
		title, _ := source["title"].(string)
		entry := SyncReceiptDelete{SourceID: id, Title: title, Raw: source, Status: status}
		if status == "deleted" {
			entry.DeletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		found := false
		for i, existing := range r.Deletions {
			if existing.SourceID == id {
				r.Deletions[i] = entry
				found = true
				break
			}
		}
		if !found {
			r.Deletions = append(r.Deletions, entry)
		}
	}
}

func syncDestination(opts SyncOptions) string {
	if opts.NotebookID != "" {
		return "notebook_id:" + opts.NotebookID
	}
	return "notebook_url:" + opts.NotebookURL
}
