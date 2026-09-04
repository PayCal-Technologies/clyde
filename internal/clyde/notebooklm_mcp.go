package clyde

import "context"

var defaultMCPCommand = []string{"npx", "-y", "notebooklm-mcp@2.0.0"}

var defaultMCPEnv = map[string]string{
	"NOTEBOOKLM_TRANSPORT":      "stdio",
	"NOTEBOOKLM_PROFILE":        "all",
	"NOTEBOOKLM_DISABLED_TOOLS": "cleanup_data,re_auth,remove_notebook,update_notebook",
}

func toolAvailable(tools map[string]any, name string) bool {
	list, _ := tools["tools"].([]any)
	for _, item := range list {
		entry, _ := item.(map[string]any)
		if entry["name"] == name {
			return true
		}
	}
	return false
}

func syncChunksMCP(ctx context.Context, chunks []ChunkRecord, opts SyncOptions, receipt *SyncReceipt) (int, error) {
	command := opts.MCPCommand
	if len(command) == 0 {
		command = defaultMCPCommand
	}
	sink := opts.Progress
	client := NewMCPClient(command, defaultMCPEnv, opts.RequestTimeout)
	stop := startProgressHeartbeat(sink, opts.JobID, "starting", "connecting to NotebookLM", 0, len(chunks), "", opts.HeartbeatInterval)
	err := client.Start(ctx)
	stop()
	if err != nil {
		emitError(sink, opts.JobID, "failed", "NotebookLM MCP connection failed", 0, len(chunks), "", err)
		return 0, errf("NotebookLM MCP connection failed: %w", err)
	}
	defer client.Close()

	stop = startProgressHeartbeat(sink, opts.JobID, "checking", "checking MCP tools", 0, len(chunks), "", opts.HeartbeatInterval)
	tools, err := client.ListTools(ctx)
	stop()
	if err != nil {
		return 0, err
	}
	if !toolAvailable(tools, "add_source") {
		err := errf("NotebookLM MCP server does not expose add_source")
		emitError(sink, opts.JobID, "failed", err.Error(), 0, len(chunks), "", err)
		return 0, err
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
		args := map[string]any{"type": "text", "title": title, "content": chunk.Text}
		if opts.NotebookID != "" {
			args["notebook_id"] = opts.NotebookID
		}
		if opts.NotebookURL != "" {
			args["notebook_url"] = opts.NotebookURL
		}
		stop = startProgressHeartbeat(sink, opts.JobID, "uploading", "uploading "+title, count, len(chunks), chunk.Path, opts.HeartbeatInterval)
		result, err := client.CallTool(ctx, "add_source", args)
		stop()
		if err != nil {
			emitError(sink, opts.JobID, "failed", "failed uploading "+title, count, len(chunks), chunk.Path, err)
			if receiptErr := recordSyncReceiptChunk(opts, receipt, chunk, title, "", "failed", err); receiptErr != nil {
				return count, receiptErr
			}
			return count, errf("failed uploading %s chunk %d/%d: %w", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal, err)
		}
		count++
		if err := recordUploadedSyncReceiptChunk(opts, receipt, chunk, title, sourceIDFromAny(result)); err != nil {
			emitError(sink, opts.JobID, "failed", "failed recording receipt for "+title, count, len(chunks), chunk.Path, err)
			return count, err
		}
		emit(sink, opts.JobID, "uploaded", "uploaded "+title, count, len(chunks), chunk.Path)
	}
	emit(sink, opts.JobID, "complete", "uploaded "+itoa(int64(count))+" chunks", count, len(chunks), "")
	return count, nil
}

func planSyncMCP(ctx context.Context, chunks []ChunkRecord, opts SyncOptions, receipt *SyncReceipt) (SyncPlan, error) {
	command := opts.MCPCommand
	if len(command) == 0 {
		command = defaultMCPCommand
	}
	client := NewMCPClient(command, defaultMCPEnv, opts.RequestTimeout)
	stop := startProgressHeartbeat(opts.Progress, opts.JobID, "starting", "connecting to NotebookLM", 0, len(chunks), "", opts.HeartbeatInterval)
	err := client.Start(ctx)
	stop()
	if err != nil {
		return SyncPlan{}, errf("NotebookLM MCP connection failed: %w", err)
	}
	defer client.Close()
	stop = startProgressHeartbeat(opts.Progress, opts.JobID, "checking", "checking MCP tools", 0, len(chunks), "", opts.HeartbeatInterval)
	tools, err := client.ListTools(ctx)
	stop()
	if err != nil {
		return SyncPlan{}, err
	}
	if !toolAvailable(tools, "add_source") {
		return SyncPlan{}, errf("NotebookLM MCP server does not expose add_source")
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
	return plan, nil
}
