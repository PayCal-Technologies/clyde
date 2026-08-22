# Clyde

Clyde is a small Go MCP-client harness for moving auditable repository source bundles
into Google NotebookLM.

Current version: `0.2.3`

Official resources:

- Clyde homepage: [paycaltech.com/clyde](https://paycaltech.com/clyde)
- Clyde help: [paycaltech.com/clyde/help](https://paycaltech.com/clyde/help)
- GitHub: [github.com/PayCal-Technologies/clyde](https://github.com/PayCal-Technologies/clyde)
- Created by [PayCal Technologies](https://paycaltech.com)

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

## Standard Release Procedure

Use this procedure for every Clyde release and performance pass:

1. Check `git status --short` first. Commit and push any unrelated pending work
   before starting a review.
2. Run a targeted Clyde-on-Clyde review, such as:

   ```bash
   clyde agent . \
     --include "internal/**/*.go" \
     --include "cmd/**/*.go" \
     --include "README.md" \
     --include "CHANGELOG.md" \
     "Performance review for Clyde itself. Identify concrete optimizations worth implementing now."
   ```

3. Implement only concrete, testable improvements. Record rejected suggestions
   when they do not fit Clyde's CLI execution model.
4. Run `gofmt`, `go test ./...`, `go run ./cmd/clyde --about`, and
   `go run ./cmd/clyde help --json`.
5. Update `VERSION`, `productVersion`, `CHANGELOG.md`, and this README.
6. Commit, push, create an annotated version tag, push the tag, and publish a
   GitHub release with implementation notes and verification results.
7. Rebuild the local global install after the release when this workstation
   needs the new `clyde` binary.

Install from source:

```bash
go install github.com/PayCal-Technologies/clyde/cmd/clyde@main
```

Windows from source in PowerShell:

```powershell
go install github.com/PayCal-Technologies/clyde/cmd/clyde@main
clyde --about
```

Clyde is portable Go and is intended to run on Windows, macOS, and Linux.
Repository previewing, bundling, local Ollama commands, the status daemon, and
NotebookLM sync use cross-platform Go APIs. Windows users still need local
dependencies for the commands they run, such as Git for fast repository file
discovery, Ollama for `ask` and `agent`, and Node/npm or `nlm` for NotebookLM
sync backends.

## Usage

Initialize Clyde's config file:

```bash
clyde config init
clyde config show
clyde help
clyde help agent
clyde help --json
clyde --about
clyde doctor
```

Generate shell completion:

```bash
clyde completion zsh > /opt/homebrew/share/zsh/site-functions/_clyde
clyde completion bash > ~/.clyde-completion.bash
clyde completion fish > ~/.config/fish/completions/clyde.fish
```

PowerShell completion can be loaded for the current session:

```powershell
clyde completion powershell | Out-String | Invoke-Expression
```

To make PowerShell completion persistent, add that line to `$PROFILE`.

Additional completion targets are available for users of smaller shells and
Windows command-line enhancers:

| Shell | Command |
| --- | --- |
| Elvish | `clyde completion elvish > ~/.elvish/completions/clyde.elv` |
| Nushell | `clyde completion nushell > clyde-completions.nu` |
| Xonsh | `clyde completion xonsh > ~/.xonshrc.d/clyde-completions.xsh` |
| Tcsh | `clyde completion tcsh >> ~/.tcshrc` |
| Clink | `clyde completion clink > %LOCALAPPDATA%\\clink\\clyde.lua` |
| Yash | `clyde completion yash >> ~/.yashrc` |
| Oil/OSH/YSH | `clyde completion oil > ~/.config/oils/clyde-completions.sh` |

Help surfaces are designed for both humans and automation:

- `clyde --help` prints concise human-readable top-level help.
- `clyde help COMMAND` prints command-specific help without requiring a valid
  local config file.
- `clyde help --json` prints a structured command catalog with command names,
  categories, summaries, access level, network behavior, syntax, and examples.
- `clyde --about` and `clyde about` print product links for the official
  homepage, help site, GitHub repository, and PayCal Technologies.
- `clyde doctor` prints read-only environment diagnostics. Use
  `clyde doctor --json` or `clyde doctor /path/to/repo --json` for automation
  and AI-readable troubleshooting.

Doctor checks:

```bash
clyde doctor
clyde doctor /path/to/repo
clyde doctor /path/to/repo --json
```

The diagnostic report covers Clyde's version, OS/architecture, config file
status, PATH availability for Git, `npx`, and `nlm`, local Ollama reachability,
and optional repository scan readiness. It does not upload source or modify
local files.

By default Clyde reads `~/.config/clyde/config.json`. Set `CLYDE_CONFIG` to use
a different file. CLI flags override environment variables, environment
variables override the config file, and the config file overrides Clyde's built
in v0.2 defaults. A copyable example lives at `examples/config.json`.

Example config:

```json
{
  "ollama_url": "http://127.0.0.1:11434",
  "model": "qwen2.5-coder:7b",
  "num_ctx": 8192,
  "ask_timeout_seconds": 120,
  "agent_timeout_seconds": 180,
  "max_context_chars": 16000,
  "max_file_bytes": 250000,
  "max_chunk_chars": 18000
}
```

Configuration reference:

| Field | Purpose | Default |
| --- | --- | --- |
| `ollama_url` | Ollama API base URL. Must be `http` or `https` with a host. | `http://127.0.0.1:11434` |
| `model` | Preferred local Ollama model for `ask` and `agent`. | `qwen2.5-coder:7b` |
| `num_ctx` | Context window sent to Ollama generation requests. | `8192` |
| `ask_timeout_seconds` | Timeout for direct `ask` requests. | `120` |
| `agent_timeout_seconds` | Timeout for repo-scanning `agent` requests. | `180` |
| `max_context_chars` | Maximum direct prompt/context size Clyde will prepare. | `16000` |
| `max_file_bytes` | Per-file source scanning cap. | `250000` |
| `max_chunk_chars` | Source bundle chunk size for NotebookLM upload records. | `18000` |

Omitted or zero numeric config values are replaced with defaults. String values
are trimmed. Negative numeric values, blank model names, NUL bytes, invalid
Ollama URLs, and oversized limits are rejected during config load.
`CLYDE_OLLAMA_URL` and `CLYDE_MODEL` override the file and are validated the
same way. `CLYDE_CONFIG` points Clyde at an alternate config file.

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

Machine-readable preview:

```bash
clyde preview /path/to/repo --json
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
clyde models --json
```

Ask the selected local model directly:

```bash
clyde ask --model qwen2.5-coder:7b "Review this function for edge cases"
```

Longer prompts can be passed through a file or stdin:

```bash
clyde ask --model qwen2.5-coder:7b --prompt-file prompt.md
cat prompt.md | clyde ask --model qwen2.5-coder:7b --stdin
```

Ask Clyde's local feedback agent to scan a repo and request guidance from the
local model:

```bash
clyde agent . \
  --include "internal/**/*.go" \
  "Review this MCP harness for missing tests and design risks"
```

For source-scanning `agent` runs, Clyde treats local Ollama as the safe default.
If the configured Ollama URL, `--ollama-url`, or `CLYDE_OLLAMA_URL` points away
from localhost, `agent` refuses to send repository context unless
`--allow-remote-ollama` is present. This keeps local feedback from accidentally
becoming source upload.

Agent prompts also support files and stdin:

```bash
clyde agent . --model qwen2.5-coder:7b --prompt-file review.md
cat review.md | clyde agent . --model qwen2.5-coder:7b --stdin
```

Use `--num-ctx` with larger local prompts when the selected Ollama model and
machine can handle the larger context window.

Model selection order is:

1. `--model`
2. `CLYDE_MODEL`
3. `model` in Clyde's config file
4. the first model returned by Ollama

Run `clyde` with no arguments in a terminal to open the basic terminal UI. You
can also launch it explicitly:

```bash
clyde tui
```

The TUI is intentionally dependency-free in v0.2. It can list/select models,
ask the selected local model, preview the current repo, and run the local agent
against the current repo.

TUI command map:

| Key | Action |
| --- | --- |
| `1` | Ask the selected local model. |
| `2` | Refresh and list local Ollama models. |
| `3` | Preview the current repo with a concise file/skip list. |
| `4` | Select a model by name or number. |
| `5` | Run the local agent against the current repo. |
| `q` | Quit. |

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

Clyde also applies several guardrails before data leaves the local machine:

- `agent` refuses non-local Ollama URLs unless `--allow-remote-ollama` is set.
- Repo scans skip symlinks, non-regular files, binary files, likely secrets,
  dependency/build folders, and files larger than `max_file_bytes`.
- CLI duration, context, port, URL, and backend command flags are validated
  before network or process work starts.
- Clyde-written config files use private file permissions, and Clyde rejects
  group- or world-writable config files.
- MCP responses have a maximum frame size to avoid accidental large allocation.
- Ollama error bodies are size-limited before being included in error messages.
- Ollama JSON and streaming responses, Git file discovery, prompt input, and
  subprocess output capture are bounded.
- Subprocess timeout/error summaries redact large `--text` payloads before they
  are printed or stored in status events.

## What Clyde Does Not Do

- Clyde does not sandbox commands or model runtimes.
- Clyde does not guarantee that scanned source is safe to upload.
- Clyde does not create or delete Google notebooks.
- Clyde does not replace code review, CI, or secret scanning.
- Clyde does not expose an MCP server yet; it currently acts as an MCP client
  and local model harness.
