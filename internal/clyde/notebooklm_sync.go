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
	receipt, err := prepareSyncReceipt(opts)
	if err != nil {
		return 0, err
	}
	if opts.Backend == "nlm" {
		return syncChunksNLM(ctx, chunks, opts, receipt)
	}
	return syncChunksMCP(ctx, chunks, opts, receipt)
}
