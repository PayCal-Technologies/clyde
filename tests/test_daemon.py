import json
import threading
import urllib.request

from clyde.daemon import StatusHTTPServer, StatusStore


def _post(url: str, method: str, params: dict | None = None) -> dict:
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": method,
        "params": params or {},
    }
    request = urllib.request.Request(
        url,
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=5) as response:
        return json.loads(response.read().decode())


def test_daemon_records_and_returns_status_event() -> None:
    server = StatusHTTPServer(("127.0.0.1", 0), StatusStore())
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        url = f"http://127.0.0.1:{server.server_port}/rpc"

        event = {
            "job_id": "sync-1",
            "phase": "uploading",
            "message": "uploading a.py [1/1]",
            "done": 0,
            "total": 1,
            "rel_path": "a.py",
            "timestamp": 10.0,
        }
        response = _post(url, "status.event", event)
        status = _post(url, "status.get", {"job_id": "sync-1"})

        assert response["result"]["phase"] == "uploading"
        assert status["result"]["job"]["job_id"] == "sync-1"
        assert status["result"]["job"]["events"] == [event | {"error": None}]
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def test_daemon_rejects_unknown_rpc_method() -> None:
    server = StatusHTTPServer(("127.0.0.1", 0), StatusStore())
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        url = f"http://127.0.0.1:{server.server_port}/rpc"
        response = _post(url, "missing.method")

        assert response["error"]["code"] == -32601
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
