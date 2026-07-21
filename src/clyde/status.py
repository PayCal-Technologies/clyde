from __future__ import annotations

import json
import sys
import time
import urllib.error
import urllib.request
from collections.abc import Iterable
from dataclasses import asdict, dataclass, field
from datetime import datetime
from typing import Any, Protocol


@dataclass(frozen=True)
class ProgressEvent:
    job_id: str
    phase: str
    message: str
    done: int = 0
    total: int = 0
    rel_path: str | None = None
    error: str | None = None
    timestamp: float = field(default_factory=time.time)

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


class ProgressSink(Protocol):
    def emit(self, event: ProgressEvent) -> None:
        pass


class NullProgressSink:
    def emit(self, event: ProgressEvent) -> None:
        return None


class ConsoleProgressSink:
    def __init__(self, stream: Any | None = None) -> None:
        self.stream = stream or sys.stderr

    def emit(self, event: ProgressEvent) -> None:
        total = event.total or 0
        progress = f"{event.done}/{total}" if total else str(event.done)
        timestamp = datetime.fromtimestamp(event.timestamp).strftime("%H:%M:%S")
        detail = f" - {event.rel_path}" if event.rel_path else ""
        error = f" ({event.error})" if event.error else ""
        print(
            f"[{timestamp}] {event.job_id} {event.phase} {progress}: "
            f"{event.message}{detail}{error}",
            file=self.stream,
            flush=True,
        )


class TeeProgressSink:
    def __init__(self, sinks: Iterable[ProgressSink], *, ignore_errors: bool = False) -> None:
        self.sinks = list(sinks)
        self.ignore_errors = ignore_errors

    def emit(self, event: ProgressEvent) -> None:
        for sink in self.sinks:
            try:
                sink.emit(event)
            except Exception:
                if not self.ignore_errors:
                    raise


class HTTPProgressSink:
    def __init__(self, url: str, *, timeout: float = 2.0) -> None:
        self.url = url.rstrip("/")
        self.timeout = timeout
        self._request_id = 0

    def emit(self, event: ProgressEvent) -> None:
        self._request_id += 1
        payload = {
            "jsonrpc": "2.0",
            "id": self._request_id,
            "method": "status.event",
            "params": event.to_dict(),
        }
        data = json.dumps(payload, separators=(",", ":")).encode("utf-8")
        request = urllib.request.Request(
            self.url,
            data=data,
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(request, timeout=self.timeout) as response:
                response.read()
        except (OSError, urllib.error.URLError) as exc:
            raise RuntimeError(f"failed to report status event to {self.url}: {exc}") from exc
