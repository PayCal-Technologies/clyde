# Contributing to Clyde

Clyde is an open-source local-first CLI under the 0BSD license. Contributions
are welcome when they keep repository review auditable, bounded, and useful for
both humans and automation.

## Development

```bash
go test ./...
go build -o bin/clyde ./cmd/clyde
./bin/clyde --about
./bin/clyde help --json
```

Commands that upload, write files, start services, or contact non-local model
endpoints must keep explicit user intent visible in flags, help text, and JSON
metadata.

## Pull Requests

- Keep changes focused and explain the user-facing behavior.
- Add or update tests for CLI behavior, JSON contracts, repository scanning,
  config validation, and local/remote network boundaries.
- Preserve deterministic JSON output for scripts and AI callers.
- Avoid adding private deployment assumptions to the public repository.
- Update `README.md`, `TESTING.md`, `CHANGELOG.md`, help assets, or examples
  when behavior changes.

## Release Hygiene

Before requesting review for a release-facing change, run:

```bash
gofmt -w $(find . -name '*.go')
go test ./...
go run ./cmd/clyde --about
go run ./cmd/clyde help --json
go run ./cmd/clyde scan-report . --json
```

Security reports should follow `SECURITY.md`, not public issues.
