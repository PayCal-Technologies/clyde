# Clyde Testing Guide

This document explains how to test Clyde locally, what the current tests cover,
and which checks should run before a pull request, release, or feature pass.

Clyde is a small Go CLI for preparing auditable repository source bundles,
syncing approved context to NotebookLM, and asking local Ollama models for
repo-aware feedback. Its tests focus on safety, predictable CLI behavior,
bounded input/output, local-first defaults, and cross-platform portability.

## Official Resources

- Clyde homepage: [paycaltech.com/clyde](https://paycaltech.com/clyde)
- Clyde help: [paycaltech.com/clyde/help](https://paycaltech.com/clyde/help)
- GitHub repository: [github.com/PayCal-Technologies/clyde](https://github.com/PayCal-Technologies/clyde)
- Created by [PayCal Technologies](https://paycaltech.com/)

## Quick Start

Run the full test suite from the repository root:

```bash
go test ./...
```

For a normal code change, run:

```bash
gofmt -w $(find . -name '*.go')
go test ./...
```

For a release or feature pass, run the standard smoke set:

```bash
gofmt -w $(find . -name '*.go')
go test ./...
go run ./cmd/clyde --about
go run ./cmd/clyde help --json
go run ./cmd/clyde scan-report . --json
```

## What Exists Today

Current test inventory:

| Area | Count |
| --- | ---: |
| Go test files | 19 |
| `func Test...` tests | 138 |
| `func Fuzz...` targets | 4 |

All tests currently live under `internal/clyde`.

| Test file | Main coverage |
| --- | --- |
| `agent_test.go` | Agent prompt construction, model selection, UTF-8-safe truncation, context prioritization. |
| `book_test.go` | Notebook/book naming plans and title-derived naming. |
| `bundler_test.go` | Chunk generation, chunk boundaries, bundle manifests, metadata, skip recording, invalid output paths. |
| `cli_test.go` | Preview, scan-report, bundle, sync guardrails, help, JSON help catalog, doctor, about links, completions, numeric flag validation, prompt input limits. |
| `config_test.go` | Config loading, environment overrides, URL validation, string hardening, numeric limits, file permission checks, defaults, config commands. |
| `jsonrpc_test.go` | JSON-RPC HTTP failure handling and request body size limits. |
| `mcp_test.go` | MCP framing, lowercase content-length support, oversized frame rejection. |
| `notebooklm_mcp_test.go` | MCP sync source ID capture and receipt persistence failures. |
| `notebooklm_test.go` | Subprocess payload redaction, timeout summaries, limited buffer truncation. |
| `ollama_test.go` | Ollama model listing, generation, streaming, malformed streams, metadata sanitization, hardened inputs, CLI ask/agent/model behavior, remote URL guard. |
| `path_strings_test.go` | Suspicious Unicode, NUL, absolute, parent, and Windows-style path string rejection. |
| `scanner_test.go` | Repository scanning, secret skips, include/exclude behavior, invalid paths, invalid size limits, symlink skipping. |
| `scan_report_test.go` | Scan report sorting, counts, and top-file behavior. |
| `sync_receipt_test.go` | Receipt matching, resume safety, destructive deletion inventories, and private receipt writes. |
| `status_test.go` | Status store formatting and recorded events. |
| `catalog_test.go` | Command catalog alignment with command registration and help output. |
| `tui_test.go` | TUI model index behavior. |
| `util_test.go` | Safe numeric conversion boundaries. |

## CI Test Matrix

The GitHub Actions workflow is `.github/workflows/test.yml`.

The main matrix runs:

```bash
go test ./...
go test -race ./internal/clyde
go vet ./...
go build ./cmd/clyde
```

on:

- `ubuntu-latest`
- `macos-latest`
- `windows-latest`

Linux also runs fuzz smoke tests for chunk splitting, JSON-RPC envelope parsing,
glob matching, and secret detection. A separate analysis job runs pinned
Staticcheck and govulncheck versions. These checks do not prove external tools such as Ollama,
Node/npm, NotebookLM MCP, or `nlm` are available in every environment.

## Test Commands By Purpose

| Goal | Command |
| --- | --- |
| Run all tests | `go test ./...` |
| Run tests without cache | `go test -count=1 ./...` |
| Run one test | `go test ./internal/clyde -run TestName` |
| Run tests with race detector | `go test -race ./...` |
| Run fuzz smoke targets | `go test ./internal/clyde -run '^$' -fuzz=FuzzSplitTextPreservesUTF8 -fuzztime=5s` |
| Check formatting | `gofmt -l $(find . -name '*.go')` |
| Apply formatting | `gofmt -w $(find . -name '*.go')` |
| Build Clyde | `go build -o bin/clyde ./cmd/clyde` |
| Smoke `--about` links | `go run ./cmd/clyde --about` |
| Smoke JSON help catalog | `go run ./cmd/clyde help --json` |
| Smoke repository scan report | `go run ./cmd/clyde scan-report . --json` |
| Generate shell completion | `go run ./cmd/clyde completion zsh` |
| Cross-build Windows binary | `GOOS=windows GOARCH=amd64 go build -o /tmp/clyde.exe ./cmd/clyde` |

## Core Test Areas

### Repository Scanning

Scanner tests verify that Clyde includes useful source files while skipping
unsafe or noisy material.

Covered behavior:

- likely secret files are skipped;
- caller-provided excludes are respected;
- symlinks are skipped;
- non-directory repo paths are rejected;
- invalid file-size limits are rejected;
- include globs may match no files without crashing.

Run focused scanner tests:

```bash
go test ./internal/clyde -run 'TestScanRepo'
```

### Bundling

Bundler tests verify the local bundle written by `clyde bundle`.

Covered behavior:

- chunk headers are preserved;
- chunk splitting preserves complete text;
- invalid chunk sizes fall back safely;
- book metadata is recorded;
- `manifest.json` and `chunks.jsonl` records are written;
- output paths that are files are rejected;
- skipped files are recorded.

Run focused bundler tests:

```bash
go test ./internal/clyde -run 'TestMakeChunks|TestWriteBundle|TestSplitText'
```

### CLI Behavior

CLI tests verify user-facing commands and AI-ready command surfaces.

Covered behavior:

- `preview` text and JSON output;
- `scan-report` JSON output and invalid `--top` rejection;
- `bundle` manifest writing;
- `sync` safety checks;
- top-level help and command help;
- `help --json` command catalog;
- help does not require a valid config;
- `doctor` text/JSON behavior;
- official product links in help and about output;
- shell completion output for supported shells;
- unsupported completion shell rejection;
- invalid numeric flags;
- prompt file and stdin size limits.

Run focused CLI tests:

```bash
go test ./internal/clyde -run 'Test.*Help|Test.*Doctor|Test.*Completion|TestPreview|TestScanReport|TestBundle'
```

### Configuration

Config tests verify Clyde accepts predictable configuration and rejects unsafe
or malformed values.

Covered behavior:

- config file loading;
- environment variables overriding file values;
- Ollama URL validation;
- invalid environment URL rejection;
- blank model rejection;
- string trimming;
- NUL byte rejection;
- negative limit rejection;
- writable config file rejection;
- zero-value defaults;
- `config init` and `config show`.

Run focused config tests:

```bash
go test ./internal/clyde -run 'TestLoadConfig|TestConfigCommand'
```

### Ollama And Local AI Commands

Ollama tests use local test HTTP servers. They do not require a real Ollama
daemon.

Covered behavior:

- model listing;
- metadata sanitization;
- generation request shape;
- `num_ctx` forwarding;
- hardened input rejection;
- streaming responses;
- oversized stream line rejection;
- non-2xx and malformed stream reporting;
- environment-selected model and URL;
- CLI `ask`, `agent`, and `models --json`;
- remote Ollama URLs are rejected by `agent` unless explicitly allowed.

Run focused Ollama tests:

```bash
go test ./internal/clyde -run 'TestOllama|TestCLIAsk|TestCLIModels|TestAgentRejectsRemote'
```

### NotebookLM, MCP, And JSON-RPC Boundaries

These tests protect Clyde's external-process and protocol boundaries.

Covered behavior:

- command summaries redact source text payloads;
- subprocess timeout errors do not leak full text payloads;
- limited buffers truncate output;
- MCP content-length parsing is tolerant where appropriate;
- oversized MCP frames are rejected;
- JSON-RPC HTTP failures are reported;
- JSON-RPC request bodies are bounded.

Run focused protocol tests:

```bash
go test ./internal/clyde -run 'TestMCP|TestRPC|TestCommandSummary|TestRunCommand|TestLimitedBuffer'
```

### Agent Prompting

Agent tests verify that Clyde builds useful bounded prompts for local models.

Covered behavior:

- task and repository context are included;
- configured default model selection;
- UTF-8-safe truncation;
- file priority ordering;
- already-prioritized chunks avoid unnecessary sorting;
- safe prefix edge cases.

Run focused agent tests:

```bash
go test ./internal/clyde -run 'TestBuildAgentPrompt|TestPrioritizeAgentChunks|TestSafePrefix'
```

## Cross-Platform Testing

Clyde is intended to run on macOS, Linux, and Windows.

CI runs tests on all three operating systems. Locally, you can at least
cross-build:

```bash
for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  suffix=""
  if [ "$os" = windows ]; then suffix=".exe"; fi
  GOOS="$os" GOARCH="$arch" go build -o "/tmp/clyde-${os}-${arch}${suffix}" ./cmd/clyde
done
```

Cross-building proves the code compiles for a target. It does not prove all
external tools are installed or that terminal integration behaves identically on
that operating system.

Windows-specific notes:

- PowerShell completion is loaded with:

  ```powershell
  clyde completion powershell | Out-String | Invoke-Expression
  ```

- Clink completion can be generated with:

  ```powershell
  clyde completion clink > "$env:LOCALAPPDATA\\clink\\clyde.lua"
  ```

- Windows users still need Git for fast repository discovery and Ollama/Node
  tools for local model or NotebookLM workflows.

## Release Verification

Before publishing a Clyde release, run:

```bash
git status --short
gofmt -w $(find . -name '*.go')
go test ./...
go test -race ./internal/clyde
go vet ./...
go run ./cmd/clyde --about
go run ./cmd/clyde help --json
go run ./cmd/clyde doctor --json
go run ./cmd/clyde scan-report . --json
```

Also generate every completion target:

```bash
for shell in bash zsh fish powershell pwsh elvish nushell nu xonsh tcsh clink yash oil osh ysh; do
  go run ./cmd/clyde completion "$shell" >/tmp/clyde-completion-"$shell"
done
```

Recommended cross-build release smoke:

```bash
GOOS=windows GOARCH=amd64 go build -o /tmp/clyde.exe ./cmd/clyde
GOOS=windows GOARCH=amd64 go test -c ./internal/clyde -o /tmp/clyde-windows-test.exe
```

The release workflow builds deterministic tar/zip archives, checks every archive
for required files, runs the Linux amd64 binary smoke test, and verifies
published release asset checksums after upload.

Windows cross-build checks are useful after MCP process lifecycle changes
because the Windows implementation uses Job Objects behind build tags.

If the release changes AI, NotebookLM, Ollama, or config behavior, include a
short note in the release body explaining which focused tests covered that area.

## When To Add Tests

Add or update tests whenever behavior changes.

Use this guide:

- New command: add CLI tests for help, success behavior, invalid arguments, and
  JSON output if the command is automation-facing.
- New flag: test valid value, invalid value, and interaction with config/env
  defaults.
- New config field: test file loading, env override, validation, defaulting, and
  string hardening if applicable.
- New external process call: test timeout behavior, redacted errors, bounded
  stdout/stderr, and missing command behavior.
- New HTTP/protocol code: test non-2xx, malformed response, oversized response,
  and request shape.
- New repository scanner rule: test inclusion, exclusion, and skip reason.
- New AI prompt behavior: test prompt content, ordering, truncation, and UTF-8
  boundaries.
- New shell completion support: test generated output and unsupported shell
  rejection.

## Common Failures

`gofmt` output lists files:

- Run `gofmt -w` on those files.

`go test ./...` fails only on Windows:

- Check path separators, executable suffixes, file permissions, shell
  assumptions, and hard-coded `/tmp` paths.

Config tests fail with permission expectations:

- Clyde rejects writable config files as a hardening measure. Preserve that
  unless the security model changes deliberately.

Ollama tests fail:

- They should use test servers, not a real local daemon. Check request paths,
  JSON shape, timeout values, and stream parsing.

NotebookLM subprocess tests fail:

- Check that text payloads stay redacted in errors and summaries. Avoid logging
  full source text in failure paths.

Completion tests fail:

- Update command lists and shell-specific expected snippets together.
- Keep `help --json`, human help, and completions aligned.

## Before Pushing

Minimum:

```bash
git status --short
gofmt -w $(find . -name '*.go')
go test ./...
git diff --check
```

Recommended for release or broad feature work:

```bash
go test -race ./...
go vet ./...
go run ./cmd/clyde --about
go run ./cmd/clyde help --json
go run ./cmd/clyde doctor --json
go run ./cmd/clyde scan-report . --json
```

Commit only after the relevant suite passes or the remaining failure is clearly
unrelated and documented.
