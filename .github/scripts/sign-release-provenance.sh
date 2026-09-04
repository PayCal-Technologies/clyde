#!/usr/bin/env bash
set -euo pipefail

if [ ! -f dist/clyde.provenance.intoto.json ]; then
  echo "dist/clyde.provenance.intoto.json not found" >&2
  exit 1
fi

if ! command -v cosign >/dev/null 2>&1; then
  go install github.com/sigstore/cosign/v2/cmd/cosign@v2.6.1
  export PATH="$(go env GOPATH)/bin:${PATH}"
fi

cosign sign-blob \
  --yes \
  --oidc-provider github-actions \
  --bundle dist/clyde.provenance.intoto.json.sigstore.json \
  dist/clyde.provenance.intoto.json

.github/scripts/write-release-checksums.sh
