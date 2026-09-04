package clyde

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

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
	if len(flags.excludeFolder) > 0 {
		fmt.Fprintf(out, "Extra exclude folders: %s\n", strings.Join(flags.excludeFolder, ", "))
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
		"exclude_folders": []string(flags.excludeFolder),
		"files":           files,
		"skips":           skips,
	}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(out, string(data))
	return nil
}
