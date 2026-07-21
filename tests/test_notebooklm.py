import sys

import pytest

from clyde.models import ChunkRecord
from clyde.notebooklm import NotebookLMSyncError, sync_chunks


class RecordingProgress:
    def __init__(self) -> None:
        self.events = []

    def emit(self, event) -> None:
        self.events.append(event)


def _write_mcp_server(
    path,
    *,
    tools: list[str],
    fail_call: bool = False,
    call_delay: float = 0.0,
) -> None:
    path.write_text(
        f"""
import json
import sys


def read_message():
    headers = {{}}
    while True:
        line = sys.stdin.buffer.readline()
        if not line:
            raise SystemExit(0)
        if line in (b"\\r\\n", b"\\n"):
            break
        name, _, value = line.decode().partition(":")
        headers[name.lower()] = value.strip()
    return json.loads(sys.stdin.buffer.read(int(headers["content-length"])))


def send(message):
    payload = json.dumps(message, separators=(",", ":")).encode()
    sys.stdout.buffer.write(b"Content-Length: %d\\r\\n\\r\\n" % len(payload))
    sys.stdout.buffer.write(payload)
    sys.stdout.buffer.flush()


TOOLS = {tools!r}
FAIL_CALL = {fail_call!r}
CALL_DELAY = {call_delay!r}


while True:
    message = read_message()
    method = message.get("method")
    if method == "initialize":
        send({{"jsonrpc": "2.0", "id": message["id"], "result": {{}}}})
    elif method == "tools/list":
        send({{"jsonrpc": "2.0", "id": message["id"], "result": {{"tools": [{{"name": name}} for name in TOOLS]}}}})
    elif method == "tools/call":
        if CALL_DELAY:
            import time
            time.sleep(CALL_DELAY)
        if FAIL_CALL:
            send({{"jsonrpc": "2.0", "id": message["id"], "error": {{"code": -32000, "message": "upload rejected"}}}})
        else:
            send({{"jsonrpc": "2.0", "id": message["id"], "result": {{"ok": True}}}})
""".strip()
        + "\n"
    )


def test_sync_chunks_uploads_each_chunk(tmp_path) -> None:
    server = tmp_path / "server.py"
    _write_mcp_server(server, tools=["add_source"])
    chunks = [
        ChunkRecord(rel_path="a.py", sha256="abc", index=1, total=1, text="Path: a.py\n"),
        ChunkRecord(rel_path="b.py", sha256="def", index=1, total=1, text="Path: b.py\n"),
    ]

    assert sync_chunks(chunks, notebook_id="nb", command=[sys.executable, str(server)]) == 2


def test_sync_chunks_emits_progress_events(tmp_path) -> None:
    server = tmp_path / "server.py"
    _write_mcp_server(server, tools=["add_source"])
    progress = RecordingProgress()
    chunks = [
        ChunkRecord(rel_path="a.py", sha256="abc", index=1, total=1, text="Path: a.py\n"),
        ChunkRecord(rel_path="b.py", sha256="def", index=1, total=1, text="Path: b.py\n"),
    ]

    count = sync_chunks(
        chunks,
        notebook_id="nb",
        command=[sys.executable, str(server)],
        progress=progress,
        job_id="job-1",
    )

    assert count == 2
    assert [event.phase for event in progress.events] == [
        "starting",
        "checking",
        "uploading",
        "uploaded",
        "uploading",
        "uploaded",
        "complete",
    ]
    assert progress.events[-1].job_id == "job-1"
    assert progress.events[-1].done == 2


def test_sync_chunks_emits_heartbeat_during_slow_upload(tmp_path) -> None:
    server = tmp_path / "server.py"
    _write_mcp_server(server, tools=["add_source"], call_delay=0.2)
    progress = RecordingProgress()
    chunks = [ChunkRecord(rel_path="a.py", sha256="abc", index=1, total=1, text="Path: a.py\n")]

    count = sync_chunks(
        chunks,
        notebook_id="nb",
        command=[sys.executable, str(server)],
        progress=progress,
        heartbeat_interval=0.05,
        job_id="job-1",
    )

    uploading = [event for event in progress.events if event.phase == "uploading"]
    assert count == 1
    assert len(uploading) >= 2
    assert progress.events[-1].phase == "complete"


def test_sync_chunks_reports_missing_add_source(tmp_path) -> None:
    server = tmp_path / "server.py"
    _write_mcp_server(server, tools=["ask_question"])
    chunks = [ChunkRecord(rel_path="a.py", sha256="abc", index=1, total=1, text="Path: a.py\n")]

    with pytest.raises(NotebookLMSyncError, match="does not expose add_source.*ask_question"):
        sync_chunks(chunks, notebook_id="nb", command=[sys.executable, str(server)])


def test_sync_chunks_reports_failed_file_chunk(tmp_path) -> None:
    server = tmp_path / "server.py"
    _write_mcp_server(server, tools=["add_source"], fail_call=True)
    chunks = [ChunkRecord(rel_path="a.py", sha256="abc", index=1, total=2, text="Path: a.py\n")]

    with pytest.raises(NotebookLMSyncError, match=r"Failed uploading a.py chunk 1/2"):
        sync_chunks(chunks, notebook_id="nb", command=[sys.executable, str(server)])
