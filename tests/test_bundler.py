from pathlib import Path

from clyde.bundler import make_chunks
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
