#!/usr/bin/env bash
set -euo pipefail

if [ ! -d dist ]; then
  echo "dist directory not found" >&2
  exit 1
fi

mapfile -d '' files < <(find dist -maxdepth 1 -type f ! -name SHA256SUMS -print0 | sort -z)
if [ "${#files[@]}" -eq 0 ]; then
  echo "no release files found in dist" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  (cd dist && sha256sum "${files[@]#dist/}" > SHA256SUMS)
else
  (cd dist && shasum -a 256 "${files[@]#dist/}" > SHA256SUMS)
fi
