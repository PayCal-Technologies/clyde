# Clyde GitHub Workflow Policy

Clyde's GitHub organization requires local-only Actions execution. Workflows in
this repository must therefore stay fully under repository control.

Rules for every workflow:

- Do not use `uses:` steps, including first-party actions such as
  `actions/checkout` or `actions/setup-go`.
- Keep bootstrap checkout inline because repository scripts are unavailable
  before checkout.
- Put all post-checkout CI and release logic in `.github/scripts/`.
- Pin Go-based CI tools to exact versions; do not use `@latest`.
- Keep workflow permissions minimal and explicit.
- Run `.github/scripts/check-github-policy.sh` before pushing workflow changes.

Current controlled scripts:

- `install-go.sh` installs the Go version declared in `go.mod`.
- `check-github-policy.sh` rejects blocked workflow patterns.
- `build-release.sh` creates deterministic release archives.
- `generate-sbom.sh` writes the release SPDX JSON.
- `smoke-release.sh` validates archive contents and checksums.
