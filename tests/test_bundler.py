from pathlib import Path

from clyde.bundler import make_chunks, write_bundle
from clyde.models import FileRecord, ScanResult


def test_make_chunks_preserves_path_header() -> None:
    result = ScanResult(
        repo=Path("/tmp/example"),
        files=[
            FileRecord(
                path=Path("/tmp/example/app.py"),
                rel_path="app.py",
                size=12,
                sha256="abc123",
                text="print('hi')\n",
            )
        ],
    )

    chunks = make_chunks(result, max_chunk_chars=100)

    assert len(chunks) == 1
    assert "Path: app.py" in chunks[0].text
    assert "SHA-256: abc123" in chunks[0].text


def test_make_chunks_can_include_book_header() -> None:
    result = ScanResult(
        repo=Path("/tmp/example"),
        files=[
            FileRecord(
                path=Path("/tmp/example/app.py"),
                rel_path="app.py",
                size=12,
                sha256="abc123",
                text="print('hi')\n",
            )
        ],
    )

    chunks = make_chunks(result, max_chunk_chars=100, book_title="2026-07-21 1435 - Demo")

    assert "Book: 2026-07-21 1435 - Demo" in chunks[0].text


def test_write_bundle_records_book_metadata(tmp_path) -> None:
    result = ScanResult(repo=tmp_path)

    manifest = write_bundle(
        result,
        tmp_path / "out",
        book_title="2026-07-21 1435 - Demo",
        book_slug="20260721-1435-demo",
    )

    assert manifest["book"] == {
        "title": "2026-07-21 1435 - Demo",
        "slug": "20260721-1435-demo",
    }
