# Clyde

Clyde is a small Go MCP harness for moving auditable repository source bundles
into Google NotebookLM.

It is intentionally conservative:

- `preview` reports what would be included or skipped.
- `bundle` writes local `manifest.json` and `chunks.jsonl` files for review.
- `sync` uploads only after `--approve-upload`.
- Binary files, large files, common dependency/build directories, and likely
  secrets are skipped by default.
- The default NotebookLM backend is the pinned `notebooklm-mcp@2.0.0` MCP server
  over local stdio.

## Build

```bash
go build -o bin/clyde ./cmd/clyde
```

Run tests:

```bash
go test ./...
```

## Usage

Preview a repo:

```bash
clyde preview /path/to/repo
```

Preview with tighter scope:

```bash
clyde preview /path/to/repo \
  --include "internal/**/*.go" \
  --exclude "testdata/**" \
  --max-file-bytes 100000 \
  --show-files 10 \
  --show-skips 25
```

Create a local bundle:

```bash
clyde bundle /path/to/repo --out .clyde/out
```

Plan a dated NotebookLM book name:

```bash
clyde book "Clyde self feedback"
```

List local Ollama models:

```bash
clyde models
```

Ask the selected local model directly:

```bash
clyde ask --model qwen2.5-coder:14b "Review this function for edge cases"
```

Longer prompts can be passed through a file or stdin:

```bash
clyde ask --model qwen2.5-coder:14b --prompt-file prompt.md
cat prompt.md | clyde ask --model qwen2.5-coder:14b --stdin
```

Ask Clyde's local feedback agent to scan a repo and request guidance from the
local model:

```bash
clyde agent . \
  --model qwen2.5-coder:14b \
  --include "internal/**/*.go" \
  "Review this MCP harness for missing tests and design risks"
```

For source-scanning `agent` runs, Clyde treats local Ollama as the safe default.
If `--ollama-url` or `CLYDE_OLLAMA_URL` points away from localhost, `agent`
refuses to send repository context unless `--allow-remote-ollama` is present.
This keeps local feedback from accidentally becoming source upload.

Agent prompts also support files and stdin:

```bash
clyde agent . --model qwen2.5-coder:14b --prompt-file review.md
cat review.md | clyde agent . --model qwen2.5-coder:14b --stdin
```

Model selection order is:

1. `--model`
2. `CLYDE_MODEL`
3. `qwen2.5-coder:14b` when installed
4. `qwen2.5-coder:7b` when installed
5. the first model returned by Ollama

Run `clyde` with no arguments in a terminal to open the basic terminal UI. You
can also launch it explicitly:

```bash
clyde tui
```

Sync through the MCP backend:

```bash
clyde sync /path/to/repo \
  --notebook-id "your-notebook-id" \
  --approve-upload
```

By default `sync` runs:

```bash
npx -y notebooklm-mcp@2.0.0
```

with `NOTEBOOKLM_TRANSPORT=stdio`, `NOTEBOOKLM_PROFILE=all`, and destructive
NotebookLM tools disabled.

For faster upload sessions, Clyde can also use the `nlm` CLI from
`notebooklm-mcp-cli`:

```bash
clyde sync /path/to/repo \
  --notebook-id "your-notebook-id" \
  --approve-upload \
  --backend nlm
```

To replace the target notebook's existing sources when using `nlm`:

```bash
clyde sync /path/to/repo \
  --notebook-id "your-notebook-id" \
  --approve-upload \
  --backend nlm \
  --delete-existing-sources
```

That deletion is intentional and permanent for the target NotebookLM notebook.

## Status Daemon

Clyde includes a localhost-only JSON-RPC status daemon:

```bash
clyde daemon
clyde sync /path/to/repo \
  --notebook-id "your-notebook-id" \
  --approve-upload \
  --status-url http://127.0.0.1:5876/rpc \
  --job-id clyde-sync
clyde status --job-id clyde-sync
```

The daemon refuses non-localhost bind addresses.

## Security Notes

Clyde is a transfer harness, not a security boundary. Review generated
`manifest.json` before upload. Use a dedicated Google account and a private
NotebookLM notebook. Do not upload secrets, production records, customer data,
tokens, credentials, browser state, or private keys.
