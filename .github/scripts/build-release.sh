#!/usr/bin/env bash
set -euo pipefail

version="$(cat VERSION)"
source_date_epoch="$(git log -1 --format=%ct)"

.github/scripts/generate-man-page.sh --check

rm -rf dist
mkdir -p dist

touch_release_tree() {
  local path="$1"
  if touch -h -d "@${source_date_epoch}" "$path" 2>/dev/null; then
    return
  fi
  local stamp
  stamp="$(date -u -r "$source_date_epoch" +%Y%m%d%H%M.%S)"
  TZ=UTC touch -h -t "$stamp" "$path"
}

sha256_write() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$@"
  else
    shasum -a 256 "$@"
  fi
}

for goos in linux darwin windows; do
  for goarch in amd64 arm64; do
    name="clyde_${version}_${goos}_${goarch}"
    bin="clyde"
    if [ "$goos" = "windows" ]; then bin="clyde.exe"; fi
    mkdir -p "dist/${name}"
    GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
      -trimpath \
      -ldflags "-s -w" \
      -o "dist/${name}/${bin}" ./cmd/clyde
    cp README.md LICENSE VERSION docs/clyde.1 "dist/${name}/"
    while IFS= read -r -d '' path; do
      touch_release_tree "$path"
    done < <(find "dist/${name}" -print0)
    if [ "$goos" = "windows" ]; then
      (cd dist && TZ=UTC zip -X -qr "${name}.zip" "${name}")
    else
      python3 - "$source_date_epoch" "dist/${name}" "dist/${name}.tar.gz" <<'PY'
import gzip
import os
import sys
import tarfile

epoch = int(sys.argv[1])
root = sys.argv[2]
target = sys.argv[3]
base = os.path.basename(root)

def reset(info):
    info.uid = 0
    info.gid = 0
    info.uname = ""
    info.gname = ""
    info.mtime = epoch
    return info

with open(target, "wb") as raw:
    with gzip.GzipFile(fileobj=raw, mode="wb", filename="", mtime=epoch) as gz:
        with tarfile.open(fileobj=gz, mode="w") as tar:
            for current, dirs, files in os.walk(root):
                dirs.sort()
                files.sort()
                rel_dir = os.path.relpath(current, os.path.dirname(root))
                tar.add(current, arcname=rel_dir, recursive=False, filter=reset)
                for filename in files:
                    path = os.path.join(current, filename)
                    rel = os.path.join(base, os.path.relpath(path, root))
                    tar.add(path, arcname=rel, recursive=False, filter=reset)
PY
    fi
    rm -rf "dist/${name}"
  done
done

.github/scripts/write-release-checksums.sh
