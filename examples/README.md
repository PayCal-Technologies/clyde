# Clyde Examples

Copy these examples when evaluating Clyde locally or wiring it into an
AI-assisted repository workflow.

## First Run

```bash
clyde config init
clyde config show
clyde doctor .
```

## Repository Shape Summary

```bash
clyde scan-report . --json
clyde scan-report . --include "internal/**/*.go" --top 20
```

Use JSON output when a script or AI assistant needs compact repository context
without creating bundle files or uploading data.

## Local Model Review

```bash
clyde agent . \
  --include "internal/**/*.go" \
  --include "cmd/**/*.go" \
  "Review command safety, input validation, and missing tests."
```

Clyde keeps `agent` local by default. Remote Ollama endpoints require explicit
approval with `--allow-remote-ollama`.

## Reviewable Bundle

```bash
clyde preview .
clyde bundle . --out .clyde/out
```

Review `.clyde/out/manifest.json` and `.clyde/out/chunks.jsonl` before any
NotebookLM upload.

## NotebookLM Sync

```bash
clyde bundle . --out .clyde/out
clyde bundle verify .clyde/out
clyde sync --bundle .clyde/out \
  --notebook-id "your-notebook-id" \
  --approve-digest "sha256:..." \
  --approve-upload
```

Only use this after reviewing `.clyde/out/manifest.json`,
`.clyde/out/chunks.jsonl`, and the digest printed by `bundle verify`.

## Shell Completion

```bash
clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde
clyde completion bash > ~/.clyde-completion.bash
clyde completion fish > ~/.config/fish/completions/clyde.fish
```

PowerShell:

```powershell
clyde completion powershell | Out-String | Invoke-Expression
```
