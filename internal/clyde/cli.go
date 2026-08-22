package clyde

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	productName        = "Clyde"
	productVersion     = "0.2.4"
	productDescription = "local repository review, bundling, and NotebookLM sync harness"
	productHomeURL     = "https://paycaltech.com/clyde"
	productHelpURL     = "https://paycaltech.com/clyde/help"
	productGitHubURL   = "https://github.com/PayCal-Technologies/clyde"
	productCreator     = "PayCal Technologies"
	productCreatorURL  = "https://paycaltech.com"
)

const maxCLISeconds = 24 * 60 * 60

const maxPromptInputBytes = 1 << 20

func Main(args []string, stdout, stderr io.Writer) int {
	return MainWithInput(args, os.Stdin, stdout, stderr)
}

func MainWithInput(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		if file, ok := stdin.(*os.File); ok {
			if stat, err := file.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
				if err := RunTUI(stdin, stdout, stderr); err != nil {
					fmt.Fprintf(stderr, "clyde: error: %v\n", err)
					return 1
				}
				return 0
			}
		}
		printHelp(stdout)
		return 0
	}
	if args[0] == "-h" || args[0] == "--help" {
		printHelp(stdout)
		return 0
	}
	if args[0] == "--about" || args[0] == "about" {
		printAbout(stdout)
		return 0
	}
	if err := run(args, stdin, stdout, stderr); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "clyde: error: %v\n", err)
		return 1
	}
	return 0
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	switch args[0] {
	case "about":
		printAbout(stdout)
		return nil
	case "help":
		return cmdHelp(args[1:], stdin, stdout, stderr)
	case "completion":
		return cmdCompletion(args[1:], stdout)
	case "doctor":
		return cmdDoctor(args[1:], stdout)
	case "tui":
		return RunTUI(stdin, stdout, stderr)
	case "config":
		return cmdConfig(args[1:], stdout)
	case "preview":
		return cmdPreview(args[1:], stdout)
	case "scan-report":
		return cmdScanReport(args[1:], stdout)
	case "bundle":
		return cmdBundle(args[1:], stdout)
	case "sync":
		return cmdSync(args[1:], stdout, stderr)
	case "daemon":
		return cmdDaemon(args[1:], stdout)
	case "status":
		return cmdStatus(args[1:], stdout)
	case "book":
		return cmdBook(args[1:], stdout)
	case "models":
		return cmdModels(args[1:], stdout)
	case "ask":
		return cmdAsk(args[1:], stdin, stdout)
	case "agent":
		return cmdAgent(args[1:], stdin, stdout)
	default:
		return errf("unknown command: %s", args[0])
	}
}

type scanFlags struct {
	include       multiFlag
	exclude       multiFlag
	maxFileBytes  int64
	maxChunkChars int
}

func addScanFlags(fs *flag.FlagSet, flags *scanFlags) {
	fs.Var(&flags.include, "include", "only include paths matching this glob; repeatable")
	fs.Var(&flags.exclude, "exclude", "skip paths matching this glob in addition to Clyde defaults; repeatable")
	fs.Int64Var(&flags.maxFileBytes, "max-file-bytes", flags.maxFileBytes, "skip files larger than this many bytes")
	fs.IntVar(&flags.maxChunkChars, "max-chunk-chars", flags.maxChunkChars, "split uploaded source text at this many characters")
}

func cmdPreview(args []string, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	flags := scanFlags{maxFileBytes: cfg.MaxFileBytes, maxChunkChars: cfg.MaxChunkChars}
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	fs.SetOutput(out)
	showFiles := fs.Int("show-files", 20, "show first N included files")
	showSkips := fs.Int("show-skips", 50, "show first N skipped files")
	jsonOut := fs.Bool("json", false, "print machine-readable JSON summary")
	addScanFlags(fs, &flags)
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"json": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errf("preview requires REPO")
	}
	if err := validateScanFlags(flags); err != nil {
		return err
	}
	result, chunks, err := scanAndChunk(fs.Arg(0), flags, "")
	if err != nil {
		return err
	}
	if *jsonOut {
		return printPreviewJSON(out, result, len(chunks), flags)
	}
	printSummary(out, result, len(chunks), flags)
	if len(result.Files) > 0 && *showFiles > 0 {
		fmt.Fprintf(out, "\nIncluded files (first %d):\n", min(*showFiles, len(result.Files)))
		for _, file := range result.Files[:min(*showFiles, len(result.Files))] {
			fmt.Fprintf(out, "  %s (%s)\n", file.Rel, formatBytes(file.Size))
		}
		if len(result.Files) > *showFiles {
			fmt.Fprintf(out, "  ... %d more\n", len(result.Files)-*showFiles)
		}
	}
	if len(result.Skips) > 0 && *showSkips > 0 {
		fmt.Fprintln(out, "\nSkipped:")
		for _, skip := range result.Skips[:min(*showSkips, len(result.Skips))] {
			fmt.Fprintf(out, "  %s: %s\n", skip.Path, skip.Reason)
		}
		if len(result.Skips) > *showSkips {
			fmt.Fprintf(out, "  ... %d more\n", len(result.Skips)-*showSkips)
		}
	}
	if len(result.Files) == 0 {
		fmt.Fprintln(out, "\nNo files matched. Check --include/--exclude or the repo path.")
	}
	return nil
}

type scanReportFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type scanReportCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type scanReport struct {
	Repo           string            `json:"repo"`
	IncludedFiles  int               `json:"included_files"`
	SkippedFiles   int               `json:"skipped_files"`
	TotalBytes     int64             `json:"total_bytes"`
	ChunkCount     int               `json:"chunk_count"`
	MaxFileBytes   int64             `json:"max_file_bytes"`
	MaxChunkChars  int               `json:"max_chunk_chars"`
	Include        []string          `json:"include"`
	Exclude        []string          `json:"exclude"`
	TopFiles       []scanReportFile  `json:"top_files"`
	SkipReasons    []scanReportCount `json:"skip_reasons"`
	ExtensionStats []scanReportCount `json:"extension_stats"`
}

func cmdScanReport(args []string, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	flags := scanFlags{maxFileBytes: cfg.MaxFileBytes, maxChunkChars: cfg.MaxChunkChars}
	fs := flag.NewFlagSet("scan-report", flag.ContinueOnError)
	fs.SetOutput(out)
	jsonOut := fs.Bool("json", false, "print machine-readable scan report")
	top := fs.Int("top", 10, "number of largest files to include")
	addScanFlags(fs, &flags)
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"json": true})); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errf("scan-report requires REPO")
	}
	if *top < 0 {
		return errf("--top must be zero or greater")
	}
	if err := validateScanFlags(flags); err != nil {
		return err
	}
	result, chunks, err := scanAndChunk(fs.Arg(0), flags, "")
	if err != nil {
		return err
	}
	report := buildScanReport(result, len(chunks), flags, *top)
	if *jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	printScanReport(out, report)
	return nil
}

func buildScanReport(result ScanResult, chunkCount int, flags scanFlags, top int) scanReport {
	files := append([]FileRecord(nil), result.Files...)
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].Size == files[j].Size {
			return files[i].Rel < files[j].Rel
		}
		return files[i].Size > files[j].Size
	})
	if top > len(files) {
		top = len(files)
	}
	topFiles := make([]scanReportFile, 0, top)
	for _, file := range files[:top] {
		topFiles = append(topFiles, scanReportFile{Path: file.Rel, Size: file.Size, SHA256: file.SHA256})
	}
	return scanReport{
		Repo:           result.Repo,
		IncludedFiles:  len(result.Files),
		SkippedFiles:   len(result.Skips),
		TotalBytes:     result.TotalBytes(),
		ChunkCount:     chunkCount,
		MaxFileBytes:   flags.maxFileBytes,
		MaxChunkChars:  flags.maxChunkChars,
		Include:        []string(flags.include),
		Exclude:        []string(flags.exclude),
		TopFiles:       topFiles,
		SkipReasons:    sortedCounts(skipReasonCounts(result.Skips)),
		ExtensionStats: sortedCounts(extensionCounts(result.Files)),
	}
}

func skipReasonCounts(skips []SkipRecord) map[string]int {
	counts := map[string]int{}
	for _, skip := range skips {
		counts[skip.Reason]++
	}
	return counts
}

