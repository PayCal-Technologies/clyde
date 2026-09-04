# Clyde Glossary

## Preview

A read-only report of the files Clyde would include and skip. It does not write,
upload, or contact a backend.

## Bundle

A local review package containing `manifest.json` and `chunks.jsonl`. A bundle is
written before any upload so its exact contents can be inspected and verified.

## Digest

A `sha256:` identifier bound to one exact bundle. Clyde requires that digest at
upload approval, preventing a reviewed bundle from being substituted later.

## Dry run

A backend check and transfer plan with no upload, remote deletion, or receipt
write. Use it before any real sync.

## Approval

The explicit `--approve-upload` flag required before Clyde can upload repository
chunks. It is separate from choosing a destination and approving a bundle digest.

## Receipt

A local record of a sync's plan, progress, backend identity, and returned source
IDs. A receipt supports inspection and safe resume when a transfer is interrupted.

## Backend

The local integration Clyde uses to communicate with NotebookLM. The default MCP
backend runs locally; the `nlm` backend is required for operations that delete
existing NotebookLM sources.

## Safety labels

- **Read-only**: inspects local files or local readiness only.
- **Writes locally**: creates or updates local bundles, configuration, or receipts.
- **Uploads repository chunks**: sends reviewed source chunks to the selected
  NotebookLM backend only after explicit approval.
- **Deletes existing sources**: removes existing NotebookLM sources and requires
  the `nlm` backend plus explicit upload approval.
