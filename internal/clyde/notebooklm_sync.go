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
	HeartbeatInterval     time.Duration
}

type SyncPlan struct {
	Uploads []string
	Deletes []map[string]any
}

// PlanSync checks the selected backend and returns the exact remote changes a sync would make.
func PlanSync(ctx context.Context, chunks []ChunkRecord, opts SyncOptions) (SyncPlan, error) {
	if opts.NotebookID == "" && opts.NotebookURL == "" {
		return SyncPlan{}, errf("sync requires notebook-id or notebook-url")
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
	receipt, err := loadSyncReceiptForPlan(opts)
	if err != nil {
		return SyncPlan{}, err
	}
	if opts.Backend == "nlm" {
		return planSyncNLM(ctx, chunks, opts, receipt)
	}
	return planSyncMCP(ctx, chunks, opts, receipt)
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

func loadSyncReceiptForPlan(opts SyncOptions) (*SyncReceipt, error) {
	if !opts.Resume {
		return nil, nil
	}
	if opts.ReceiptPath == "" {
		return nil, errf("--resume requires an existing sync receipt")
	}
	receipt, err := loadSyncReceipt(opts.ReceiptPath)
	if err != nil {
		return nil, err
	}
	if !receipt.canResume(opts) {
		return nil, errf("sync receipt does not match requested transfer")
	}
	return &receipt, nil
}

func runSyncCommand(ctx context.Context, command, args []string, opts SyncOptions, phase, message string, done, total int, relPath string) ([]byte, error) {
	stop := startProgressHeartbeat(opts.Progress, opts.JobID, phase, message, done, total, relPath, opts.HeartbeatInterval)
	defer stop()
	return runCommand(ctx, command, args, opts.RequestTimeout)
}
