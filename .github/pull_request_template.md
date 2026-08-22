## Summary

## Verification

- [ ] `go test ./...`
- [ ] `go build -o bin/clyde ./cmd/clyde`
- [ ] `go run ./cmd/clyde --about`
- [ ] `go run ./cmd/clyde help --json`

## Safety and Contracts

- [ ] Local-first defaults are preserved.
- [ ] Uploads and non-local model endpoints remain explicit.
- [ ] JSON output remains deterministic where applicable.
- [ ] README/help/examples were updated if behavior changed.
