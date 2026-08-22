package clyde

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreviewShowsIncludedFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")

	var out, errOut bytes.Buffer
	status := Main([]string{"preview", dir}, &out, &errOut)

	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), "Included files: 1") || !strings.Contains(out.String(), "app.go") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestPreviewJSON(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")

	var out, errOut bytes.Buffer
	status := Main([]string{"preview", dir, "--json"}, &out, &errOut)

	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["included_files"].(float64) != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestScanReportJSON(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "small.go"), "package main\n")
	mustWrite(t, filepath.Join(dir, "large.md"), strings.Repeat("x", 80))
	mustWrite(t, filepath.Join(dir, "secret.env"), "API_KEY='abcdefghijklmnopqrstuvwxyz123456'\n")

	var out, errOut bytes.Buffer
	status := Main([]string{"scan-report", dir, "--json", "--top", "1"}, &out, &errOut)

	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	var payload scanReport
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.IncludedFiles != 2 || payload.SkippedFiles != 1 || payload.ChunkCount == 0 {
		t.Fatalf("unexpected scan report: %#v", payload)
	}
	if len(payload.TopFiles) != 1 || payload.TopFiles[0].Path != "large.md" {
		t.Fatalf("unexpected top files: %#v", payload.TopFiles)
	}
	if !scanReportCountHas(payload.SkipReasons, "possible secret material", 1) {
		t.Fatalf("missing skip reason: %#v", payload.SkipReasons)
	}
	if !scanReportCountHas(payload.ExtensionStats, ".go", 1) || !scanReportCountHas(payload.ExtensionStats, ".md", 1) {
		t.Fatalf("missing extension stats: %#v", payload.ExtensionStats)
	}
}

func TestScanReportRejectsNegativeTop(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"scan-report", ".", "--top", "-1"}, &out, &errOut)
	if status != 1 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(errOut.String(), "--top must be zero or greater") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func scanReportCountHas(items []scanReportCount, name string, count int) bool {
	for _, item := range items {
		if item.Name == name && item.Count == count {
			return true
		}
	}
	return false
}

func TestBundleWritesManifest(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")
	outDir := filepath.Join(dir, "out")

	var out, errOut bytes.Buffer
	status := Main([]string{"bundle", dir, "--out", outDir, "--subject", "Demo Sync"}, &out, &errOut)

	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	data, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FileCount != 1 || manifest.Book == nil {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
}

func TestSyncDeleteExistingSourcesRequiresNLMBackend(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")

	var out, errOut bytes.Buffer
	status := Main([]string{"sync", dir, "--notebook-id", "nb", "--approve-upload", "--delete-existing-sources"}, &out, &errOut)

	if status != 1 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(errOut.String(), "--delete-existing-sources requires --backend nlm") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestBookCommandPrintsDatedName(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"book", "Demo", "Sync"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), " - Demo Sync") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestSubcommandHelpExitsCleanly(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"agent", "--help"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), "Usage of agent") {
		t.Fatalf("unexpected help: %s", out.String())
	}
}

func TestHelpCommandPrintsTopLevelAndCommandHelp(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"help"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), "usage: clyde") || !strings.Contains(out.String(), "clyde help agent") {
		t.Fatalf("unexpected help: %s", out.String())
	}

	out.Reset()
	errOut.Reset()
	status = Main([]string{"help", "agent"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), "Usage of agent") || !strings.Contains(out.String(), "allow-remote-ollama") {
		t.Fatalf("unexpected command help: %s", out.String())
	}

	out.Reset()
	errOut.Reset()
	status = Main([]string{"help", "config", "init"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), "clyde config init") || !strings.Contains(out.String(), "Writes local config") {
		t.Fatalf("unexpected subcommand help: %s", out.String())
	}
}

