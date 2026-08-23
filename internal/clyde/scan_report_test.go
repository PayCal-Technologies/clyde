package clyde

import "testing"

func TestBuildScanReportSortsTopFilesAndCounts(t *testing.T) {
	result := ScanResult{
		Repo: "/tmp/repo",
		Files: []FileRecord{
			{Rel: "b.txt", Size: 20, SHA256: "b"},
			{Rel: "a.go", Size: 20, SHA256: "a"},
			{Rel: "c", Size: 5, SHA256: "c"},
		},
		Skips: []SkipRecord{
			{Path: "secret.env", Reason: "possible secret material"},
			{Path: "node_modules/a.js", Reason: "excluded by glob"},
			{Path: "node_modules/b.js", Reason: "excluded by glob"},
		},
	}

	report := buildScanReport(result, 3, scanFlags{maxFileBytes: 100, maxChunkChars: 50}, 2)

	if len(report.TopFiles) != 2 || report.TopFiles[0].Path != "a.go" || report.TopFiles[1].Path != "b.txt" {
		t.Fatalf("unexpected top files: %#v", report.TopFiles)
	}
	if !scanReportCountHas(report.SkipReasons, "excluded by glob", 2) {
		t.Fatalf("missing skip count: %#v", report.SkipReasons)
	}
	if !scanReportCountHas(report.ExtensionStats, ".go", 1) || !scanReportCountHas(report.ExtensionStats, "(none)", 1) {
		t.Fatalf("missing extension count: %#v", report.ExtensionStats)
	}
}
