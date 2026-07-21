import json

import pytest

from clyde.cli import main
from clyde.status import ProgressEvent


def test_preview_shows_included_files_and_human_size(tmp_path, capsys) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")

    status = main(["preview", str(tmp_path)])

    output = capsys.readouterr().out
    assert status == 0
    assert "Included files: 1" in output
    assert "Total included bytes: 14 (14 B)" in output
    assert "Included files (first 1):" in output
    assert "app.py (14 B)" in output


def test_preview_can_hide_file_and_skip_lists(tmp_path, capsys) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")
    (tmp_path / "notes.md").write_text("# Notes\n")

    status = main(
        [
            "preview",
            str(tmp_path),
            "--exclude",
            "*.md",
            "--show-files",
            "0",
            "--show-skips",
            "0",
        ]
    )

    output = capsys.readouterr().out
    assert status == 0
    assert "Included files (first" not in output
    assert "\nSkipped:" not in output
    assert "Skip reasons:" in output


def test_bundle_rejects_file_as_output_path(tmp_path, capsys) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")
    out_file = tmp_path / "bundle.json"
    out_file.write_text("{}\n")

    status = main(["bundle", str(tmp_path), "--out", str(out_file)])

    error = capsys.readouterr().err
    assert status == 1
    assert "--out must be a directory" in error


def test_bundle_writes_manifest_and_review_prompt(tmp_path, capsys) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")
    out_dir = tmp_path / "out"

    status = main(["bundle", str(tmp_path), "--out", str(out_dir)])

    output = capsys.readouterr().out
    manifest = json.loads((out_dir / "manifest.json").read_text())
    assert status == 0
    assert manifest["file_count"] == 1
    assert "Review manifest.json before running sync." in output


def test_bundle_subject_prints_and_records_book_plan(tmp_path, capsys) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")
    out_dir = tmp_path / "out"

    status = main(["bundle", str(tmp_path), "--out", str(out_dir), "--subject", "Demo Sync"])

    output = capsys.readouterr().out
    manifest = json.loads((out_dir / "manifest.json").read_text())
    assert status == 0
    assert "Book title:" in output
    assert manifest["book"]["title"].endswith(" - Demo Sync")


def test_book_command_prints_dated_name(capsys) -> None:
    status = main(["book", "Demo", "Sync"])

    output = capsys.readouterr().out
    assert status == 0
    assert "Book title:" in output
    assert " - Demo Sync" in output


def test_bundle_can_use_exact_book_title(tmp_path) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")
    out_dir = tmp_path / "out"

    status = main(
        [
            "bundle",
            str(tmp_path),
            "--out",
            str(out_dir),
            "--book-title",
            "2026-07-21 1030 - Demo Sync",
        ]
    )

    manifest = json.loads((out_dir / "manifest.json").read_text())
    assert status == 0
    assert manifest["book"]["title"] == "2026-07-21 1030 - Demo Sync"


def test_scan_options_require_positive_numbers(tmp_path, capsys) -> None:
    with pytest.raises(SystemExit) as excinfo:
        main(["preview", str(tmp_path), "--max-file-bytes", "0"])

    error = capsys.readouterr().err
    assert excinfo.value.code == 2
    assert "must be greater than 0" in error


def test_status_formats_empty_daemon_response(monkeypatch, capsys) -> None:
    monkeypatch.setattr("clyde.cli.rpc", lambda url, method, params: {"jobs": []})

    status = main(["status"])

    assert status == 0
    assert capsys.readouterr().out.strip() == "No jobs."


def test_sync_prints_realtime_progress_by_default(tmp_path, monkeypatch, capsys) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")

    def fake_sync(chunks, *, progress, job_id, **kwargs):
        assert progress is not None
        progress.emit(
            ProgressEvent(
                job_id=job_id,
                phase="uploading",
                message="uploading app.py [1/1]",
                done=0,
                total=len(list(chunks)),
                rel_path="app.py",
                timestamp=10.0,
            )
        )
        return 1

    monkeypatch.setattr("clyde.cli.sync_chunks", fake_sync)

    status = main(["sync", str(tmp_path), "--notebook-id", "nb", "--approve-upload"])

    captured = capsys.readouterr()
    assert status == 0
    assert "sync uploading 0/1: uploading app.py [1/1] - app.py" in captured.err
    assert "Uploaded 1 chunks to notebook nb." in captured.out


def test_sync_can_suppress_realtime_progress(tmp_path, monkeypatch, capsys) -> None:
    (tmp_path / "app.py").write_text("print('safe')\n")

    def fake_sync(chunks, *, progress, **kwargs):
        assert progress is None
        return 1

    monkeypatch.setattr("clyde.cli.sync_chunks", fake_sync)

    status = main(
        [
            "sync",
            str(tmp_path),
            "--notebook-id",
            "nb",
            "--approve-upload",
            "--quiet-progress",
        ]
    )

    captured = capsys.readouterr()
    assert status == 0
    assert captured.err == ""
