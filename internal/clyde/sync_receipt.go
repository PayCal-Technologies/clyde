package clyde

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type SyncReceipt struct {
	Schema                string              `json:"schema"`
	ClydeVersion          string              `json:"clyde_version"`
	CreatedAt             string              `json:"created_at"`
	UpdatedAt             string              `json:"updated_at"`
	BundleDigest          string              `json:"bundle_digest,omitempty"`
	Destination           string              `json:"destination"`
	Backend               string              `json:"backend"`
	BackendIdentity       SyncBackendIdentity `json:"backend_identity"`
	TitlePrefix           string              `json:"title_prefix,omitempty"`
	DeleteExistingSources bool                `json:"delete_existing_sources,omitempty"`
	DeletionPhase         string              `json:"deletion_phase,omitempty"`
	Chunks                []SyncReceiptChunk  `json:"chunks"`
	Deletions             []SyncReceiptDelete `json:"deletions,omitempty"`
}

type SyncBackendIdentity struct {
	Command          []string `json:"command,omitempty"`
	ResolvedPath     string   `json:"resolved_path,omitempty"`
	ExecutableSHA256 string   `json:"executable_sha256,omitempty"`
	Package          string   `json:"package,omitempty"`
	Runtime          string   `json:"runtime,omitempty"`
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
		Schema:                "clyde.sync_receipt.v1",
		ClydeVersion:          productVersion,
		CreatedAt:             now,
		UpdatedAt:             now,
		BundleDigest:          opts.BundleDigest,
		Destination:           syncDestination(opts),
		Backend:               opts.Backend,
		BackendIdentity:       syncBackendIdentity(opts),
		TitlePrefix:           opts.TitlePrefix,
		DeleteExistingSources: opts.DeleteExistingSources,
	}
}

func (r SyncReceipt) canResume(opts SyncOptions) bool {
	return r.BundleDigest == opts.BundleDigest &&
		r.Destination == syncDestination(opts) &&
		r.Backend == opts.Backend &&
		r.TitlePrefix == opts.TitlePrefix &&
		r.DeleteExistingSources == opts.DeleteExistingSources &&
		r.backendIdentityCompatible(opts)
}

func (r SyncReceipt) uploaded(chunk ChunkRecord, title string) bool {
	status, ok := r.chunkStatus(chunk, title)
	return ok && status == "uploaded"
}

func (r SyncReceipt) unresolved(chunk ChunkRecord, title string) (string, bool) {
	status, ok := r.chunkStatus(chunk, title)
	if !ok {
		return "", false
	}
	return status, status == "pending" || status == "ambiguous"
}

func (r SyncReceipt) chunkStatus(chunk ChunkRecord, title string) (string, bool) {
	for _, entry := range r.Chunks {
		if entry.Path == chunk.Path &&
			entry.ChunkIndex == chunk.ChunkIndex &&
			entry.ChunkTotal == chunk.ChunkTotal &&
			entry.ChunkSHA256 == chunk.ChunkSHA256 &&
			entry.Title == title {
			return entry.Status, true
		}
	}
	return "", false
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

func (r SyncReceipt) deletionCompleted() bool {
	return r.DeletionPhase == "completed"
}

func (r SyncReceipt) deletionPlanned() bool {
	return r.DeletionPhase == "planned"
}

func (r *SyncReceipt) recordDeletionPhase(phase string) {
	r.DeletionPhase = phase
}

func (r SyncReceipt) plannedDeletionSources() []map[string]any {
	sources := make([]map[string]any, 0, len(r.Deletions))
	for _, deletion := range r.Deletions {
		if deletion.SourceID == "" || deletion.Status == "deleted" {
			continue
		}
		source := deletion.Raw
		if source == nil {
			source = map[string]any{}
		}
		source["id"] = deletion.SourceID
		if deletion.Title != "" {
			source["title"] = deletion.Title
		}
		sources = append(sources, source)
	}
	return sources
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

func syncBackendIdentity(opts SyncOptions) SyncBackendIdentity {
	command := opts.MCPCommand
	if opts.Backend == "nlm" {
		command = opts.NLMCommand
	}
	identity := SyncBackendIdentity{
		Command: append([]string{}, command...),
		Runtime: runtime.GOOS + "/" + runtime.GOARCH,
	}
	if len(command) == 0 {
		return identity
	}
	if path, err := exec.LookPath(command[0]); err == nil {
		identity.ResolvedPath = path
		if digest, err := fileSHA256(path); err == nil {
			identity.ExecutableSHA256 = digest
		}
	}
	identity.Package = backendPackage(command)
	return identity
}

func (r SyncReceipt) backendIdentityCompatible(opts SyncOptions) bool {
	if len(r.BackendIdentity.Command) == 0 {
		return true
	}
	want := syncBackendIdentity(opts).Command
	if len(r.BackendIdentity.Command) != len(want) {
		return false
	}
	for i, part := range r.BackendIdentity.Command {
		if part != want[i] {
			return false
		}
	}
	return true
}

func backendPackage(command []string) string {
	for _, arg := range command {
		if strings.Contains(arg, "@") && !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