func extensionCounts(files []FileRecord) map[string]int {
	counts := map[string]int{}
	for _, file := range files {
		ext := strings.ToLower(filepath.Ext(file.Rel))
		if ext == "" {
			ext = "(none)"
		}
		counts[ext]++
	}
	return counts
}

func sortedCounts(counts map[string]int) []scanReportCount {
	out := make([]scanReportCount, 0, len(counts))
	for name, count := range counts {
		out = append(out, scanReportCount{Name: name, Count: count})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out
}

func printScanReport(out io.Writer, report scanReport) {
	fmt.Fprintf(out, "Repo: %s\n", report.Repo)
	fmt.Fprintf(out, "Included files: %d\n", report.IncludedFiles)
	fmt.Fprintf(out, "Skipped files: %d\n", report.SkippedFiles)
	fmt.Fprintf(out, "Total included bytes: %d (%s)\n", report.TotalBytes, formatBytes(report.TotalBytes))
	fmt.Fprintf(out, "Chunks: %d\n", report.ChunkCount)
	if len(report.TopFiles) > 0 {
		fmt.Fprintln(out, "\nLargest included files:")
		for _, file := range report.TopFiles {
			fmt.Fprintf(out, "  %s (%s)\n", file.Path, formatBytes(file.Size))
		}
	}
	if len(report.ExtensionStats) > 0 {
		fmt.Fprintln(out, "\nExtensions:")
		for _, item := range report.ExtensionStats {
			fmt.Fprintf(out, "  %s: %d\n", item.Name, item.Count)
		}
	}
	if len(report.SkipReasons) > 0 {
		fmt.Fprintln(out, "\nSkip reasons:")
		for _, item := range report.SkipReasons {
			fmt.Fprintf(out, "  %s: %d\n", item.Name, item.Count)
		}
	}
}

func cmdBundle(args []string, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	flags := scanFlags{maxFileBytes: cfg.MaxFileBytes, maxChunkChars: cfg.MaxChunkChars}
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	fs.SetOutput(out)
	outDir := fs.String("out", ".clyde/out", "directory for manifest.json and chunks.jsonl")
	subject := fs.String("subject", "", "generate a dated NotebookLM book title")
	bookTitle := fs.String("book-title", "", "use an exact NotebookLM book title")
	addScanFlags(fs, &flags)
	if err := fs.Parse(interspersedArgs(args, map[string]bool{})); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errf("bundle requires REPO")
	}
	if *subject != "" && *bookTitle != "" {
		return errf("--subject and --book-title are mutually exclusive")
	}
	if err := validateScanFlags(flags); err != nil {
		return err
	}
	var plan *BookPlan
	if *subject != "" || *bookTitle != "" {
		p, err := planFromArgs(*subject, *bookTitle)
		if err != nil {
			return err
		}
		plan = &p
	}
	info, err := os.Stat(*outDir)
	if err == nil && !info.IsDir() {
		return errf("--out must be a directory, not a file: %s", *outDir)
	}
	result, err := ScanRepo(fs.Arg(0), flags.include, flags.exclude, flags.maxFileBytes)
	if err != nil {
		return err
	}
	title, slug := "", ""
	if plan != nil {
		title, slug = plan.Title(), plan.Slug()
	}
	manifest, err := WriteBundle(result, *outDir, flags.maxChunkChars, title, slug)
	if err != nil {
		return err
	}
	printSummary(out, result, manifest.ChunkCount, flags)
	if plan != nil {
		printBookPlan(out, *plan)
	}
	fmt.Fprintf(out, "\nWrote: %s\n", filepath.Join(*outDir, "manifest.json"))
	fmt.Fprintf(out, "Wrote: %s\n", filepath.Join(*outDir, "chunks.jsonl"))
	fmt.Fprintln(out, "Review manifest.json before running sync.")
	return nil
}

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
		"approve-upload":          true,
		"delete-existing-sources": true,
		"quiet-progress":          true,
	}
	if err := fs.Parse(interspersedArgs(args, boolFlags)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errf("sync requires REPO")
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
	result, chunks, err := scanAndChunk(fs.Arg(0), flags, title)
	if err != nil {
		return err
	}
	printSummary(out, result, len(chunks), flags)
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
	return nil
}

func cmdDaemon(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("daemon", flag.ContinueOnError)
	fs.SetOutput(out)
	host := fs.String("host", "127.0.0.1", "host")
	port := fs.Int("port", 5876, "port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *port <= 0 || *port > 65535 {
		return errf("--port must be between 1 and 65535")
	}
	return ServeStatus(*host, *port, out)
}

func cmdStatus(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(out)
	host := fs.String("host", "127.0.0.1", "host")
	port := fs.Int("port", 5876, "port")
	jobID := fs.String("job-id", "", "job id")
	jsonOut := fs.Bool("json", false, "print raw JSON")
	watch := fs.Bool("watch", false, "poll until terminal")
	interval := fs.Float64("interval", 1, "poll interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *port <= 0 || *port > 65535 {
		return errf("--port must be between 1 and 65535")
	}
	if err := validatePositiveSeconds("interval", *interval); err != nil {
		return err
	}
	seen := ""
	for {
		result, err := FetchStatus(StatusURL(*host, *port), *jobID)
		if err != nil {
			return err
		}
		rendered := FormatStatus(result)
		if *jsonOut {
			data, _ := json.MarshalIndent(result, "", "  ")
			rendered = string(data)
		}
		if rendered != seen {
			fmt.Fprintln(out, rendered)
			seen = rendered
		}
		if !*watch || terminalStatus(result) {
			return nil
		}
		time.Sleep(time.Duration(*interval * float64(time.Second)))
	}
}

type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorReport struct {
	Product string        `json:"product"`
	Version string        `json:"version"`
	OS      string        `json:"os"`
	Arch    string        `json:"arch"`
	Checks  []doctorCheck `json:"checks"`
}

func cmdDoctor(args []string, out io.Writer) error {
	if isHelpArgs(args) {
		printDoctorHelp(out)
		return flag.ErrHelp
	}
	cfg := DefaultConfig()
	configPath, pathErr := ConfigPath()
	loaded, loadedPath, configErr := LoadConfig()
	if configErr == nil {
		cfg = loaded
		if loadedPath != "" {
			configPath = loadedPath
		}
	}
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(out)
	jsonOut := fs.Bool("json", false, "print machine-readable diagnostics")
	ollamaTimeout := fs.Float64("ollama-timeout", 2, "seconds to wait for local Ollama diagnostics")
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"json": true})); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errf("doctor accepts at most one optional REPO")
	}
	if err := validatePositiveSeconds("ollama-timeout", *ollamaTimeout); err != nil {
		return err
	}
	report := doctorReport{
		Product: productName,
		Version: productVersion,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
	report.Checks = append(report.Checks, doctorCheck{Name: "version", Status: "ok", Message: productName + " " + productVersion})
	report.Checks = append(report.Checks, doctorCheck{Name: "platform", Status: "ok", Message: runtime.GOOS + "/" + runtime.GOARCH})
	if pathErr != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "error", Message: pathErr.Error()})
	} else if configErr != nil {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "error", Message: configPath + ": " + configErr.Error()})
	} else if _, err := os.Stat(configPath); os.IsNotExist(err) {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "warn", Message: "no config file at " + configPath + "; using defaults"})
	} else {
		report.Checks = append(report.Checks, doctorCheck{Name: "config", Status: "ok", Message: "loaded " + configPath})
	}
	report.Checks = append(report.Checks, doctorExecutableCheck("git", "fast repository file discovery"))
	report.Checks = append(report.Checks, doctorExecutableCheck("npx", "default NotebookLM MCP backend"))
	report.Checks = append(report.Checks, doctorExecutableCheck("nlm", "optional NotebookLM CLI backend"))
	report.Checks = append(report.Checks, doctorOllamaCheck(cfg, time.Duration(*ollamaTimeout*float64(time.Second))))
	if fs.NArg() == 1 {
		report.Checks = append(report.Checks, doctorRepoCheck(fs.Arg(0), cfg))
	}
	if *jsonOut {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return doctorResultError(report)
	}
	printDoctorReport(out, report)
	return doctorResultError(report)
}

func doctorExecutableCheck(name, purpose string) doctorCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		return doctorCheck{Name: name, Status: "warn", Message: name + " not found on PATH; " + purpose + " may be unavailable"}
	}
	return doctorCheck{Name: name, Status: "ok", Message: path}
}

