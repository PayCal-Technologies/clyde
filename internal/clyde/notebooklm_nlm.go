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
	if _, err := runSyncCommand(ctx, command, []string{"login", "--check"}, opts, "checking", "checking nlm CLI", 0, len(chunks), ""); err != nil {
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
		if receipt != nil && opts.Resume {
			if status, ok := receipt.unresolved(chunk, title); ok {
				return count, errf("sync receipt has %s chunk with ambiguous remote state; reconcile or start a new receipt before retrying: %s", status, title)
			}
		}
		if err := recordSyncReceiptChunk(opts, receipt, chunk, title, "", "pending", nil); err != nil {
			emitError(sink, opts.JobID, "failed", "failed recording pending receipt for "+title, count, len(chunks), chunk.Path, err)
			return count, err
		}
		out, err := runSyncCommand(ctx, command, []string{"source", "add", target, "--text", chunk.Text, "--title", title, "--json"}, opts, "uploading", "uploading "+title, count, len(chunks), chunk.Path)
		if err != nil {
			emitError(sink, opts.JobID, "failed", "failed uploading "+title, count, len(chunks), chunk.Path, err)
			if receiptErr := recordSyncReceiptChunk(opts, receipt, chunk, title, "", "failed", err); receiptErr != nil {
				return count, receiptErr
			}
			return count, errf("failed uploading %s chunk %d/%d: %w", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal, err)
		}
		count++
		if err := recordUploadedSyncReceiptChunk(opts, receipt, chunk, title, sourceIDFromJSON(out)); err != nil {
			emitError(sink, opts.JobID, "failed", "failed recording receipt for "+title, count, len(chunks), chunk.Path, err)
			return count, err
		}
		emit(sink, opts.JobID, "uploaded", "uploaded "+title, count, len(chunks), chunk.Path)
	}
	emit(sink, opts.JobID, "complete", "uploaded "+itoa(int64(count))+" chunks", count, len(chunks), "")
	return count, nil
}

func planSyncNLM(ctx context.Context, chunks []ChunkRecord, opts SyncOptions, receipt *SyncReceipt) (SyncPlan, error) {
	command := opts.NLMCommand
	if len(command) == 0 {
		command = []string{"nlm"}
	}
	target := opts.NotebookID
	if target == "" {
		target = opts.NotebookURL
	}
	if _, err := runSyncCommand(ctx, command, []string{"login", "--check"}, opts, "checking", "checking nlm CLI", 0, len(chunks), ""); err != nil {
		return SyncPlan{}, err
	}
	plan := SyncPlan{}
	for _, chunk := range chunks {
		title := opts.TitlePrefix + chunk.Path + " [" + itoa(int64(chunk.ChunkIndex)) + "/" + itoa(int64(chunk.ChunkTotal)) + "]"
		if receipt != nil && receipt.uploaded(chunk, title) {
			continue
		}
		if receipt != nil {
			if status, ok := receipt.unresolved(chunk, title); ok {
				return SyncPlan{}, errf("sync receipt has %s chunk with ambiguous remote state; reconcile or start a new receipt before retrying: %s", status, title)
			}
		}
		plan.Uploads = append(plan.Uploads, title)
	}
	if !opts.DeleteExistingSources || (receipt != nil && receipt.deletionCompleted()) {
		return plan, nil
	}
	sources, err := listNLMSources(ctx, command, target, opts)
	if err != nil {
		return SyncPlan{}, err
	}
	if receipt != nil && receipt.deletionPlanned() {
		planned := make(map[string]bool, len(receipt.Deletions))
		for _, deletion := range receipt.Deletions {
			if deletion.SourceID != "" && deletion.Status != "deleted" {
				planned[deletion.SourceID] = true
			}
		}
		for _, source := range sources {
			if id, _ := source["id"].(string); planned[id] {
				plan.Deletes = append(plan.Deletes, source)
			}
		}
		return plan, nil
	}
	plan.Deletes = sources
	return plan, nil
}

func deleteExistingNLMSources(ctx context.Context, command []string, target string, opts SyncOptions, receipt *SyncReceipt) error {
	if receipt != nil && receipt.deletionCompleted() {
		emit(opts.Progress, opts.JobID, "pruned", "existing NotebookLM sources already deleted for this receipt", 0, 0, "")
		return nil
	}
	var sources []map[string]any
	if receipt != nil && receipt.deletionPlanned() {
		current, err := listNLMSources(ctx, command, target, opts)
		if err != nil {
			return err
		}
		sources = receipt.reconcilePlannedDeletions(current)
		if len(sources) == 0 {
			receipt.recordDeletionPhase("completed")
		}
		if err := saveSyncReceipt(opts.ReceiptPath, *receipt); err != nil {
			return err
		}
	} else {
		var err error
		sources, err = listNLMSources(ctx, command, target, opts)
		if err != nil {
			return err
		}
		if receipt != nil {
			receipt.recordDeletionPhase("planned")
			receipt.recordDeletions(sources, "planned")
			if err := saveSyncReceipt(opts.ReceiptPath, *receipt); err != nil {
				return err
			}
		}
	}
	var ids []string
	for _, source := range sources {
		if id, ok := source["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		if receipt != nil {
			receipt.recordDeletionPhase("completed")
			if err := saveSyncReceipt(opts.ReceiptPath, *receipt); err != nil {
				return err
			}
		}
		emit(opts.Progress, opts.JobID, "pruned", "no existing NotebookLM sources to delete", 0, 0, "")
		return nil
	}
	args := append([]string{"source", "delete"}, ids...)
	args = append(args, "--confirm", "--json")
	if _, err := runSyncCommand(ctx, command, args, opts, "pruning", "deleting "+itoa(int64(len(ids)))+" existing NotebookLM sources", 0, len(ids), ""); err != nil {
		emitError(opts.Progress, opts.JobID, "failed", "failed deleting existing NotebookLM sources", 0, 0, "", err)
		return err
	}
	if receipt != nil {
		receipt.recordDeletionPhase("completed")
		receipt.recordDeletions(sources, "deleted")
		if err := saveSyncReceipt(opts.ReceiptPath, *receipt); err != nil {
			ambiguousErr := errf("remote delete succeeded but local receipt save failed: %v", err)
			receipt.recordDeletionPhase("ambiguous")
			receipt.recordDeletions(sources, "ambiguous")
			if saveErr := saveSyncReceipt(opts.ReceiptPath, *receipt); saveErr != nil {
				return errf("remote delete succeeded but local receipt is ambiguous and could not be saved: completed_save=%v ambiguous_save=%v", err, saveErr)
			}
			return ambiguousErr
		}
	}
	emit(opts.Progress, opts.JobID, "pruned", "deleted "+itoa(int64(len(ids)))+" existing NotebookLM sources", len(ids), len(ids), "")
	return nil
}

func listNLMSources(ctx context.Context, command []string, target string, opts SyncOptions) ([]map[string]any, error) {
	out, err := runSyncCommand(ctx, command, []string{"source", "list", target, "--json"}, opts, "pruning", "listing existing NotebookLM sources", 0, 0, "")
	if err != nil {
		emitError(opts.Progress, opts.JobID, "failed", "failed listing existing NotebookLM sources", 0, 0, "", err)
		return nil, err
	}
	var sources []map[string]any
	if err := json.Unmarshal(out, &sources); err != nil {
		return nil, errf("nlm source list did not return JSON: %w", err)
	}
	return sources, nil
}
