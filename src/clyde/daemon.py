from __future__ import annotations

import argparse
import json
import threading
from collections import deque
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
from urllib.parse import parse_qs, urlparse

from .status import ProgressEvent

DEFAULT_HOST = "127.0.0.1"
DEFAULT_PORT = 5876
MAX_EVENTS = 500


@dataclass
class JobStatus:
    job_id: str
    phase: str = "unknown"
    message: str = ""
    done: int = 0
    total: int = 0
    rel_path: str | None = None
    error: str | None = None
    updated_at: float = 0.0
    events: deque[dict[str, Any]] = field(default_factory=lambda: deque(maxlen=MAX_EVENTS))

    @classmethod
    def from_event(cls, event: ProgressEvent) -> "JobStatus":
        status = cls(job_id=event.job_id)
        status.apply(event)
        return status

    def apply(self, event: ProgressEvent) -> None:
        self.phase = event.phase
        self.message = event.message
        self.done = event.done
        self.total = event.total
        self.rel_path = event.rel_path
        self.error = event.error
        self.updated_at = event.timestamp
        self.events.append(event.to_dict())

    def summary(self) -> dict[str, Any]:
        return {
            "job_id": self.job_id,
            "phase": self.phase,
            "message": self.message,
            "done": self.done,
            "total": self.total,
            "rel_path": self.rel_path,
            "error": self.error,
            "updated_at": self.updated_at,
        }

    def detail(self) -> dict[str, Any]:
        data = self.summary()
        data["events"] = list(self.events)
        return data


class StatusStore:
    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._jobs: dict[str, JobStatus] = {}

    def event(self, event: ProgressEvent) -> dict[str, Any]:
        with self._lock:
            job = self._jobs.get(event.job_id)
            if job is None:
                job = JobStatus.from_event(event)
                self._jobs[event.job_id] = job
            else:
                job.apply(event)
            return job.summary()

    def get(self, job_id: str | None = None) -> dict[str, Any]:
        with self._lock:
            if job_id:
                job = self._jobs.get(job_id)
                return {"job": job.detail() if job else None}
            return {"jobs": [job.summary() for job in self._jobs.values()]}

    def reset(self, job_id: str | None = None) -> dict[str, Any]:
        with self._lock:
            if job_id:
                removed = self._jobs.pop(job_id, None) is not None
                return {"removed": removed}
            count = len(self._jobs)
            self._jobs.clear()
            return {"removed": count}


class StatusHTTPServer(ThreadingHTTPServer):
    def __init__(self, server_address: tuple[str, int], store: StatusStore) -> None:
        super().__init__(server_address, StatusRequestHandler)
        self.store = store


class StatusRequestHandler(BaseHTTPRequestHandler):
    server: StatusHTTPServer

    def do_GET(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path not in {"/", "/status"}:
            self._send_json(404, {"error": "not found"})
            return
        query = parse_qs(parsed.query)
        job_id = query.get("job_id", [None])[0]
        self._send_json(200, self.server.store.get(job_id))

    def do_POST(self) -> None:
        parsed = urlparse(self.path)
        if parsed.path not in {"/", "/rpc"}:
            self._send_json(404, {"error": "not found"})
            return
        try:
            payload = self._read_json()
            response = self._handle_rpc(payload)
        except ValueError as exc:
            response = _rpc_error(None, -32700, str(exc))
        self._send_json(200, response)

    def log_message(self, format: str, *args: object) -> None:
        return None

    def _handle_rpc(self, payload: dict[str, Any]) -> dict[str, Any]:
        request_id = payload.get("id")
        if payload.get("jsonrpc") != "2.0":
            return _rpc_error(request_id, -32600, "expected jsonrpc 2.0")
        method = payload.get("method")
        params = payload.get("params") or {}
        if not isinstance(params, dict):
            return _rpc_error(request_id, -32602, "params must be an object")

        if method == "daemon.ping":
            return _rpc_result(request_id, {"ok": True})
        if method == "status.get":
            return _rpc_result(request_id, self.server.store.get(params.get("job_id")))
        if method == "status.reset":
            return _rpc_result(request_id, self.server.store.reset(params.get("job_id")))
        if method == "status.event":
            try:
                event = ProgressEvent(**params)
            except TypeError as exc:
                return _rpc_error(request_id, -32602, f"invalid status event: {exc}")
            return _rpc_result(request_id, self.server.store.event(event))
        return _rpc_error(request_id, -32601, f"unknown method: {method}")

    def _read_json(self) -> dict[str, Any]:
        length = int(self.headers.get("Content-Length", "0"))
        if length <= 0:
            raise ValueError("missing request body")
        try:
            payload = json.loads(self.rfile.read(length).decode("utf-8"))
        except json.JSONDecodeError as exc:
            raise ValueError(f"invalid json: {exc}") from exc
        if not isinstance(payload, dict):
            raise ValueError("request body must be a JSON object")
        return payload

    def _send_json(self, status: int, payload: dict[str, Any]) -> None:
        body = json.dumps(payload, indent=2, sort_keys=True).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


def serve(host: str = DEFAULT_HOST, port: int = DEFAULT_PORT) -> None:
    if host not in {"127.0.0.1", "localhost", "::1"}:
        raise ValueError("Clyde daemon only binds to localhost addresses")
    server = StatusHTTPServer((host, port), StatusStore())
    print(f"clyde daemon listening on http://{host}:{server.server_port}/rpc", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


def rpc(url: str, method: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
    import urllib.request

    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": method,
        "params": params or {},
    }
    data = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(request, timeout=5) as response:
        parsed = json.loads(response.read().decode("utf-8"))
    if "error" in parsed:
        raise RuntimeError(parsed["error"].get("message", "JSON-RPC error"))
    return parsed["result"]


def status_url(args: argparse.Namespace) -> str:
    return f"http://{args.host}:{args.port}/rpc"


def _rpc_result(request_id: Any, result: dict[str, Any]) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "result": result}


def _rpc_error(request_id: Any, code: int, message: str) -> dict[str, Any]:
    return {"jsonrpc": "2.0", "id": request_id, "error": {"code": code, "message": message}}