func doctorOllamaCheck(cfg Config, timeout time.Duration) doctorCheck {
	if err := validateOllamaURL(cfg.OllamaURL); err != nil {
		return doctorCheck{Name: "ollama", Status: "error", Message: err.Error()}
	}
	client := NewOllamaClient(cfg.OllamaURL, timeout)
	models, err := client.ListModels(context.Background())
	if err != nil {
		return doctorCheck{Name: "ollama", Status: "warn", Message: err.Error()}
	}
	if len(models) == 0 {
		return doctorCheck{Name: "ollama", Status: "warn", Message: "reachable at " + client.BaseURL + " but no models are installed"}
	}
	selected, _ := SelectModel("", cfg, models)
	return doctorCheck{Name: "ollama", Status: "ok", Message: "reachable at " + client.BaseURL + "; selected model " + selected}
}

func doctorRepoCheck(repo string, cfg Config) doctorCheck {
	result, err := ScanRepo(repo, nil, nil, cfg.MaxFileBytes)
	if err != nil {
		return doctorCheck{Name: "repo", Status: "error", Message: err.Error()}
	}
	chunks := MakeChunks(result, cfg.MaxChunkChars, "")
	status := "ok"
	if len(result.Files) == 0 {
		status = "warn"
	}
	return doctorCheck{
		Name:   "repo",
		Status: status,
		Message: fmt.Sprintf("%d files, %d skips, %d chunks, %s total",
			len(result.Files), len(result.Skips), len(chunks), formatBytes(result.TotalBytes())),
	}
}

func printDoctorReport(out io.Writer, report doctorReport) {
	fmt.Fprintf(out, "%s doctor %s (%s/%s)\n", report.Product, report.Version, report.OS, report.Arch)
	for _, check := range report.Checks {
		fmt.Fprintf(out, "%s %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Message)
	}
}

func doctorResultError(report doctorReport) error {
	for _, check := range report.Checks {
		if check.Status == "error" {
			return errf("doctor found errors")
		}
	}
	return nil
}

func cmdBook(args []string, out io.Writer) error {
	if isHelpArgs(args) {
		printBookHelp(out)
		return flag.ErrHelp
	}
	if len(args) == 0 {
		return errf("book requires subject")
	}
	plan, err := NewBookPlan(strings.Join(args, " "), time.Now())
	if err != nil {
		return err
	}
	printBookPlan(out, plan)
	return nil
}

func cmdModels(args []string, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	fs.SetOutput(out)
	ollamaURL := fs.String("ollama-url", cfg.OllamaURL, "Ollama base URL")
	timeout := fs.Float64("timeout", 10, "seconds to wait for Ollama")
	jsonOut := fs.Bool("json", false, "print machine-readable JSON")
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"json": true})); err != nil {
		return err
	}
	if err := validateOllamaURL(*ollamaURL); err != nil {
		return err
	}
	if err := validatePositiveSeconds("timeout", *timeout); err != nil {
		return err
	}
	client := NewOllamaClient(*ollamaURL, time.Duration(*timeout*float64(time.Second)))
	models, err := client.ListModels(context.Background())
	if err != nil {
		return err
	}
	selected, _ := SelectModel("", cfg, models)
	if *jsonOut {
		data, err := json.MarshalIndent(map[string]any{
			"ollama_url": client.BaseURL,
			"selected":   selected,
			"models":     models,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	if len(models) == 0 {
		fmt.Fprintln(out, "No Ollama models found.")
		return nil
	}
	for _, model := range models {
		marker := " "
		if model.Name == selected {
			marker = "*"
		}
		fmt.Fprintf(out, "%s %s\t%s\n", marker, model.Name, formatBytes(model.Size))
	}
	return nil
}

func cmdAsk(args []string, stdin io.Reader, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	fs := flag.NewFlagSet("ask", flag.ContinueOnError)
	fs.SetOutput(out)
	model := fs.String("model", cfg.Model, "Ollama model name")
	ollamaURL := fs.String("ollama-url", cfg.OllamaURL, "Ollama base URL")
	timeout := fs.Float64("timeout", float64(cfg.AskTimeoutSeconds), "seconds to wait for Ollama")
	numCtx := fs.Int("num-ctx", cfg.NumCtx, "Ollama context window tokens; 0 uses the model default")
	noStream := fs.Bool("no-stream", false, "wait for the full response before printing")
	promptFile := fs.String("prompt-file", "", "read prompt from file")
	readStdin := fs.Bool("stdin", false, "read prompt from stdin")
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"no-stream": true, "stdin": true})); err != nil {
		return err
	}
	if err := validateOllamaURL(*ollamaURL); err != nil {
		return err
	}
	if err := validatePositiveSeconds("timeout", *timeout); err != nil {
		return err
	}
	if err := validateNumCtxFlag(*numCtx); err != nil {
		return err
	}
	prompt, err := promptText(stdin, fs.Args(), *promptFile, *readStdin)
	if err != nil {
		return err
	}
	if strings.TrimSpace(prompt) == "" {
		return errf("ask requires a prompt")
	}
	client := NewOllamaClient(*ollamaURL, time.Duration(*timeout*float64(time.Second)))
	models, err := client.ListModels(context.Background())
	if err != nil {
		return err
	}
	selected, err := SelectModel(*model, cfg, models)
	if err != nil {
		return err
	}
	response, err := client.GenerateWithOptions(context.Background(), GenerateOptions{
		Model:  selected,
		Prompt: prompt,
		Stream: !*noStream,
		NumCtx: *numCtx,
	}, out)
	if err != nil {
		return err
	}
	if *noStream {
		fmt.Fprintln(out, response)
	} else {
		fmt.Fprintln(out)
	}
	return nil
}

func cmdAgent(args []string, stdin io.Reader, out io.Writer) error {
	cfg := DefaultConfig()
	if !isHelpArgs(args) {
		loaded, _, err := LoadConfig()
		if err != nil {
			return err
		}
		cfg = loaded
	}
	flags := scanFlags{maxFileBytes: cfg.MaxFileBytes, maxChunkChars: cfg.MaxChunkChars}
	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	fs.SetOutput(out)
	model := fs.String("model", cfg.Model, "Ollama model name")
	ollamaURL := fs.String("ollama-url", cfg.OllamaURL, "Ollama base URL")
	timeout := fs.Float64("timeout", float64(cfg.AgentTimeoutSeconds), "seconds to wait for Ollama")
	maxContext := fs.Int("max-context-chars", cfg.MaxContextChars, "maximum repository context characters to send to the model")
	numCtx := fs.Int("num-ctx", cfg.NumCtx, "Ollama context window tokens; 0 uses the model default")
	noStream := fs.Bool("no-stream", false, "wait for the full response before printing")
	promptFile := fs.String("prompt-file", "", "read feedback prompt from file")
	readStdin := fs.Bool("stdin", false, "read feedback prompt from stdin")
	allowRemote := fs.Bool("allow-remote-ollama", false, "allow sending scanned source context to a non-local Ollama URL")
	addScanFlags(fs, &flags)
	boolFlags := map[string]bool{"no-stream": true, "stdin": true, "allow-remote-ollama": true}
	if err := fs.Parse(interspersedArgs(args, boolFlags)); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errf("agent requires REPO and optional feedback prompt")
	}
	if err := validateOllamaURL(*ollamaURL); err != nil {
		return err
	}
	if err := validatePositiveSeconds("timeout", *timeout); err != nil {
		return err
	}
	if err := validatePositiveInt("max-context-chars", *maxContext, maxConfigContextChars); err != nil {
		return err
	}
	if err := validateNumCtxFlag(*numCtx); err != nil {
		return err
	}
	if !*allowRemote && !isLocalOllamaURL(*ollamaURL) {
		return errf("agent refuses to send scanned source to non-local Ollama URL; use --allow-remote-ollama to override")
	}
	if err := validateScanFlags(flags); err != nil {
		return err
	}
	task, err := promptText(stdin, fs.Args()[1:], *promptFile, *readStdin)
	if err != nil {
		return err
	}
	result, chunks, err := scanAndChunk(fs.Arg(0), flags, "")
	if err != nil {
		return err
	}
	client := NewOllamaClient(*ollamaURL, time.Duration(*timeout*float64(time.Second)))
	models, err := client.ListModels(context.Background())
	if err != nil {
		return err
	}
	selected, err := SelectModel(*model, cfg, models)
	if err != nil {
		return err
	}
	prompt := BuildAgentPrompt(result, chunks, AgentPromptOptions{
		Task:            task,
		MaxContextChars: *maxContext,
	})
	fmt.Fprintf(out, "Clyde agent using local model: %s\n", selected)
	fmt.Fprintf(out, "Included files: %d, chunks: %d, prompt chars: %d\n\n", len(result.Files), len(chunks), len(prompt))
	response, err := client.GenerateWithOptions(context.Background(), GenerateOptions{
		Model:  selected,
		Prompt: prompt,
		Stream: !*noStream,
		NumCtx: *numCtx,
	}, out)
	if err != nil {
		return err
	}
	if *noStream {
		fmt.Fprintln(out, response)
	} else {
		fmt.Fprintln(out)
	}
	return nil
}

func printHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde {about,help,completion,doctor,tui,config,preview,scan-report,bundle,sync,daemon,status,book,models,ask,agent} ...")
	fmt.Fprintln(out, "run clyde with no arguments in a terminal to open the TUI")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde help agent")
	fmt.Fprintln(out, "  clyde help --json")
	fmt.Fprintln(out, "  clyde --about")
	fmt.Fprintln(out, "  clyde preview . --include 'internal/**/*.go'")
	fmt.Fprintln(out, "  clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde")
	fmt.Fprintln(out, "  clyde completion powershell | Out-String | Invoke-Expression")
	fmt.Fprintln(out, "  clyde completion nushell > clyde-completions.nu")
	fmt.Fprintln(out, "  clyde doctor . --json")
	fmt.Fprintln(out, "  clyde scan-report . --json")
	fmt.Fprintln(out, "  clyde models")
	fmt.Fprintln(out, "  clyde config show")
	fmt.Fprintln(out, "  clyde ask --model qwen2.5-coder:7b --stdin")
	fmt.Fprintln(out, "  clyde agent . --model qwen2.5-coder:7b 'review this repo'")
	fmt.Fprintln(out)
	printLinks(out)
	fmt.Fprintln(out, "run `clyde --about` for product details")
}

type commandInfo struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
	Access   string   `json:"access"`
	Network  string   `json:"network"`
	Syntax   string   `json:"syntax"`
	Examples []string `json:"examples"`
}

var commandCatalog = []commandInfo{
	{Name: "about", Category: "Core", Summary: "Show Clyde product details and official links.", Access: "Read-only", Network: "None", Syntax: "clyde --about\nclyde about", Examples: []string{"clyde --about", "clyde about"}},
	{Name: "help", Category: "Core", Summary: "Show human-readable help or a JSON command catalog.", Access: "Read-only", Network: "None", Syntax: "clyde help [--json|COMMAND]", Examples: []string{"clyde help", "clyde help agent", "clyde help --json"}},
	{Name: "completion", Category: "Packaging", Summary: "Generate shell completion for Bash, Zsh, Fish, PowerShell, Elvish, Nushell, Xonsh, Tcsh, Clink, Yash, or Oil.", Access: "Read-only", Network: "None", Syntax: "clyde completion {bash|zsh|fish|powershell|pwsh|elvish|nushell|nu|xonsh|tcsh|clink|yash|oil|osh|ysh}", Examples: []string{"clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde", "clyde completion powershell | Out-String | Invoke-Expression", "clyde completion nushell > clyde-completions.nu"}},
	{Name: "doctor", Category: "Diagnostics", Summary: "Check Clyde configuration, platform, PATH tools, Ollama, and optional repo scan readiness.", Access: "Read-only", Network: "Local Ollama", Syntax: "clyde doctor [REPO] [--json] [--ollama-timeout SECONDS]", Examples: []string{"clyde doctor", "clyde doctor . --json"}},
	{Name: "tui", Category: "Interactive", Summary: "Open Clyde's dependency-free terminal UI.", Access: "Local interactive", Network: "Optional local Ollama", Syntax: "clyde tui", Examples: []string{"clyde", "clyde tui"}},
	{Name: "config", Category: "Configuration", Summary: "Show, initialize, or print the Clyde configuration path.", Access: "Reads or writes local config", Network: "None", Syntax: "clyde config {show|init|path}", Examples: []string{"clyde config show", "clyde config init", "clyde config path"}},
	{Name: "config init", Category: "Configuration", Summary: "Write Clyde's default configuration file.", Access: "Writes local config", Network: "None", Syntax: "clyde config init", Examples: []string{"clyde config init"}},
	{Name: "config path", Category: "Configuration", Summary: "Print the config file path Clyde will use.", Access: "Read-only", Network: "None", Syntax: "clyde config path", Examples: []string{"clyde config path"}},
	{Name: "config show", Category: "Configuration", Summary: "Print the effective Clyde configuration as JSON.", Access: "Read-only", Network: "None", Syntax: "clyde config show", Examples: []string{"clyde config show"}},
	{Name: "preview", Category: "Repository Bundles", Summary: "Report included and skipped repository files before bundling or upload.", Access: "Read-only", Network: "None", Syntax: "clyde preview REPO [scan flags] [--json]", Examples: []string{"clyde preview .", "clyde preview . --include 'internal/**/*.go' --json"}},
	{Name: "scan-report", Category: "Repository Bundles", Summary: "Summarize repository scan shape with largest files, extensions, skip reasons, and chunk counts.", Access: "Read-only", Network: "None", Syntax: "clyde scan-report REPO [scan flags] [--json] [--top N]", Examples: []string{"clyde scan-report .", "clyde scan-report . --json", "clyde scan-report . --top 20"}},
	{Name: "bundle", Category: "Repository Bundles", Summary: "Write a local manifest and source chunks for review.", Access: "Writes local files", Network: "None", Syntax: "clyde bundle REPO --out DIR [scan flags]", Examples: []string{"clyde bundle . --out .clyde/out"}},
	{Name: "sync", Category: "NotebookLM Sync", Summary: "Upload scanned repository chunks to NotebookLM after explicit approval.", Access: "Uploads repository chunks", Network: "NotebookLM backend", Syntax: "clyde sync REPO --notebook-id ID --approve-upload", Examples: []string{"clyde sync . --notebook-id nb --approve-upload"}},
	{Name: "daemon", Category: "Status", Summary: "Start the localhost-only JSON-RPC status daemon.", Access: "Starts local server", Network: "Localhost only", Syntax: "clyde daemon [--host HOST] [--port PORT]", Examples: []string{"clyde daemon"}},
	{Name: "status", Category: "Status", Summary: "Read Clyde daemon progress for a job.", Access: "Read-only", Network: "Localhost by default", Syntax: "clyde status [--job-id ID] [--json] [--watch]", Examples: []string{"clyde status", "clyde status --job-id clyde-sync"}},
	{Name: "book", Category: "NotebookLM Sync", Summary: "Generate a dated NotebookLM book title and slug.", Access: "Read-only", Network: "None", Syntax: "clyde book SUBJECT...", Examples: []string{"clyde book Clyde self feedback"}},
	{Name: "models", Category: "Local Models", Summary: "List local Ollama models and the selected default.", Access: "Read-only", Network: "Local Ollama", Syntax: "clyde models [--json]", Examples: []string{"clyde models", "clyde models --json"}},
	{Name: "ask", Category: "Local Models", Summary: "Send a direct prompt to the selected local Ollama model.", Access: "Sends prompt to Ollama", Network: "Configured Ollama", Syntax: "clyde ask [flags] PROMPT", Examples: []string{"clyde ask 'Review this function'", "cat prompt.md | clyde ask --stdin"}},
	{Name: "agent", Category: "Local Models", Summary: "Scan repository context and ask a local model for feedback.", Access: "Reads repo and sends selected context", Network: "Local Ollama by default", Syntax: "clyde agent REPO [scan flags] [PROMPT]", Examples: []string{"clyde agent . 'Review this repo'", "clyde agent . --include 'internal/**/*.go' --prompt-file review.md"}},
}

