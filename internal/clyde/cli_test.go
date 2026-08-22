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
	var out, errOut bytes.Buffer
	status := Main([]string{"completion", "zsh"}, &out, &errOut)
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, errOut.String())
	}
	if !strings.Contains(out.String(), "#compdef clyde") || !strings.Contains(out.String(), "agent:scan repo") {
		t.Fatalf("unexpected completion output: %s", out.String())
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	var out, errOut bytes.Buffer
	status := Main([]string{"completion", "powershell"}, &out, &errOut)
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
