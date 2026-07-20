from __future__ import annotations

import fnmatch
import hashlib
import os
import re
import subprocess
from pathlib import Path

from .models import FileRecord, ScanResult, SkipRecord

DEFAULT_EXCLUDES = [
    ".git/**",
    ".hg/**",
    ".svn/**",
    ".venv/**",
    "__pycache__/**",
    "node_modules/**",
    "vendor/**",
    "build/**",
    "dist/**",
    ".next/**",
    ".turbo/**",
    "DerivedData/**",
    "*.pyc",
    "*.pyo",
    "*.o",
    "*.a",
    "*.so",
    "*.dylib",
    "*.dll",
    "*.exe",
    "*.zip",
    "*.tar",
    "*.gz",
    "*.7z",
    "*.dmg",
    "*.png",
    "*.jpg",
    "*.jpeg",
    "*.gif",
    "*.webp",
    "*.pdf",
    "*.sqlite",
    "*.db",
    "*.lock",
]

SECRET_PATTERNS = [
    re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----"),
    re.compile(r"(?i)\b(api[_-]?key|secret|token|password)\b\s*[:=]\s*['\"][^'\"]{12,}['\"]"),
    re.compile(r"\b[A-Za-z0-9_=-]{32,}\.[A-Za-z0-9_=-]{16,}\.[A-Za-z0-9_=-]{16,}\b"),
]


def scan_repo(
    repo: Path,
    *,
    include: list[str] | None = None,
    exclude: list[str] | None = None,
    max_file_bytes: int = 250_000,
) -> ScanResult:
    repo = repo.expanduser().resolve()
    if not repo.exists() or not repo.is_dir():
        raise ValueError(f"repo path is not a directory: {repo}")

    result = ScanResult(repo=repo)
    candidates = _candidate_paths(repo)
    excludes = [*DEFAULT_EXCLUDES, *(exclude or [])]

    for path in candidates:
        rel = path.relative_to(repo).as_posix()
        if include and not _matches_any(rel, include):
            result.skips.append(SkipRecord(rel, "not matched by include globs"))
            continue
        if _matches_any(rel, excludes):
            result.skips.append(SkipRecord(rel, "excluded by glob"))
            continue
        try:
            stat = path.stat()
        except OSError as exc:
            result.skips.append(SkipRecord(rel, f"stat failed: {exc}"))
            continue
        if stat.st_size > max_file_bytes:
            result.skips.append(SkipRecord(rel, f"larger than {max_file_bytes} bytes"))
            continue
        try:
            data = path.read_bytes()
        except OSError as exc:
            result.skips.append(SkipRecord(rel, f"read failed: {exc}"))
            continue
        if _looks_binary(data):
            result.skips.append(SkipRecord(rel, "binary file"))
            continue
        text = _decode_text(data)
        if _looks_secret(text):
            result.skips.append(SkipRecord(rel, "possible secret material"))
            continue
        result.files.append(
            FileRecord(
                path=path,
                rel_path=rel,
                size=stat.st_size,
                sha256=hashlib.sha256(data).hexdigest(),
                text=text,
            )
        )
    return result


def _candidate_paths(repo: Path) -> list[Path]:
    if (repo / ".git").exists():
        try:
            proc = subprocess.run(
                ["git", "-C", str(repo), "ls-files", "-co", "--exclude-standard"],
                check=True,
                text=True,
                capture_output=True,
            )
            return [repo / line for line in proc.stdout.splitlines() if line]
        except (OSError, subprocess.CalledProcessError):
            pass

    paths: list[Path] = []
    for root, dirs, files in os.walk(repo):
        dirs[:] = [name for name in dirs if name not in {".git", "node_modules", ".venv"}]
        base = Path(root)
        paths.extend(base / name for name in files)
    return sorted(paths)


def _matches_any(rel_path: str, patterns: list[str]) -> bool:
    return any(fnmatch.fnmatch(rel_path, pattern) for pattern in patterns)


def _looks_binary(data: bytes) -> bool:
    if b"\0" in data:
        return True
    sample = data[:4096]
    if not sample:
        return False
    control = sum(byte < 9 or 13 < byte < 32 for byte in sample)
    return control / len(sample) > 0.05


def _decode_text(data: bytes) -> str:
    return data.decode("utf-8", errors="replace")


def _looks_secret(text: str) -> bool:
    return any(pattern.search(text) for pattern in SECRET_PATTERNS)
