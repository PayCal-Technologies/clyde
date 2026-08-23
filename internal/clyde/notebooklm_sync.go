package clyde

import (
	"context"
	"time"
)

type SyncOptions struct {
	NotebookID            string
	NotebookURL           string
	Backend               string
	BundleDigest          string
	ReceiptPath           string
	Resume                bool
	MCPCommand            []string
	NLMCommand            []string
	DeleteExistingSources bool
	RequestTimeout        time.Duration
	Progress              ProgressSink
	JobID                 string
	TitlePrefix           string
}

func SyncChunks(ctx context.Context, chunks []ChunkRecord, opts SyncOptions) (int, error) {
	if opts.NotebookID == "" && opts.NotebookURL == "" {
		return 0, errf("sync requires notebook-id or notebook-url")
	}
	if opts.JobID == "" {
		opts.JobID = "sync"
	}
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = 120 * time.Second
	}
	if opts.Backend == "" {
		opts.Backend = "mcp"
	}
	if opts.Backend == "mcp" && len(opts.MCPCommand) == 0 {
		opts.MCPCommand = append([]string{}, defaultMCPCommand...)
	}
	if opts.Backend == "nlm" && len(opts.NLMCommand) == 0 {
		opts.NLMCommand = []string{"nlm"}
	}
	if opts.DeleteExistingSources && opts.ReceiptPath == "" {
		return 0, errf("--delete-existing-sources requires --receipt")
	}
	receipt, err := prepareSyncReceipt(opts)
	if err != nil {
		return 0, err
	}
	if opts.Backend == "nlm" {
		return syncChunksNLM(ctx, chunks, opts, receipt)
	}
	return syncChunksMCP(ctx, chunks, opts, receipt)
}
