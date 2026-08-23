package clyde

import (
	"context"
	"encoding/json"
)

func syncChunksNLM(ctx context.Context, chunks []ChunkRecord, opts SyncOptions, receipt *SyncReceipt) (int, error) {
	command := opts.NLMCommand
	if len(command) == 0 {
		command = []string{"nlm"}
	}
	target := opts.NotebookID
	if target == "" {
		target = opts.NotebookURL
	}
	sink := opts.Progress
	emit(sink, opts.JobID, "checking", "checking nlm CLI", 0, len(chunks), "")
	if _, err := runCommand(ctx, command, []string{"login", "--check"}, opts.RequestTimeout); err != nil {
		emitError(sink, opts.JobID, "failed", "nlm CLI check failed", 0, len(chunks), "", err)
		return 0, err
	}
	if opts.DeleteExistingSources {
		if err := deleteExistingNLMSources(ctx, command, target, opts, receipt); err != nil {
			return 0, err
		}
	}
	count := 0
	for _, chunk := range chunks {
		title := opts.TitlePrefix + chunk.Path + " [" + itoa(int64(chunk.ChunkIndex)) + "/" + itoa(int64(chunk.ChunkTotal)) + "]"
		if receipt != nil && opts.Resume && receipt.uploaded(chunk, title) {
			count++
			emit(sink, opts.JobID, "skipped", "already uploaded "+title, count, len(chunks), chunk.Path)
			continue
		}
		emit(sink, opts.JobID, "uploading", "uploading "+title, count, len(chunks), chunk.Path)
		out, err := runCommand(ctx, command, []string{"source", "add", target, "--text", chunk.Text, "--title", title, "--json"}, opts.RequestTimeout)
		if err != nil {
			emitError(sink, opts.JobID, "failed", "failed uploading "+title, count, len(chunks), chunk.Path, err)
			recordSyncReceiptChunk(opts, receipt, chunk, title, "", "failed", err)
			return count, errf("failed uploading %s chunk %d/%d: %w", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal, err)
		}
		count++
		recordSyncReceiptChunk(opts, receipt, chunk, title, sourceIDFromJSON(out), "uploaded", nil)
		emit(sink, opts.JobID, "uploaded", "uploaded "+title, count, len(chunks), chunk.Path)
	}
	emit(sink, opts.JobID, "complete", "uploaded "+itoa(int64(count))+" chunks", count, len(chunks), "")
	return count, nil
}

func deleteExistingNLMSources(ctx context.Context, command []string, target string, opts SyncOptions, receipt *SyncReceipt) error {
	emit(opts.Progress, opts.JobID, "pruning", "listing existing NotebookLM sources", 0, 0, "")
	out, err := runCommand(ctx, command, []string{"source", "list", target, "--json"}, opts.RequestTimeout)
	if err != nil {
		emitError(opts.Progress, opts.JobID, "failed", "failed deleting existing NotebookLM sources", 0, 0, "", err)
		return err
	}
	var sources []map[string]any
	if err := json.Unmarshal(out, &sources); err != nil {
		return errf("nlm source list did not return JSON: %w", err)
	}
	if receipt != nil {
		receipt.recordDeletions(sources, "planned")
		_ = saveSyncReceipt(opts.ReceiptPath, *receipt)
	}
	var ids []string
	for _, source := range sources {
		if id, ok := source["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		emit(opts.Progress, opts.JobID, "pruned", "no existing NotebookLM sources to delete", 0, 0, "")
		return nil
	}
	args := append([]string{"source", "delete"}, ids...)
	args = append(args, "--confirm", "--json")
	emit(opts.Progress, opts.JobID, "pruning", "deleting "+itoa(int64(len(ids)))+" existing NotebookLM sources", 0, len(ids), "")
	if _, err := runCommand(ctx, command, args, opts.RequestTimeout); err != nil {
		emitError(opts.Progress, opts.JobID, "failed", "failed deleting existing NotebookLM sources", 0, 0, "", err)
		return err
	}
	if receipt != nil {
		receipt.recordDeletions(sources, "deleted")
		_ = saveSyncReceipt(opts.ReceiptPath, *receipt)
	}
	emit(opts.Progress, opts.JobID, "pruned", "deleted "+itoa(int64(len(ids)))+" existing NotebookLM sources", len(ids), len(ids), "")
	return nil
}
