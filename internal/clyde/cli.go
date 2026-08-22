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
	"path/filepath"
	"strings"
	"time"
)

const (
	productName        = "Clyde"
	productVersion     = "0.1.0"
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
	case "completion":
		return cmdCompletion(args[1:], stdout)
	case "tui":
		return RunTUI(stdin, stdout, stderr)
	case "config":
		return cmdConfig(args[1:], stdout)
	case "preview":
		return cmdPreview(args[1:], stdout)
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
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
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

func cmdBundle(args []string, out io.Writer) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
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
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
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

func cmdBook(args []string, out io.Writer) error {
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
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
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
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
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
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
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
	fmt.Fprintln(out, "usage: clyde {about,completion,tui,config,preview,bundle,sync,daemon,status,book,models,ask,agent} ...")
	fmt.Fprintln(out, "run clyde with no arguments in a terminal to open the TUI")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "examples:")
	fmt.Fprintln(out, "  clyde preview . --include 'internal/**/*.go'")
	fmt.Fprintln(out, "  clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde")
	fmt.Fprintln(out, "  clyde models")
	fmt.Fprintln(out, "  clyde config show")
	fmt.Fprintln(out, "  clyde ask --model qwen2.5-coder:7b --stdin")
	fmt.Fprintln(out, "  clyde agent . --model qwen2.5-coder:7b 'review this repo'")
	fmt.Fprintln(out)
	printLinks(out)
	fmt.Fprintln(out, "run `clyde --about` for product details")
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
	if len(args) != 1 {
		return errf("completion requires shell: bash, zsh, or fish")
	}
	switch args[0] {
	case "bash":
		fmt.Fprint(out, bashCompletionScript)
	case "zsh":
		fmt.Fprint(out, zshCompletionScript)
	case "fish":
		fmt.Fprint(out, fishCompletionScript)
	default:
		return errf("unsupported shell %q; expected bash, zsh, or fish", args[0])
	}
	return nil
}

func cmdConfig(args []string, out io.Writer) error {
	if len(args) == 0 {
		args = []string{"show"}
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
  commands="about completion tui config preview bundle sync daemon status book models ask agent"

  if [[ ${COMP_CWORD} -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
    return 0
  fi

  case "${COMP_WORDS[1]}" in
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "${cur}") )
      ;;
    config)
      COMPREPLY=( $(compgen -W "path show init" -- "${cur}") )
      ;;
    preview)
      COMPREPLY=( $(compgen -W "--include --exclude --max-file-bytes --max-chunk-chars --show-files --show-skips --json --help" -- "${cur}") )
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
    'completion:print shell completion script'
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
        completion) _values 'shell' bash zsh fish ;;
        config) _values 'config command' path show init ;;
        preview) _arguments '--include=[]' '--exclude=[]' '--max-file-bytes=[]' '--max-chunk-chars=[]' '--show-files=[]' '--show-skips=[]' '--json' '--help' ;;
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
complete -c clyde -n "__fish_use_subcommand" -a "about completion tui config preview bundle sync daemon status book models ask agent"
complete -c clyde -n "__fish_seen_subcommand_from completion" -a "bash zsh fish"
complete -c clyde -n "__fish_seen_subcommand_from config" -a "path show init"
complete -c clyde -n "__fish_seen_subcommand_from preview" -l include -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l exclude -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l max-file-bytes -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l max-chunk-chars -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l show-files -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l show-skips -r
complete -c clyde -n "__fish_seen_subcommand_from preview" -l json
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
