# Changelog

## Unreleased

- Polish CLI help and examples for the Go v0.1 command surface.
- Add JSON output for `preview` and `models`.
- Improve local model selection and Ollama status output.
- Improve `agent` context packing with stable file prioritization and UTF-8-safe truncation.
- Keep source-scanning agent runs local by default unless remote Ollama is explicitly allowed.
- Expand the basic TUI with model selection and agent feedback on the current repo.
- Add GitHub Actions test workflow.

## v0.1.0

- Convert Clyde from a Python NotebookLM helper into a Go MCP/Ollama harness.
- Preserve repository preview, bundle, NotebookLM sync, daemon/status, and book title commands.
- Add local Ollama `models`, `ask`, and `agent` commands.
