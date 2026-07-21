# Clyde

Clyde is a macOS-friendly Python CLI that turns a repository into an
auditable, chunked source stream for NotebookLM.

It is intentionally conservative:

- `preview` reports what would be included or skipped, including capped file
  lists and skip reason counts.
- `bundle` writes local JSONL source chunks plus a manifest.
- `sync` uploads only after an explicit command, using a pinned
  `notebooklm-mcp` subprocess over local stdio.
- Binary files, large files, common dependency/build directories, and likely
  secrets are skipped by default.

## Install for Development

```bash
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -e .
```

## Usage

Preview a repo:

```bash
clyde preview /path/to/repo
```

Preview with tighter scope:

```bash
clyde preview /path/to/repo \
  --include "src/**/*.py" \
  --exclude "tests/fixtures/**" \
  --max-file-bytes 100000 \
  --show-files 10 \
  --show-skips 25
```

Create a local bundle:

```bash
clyde bundle /path/to/repo --out .clyde/out
```

Sync to NotebookLM:

```bash
clyde sync /path/to/repo \
  --notebook-id "your-notebook-id" \
  --approve-upload
```

By default `sync` runs:

```bash
npx -y notebooklm-mcp@2.0.0
```

with `NOTEBOOKLM_TRANSPORT=stdio`, `NOTEBOOKLM_PROFILE=all`, and
`NOTEBOOKLM_DISABLED_TOOLS=cleanup_data,re_auth,remove_notebook,update_notebook`.
It also defaults `NOTEBOOKLM_ACCOUNT=codex-test`, matching the local Codex
NotebookLM setup documented in the parent Voci repository.

This enables `add_source` for the CLI process while keeping destructive notebook
tools disabled. Use a dedicated Google account and a private NotebookLM notebook.

The MCP client speaks JSON-RPC over stdio with `Content-Length` frames by
default, and falls back to newline-delimited JSON for servers that use that
transport style. Clyde times out stalled MCP requests, ignores unrelated
notifications while waiting for a matching response id, and includes recent MCP
stderr in connection failures.
Use `--mcp-timeout SECONDS` when NotebookLM browser startup is slow. Clyde emits
a heartbeat every second during slow MCP calls by default; adjust it with
`--heartbeat-interval SECONDS`.

During sync, Clyde verifies that the server exposes `add_source` before sending
chunks. Upload errors report the repository path and chunk ordinal that failed.
By default, `sync` prints flushed real-time progress lines to stderr for MCP
startup, tool checks, each chunk upload, failures, and completion. Use
`--quiet-progress` to suppress those lines.

Run a local status daemon:

```bash
clyde daemon
```

Mirror sync progress into the daemon:

```bash
clyde sync /path/to/repo \
  --notebook-id "your-notebook-id" \
  --approve-upload \
  --status-url http://127.0.0.1:5876/rpc \
  --job-id clyde-self-sync
```

Watch daemon status:

```bash
clyde status --job-id clyde-self-sync --watch
```

## Local Status Daemon

Clyde includes a small localhost-only JSON-RPC daemon for live sync status. It
is intentionally local and in-memory; it is a progress surface for nearby tools,
not a remote control plane.

Start the daemon:

```bash
clyde daemon
```

This binds to `127.0.0.1:5876` by default and exposes:

- `POST /rpc`: JSON-RPC 2.0 endpoint.
- `GET /status`: quick JSON status view.

The current methods are:

- `daemon.ping`: health check.
- `status.event`: record a progress event.
- `status.get`: read all jobs or one job by `job_id`.
- `status.reset`: clear all jobs or one job by `job_id`.

Send sync progress to the daemon:

```bash
clyde sync /path/to/repo \
  --notebook-id "your-notebook-id" \
  --approve-upload \
  --status-url http://127.0.0.1:5876/rpc \
  --job-id clyde-self-sync
```

Read status:

```bash
clyde status --job-id clyde-self-sync
clyde status --job-id clyde-self-sync --watch
clyde status --json
```

Progress events use this shape:

```json
{
  "job_id": "clyde-self-sync",
  "phase": "uploading",
  "message": "uploading src/clyde/cli.py [1/1]",
  "done": 3,
  "total": 12,
  "rel_path": "src/clyde/cli.py",
  "error": null,
  "timestamp": 1784592000.0
}
```

The daemon stores recent events per job and the latest summary. Direct sync
progress callbacks can emit the same event model without depending on the
daemon.

## Common Options

All commands accept:

- `--include GLOB`: include only matching repo-relative paths. Repeat to allow
  multiple patterns.
- `--exclude GLOB`: skip matching repo-relative paths in addition to Clyde's
  default excludes. Repeat to deny multiple patterns.
- `--max-file-bytes BYTES`: skip files larger than this positive byte count.
- `--max-chunk-chars CHARS`: split source text at this positive character count.

`preview` also accepts:

- `--show-files N`: show the first N included files. Use `0` to hide the list.
- `--show-skips N`: show the first N skipped files. Use `0` to hide the list.

`sync` also accepts:

- `--mcp-timeout SECONDS`: wait longer for slow NotebookLM browser automation.
- `--heartbeat-interval SECONDS`: emit progress while an MCP request is still
  in flight. Defaults to `1`.
- `--status-url URL`: send JSON-RPC progress events to a Clyde daemon.
- `--job-id ID`: label progress events for daemon lookup.
- `--quiet-progress`: suppress terminal progress output.

The daemon exposes localhost JSON-RPC methods:

- `daemon.ping`
- `status.event`
- `status.get`
- `status.reset`

## Bundle Format

`bundle` creates:

- `manifest.json`: summary, file inventory, skip reasons, and chunk metadata.
- `chunks.jsonl`: one JSON object per chunk with repo-relative path, SHA-256,
  chunk ordinal, and text payload.

Each chunk is formatted as text with a path header so NotebookLM citations remain
traceable to repository files.

## Safety Notes

Clyde is a transfer tool, not a security product. Review the manifest before
uploading. Keep allow/deny globs tight for private repositories, and prefer
running `preview` before `bundle` or `sync`.