func TestHelpJSONPrintsCommandCatalog(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"help", "--json"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	var payload struct {
		Product  string        `json:"product"`
		Commands []commandInfo `json:"commands"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Product != productName || len(payload.Commands) != 19 {
		t.Fatalf("unexpected catalog: %#v", payload)
	}
	foundHelp := false
	foundConfigInit := false
	foundDoctor := false
	foundScanReport := false
	for _, command := range payload.Commands {
		if command.Name == "help" && command.Syntax != "" && len(command.Examples) > 0 {
			foundHelp = true
		}
		if command.Name == "config init" && command.Access == "Writes local config" {
			foundConfigInit = true
		}
		if command.Name == "doctor" && command.Category == "Diagnostics" {
			foundDoctor = true
		}
		if command.Name == "scan-report" && command.Category == "Repository Bundles" {
			foundScanReport = true
		}
	}
	if !foundHelp || !foundConfigInit || !foundDoctor || !foundScanReport {
		t.Fatalf("expected commands missing from catalog: %#v", payload.Commands)
	}
}

func TestHelpDoesNotRequireValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", path)
	mustWrite(t, path, `{"ollama_url":"not-a-url"}`)

	for _, args := range [][]string{
		{"agent", "--help"},
		{"ask", "--help"},
		{"models", "--help"},
		{"preview", "--help"},
		{"scan-report", "--help"},
		{"bundle", "--help"},
		{"sync", "--help"},
		{"doctor", "--help"},
		{"help", "agent"},
		{"help", "doctor"},
		{"help", "scan-report"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			status := Main(args, &out, &errOut)
			if status != 0 {
				t.Fatalf("status=%d out=%q stderr=%q", status, out.String(), errOut.String())
			}
			if !strings.Contains(out.String(), "Usage") && !strings.Contains(out.String(), "usage") {
				t.Fatalf("unexpected help output: %s", out.String())
			}
		})
	}
}

func TestDoctorReportsEnvironmentJSON(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "app.go"), "package main\n")
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", configPath)
	mustWrite(t, configPath, `{"ollama_url":"http://127.0.0.1:1","model":"qwen2.5-coder:7b","ollama_timeout_seconds":1}`)

	var out, errOut bytes.Buffer
	status := Main([]string{"doctor", dir, "--json", "--ollama-timeout", "0.01"}, &out, &errOut)

	if status != 0 {
		t.Fatalf("status=%d stderr=%s out=%s", status, errOut.String(), out.String())
	}
	var payload doctorReport
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Product != productName || payload.Version != productVersion {
		t.Fatalf("unexpected doctor payload: %#v", payload)
	}
	if !doctorPayloadHas(payload, "repo", "ok") {
		t.Fatalf("expected ok repo check: %#v", payload.Checks)
	}
	if !doctorPayloadHas(payload, "ollama", "warn") {
		t.Fatalf("expected warning ollama check: %#v", payload.Checks)
	}
}

func TestDoctorReportsInvalidConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	t.Setenv("CLYDE_CONFIG", configPath)
	mustWrite(t, configPath, `{"ollama_url":"not-a-url"}`)

	var out, errOut bytes.Buffer
	status := Main([]string{"doctor", "--json", "--ollama-timeout", "0.01"}, &out, &errOut)

	if status != 1 {
		t.Fatalf("expected failure, got status=%d out=%s", status, out.String())
	}
	if !strings.Contains(errOut.String(), "doctor found errors") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
	var payload doctorReport
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !doctorPayloadHas(payload, "config", "error") {
		t.Fatalf("expected config error: %#v", payload.Checks)
	}
}

func doctorPayloadHas(report doctorReport, name, status string) bool {
	for _, check := range report.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func TestTopLevelHelpIncludesProductLinks(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"--help"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	for _, want := range []string{productHomeURL, productHelpURL, productGitHubURL, productCreatorURL} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q: %s", want, out.String())
		}
	}
}

func TestAboutIncludesProductLinks(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"--about"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	for _, want := range []string{productName, productDescription, productHomeURL, productGitHubURL} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("about missing %q: %s", want, out.String())
		}
	}
}

func TestCompletionCommandPrintsShellScript(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{shell: "bash", want: "complete -F _clyde_completion clyde"},
		{shell: "zsh", want: "#compdef clyde"},
		{shell: "fish", want: "complete -c clyde"},
		{shell: "powershell", want: "Register-ArgumentCompleter"},
		{shell: "pwsh", want: "Register-ArgumentCompleter"},
		{shell: "elvish", want: "edit:completion:arg-completer[clyde]"},
		{shell: "nushell", want: `extern "clyde"`},
		{shell: "nu", want: `extern "clyde"`},
		{shell: "xonsh", want: "add_one_completer"},
		{shell: "tcsh", want: "complete clyde"},
		{shell: "clink", want: `clink.argmatcher("clyde")`},
		{shell: "yash", want: "completion//argument/clyde"},
		{shell: "oil", want: "Oil/OSH/YSH completion"},
		{shell: "osh", want: "Oil/OSH/YSH completion"},
		{shell: "ysh", want: "Oil/OSH/YSH completion"},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			var out, errOut bytes.Buffer
			status := Main([]string{"completion", tt.shell}, &out, &errOut)
			if status != 0 {
				t.Fatalf("status=%d stderr=%s", status, errOut.String())
			}
			if !strings.Contains(out.String(), tt.want) || !strings.Contains(out.String(), "agent") {
				t.Fatalf("unexpected completion output: %s", out.String())
			}
		})
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"completion", "planet9"}, &out, &errOut)
	if status != 1 {
		t.Fatalf("expected failure")
	}
	if !strings.Contains(errOut.String(), "unsupported shell") {
		t.Fatalf("unexpected stderr: %s", errOut.String())
	}
}

func TestCLIRejectsInvalidNumericFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "ask timeout", args: []string{"ask", "--timeout", "-1", "hello"}, want: "--timeout must be greater than 0"},
		{name: "ask num ctx", args: []string{"ask", "--num-ctx", "-1", "hello"}, want: "--num-ctx must be zero or greater"},
		{name: "ask num ctx cap", args: []string{"ask", "--num-ctx", "1000001", "hello"}, want: "--num-ctx is too large"},
		{name: "agent context", args: []string{"agent", ".", "--max-context-chars", "0", "review"}, want: "max-context-chars must be positive"},
		{name: "models timeout", args: []string{"models", "--timeout", "0"}, want: "--timeout must be greater than 0"},
		{name: "daemon port", args: []string{"daemon", "--port", "70000"}, want: "--port must be between 1 and 65535"},
		{name: "status interval", args: []string{"status", "--interval", "0"}, want: "--interval must be greater than 0"},
		{name: "sync command", args: []string{"sync", ".", "--notebook-id", "nb", "--approve-upload", "--mcp-command", ""}, want: "--mcp-command must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			status := MainWithInput(tt.args, strings.NewReader(""), &out, &errOut)
			if status != 1 {
				t.Fatalf("expected failure, got status=%d out=%q err=%q", status, out.String(), errOut.String())
			}
			if !strings.Contains(errOut.String(), tt.want) {
				t.Fatalf("stderr missing %q: %s", tt.want, errOut.String())
			}
		})
	}
}

func TestPromptTextLimitsPromptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(path, bytes.Repeat([]byte("a"), maxPromptInputBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := promptText(strings.NewReader(""), nil, path, false)

	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPromptTextLimitsStdin(t *testing.T) {
	_, err := promptText(io.LimitReader(strings.NewReader(strings.Repeat("a", maxPromptInputBytes+1)), maxPromptInputBytes+1), nil, "", true)

	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}
