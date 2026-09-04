#!/usr/bin/env bash
set -euo pipefail

if [ ! -f go.mod ]; then
  echo "go.mod not found; run from repository root" >&2
  exit 1
fi

version="$(awk '$1 == "go" { print $2 }' go.mod)"
if [ -z "$version" ]; then
  echo "go.mod does not declare a Go version" >&2
  exit 1
fi

os_name="${1:-$(uname -s)}"
arch_name="${2:-$(uname -m)}"

case "$(printf '%s' "$os_name" | tr '[:upper:]' '[:lower:]')" in
  linux*) goos="linux" ;;
  darwin*) goos="darwin" ;;
  mingw*|msys*|cygwin*|windows*) goos="windows" ;;
  *) echo "unsupported OS: $os_name" >&2; exit 1 ;;
esac

case "$(printf '%s' "$arch_name" | tr '[:upper:]' '[:lower:]')" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "unsupported architecture: $arch_name" >&2; exit 1 ;;
esac

install_root="${RUNNER_TEMP:-/tmp}/go"
rm -rf "$install_root"

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{ print $1 }'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{ print $1 }'
    return
  fi
  if command -v python3 >/dev/null 2>&1; then
    python3 - "$path" <<'PY'
import hashlib
import sys

h = hashlib.sha256()
with open(sys.argv[1], "rb") as f:
    for chunk in iter(lambda: f.read(1024 * 1024), b""):
        h.update(chunk)
print(h.hexdigest())
PY
    return
  fi
  if command -v python >/dev/null 2>&1; then
    python - "$path" <<'PY'
import hashlib
import sys

h = hashlib.sha256()
with open(sys.argv[1], "rb") as f:
    for chunk in iter(lambda: f.read(1024 * 1024), b""):
        h.update(chunk)
print(h.hexdigest())
PY
    return
  fi
  echo "no SHA-256 tool found" >&2
  exit 1
}

if [ "$goos" = "windows" ]; then
  archive="${RUNNER_TEMP:-/tmp}/go.zip"
  checksum_file="${archive}.sha256"
  curl -fsSLo "$archive" "https://go.dev/dl/go${version}.${goos}-${goarch}.zip"
  curl -fsSLo "$checksum_file" "https://go.dev/dl/go${version}.${goos}-${goarch}.zip.sha256"
  expected="$(awk '{ print $1 }' "$checksum_file")"
  actual="$(sha256_file "$archive")"
  test "$actual" = "$expected"
  unzip -q "$archive" -d "${RUNNER_TEMP:-/tmp}"
else
  archive="${RUNNER_TEMP:-/tmp}/go.tar.gz"
  checksum_file="${archive}.sha256"
  curl -fsSLo "$archive" "https://go.dev/dl/go${version}.${goos}-${goarch}.tar.gz"
  curl -fsSLo "$checksum_file" "https://go.dev/dl/go${version}.${goos}-${goarch}.tar.gz.sha256"
  expected="$(awk '{ print $1 }' "$checksum_file")"
  actual="$(sha256_file "$archive")"
  test "$actual" = "$expected"
  tar -C "${RUNNER_TEMP:-/tmp}" -xzf "$archive"
fi

if [ -n "${GITHUB_PATH:-}" ]; then
  echo "${install_root}/bin" >> "$GITHUB_PATH"
fi

"${install_root}/bin/go" version
