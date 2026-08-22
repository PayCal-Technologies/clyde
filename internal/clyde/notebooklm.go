package clyde

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

var defaultMCPCommand = []string{"npx", "-y", "notebooklm-mcp@2.0.0"}

var defaultMCPEnv = map[string]string{
	"NOTEBOOKLM_ACCOUNT":        "codex-test",
	"NOTEBOOKLM_TRANSPORT":      "stdio",
	"NOTEBOOKLM_PROFILE":        "all",
	"NOTEBOOKLM_DISABLED_TOOLS": "cleanup_data,re_auth,remove_notebook,update_notebook",
}

const (
	maxCommandStderrBytes = 64 * 1024
	maxCommandOutputBytes = 16 * 1024 * 1024
)

type SyncOptions struct {
	NotebookID            string
	NotebookURL           string
	Backend               string
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
	if opts.Backend == "nlm" {
		return syncChunksNLM(ctx, chunks, opts)
	}
	return syncChunksMCP(ctx, chunks, opts)
}

func syncChunksMCP(ctx context.Context, chunks []ChunkRecord, opts SyncOptions) (int, error) {
	command := opts.MCPCommand
	if len(command) == 0 {
		command = defaultMCPCommand
	}
	sink := opts.Progress
	emit(sink, opts.JobID, "starting", "connecting to NotebookLM", 0, len(chunks), "")
	client := NewMCPClient(command, defaultMCPEnv, opts.RequestTimeout)
	if err := client.Start(ctx); err != nil {
		emitError(sink, opts.JobID, "failed", "NotebookLM MCP connection failed", 0, len(chunks), "", err)
		return 0, errf("NotebookLM MCP connection failed: %w", err)
	}
	defer client.Close()

	emit(sink, opts.JobID, "checking", "checking MCP tools", 0, len(chunks), "")
	tools, err := client.ListTools(ctx)
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
		emit(sink, opts.JobID, "uploading", "uploading "+title, count, len(chunks), chunk.Path)
		args := map[string]any{"type": "text", "title": title, "content": chunk.Text}
		if opts.NotebookID != "" {
			args["notebook_id"] = opts.NotebookID
		}
		if opts.NotebookURL != "" {
			args["notebook_url"] = opts.NotebookURL
		}
		if _, err := client.CallTool(ctx, "add_source", args); err != nil {
			emitError(sink, opts.JobID, "failed", "failed uploading "+title, count, len(chunks), chunk.Path, err)
			return count, errf("failed uploading %s chunk %d/%d: %w", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal, err)
		}
		count++
		emit(sink, opts.JobID, "uploaded", "uploaded "+title, count, len(chunks), chunk.Path)
	}
	emit(sink, opts.JobID, "complete", "uploaded "+itoa(int64(count))+" chunks", count, len(chunks), "")
	return count, nil
}

func syncChunksNLM(ctx context.Context, chunks []ChunkRecord, opts SyncOptions) (int, error) {
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
		if err := deleteExistingNLMSources(ctx, command, target, opts); err != nil {
			return 0, err
		}
	}
	count := 0
	for _, chunk := range chunks {
		title := opts.TitlePrefix + chunk.Path + " [" + itoa(int64(chunk.ChunkIndex)) + "/" + itoa(int64(chunk.ChunkTotal)) + "]"
		emit(sink, opts.JobID, "uploading", "uploading "+title, count, len(chunks), chunk.Path)
		_, err := runCommand(ctx, command, []string{"source", "add", target, "--text", chunk.Text, "--title", title, "--json"}, opts.RequestTimeout)
		if err != nil {
			emitError(sink, opts.JobID, "failed", "failed uploading "+title, count, len(chunks), chunk.Path, err)
			return count, errf("failed uploading %s chunk %d/%d: %w", chunk.Path, chunk.ChunkIndex, chunk.ChunkTotal, err)
		}
		count++
		emit(sink, opts.JobID, "uploaded", "uploaded "+title, count, len(chunks), chunk.Path)
	}
	emit(sink, opts.JobID, "complete", "uploaded "+itoa(int64(count))+" chunks", count, len(chunks), "")
	return count, nil
}

func deleteExistingNLMSources(ctx context.Context, command []string, target string, opts SyncOptions) error {
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
	emit(opts.Progress, opts.JobID, "pruned", "deleted "+itoa(int64(len(ids)))+" existing NotebookLM sources", len(ids), len(ids), "")
	return nil
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

func runCommand(ctx context.Context, command, args []string, timeout time.Duration) ([]byte, error) {
	if len(command) == 0 {
		return nil, errf("empty command")
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, command[0], append(command[1:], args...)...)
	var stdout, stderr limitedBuffer
	stdout.Limit = maxCommandOutputBytes
	stderr.Limit = maxCommandStderrBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if runCtx.Err() == context.DeadlineExceeded {
		return nil, errf("timed out running %s after %s", commandSummary(command, args), timeout)
	}
	if stdout.Truncated {
		return nil, errf("command output too large from %s", commandSummary(command, args))
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		if stderr.Truncated {
			detail += " [stderr truncated]"
		}
		return nil, errf("%s", detail)
	}
	return bytes.TrimSpace(stdout.Bytes()), nil
}

type limitedBuffer struct {
	Limit     int
	buf       bytes.Buffer
	Truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Limit <= 0 {
		b.Truncated = true
		return len(p), nil
	}
	remaining := b.Limit - b.buf.Len()
	if remaining <= 0 {
		b.Truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.Truncated = true
		return len(p), nil
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func commandSummary(command, args []string) string {
	parts := append([]string{}, command...)
	for i := 0; i < len(args); i++ {
		parts = append(parts, args[i])
		if args[i] == "--text" && i+1 < len(args) {
			parts = append(parts, "[redacted]")
			i++
		}
	}
	return strings.Join(parts, " ")
}

func emit(sink ProgressSink, jobID, phase, message string, done, total int, relPath string) {
	if sink == nil {
		return
	}
	event := NewProgressEvent(jobID, phase, message, done, total)
	event.RelPath = relPath
	sink.Emit(event)
}

func emitError(sink ProgressSink, jobID, phase, message string, done, total int, relPath string, err error) {
	if sink == nil {
		return
	}
	event := NewProgressEvent(jobID, phase, message, done, total)
	event.RelPath = relPath
	event.Error = err.Error()
	sink.Emit(event)
}