func cmdHelp(args []string, stdin io.Reader, out, errOut io.Writer) error {
	if len(args) == 0 {
		printHelp(out)
		return nil
	}
	if len(args) == 1 && args[0] == "--json" {
		data, err := json.MarshalIndent(map[string]any{
			"product":  productName,
			"version":  productVersion,
			"home":     productHomeURL,
			"help":     productHelpURL,
			"github":   productGitHubURL,
			"commands": commandCatalog,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	}
	target := strings.Join(args, " ")
	if info, ok := commandByName(target); ok && strings.Contains(target, " ") {
		printCommandInfo(out, info)
		return nil
	}
	if len(args) != 1 {
		return errf("help accepts one command, or one command plus a subcommand")
	}
	switch target {
	case "about":
		printAbout(out)
		return nil
	case "help":
		printHelpCommand(out)
		return nil
	case "completion":
		printCompletionHelp(out)
		return nil
	case "doctor":
		printDoctorHelp(out)
		return nil
	case "scan-report":
		printScanReportHelp(out)
		return nil
	case "config":
		printConfigHelp(out)
		return nil
	case "tui":
		printTUIHelp(out)
		return nil
	case "book":
		printBookHelp(out)
		return nil
	}
	return run([]string{target, "--help"}, stdin, out, errOut)
}

func printHelpCommand(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde help [--json|command]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Show Clyde's top-level help, command-specific help, or a JSON command catalog.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde help")
	fmt.Fprintln(out, "  clyde help agent")
	fmt.Fprintln(out, "  clyde help --json")
}

func printCompletionHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde completion {bash|zsh|fish|powershell|pwsh|elvish|nushell|nu|xonsh|tcsh|clink|yash|oil|osh|ysh}")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Generate shell completion scripts for Clyde.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde")
	fmt.Fprintln(out, "  clyde completion bash > ~/.clyde-completion.bash")
	fmt.Fprintln(out, "  clyde completion fish > ~/.config/fish/completions/clyde.fish")
	fmt.Fprintln(out, "  clyde completion powershell | Out-String | Invoke-Expression")
	fmt.Fprintln(out, "  clyde completion elvish > ~/.elvish/completions/clyde.elv")
	fmt.Fprintln(out, "  clyde completion nushell > clyde-completions.nu")
	fmt.Fprintln(out, "  clyde completion xonsh > ~/.xonshrc.d/clyde-completions.xsh")
	fmt.Fprintln(out, "  clyde completion tcsh >> ~/.tcshrc")
	fmt.Fprintln(out, "  clyde completion clink > %LOCALAPPDATA%\\clink\\clyde.lua")
	fmt.Fprintln(out, "  clyde completion yash >> ~/.yashrc")
	fmt.Fprintln(out, "  clyde completion oil > ~/.config/oils/clyde-completions.sh")
}

func printDoctorHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde doctor [repo] [--json] [--ollama-timeout seconds]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Check Clyde's local environment without uploading data.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde doctor")
	fmt.Fprintln(out, "  clyde doctor .")
	fmt.Fprintln(out, "  clyde doctor . --json")
}

func printScanReportHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde scan-report REPO [scan flags] [--json] [--top N]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Summarize repository scan shape without writing bundles or uploading data.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde scan-report .")
	fmt.Fprintln(out, "  clyde scan-report . --json")
	fmt.Fprintln(out, "  clyde scan-report . --include \"internal/**/*.go\" --top 20")
}

func printConfigHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde config {show|init|path}")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Manage Clyde's JSON configuration file.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde config show")
	fmt.Fprintln(out, "  clyde config init")
	fmt.Fprintln(out, "  clyde config path")
}

func printTUIHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde tui")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Open Clyde's dependency-free terminal UI.")
	fmt.Fprintln(out, "Running `clyde` with no arguments in an interactive terminal also opens the TUI.")
}

func printBookHelp(out io.Writer) {
	fmt.Fprintln(out, "usage: clyde book SUBJECT...")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Generate a dated NotebookLM book title and slug.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "example:")
	fmt.Fprintln(out, "  clyde book Clyde self feedback")
}

func commandByName(name string) (commandInfo, bool) {
	for _, command := range commandCatalog {
		if command.Name == name {
			return command, true
		}
	}
	return commandInfo{}, false
}

func printCommandInfo(out io.Writer, command commandInfo) {
	fmt.Fprintf(out, "usage: %s\n", command.Syntax)
	fmt.Fprintln(out)
	fmt.Fprintln(out, command.Summary)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "category: %s\n", command.Category)
	fmt.Fprintf(out, "access: %s\n", command.Access)
	fmt.Fprintf(out, "network: %s\n", command.Network)
	if len(command.Examples) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "examples:")
		for _, example := range command.Examples {
			fmt.Fprintf(out, "  %s\n", example)
		}
	}
}

func printAbout(out io.Writer) {
	fmt.Fprintf(out, "%s %s\n", productName, productVersion)
	fmt.Fprintln(out, productDescription)
	fmt.Fprintf(out, "Created by %s\n", productCreator)
	fmt.Fprintln(out)
	printLinks(out)
}

func printLinks(out io.Writer) {
	fmt.Fprintf(out, "Home: %s\n", productHomeURL)
	fmt.Fprintf(out, "Help: %s\n", productHelpURL)
	fmt.Fprintf(out, "GitHub: %s\n", productGitHubURL)
	fmt.Fprintf(out, "PayCal Technologies: %s\n", productCreatorURL)
}

func cmdCompletion(args []string, out io.Writer) error {
	if isHelpArgs(args) {
		printCompletionHelp(out)
		return flag.ErrHelp
	}
	if len(args) != 1 {
		return errf("completion requires shell: bash, zsh, fish, powershell, pwsh, elvish, nushell, xonsh, tcsh, clink, yash, or oil")
	}
	switch args[0] {
	case "bash":
		io.WriteString(out, bashCompletionScript)
	case "zsh":
		io.WriteString(out, zshCompletionScript)
	case "fish":
		io.WriteString(out, fishCompletionScript)
	case "powershell", "pwsh":
		io.WriteString(out, powerShellCompletionScript)
	case "elvish":
		io.WriteString(out, elvishCompletionScript)
	case "nushell", "nu":
		io.WriteString(out, nushellCompletionScript)
	case "xonsh":
		io.WriteString(out, xonshCompletionScript)
	case "tcsh":
		io.WriteString(out, tcshCompletionScript)
	case "clink":
		io.WriteString(out, clinkCompletionScript)
	case "yash":
		io.WriteString(out, yashCompletionScript)
	case "oil", "osh", "ysh":
		io.WriteString(out, oilCompletionScript)
	default:
		return errf("unsupported shell %q; expected bash, zsh, fish, powershell, pwsh, elvish, nushell, xonsh, tcsh, clink, yash, or oil", args[0])
	}
	return nil
}

