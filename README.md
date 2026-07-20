# Clyde

Clyde is a macOS-friendly Python CLI that turns a repository into an
auditable, chunked source stream for NotebookLM.

It is intentionally conservative:

- `preview` reports what would be included or skipped.
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

This enables `add_source` for the CLI process while keeping destructive notebook
tools disabled. Use a dedicated Google account and a private NotebookLM notebook.

## Bundle Format

`bundle` creates:

- `manifest.json`: summary, file inventory, skip reasons, and chunk metadata.
- `chunks.jsonl`: one JSON object per chunk with repo-relative path, SHA-256,
  chunk ordinal, and text payload.

Each chunk is formatted as text with a path header so NotebookLM citations remain
traceable to repository files.

## Safety Notes

Clyde is a transfer tool, not a security product. Review the manifest before
uploading. Keep allow/deny globs tight for private repositories.
