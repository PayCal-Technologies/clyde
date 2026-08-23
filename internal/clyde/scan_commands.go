package clyde

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
)

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
