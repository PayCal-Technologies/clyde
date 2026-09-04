#!/usr/bin/env bash
set -euo pipefail

version="$(tr -d '[:space:]' < VERSION)"
heading="## v${version}"
output="${1:-dist/RELEASE_NOTES.md}"

notes="$(awk -v heading="$heading" '
  index($0, heading) == 1 { found = 1; next }
  found && /^## / { exit }
  found { print }
' CHANGELOG.md)"

if [[ -z "${notes//[[:space:]]/}" ]]; then
  echo "missing changelog notes for ${heading}" >&2
  exit 1
fi

mkdir -p "$(dirname "$output")"
{
  printf '# Clyde v%s\n\n' "$version"
  printf 'Clyde packages reviewed local source into auditable bundles and uploads only after explicit approval.\n\n'
  printf '## What changed\n\n%s\n' "$notes"
  printf '\n## Before you sync\n\n'
  printf '1. Run `clyde doctor .` and `clyde preview .`.\n'
  printf '2. Create and inspect a bundle with `clyde bundle . --out .clyde/out`.\n'
  printf '3. Run `clyde sync --bundle .clyde/out --notebook-id YOUR_NOTEBOOK_ID --dry-run`.\n'
  printf '4. Verify the bundle and use the printed approval command only after review.\n'
  printf '\nRelease archives include SHA-256 checksums, an SPDX SBOM, and signed SLSA provenance.\n'
} > "$output"
