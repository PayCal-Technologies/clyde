# Changelog

## Unreleased

## v0.2.4 - 2026-08-22

- Add `clyde scan-report` for read-only repository scan summaries with largest
  files, extension counts, skip reasons, chunk counts, and scan limits.
- Add `clyde scan-report --json` for AI-ready repository shape diagnostics.
- Wire `scan-report` into help, the JSON command catalog, and shell
  completions.

## v0.2.3 - 2026-08-22

- Add `clyde doctor` for read-only environment diagnostics across version,
  platform, config, PATH dependencies, local Ollama, and optional repo scan
  readiness.
- Add `clyde doctor --json` for AI-ready diagnostics and automation.
- Wire `doctor` into help, the JSON command catalog, and shell completions.

## v0.2.2 - 2026-08-22

- Add PowerShell completion support for Windows and cross-platform `pwsh`.
- Add completion generators for Elvish, Nushell, Xonsh, Tcsh, Clink, Yash, and
  Oil-family shells.
- Document Windows install/support expectations and PowerShell completion setup.
- Add Windows to the GitHub Actions test matrix.

## v0.2.1 - 2026-08-22

- Add a standard release procedure that starts with checking and committing any
  uncommitted work before performance review, testing, tagging, and publishing.
- Optimize source bundle chunk construction by using bounded builders for chunk
  headers and text splitting.
- Reduce repeated allocation during bundle manifest creation, scanner exclude
  handling, and Git candidate path collection.
- Avoid copying and sorting agent context chunks when they are already in
  priority order.
- Add focused tests for chunk splitting preservation and agent priority
  fast-path behavior.

## v0.2.0 - 2026-08-22

- Polish CLI help and examples for the Go command surface.
- Add JSON output for `preview` and `models`.
- Improve local model selection and Ollama status output.
- Improve `agent` context packing with stable file prioritization and UTF-8-safe truncation.
- Keep source-scanning agent runs local by default unless remote Ollama is explicitly allowed.
- Expand the basic TUI with model selection and agent feedback on the current repo.
- Add GitHub Actions test workflow.
- Add `about`, `completion`, and `help` command surfaces, including `clyde help --json`.
- Add an AI-ready command catalog with access, network, syntax, and example metadata.
- Add official PayCal Technologies, Clyde homepage, help, and GitHub links to CLI output and docs.
- Add focused hardening for config validation, config permissions, prompt input limits, scanner symlink handling, bounded Git discovery, bounded MCP/JSON-RPC/Ollama/subprocess I/O, safer numeric parsing, and redacted subprocess payload summaries.
- Add shell completions for Bash, Zsh, and Fish.
- Add example config and expanded tests for CLI behavior, config validation, scanner guards, MCP framing, JSON-RPC limits, Ollama limits, and NotebookLM subprocess handling.

## v0.1.0

- Convert Clyde from a Python NotebookLM helper into a Go MCP/Ollama harness.
- Preserve repository preview, bundle, NotebookLM sync, daemon/status, and book title commands.
- Add local Ollama `models`, `ask`, and `agent` commands.
