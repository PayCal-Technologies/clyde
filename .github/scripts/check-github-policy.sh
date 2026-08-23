#!/usr/bin/env bash
set -euo pipefail

status=0

if grep -R -n -E '^[[:space:]]*uses:' .github/workflows; then
  echo "GitHub Actions policy is local_only; workflow 'uses:' entries are not allowed." >&2
  status=1
fi

if grep -R --exclude=check-github-policy.sh -n -E 'go install .*@latest|go run .*@latest' .github/workflows .github/scripts; then
  echo "CI tools must be version-pinned; @latest is not allowed." >&2
  status=1
fi

if grep -R --exclude=check-github-policy.sh -n -E 'actions/|softprops/|anchore/|github/codeql-action|attest-build-provenance' .github/workflows .github/scripts; then
  echo "External GitHub Actions must not be referenced by Clyde workflows." >&2
  status=1
fi

exit "$status"
