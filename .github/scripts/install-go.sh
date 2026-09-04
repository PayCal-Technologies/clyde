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

temp_root="${RUNNER_TEMP:-/tmp}"
if command -v cygpath >/dev/null 2>&1 && [[ "$temp_root" =~ ^[A-Za-z]: ]]; then
  temp_root="$(cygpath -u "$temp_root")"
fi

install_root="${temp_root}/go"
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

metadata_sha256() {
  local metadata_file="$1"
  local filename="$2"
  local python_command

  if command -v python3 >/dev/null 2>&1; then
    python_command=python3
  elif command -v python >/dev/null 2>&1; then
    python_command=python
  else
    echo "Python is required to read the Go download metadata" >&2
    exit 1
  fi

  "$python_command" - "$metadata_file" "$filename" <<'PY'
import json
import sys

metadata_path, filename = sys.argv[1:]
with open(metadata_path, encoding="utf-8") as metadata_file:
    releases = json.load(metadata_file)

for release in releases:
    for download in release["files"]:
        if download["filename"] == filename:
            print(download["sha256"])
            sys.exit(0)

raise SystemExit(f"checksum not found for {filename}")
PY
}

download_filename="go${version}.${goos}-${goarch}"
metadata_file="${temp_root}/go-downloads.json"

if [ "$goos" = "windows" ]; then
  archive="${temp_root}/go.zip"
  download_filename="${download_filename}.zip"
  curl -fLsSLo "$archive" "https://go.dev/dl/${download_filename}"
  curl -fLsSLo "$metadata_file" "https://go.dev/dl/?mode=json&include=all"
  expected="$(metadata_sha256 "$metadata_file" "$download_filename")"
  actual="$(sha256_file "$archive")"
  test "$actual" = "$expected"
  unzip -q "$archive" -d "$temp_root"
else
  archive="${temp_root}/go.tar.gz"
  download_filename="${download_filename}.tar.gz"
  curl -fLsSLo "$archive" "https://go.dev/dl/${download_filename}"
  curl -fLsSLo "$metadata_file" "https://go.dev/dl/?mode=json&include=all"
  expected="$(metadata_sha256 "$metadata_file" "$download_filename")"
  actual="$(sha256_file "$archive")"
  test "$actual" = "$expected"
  tar -C "$temp_root" -xzf "$archive"
fi

if [ -n "${GITHUB_PATH:-}" ]; then
  echo "${install_root}/bin" >> "$GITHUB_PATH"
fi

go_binary="${install_root}/bin/go"
if [ "$goos" = "windows" ]; then
  go_binary="${go_binary}.exe"
fi

"$go_binary" version
