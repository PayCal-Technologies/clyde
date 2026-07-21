from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path

from .models import ChunkRecord, ScanResult


def make_chunks(
    result: ScanResult,
    *,
    max_chunk_chars: int = 18_000,
    book_title: str | None = None,
) -> list[ChunkRecord]:
    chunks: list[ChunkRecord] = []
    for file in result.files:
        pieces = _split_text(file.text, max_chunk_chars)
        total = len(pieces)
        for index, piece in enumerate(pieces, start=1):
            header = [f"Repository: {result.repo.name}"]
            if book_title:
                header.append(f"Book: {book_title}")
            header.extend(
                [
                    f"Path: {file.rel_path}",
                    f"SHA-256: {file.sha256}",
                    f"Chunk: {index}/{total}",
                ]
            )
            payload = "\n".join(header) + f"\n\n{piece}"
            chunks.append(
                ChunkRecord(
                    rel_path=file.rel_path,
                    sha256=file.sha256,
                    index=index,
                    total=total,
                    text=payload,
                )
            )
    return chunks


def write_bundle(
    result: ScanResult,
    out_dir: Path,
    *,
    max_chunk_chars: int = 18_000,
    book_title: str | None = None,
    book_slug: str | None = None,
) -> dict:
    out_dir.mkdir(parents=True, exist_ok=True)
    chunks = make_chunks(result, max_chunk_chars=max_chunk_chars, book_title=book_title)

    manifest = {
        "schema": "clyde.bundle.v1",
        "created_at": datetime.now(timezone.utc).isoformat(),
        "repo": str(result.repo),
        "repo_name": result.repo.name,
        "file_count": len(result.files),
        "chunk_count": len(chunks),
        "total_bytes": result.total_bytes,
        "book": (
            {"title": book_title, "slug": book_slug}
            if book_title or book_slug
            else None
        ),
        "files": [
            {"path": item.rel_path, "size": item.size, "sha256": item.sha256}
            for item in result.files
        ],
        "skips": [{"path": item.rel_path, "reason": item.reason} for item in result.skips],
    }

    (out_dir / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    with (out_dir / "chunks.jsonl").open("w", encoding="utf-8") as handle:
        for chunk in chunks:
            handle.write(
                json.dumps(
                    {
                        "path": chunk.rel_path,
                        "sha256": chunk.sha256,
                        "chunk_index": chunk.index,
                        "chunk_total": chunk.total,
                        "text": chunk.text,
                    }
                )
                + "\n"
            )
    return manifest


def _split_text(text: str, max_chunk_chars: int) -> list[str]:
    if len(text) <= max_chunk_chars:
        return [text]

    chunks: list[str] = []
    current: list[str] = []
    current_len = 0
    for line in text.splitlines(keepends=True):
        if current and current_len + len(line) > max_chunk_chars:
            chunks.append("".join(current))
            current = []
            current_len = 0
        if len(line) > max_chunk_chars:
            for start in range(0, len(line), max_chunk_chars):
                part = line[start : start + max_chunk_chars]
                if current:
                    chunks.append("".join(current))
                    current = []
                    current_len = 0
                chunks.append(part)
            continue
        current.append(line)
        current_len += len(line)
    if current:
        chunks.append("".join(current))
    return chunks
