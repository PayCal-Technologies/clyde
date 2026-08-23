package clyde

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func cmdBundle(args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "verify" {
		return cmdBundleVerify(args[1:], out)
	}
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
	force := fs.Bool("force", false, "replace existing bundle files")
	requireSecretScan := fs.Bool("require-secret-scan", false, "fail unless --secret-scan-command completes successfully")
	secretScanCommand := fs.String("secret-scan-command", "", "external secret scanner command; {repo} expands to the repository path")
	addScanFlags(fs, &flags)
	if err := fs.Parse(interspersedArgs(args, map[string]bool{"force": true, "require-secret-scan": true})); err != nil {
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
	if *requireSecretScan && strings.TrimSpace(*secretScanCommand) == "" {
		return errf("--require-secret-scan requires --secret-scan-command")
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
	if strings.TrimSpace(*secretScanCommand) != "" {
		if err := runSecretScanCommand(context.Background(), *secretScanCommand, result.Repo); err != nil {
			return err
		}
	}
	title, slug := "", ""
	if plan != nil {
		title, slug = plan.Title(), plan.Slug()
	}
	manifest, err := WriteBundleWithOptions(result, *outDir, flags.maxChunkChars, title, slug, WriteBundleOptions{
		Force:             *force,
		RequireSecretScan: *requireSecretScan,
		SecretScanCommand: *secretScanCommand,
	})
	if err != nil {
		return err
	}
	printSummary(out, result, manifest.ChunkCount, flags)
	if plan != nil {
		printBookPlan(out, *plan)
	}
	fmt.Fprintf(out, "\nWrote: %s\n", filepath.Join(*outDir, "manifest.json"))
	fmt.Fprintf(out, "Wrote: %s\n", filepath.Join(*outDir, "chunks.jsonl"))
	fmt.Fprintf(out, "Bundle digest: %s\n", manifest.BundleSHA256)
	fmt.Fprintln(out, "Review manifest.json before running sync.")
	return nil
}

func cmdBundleVerify(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("bundle verify", flag.ContinueOnError)
	fs.SetOutput(out)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errf("bundle verify requires BUNDLE_DIR")
	}
	bundle, err := LoadBundle(fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Bundle verified: %s\n", fs.Arg(0))
	fmt.Fprintf(out, "Bundle digest: %s\n", bundle.Digest)
	fmt.Fprintf(out, "Included files: %d\n", bundle.Manifest.FileCount)
	fmt.Fprintf(out, "Skipped files: %d\n", len(bundle.Manifest.Skips))
	fmt.Fprintf(out, "Chunks: %d\n", bundle.Manifest.ChunkCount)
	fmt.Fprintf(out, "Total bytes: %s\n", formatBytes(bundle.Manifest.TotalBytes))
	return nil
}

func runSecretScanCommand(ctx context.Context, commandLine, repo string) error {
	commandLine = strings.ReplaceAll(commandLine, "{repo}", repo)
	command := shellFields(commandLine)
	if len(command) == 0 {
		return errf("secret scan command must not be empty")
	}
	if _, err := runCommand(ctx, command, nil, 10*time.Minute); err != nil {
		return errf("secret scan command failed: %w", err)
	}
	return nil
}
