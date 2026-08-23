# Changelog

## Unreleased

- Nothing yet.

## v0.2.7 - 2026-08-23

- Fail closed when Git-aware repository discovery fails instead of falling back
  to raw filesystem traversal inside Git repositories.
- Record repository discovery provenance in bundle manifests, including the
  discovery method, whether Git exclusions were honored, Git commit, and
  working-tree state.
- Require `--resume` receipts to already exist so mistyped receipt paths cannot
  silently start a fresh transfer.
- Persist chunk receipt entries as `pending` before remote upload and refuse
  automatic resume of pending or ambiguous chunks.
- Require sync receipts for `--delete-existing-sources` and resume destructive
  deletion from the originally planned source ID inventory only.
- Reject symlinked parent directories for bundle and receipt writes, and reject
  symlinks or non-regular files when reading bundles and receipts.
- Reject symlinked parent directories during config initialization.
- Run external secret scanners against a private snapshot of Clyde's captured
  source bytes and record target/output evidence digests.
- Record backend command, resolved executable path, executable digest when
  readable, package, and runtime identity in sync receipts.
- Pin CI/release actions and analysis tool versions, make release archive
  metadata reproducible, smoke-check every archive, and verify published asset
  checksums after release upload.
- Use Windows Job Objects for MCP subprocess cleanup when available, with
  direct child termination as a fallback.
- Stream `chunks.jsonl` decoding and digest verification during bundle loading
  to avoid retaining a second raw copy of the chunk file in memory.
- Verify reconstructed bundle file digests incrementally without constructing
  duplicate whole-file strings, and avoid copying chunk text into verification
  index maps.
- Serialize MCP request/response operations so the client remains safe if
  future callers issue concurrent requests.
- Pre-count per-file chunk totals before materializing chunk records so the
  generated chunk limit is enforced before building large intermediate chunk
  slices.
- Add regression tests for fail-closed Git discovery, destructive deletion
  resume safety, receipt resume semantics, symlink path refusal, config
  initialization path safety, secret-scan
  evidence, backend identity, and bundle discovery metadata.

## v0.2.6 - 2026-08-23

- Bind reviewed bundles to uploaded content with per-chunk digests, an overall
  bundle digest, `clyde bundle verify`, and `sync --bundle --approve-digest`.
- Write bundle artifacts and sync receipts with private atomic filesystem
  semantics, symlink refusal, and explicit `--force` overwrite behavior.
- Add durable sync receipts, returned source ID capture, resumable bundle sync,
  and pre-delete NotebookLM source inventories for `--delete-existing-sources`.
- Add optional external secret-scanner enforcement for bundles with
  `--require-secret-scan` and `--secret-scan-command`.
- Add aggregate scanner limits, UTF-8-safe chunk splitting, MCP environment
  allowlisting, bounded MCP stderr, and status daemon hardening.
- Expand CI to gofmt, tests, race tests, vet, builds, CLI smoke, fuzz smoke,
  staticcheck, and govulncheck; add tag-driven release archives, checksums,
  SBOM, and provenance attestations.
- Split oversized internal domains for bundle, sync, MCP, status, receipt, and
  subprocess handling into smaller files while preserving the public CLI.
- Centralize top-level command dispatch, command metadata, and completion shell
  dispatch behind typed registries with drift tests.
- Harden scanner file reads by verifying opened-file identity and size around
  bounded reads before accepting source content.
- Harden config initialization with private atomic writes, `0700` config
  directories, and refusal to overwrite existing files or symlinks.

## v0.2.5 - 2026-08-22

- Add a complete, human-readable `TESTING.md` that documents the full Clyde
  test suite, when to run each check, release verification, Windows coverage,
  and troubleshooting.
- Add `help/testing.html` to the bundled Clyde help system with direct links to
  the official Clyde homepage, public help site, and GitHub repository.
- Add official PayCal Technologies, Clyde homepage, help, and GitHub references
  to the testing guide.

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
