package clyde

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
)

func cmdSync(args []string, out, errOut io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	flags := scanFlags{maxFileBytes: cfg.MaxFileBytes, maxChunkChars: cfg.MaxChunkChars}
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(out)
	notebookID := fs.String("notebook-id", "", "NotebookLM notebook id")
	notebookURL := fs.String("notebook-url", "", "NotebookLM notebook URL")
	approve := fs.Bool("approve-upload", false, "approve upload")
	bundleDir := fs.String("bundle", "", "upload an already reviewed Clyde bundle directory")
	approveDigest := fs.String("approve-digest", "", "approve upload of a specific bundle digest")
	receiptPath := fs.String("receipt", "", "write sync receipt JSON to this path")
	resume := fs.Bool("resume", false, "resume from an existing matching sync receipt")
	backend := fs.String("backend", "mcp", "NotebookLM backend: mcp or nlm")
	mcpCommand := fs.String("mcp-command", "npx -y notebooklm-mcp@2.0.0", "command used for MCP backend")
	nlmCommand := fs.String("nlm-command", "nlm", "command used for nlm backend")
	deleteExisting := fs.Bool("delete-existing-sources", false, "with --backend nlm, delete all existing notebook sources before upload")
	timeout := fs.Float64("mcp-timeout", 120, "seconds to wait for each backend response")
	statusURL := fs.String("status-url", "", "optional Clyde daemon JSON-RPC URL")
	quiet := fs.Bool("quiet-progress", false, "suppress real-time progress lines")
	jobID := fs.String("job-id", "sync", "status daemon job id")
	subject := fs.String("subject", "", "generate a dated NotebookLM book title")
	bookTitle := fs.String("book-title", "", "use an exact NotebookLM book title")
	fs.Float64("heartbeat-interval", 1, "accepted for compatibility; progress events emit per phase")
	addScanFlags(fs, &flags)
	boolFlags := map[string]bool{
		"approve-upload":            true,
		"delete-existing-sources":   true,
		"quiet-progress":            true,
		"resume":                    true,
		"allow-filesystem-fallback": true,
	}
	if err := fs.Parse(interspersedArgs(args, boolFlags)); err != nil {
		return err
	}
	if *bundleDir == "" && fs.NArg() != 1 {
		return errf("sync requires REPO")
	}
	if *bundleDir != "" && fs.NArg() > 1 {
		return errf("sync with --bundle accepts at most one REPO label")
	}
	if !*approve {
		return errf("sync requires --approve-upload")
	}
	if (*notebookID == "" && *notebookURL == "") || (*notebookID != "" && *notebookURL != "") {
		return errf("sync requires exactly one of --notebook-id or --notebook-url")
	}
	if *deleteExisting && *backend != "nlm" {
		return errf("--delete-existing-sources requires --backend nlm")
	}
	if *backend != "mcp" && *backend != "nlm" {
		return errf("--backend must be mcp or nlm")
	}
	if err := validatePositiveSeconds("mcp-timeout", *timeout); err != nil {
		return err
	}
	if err := validateCommandFlag("mcp-command", *mcpCommand); err != nil {
		return err
	}
	if err := validateCommandFlag("nlm-command", *nlmCommand); err != nil {
		return err
	}
	if *subject != "" && *bookTitle != "" {
		return errf("--subject and --book-title are mutually exclusive")
	}
	if err := validateScanFlags(flags); err != nil {
		return err
	}
	if *bundleDir != "" && strings.TrimSpace(*approveDigest) == "" {
		return errf("sync with --bundle requires --approve-digest")
	}
	if *resume && strings.TrimSpace(*receiptPath) == "" && *bundleDir == "" {
		return errf("--resume requires --receipt for live-repository sync")
	}
	var plan *BookPlan
	if *subject != "" || *bookTitle != "" {
		p, err := planFromArgs(*subject, *bookTitle)
		if err != nil {
			return err
		}
		plan = &p
	}
	title := ""
	prefix := ""
	if plan != nil {
		title = plan.Title()
		prefix = plan.SourcePrefix()
	}
	var chunks []ChunkRecord
	var bundleDigest string
	if *bundleDir != "" {
		bundle, err := LoadBundle(*bundleDir)
		if err != nil {
			return err
		}
		if bundle.Digest != *approveDigest {
			return errf("approved digest does not match bundle: approved=%s bundle=%s", *approveDigest, bundle.Digest)
		}
		chunks = bundle.Chunks
		bundleDigest = bundle.Digest
		if *receiptPath == "" {
			*receiptPath = filepath.Join(*bundleDir, "sync-receipt.json")
		}
		fmt.Fprintf(out, "Bundle: %s\n", *bundleDir)
		fmt.Fprintf(out, "Bundle digest: %s\n", bundleDigest)
		fmt.Fprintf(out, "Included files: %d\n", bundle.Manifest.FileCount)
		fmt.Fprintf(out, "Skipped files: %d\n", len(bundle.Manifest.Skips))
		fmt.Fprintf(out, "Chunks: %d\n", bundle.Manifest.ChunkCount)
		fmt.Fprintf(out, "Total bytes: %s\n", formatBytes(bundle.Manifest.TotalBytes))
	} else {
		addRepoPathExclude(fs.Arg(0), *receiptPath, &flags)
		result, liveChunks, err := scanAndChunk(fs.Arg(0), flags, title)
		if err != nil {
			return err
		}
		chunks = liveChunks
		printSummary(out, result, len(chunks), flags)
	}
	if plan != nil {
		printBookPlan(out, *plan)
	}
	var sinks []ProgressSink
	if !*quiet {
		sinks = append(sinks, ConsoleSink{Writer: errOut})
	}
	if *statusURL != "" {
		sinks = append(sinks, HTTPSink{URL: *statusURL})
	}
	var sink ProgressSink
	if len(sinks) == 1 {
		sink = sinks[0]
	} else if len(sinks) > 1 {
		sink = TeeSink(sinks)
	}
	count, err := SyncChunks(context.Background(), chunks, SyncOptions{
		NotebookID:            *notebookID,
		NotebookURL:           *notebookURL,
		Backend:               *backend,
		MCPCommand:            shellFields(*mcpCommand),
		NLMCommand:            shellFields(*nlmCommand),
		DeleteExistingSources: *deleteExisting,
		RequestTimeout:        time.Duration(*timeout * float64(time.Second)),
		Progress:              sink,
		JobID:                 *jobID,
		TitlePrefix:           prefix,
		BundleDigest:          bundleDigest,
		ReceiptPath:           *receiptPath,
		Resume:                *resume,
	})
	if err != nil {
		return err
	}
	target := *notebookID
	if target == "" {
		target = *notebookURL
	}
	suffix := ""
	if *backend != "mcp" {
		suffix = " via " + *backend
	}
	fmt.Fprintf(out, "\nUploaded %d chunks to notebook %s%s.\n", count, target, suffix)
	if bundleDigest != "" {
		fmt.Fprintf(out, "Uploaded bundle digest: %s\n", bundleDigest)
	}
	if *receiptPath != "" {
		fmt.Fprintf(out, "Sync receipt: %s\n", *receiptPath)
	}
	return nil
}
