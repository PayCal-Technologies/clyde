package clyde

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

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