func cmdConfig(args []string, out io.Writer) error {
	if len(args) == 0 {
		args = []string{"show"}
	}
	if isHelpArgs(args) {
		printConfigHelp(out)
		return flag.ErrHelp
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	switch args[0] {
	case "path":
		fmt.Fprintln(out, path)
		return nil
	case "show":
		cfg, path, err := LoadConfig()
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(map[string]any{
			"path":   path,
			"config": cfg,
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(out, string(data))
		return nil
	case "init":
		if _, err := os.Stat(path); err == nil {
			return errf("config already exists: %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := WriteDefaultConfig(path); err != nil {
			return err
		}
		fmt.Fprintf(out, "Wrote: %s\n", path)
		return nil
	default:
		return errf("unknown config command: %s", args[0])
	}
}

func scanAndChunk(repo string, flags scanFlags, bookTitle string) (ScanResult, []ChunkRecord, error) {
	result, err := ScanRepo(repo, flags.include, flags.exclude, flags.maxFileBytes)
	if err != nil {
		return ScanResult{}, nil, err
	}
	return result, MakeChunks(result, flags.maxChunkChars, bookTitle), nil
}

func planFromArgs(subject, bookTitle string) (BookPlan, error) {
	if bookTitle != "" {
		return BookPlanFromTitle(bookTitle)
	}
	return NewBookPlan(subject, time.Now())
}

func printBookPlan(out io.Writer, plan BookPlan) {
	fmt.Fprintf(out, "Book title: %s\n", plan.Title())
	fmt.Fprintf(out, "Book slug: %s\n", plan.Slug())
}

func printSummary(out io.Writer, result ScanResult, chunkCount int, flags scanFlags) {
	fmt.Fprintf(out, "Repo: %s\n", result.Repo)
	fmt.Fprintf(out, "Included files: %d\n", len(result.Files))
	fmt.Fprintf(out, "Skipped files: %d\n", len(result.Skips))
	fmt.Fprintf(out, "Total included bytes: %d (%s)\n", result.TotalBytes(), formatBytes(result.TotalBytes()))
	fmt.Fprintf(out, "Chunks: %d\n", chunkCount)
	fmt.Fprintf(out, "Max file size: %d bytes (%s)\n", flags.maxFileBytes, formatBytes(flags.maxFileBytes))
	fmt.Fprintf(out, "Max chunk size: %d chars\n", flags.maxChunkChars)
	if len(flags.include) > 0 {
		fmt.Fprintf(out, "Include globs: %s\n", strings.Join(flags.include, ", "))
	}
	if len(flags.exclude) > 0 {
		fmt.Fprintf(out, "Extra exclude globs: %s\n", strings.Join(flags.exclude, ", "))
	}
	if len(result.Skips) > 0 {
		counts := map[string]int{}
		for _, skip := range result.Skips {
			counts[skip.Reason]++
		}
		fmt.Fprintln(out, "Skip reasons:")
		for reason, count := range counts {
			fmt.Fprintf(out, "  %s: %d\n", reason, count)
		}
	}
}

func printPreviewJSON(out io.Writer, result ScanResult, chunkCount int, flags scanFlags) error {
	files := make([]map[string]any, 0, len(result.Files))
	for _, file := range result.Files {
		files = append(files, map[string]any{
			"path":   file.Rel,
			"size":   file.Size,
			"sha256": file.SHA256,
		})
	}
	skips := make([]map[string]any, 0, len(result.Skips))
	for _, skip := range result.Skips {
		skips = append(skips, map[string]any{"path": skip.Path, "reason": skip.Reason})
	}
	data, err := json.MarshalIndent(map[string]any{
		"repo":            result.Repo,
		"included_files":  len(result.Files),
		"skipped_files":   len(result.Skips),
		"total_bytes":     result.TotalBytes(),
		"chunk_count":     chunkCount,
		"max_file_bytes":  flags.maxFileBytes,
		"max_chunk_chars": flags.maxChunkChars,
		"include":         []string(flags.include),
		"exclude":         []string(flags.exclude),
		"files":           files,
		"skips":           skips,
	}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))
	return nil
}

func validateScanFlags(flags scanFlags) error {
	if flags.maxFileBytes <= 0 {
		return errf("max-file-bytes must be greater than 0")
	}
	if flags.maxChunkChars <= 0 {
		return errf("max-chunk-chars must be greater than 0")
	}
	return nil
}

func validatePositiveSeconds(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return errf("--%s must be greater than 0", name)
	}
	if value > maxCLISeconds {
		return errf("--%s is too large; maximum is %d seconds", name, maxCLISeconds)
	}
	return nil
}

func validateNumCtxFlag(value int) error {
	if value < 0 {
		return errf("--num-ctx must be zero or greater")
	}
	if value > maxConfigNumCtx {
		return errf("--num-ctx is too large; maximum is %d", maxConfigNumCtx)
	}
	return nil
}

func validateCommandFlag(name, value string) error {
	if strings.ContainsRune(value, 0) {
		return errf("--%s must not contain NUL bytes", name)
	}
	if len(shellFields(value)) == 0 {
		return errf("--%s must not be empty", name)
	}
	return nil
}

func isHelpArgs(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errf("must not be empty")
	}
	*m = append(*m, value)
	return nil
}

func shellFields(value string) []string {
	return strings.Fields(value)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func promptText(stdin io.Reader, args []string, promptFile string, readStdin bool) (string, error) {
	if promptFile != "" && readStdin {
		return "", errf("--prompt-file and --stdin are mutually exclusive")
	}
	if promptFile != "" {
		file, err := os.Open(promptFile)
		if err != nil {
			return "", err
		}
		defer file.Close()
		data, err := readLimitedText(file, maxPromptInputBytes, "--prompt-file")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	if readStdin {
		data, err := readLimitedText(stdin, maxPromptInputBytes, "--stdin")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return strings.Join(args, " "), nil
}

func readLimitedText(reader io.Reader, limit int64, label string) ([]byte, error) {
	limited := io.LimitReader(reader, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errf("%s input is too large; maximum is %d bytes", label, limit)
	}
	return data, nil
}

func isLocalOllamaURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}

func interspersedArgs(args []string, boolFlags map[string]bool) []string {
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if name == "h" || name == "help" {
			continue
		}
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			continue
		}
		if boolFlags[name] {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

const bashCompletionScript = `# bash completion for clyde
_clyde_completion() {
  local cur prev commands
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  commands="about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
    return 0
  fi

  case "${COMP_WORDS[1]}" in
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh" -- "${cur}") )
      ;;
    help)
      COMPREPLY=( $(compgen -W "about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json" -- "${cur}") )
      ;;
    doctor)
      COMPREPLY=( $(compgen -W "--json --ollama-timeout --help" -- "${cur}") )
      ;;
    config)
      COMPREPLY=( $(compgen -W "path show init" -- "${cur}") )
      ;;
    preview)
      COMPREPLY=( $(compgen -W "--include --exclude --max-file-bytes --max-chunk-chars --show-files --show-skips --json --help" -- "${cur}") )
      ;;
    scan-report)
      COMPREPLY=( $(compgen -W "--include --exclude --max-file-bytes --max-chunk-chars --json --top --help" -- "${cur}") )
      ;;
    bundle)
      COMPREPLY=( $(compgen -W "--out --subject --book-title --include --exclude --max-file-bytes --max-chunk-chars --help" -- "${cur}") )
      ;;
    sync)
      COMPREPLY=( $(compgen -W "--notebook-id --notebook-url --approve-upload --backend --mcp-command --nlm-command --delete-existing-sources --mcp-timeout --status-url --quiet-progress --job-id --subject --book-title --include --exclude --max-file-bytes --max-chunk-chars --help" -- "${cur}") )
      ;;
    models)
      COMPREPLY=( $(compgen -W "--ollama-url --timeout --json --help" -- "${cur}") )
      ;;
    ask)
      COMPREPLY=( $(compgen -W "--model --ollama-url --timeout --num-ctx --no-stream --prompt-file --stdin --help" -- "${cur}") )
      ;;
    agent)
      COMPREPLY=( $(compgen -W "--model --ollama-url --timeout --max-context-chars --num-ctx --no-stream --prompt-file --stdin --allow-remote-ollama --include --exclude --max-file-bytes --max-chunk-chars --help" -- "${cur}") )
      ;;
    daemon)
      COMPREPLY=( $(compgen -W "--host --port --help" -- "${cur}") )
      ;;
    status)
      COMPREPLY=( $(compgen -W "--host --port --job-id --json --watch --interval --help" -- "${cur}") )
      ;;
  esac
}
complete -F _clyde_completion clyde
`

const zshCompletionScript = `#compdef clyde
_clyde() {
  local -a commands
  commands=(
    'about:show product details and links'
    'help:show command help or JSON command catalog'
    'completion:print shell completion script'
    'doctor:check local Clyde environment'
    'tui:open the terminal UI'
    'config:manage Clyde config'
    'preview:show files Clyde would scan'
    'bundle:write manifest.json and chunks.jsonl'
    'sync:upload chunks to NotebookLM'
    'daemon:serve sync status'
    'status:read sync status'
    'book:plan a dated NotebookLM book name'
    'models:list local Ollama models'
    'ask:ask a local Ollama model'
    'agent:scan repo and ask a local Ollama model'
  )

  _arguments -C \
    '1:command:->command' \
    '*::arg:->args'

  case $state in
    command)
      _describe 'command' commands
      ;;
    args)
      case $words[2] in
        completion) _values 'shell' bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh ;;
        help) _values 'command' about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json ;;
        doctor) _arguments '--json' '--ollama-timeout=[]' '--help' ;;
        config) _values 'config command' path show init ;;
        preview) _arguments '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--show-files=[]' '--show-skips=[]' '--json' '--help' ;;
        scan-report) _arguments '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--json' '--top=[]' '--help' ;;
        bundle) _arguments '--out=[]' '--subject=[]' '--book-title=[]' '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--help' ;;
        sync) _arguments '--notebook-id=[]' '--notebook-url=[]' '--approve-upload' '--backend=[mcp or nlm]:backend:(mcp nlm)' '--mcp-command=[]' '--nlm-command=[]' '--delete-existing-sources' '--mcp-timeout=[]' '--status-url=[]' '--quiet-progress' '--job-id=[]' '--subject=[]' '--book-title=[]' '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--help' ;;
        models) _arguments '--ollama-url=[]' '--timeout=[]' '--json' '--help' ;;
        ask) _arguments '--model=[]' '--ollama-url=[]' '--timeout=[]' '--num-ctx=[]' '--no-stream' '--prompt-file=[]' '--stdin' '--help' ;;
        agent) _arguments '--model=[]' '--ollama-url=[]' '--timeout=[]' '--max-context-chars=[]' '--num-ctx=[]' '--no-stream' '--prompt-file=[]' '--stdin' '--allow-remote-ollama' '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--help' ;;
        daemon) _arguments '--host=[]' '--port=[]' '--help' ;;
        status) _arguments '--host=[]' '--port=[]' '--job-id=[]' '--json' '--watch' '--interval=[]' '--help' ;;
      esac
      ;;
  esac
}
_clyde "$@"
`

const fishCompletionScript = `# fish completion for clyde
complete -c clyde -f
complete -c clyde -n "__fish_use_subcommand" -a "about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent"
complete -c clyde -n "__fish_seen_subcommand_from completion" -a "bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh"
complete -c clyde -n "__fish_seen_subcommand_from help" -a "about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json"
complete -c clyde -n "__fish_seen_subcommand_from doctor" -l json
complete -c clyde -n "__fish_seen_subcommand_from doctor" -l ollama-timeout -r
complete -c clyde -n "__fish_seen_subcommand_from config" -a "path show init"
complete -c clyde -n "__fish_seen_subcommand_from preview" -l include -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l exclude -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l max-file-bytes -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l max-chunk-chars -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l show-files -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l show-skips -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l json
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l include -r
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l exclude -r
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l max-file-bytes -r
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l max-chunk-chars -r
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l json
complete -c clyde -n "__fish_seen_subcommand_from scan-report" -l top -r
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l out -r
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l subject -r
complete -c clyde -n "__fish_seen_subcommand_from bundle" -l book-title -r
complete -c clyde -n "__fish_seen_subcommand_from sync" -l notebook-id -r
complete -c clyde -n "__fish_seen_subcommand_from sync" -l notebook-url -r
complete -c clyde -n "__fish_seen_subcommand_from sync" -l approve-upload
complete -c clyde -n "__fish_seen_subcommand_from sync" -l backend -xa "mcp nlm"
complete -c clyde -n "__fish_seen_subcommand_from sync" -l delete-existing-sources
complete -c clyde -n "__fish_seen_subcommand_from models" -l ollama-url -r
complete -c clyde -n "__fish_seen_subcommand_from models" -l timeout -r
complete -c clyde -n "__fish_seen_subcommand_from models" -l json
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l model -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l ollama-url -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l timeout -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l num-ctx -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l no-stream
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l prompt-file -r
complete -c clyde -n "__fish_seen_subcommand_from ask agent" -l stdin
complete -c clyde -n "__fish_seen_subcommand_from agent" -l allow-remote-ollama
complete -c clyde -n "__fish_seen_subcommand_from agent" -l max-context-chars -r
complete -c clyde -n "__fish_seen_subcommand_from daemon status" -l host -r
complete -c clyde -n "__fish_seen_subcommand_from daemon status" -l port -r
complete -c clyde -n "__fish_seen_subcommand_from status" -l watch
complete -c clyde -n "__fish_seen_subcommand_from status" -l interval -r
`

const powerShellCompletionScript = `# PowerShell completion for clyde
Register-ArgumentCompleter -Native -CommandName clyde -ScriptBlock {
  param($wordToComplete, $commandAst, $cursorPosition)

  $commands = @(
    'about', 'help', 'completion', 'doctor', 'tui', 'config', 'preview', 'scan-report', 'bundle',
    'sync', 'daemon', 'status', 'book', 'models', 'ask', 'agent'
  )
  $commandDescriptions = @{
    about = 'show product details and links'
    help = 'show command help or JSON command catalog'
    completion = 'print shell completion script'
    doctor = 'check local Clyde environment'
    tui = 'open the terminal UI'
    config = 'manage Clyde config'
    preview = 'show files Clyde would scan'
    bundle = 'write manifest.json and chunks.jsonl'
    sync = 'upload chunks to NotebookLM'
    daemon = 'serve sync status'
    status = 'read sync status'
    book = 'plan a dated NotebookLM book name'
    models = 'list local Ollama models'
    ask = 'ask a local Ollama model'
    agent = 'scan repo and ask a local Ollama model'
  }
  $subcommands = @{
		completion = @('bash', 'zsh', 'fish', 'powershell', 'pwsh', 'elvish', 'nushell', 'nu', 'xonsh', 'tcsh', 'clink', 'yash', 'oil', 'osh', 'ysh')
    help = @('about', 'help', 'completion', 'doctor', 'tui', 'config', 'preview', 'scan-report', 'bundle', 'sync', 'daemon', 'status', 'book', 'models', 'ask', 'agent', '--json')
    config = @('path', 'show', 'init')
  }
  $flags = @{
    preview = @('--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--show-files', '--show-skips', '--json', '--help')
    'scan-report' = @('--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--json', '--top', '--help')
    doctor = @('--json', '--ollama-timeout', '--help')
    bundle = @('--out', '--subject', '--book-title', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--help')
    sync = @('--notebook-id', '--notebook-url', '--approve-upload', '--backend', '--mcp-command', '--nlm-command', '--delete-existing-sources', '--mcp-timeout', '--status-url', '--quiet-progress', '--job-id', '--subject', '--book-title', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--help')
    models = @('--ollama-url', '--timeout', '--json', '--help')
    ask = @('--model', '--ollama-url', '--timeout', '--num-ctx', '--no-stream', '--prompt-file', '--stdin', '--help')
    agent = @('--model', '--ollama-url', '--timeout', '--max-context-chars', '--num-ctx', '--no-stream', '--prompt-file', '--stdin', '--allow-remote-ollama', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--help')
    daemon = @('--host', '--port', '--help')
    status = @('--host', '--port', '--job-id', '--json', '--watch', '--interval', '--help')
  }
  $flagValues = @{
    '--backend' = @('mcp', 'nlm')
  }

  $words = $commandAst.CommandElements | ForEach-Object { $_.ToString() }
  if ($words.Count -le 1) {
    $candidates = $commands
  } else {
    $command = $words[1]
    $previous = if ($words.Count -gt 1) { $words[$words.Count - 2] } else { '' }
    if ($flagValues.ContainsKey($previous)) {
      $candidates = $flagValues[$previous]
    } elseif ($subcommands.ContainsKey($command)) {
      $candidates = $subcommands[$command]
    } elseif ($flags.ContainsKey($command)) {
      $candidates = $flags[$command]
    } else {
      $candidates = @()
    }
  }

  $candidates |
    Where-Object { $_ -like "$wordToComplete*" } |
    ForEach-Object {
      $description = if ($commandDescriptions.ContainsKey($_)) { $commandDescriptions[$_] } else { $_ }
      [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $description)
    }
}
`

const elvishCompletionScript = `# Elvish completion for clyde
edit:completion:arg-completer[clyde] = [@words]{
  var commands = [about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent]
  var shells = [bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh]
  var config-subcommands = [path show init]
  var common-scan-flags = [--include --exclude --max-file-bytes --max-chunk-chars]
  var flags = [
    &preview=[--include --exclude --max-file-bytes --max-chunk-chars --show-files --show-skips --json --help]
    &scan-report=[--include --exclude --max-file-bytes --max-chunk-chars --json --top --help]
    &doctor=[--json --ollama-timeout --help]
    &bundle=[--out --subject --book-title --include --exclude --max-file-bytes --max-chunk-chars --help]
    &sync=[--notebook-id --notebook-url --approve-upload --backend --mcp-command --nlm-command --delete-existing-sources --mcp-timeout --status-url --quiet-progress --job-id --subject --book-title --include --exclude --max-file-bytes --max-chunk-chars --help]
    &models=[--ollama-url --timeout --json --help]
    &ask=[--model --ollama-url --timeout --num-ctx --no-stream --prompt-file --stdin --help]
    &agent=[--model --ollama-url --timeout --max-context-chars --num-ctx --no-stream --prompt-file --stdin --allow-remote-ollama --include --exclude --max-file-bytes --max-chunk-chars --help]
    &daemon=[--host --port --help]
    &status=[--host --port --job-id --json --watch --interval --help]
  ]
  var stem = $words[-1]
  var choices = []
  if (== (count $words) 2) {
    set choices = $commands
  } elif (== $words[1] completion) {
    set choices = $shells
  } elif (== $words[1] help) {
    set choices = [about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json]
  } elif (== $words[1] config) {
    set choices = $config-subcommands
  } elif (has-key $flags $words[1]) {
    if (== $words[-2] --backend) {
      set choices = [mcp nlm]
    } else {
      set choices = $flags[$words[1]]
    }
  } else {
    set choices = $common-scan-flags
  }
  put $@choices | each [choice]{ if (has-prefix $choice $stem) { put $choice } }
}
`

const nushellCompletionScript = `# Nushell completion for clyde
def "nu-complete clyde commands" [] {
  [about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent]
}

def "nu-complete clyde shells" [] {
  [bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh]
}

def "nu-complete clyde config" [] {
  [path show init]
}

def "nu-complete clyde backend" [] {
  [mcp nlm]
}

extern "clyde" [
  command?: string@"nu-complete clyde commands"
  subcommand?: string
  --include: string
  --exclude: string
  --max-file-bytes: int
  --max-chunk-chars: int
  --show-files: int
  --show-skips: int
  --top: int
  --json
  --ollama-timeout: int
  --out: string
  --subject: string
  --book-title: string
  --notebook-id: string
  --notebook-url: string
  --approve-upload
  --backend: string@"nu-complete clyde backend"
  --mcp-command: string
  --nlm-command: string
  --delete-existing-sources
  --mcp-timeout: int
  --status-url: string
  --quiet-progress
  --job-id: string
  --model: string
  --ollama-url: string
  --timeout: int
  --num-ctx: int
  --no-stream
  --prompt-file: string
  --stdin
  --allow-remote-ollama
  --host: string
  --port: int
  --watch
  --interval: int
  --help
]
`

const xonshCompletionScript = `# Xonsh completion for clyde
from xonsh.completers.completer import add_one_completer

_CLYDE_COMMANDS = {
    'about', 'help', 'completion', 'doctor', 'tui', 'config', 'preview', 'scan-report', 'bundle',
    'sync', 'daemon', 'status', 'book', 'models', 'ask', 'agent',
}
_CLYDE_SHELLS = {
    'bash', 'zsh', 'fish', 'powershell', 'pwsh', 'elvish', 'nushell', 'nu',
    'xonsh', 'tcsh', 'clink', 'yash', 'oil', 'osh', 'ysh',
}
_CLYDE_FLAGS = {
    'preview': {'--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--show-files', '--show-skips', '--json', '--help'},
    'scan-report': {'--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--json', '--top', '--help'},
    'doctor': {'--json', '--ollama-timeout', '--help'},
    'bundle': {'--out', '--subject', '--book-title', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--help'},
    'sync': {'--notebook-id', '--notebook-url', '--approve-upload', '--backend', '--mcp-command', '--nlm-command', '--delete-existing-sources', '--mcp-timeout', '--status-url', '--quiet-progress', '--job-id', '--subject', '--book-title', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--help'},
    'models': {'--ollama-url', '--timeout', '--json', '--help'},
    'ask': {'--model', '--ollama-url', '--timeout', '--num-ctx', '--no-stream', '--prompt-file', '--stdin', '--help'},
    'agent': {'--model', '--ollama-url', '--timeout', '--max-context-chars', '--num-ctx', '--no-stream', '--prompt-file', '--stdin', '--allow-remote-ollama', '--include', '--exclude', '--max-file-bytes', '--max-chunk-chars', '--help'},
    'daemon': {'--host', '--port', '--help'},
    'status': {'--host', '--port', '--job-id', '--json', '--watch', '--interval', '--help'},
}

def _clyde_completer(prefix, line, begidx, endidx, ctx):
    parts = line[:endidx].split()
    if len(parts) <= 1:
        candidates = _CLYDE_COMMANDS
    else:
        command = parts[1]
        previous = parts[-2] if len(parts) > 1 else ''
        if previous == '--backend':
            candidates = {'mcp', 'nlm'}
        elif command == 'completion':
            candidates = _CLYDE_SHELLS
        elif command == 'help':
            candidates = _CLYDE_COMMANDS | {'--json'}
        elif command == 'config':
            candidates = {'path', 'show', 'init'}
        else:
            candidates = _CLYDE_FLAGS.get(command, set())
    return {candidate for candidate in candidates if candidate.startswith(prefix)}

add_one_completer('clyde', _clyde_completer, 'start')
`

const tcshCompletionScript = `# tcsh completion for clyde
complete clyde \
  'p/1/(about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent)/' \
  'n/completion/(bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh)/' \
  'n/help/(about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json)/' \
  'n/config/(path show init)/' \
  'n/--backend/(mcp nlm)/' \
  'c/--/(--include --exclude --max-file-bytes --max-chunk-chars --show-files --show-skips --json --top --ollama-timeout --out --subject --book-title --notebook-id --notebook-url --approve-upload --backend --mcp-command --nlm-command --delete-existing-sources --mcp-timeout --status-url --quiet-progress --job-id --model --ollama-url --timeout --num-ctx --no-stream --prompt-file --stdin --allow-remote-ollama --host --port --watch --interval --help)/'
`

const clinkCompletionScript = `-- Clink completion for clyde
local commands = {
  "about", "help", "completion", "doctor", "tui", "config", "preview", "scan-report", "bundle",
  "sync", "daemon", "status", "book", "models", "ask", "agent"
}
local shells = {
  "bash", "zsh", "fish", "powershell", "pwsh", "elvish", "nushell", "nu",
  "xonsh", "tcsh", "clink", "yash", "oil", "osh", "ysh"
}
local flags = {
  "--include", "--exclude", "--max-file-bytes", "--max-chunk-chars",
  "--show-files", "--show-skips", "--json", "--top", "--out", "--subject",
  "--book-title", "--notebook-id", "--notebook-url", "--approve-upload",
  "--backend", "--mcp-command", "--nlm-command", "--delete-existing-sources",
  "--mcp-timeout", "--status-url", "--quiet-progress", "--job-id", "--model",
  "--ollama-url", "--timeout", "--num-ctx", "--no-stream", "--prompt-file",
  "--stdin", "--allow-remote-ollama", "--host", "--port", "--watch",
  "--interval", "--ollama-timeout", "--help"
}

local parser = clink.argmatcher("clyde")
parser:addarg(commands)
parser:addarg({
  fromhistory = false,
  function(word, word_index, line_state, match_builder)
    local command = line_state:getword(2)
    local previous = line_state:getword(word_index - 1)
    local choices = flags
    if previous == "--backend" then
      choices = {"mcp", "nlm"}
    elseif command == "completion" then
      choices = shells
    elseif command == "help" then
      choices = {"about", "help", "completion", "doctor", "tui", "config", "preview", "scan-report", "bundle", "sync", "daemon", "status", "book", "models", "ask", "agent", "--json"}
    elseif command == "config" then
      choices = {"path", "show", "init"}
    end
    for _, choice in ipairs(choices) do
      if choice:sub(1, #word) == word then
        match_builder:addmatch(choice)
      end
    end
    return true
  end
})
`

const yashCompletionScript = `# Yash completion for clyde
function completion//argument/clyde {
  local words="${COMP_WORDS[*]}"
  local current="${COMP_WORDS[$COMP_CWORD]}"
  local command="${COMP_WORDS[1]}"
  local previous="${COMP_WORDS[$((COMP_CWORD - 1))]}"
  local candidates
  case "$command:$previous" in
    completion:*) candidates="bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh" ;;
    help:*) candidates="about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --json" ;;
    config:*) candidates="path show init" ;;
    *:--backend) candidates="mcp nlm" ;;
    *) candidates="about help completion doctor tui config preview scan-report bundle sync daemon status book models ask agent --include --exclude --max-file-bytes --max-chunk-chars --show-files --show-skips --json --top --ollama-timeout --out --subject --book-title --notebook-id --notebook-url --approve-upload --backend --mcp-command --nlm-command --delete-existing-sources --mcp-timeout --status-url --quiet-progress --job-id --model --ollama-url --timeout --num-ctx --no-stream --prompt-file --stdin --allow-remote-ollama --host --port --watch --interval --help" ;;
  esac
  for candidate in $candidates; do
    case "$candidate" in
      "$current"*) printf '%s\n' "$candidate" ;;
    esac
  done
}
`

const oilCompletionScript = `# Oil/OSH/YSH completion for clyde
# OSH runs Bash completion scripts, so Clyde uses its Bash completer for Oil-family shells.
` + bashCompletionScript
