from __future__ import annotations

from collections.abc import Iterable
from queue import Empty, Queue
from threading import Thread
from typing import Any, Callable, TypeVar

from .mcp_client import MCPClient, MCPError
from .models import ChunkRecord
from .status import NullProgressSink, ProgressEvent, ProgressSink

DEFAULT_COMMAND = ["npx", "-y", "notebooklm-mcp@2.0.0"]
DEFAULT_ENV = {
    "NOTEBOOKLM_ACCOUNT": "codex-test",
    "NOTEBOOKLM_TRANSPORT": "stdio",
    "NOTEBOOKLM_PROFILE": "all",
    "NOTEBOOKLM_DISABLED_TOOLS": "cleanup_data,re_auth,remove_notebook,update_notebook",
}


class NotebookLMSyncError(RuntimeError):
    pass


T = TypeVar("T")


def sync_chunks(
    chunks: Iterable[ChunkRecord],
    *,
    notebook_id: str | None = None,
    notebook_url: str | None = None,
    command: list[str] | None = None,
    env: dict[str, str] | None = None,
    request_timeout: float = 120.0,
    heartbeat_interval: float = 1.0,
    progress: ProgressSink | None = None,
    job_id: str = "sync",
    title_prefix: str = "",
) -> int:
    if not notebook_id and not notebook_url:
        raise ValueError("sync requires notebook_id or notebook_url")
    count = 0
    items = list(chunks)
    total = len(items)
    sink = progress or NullProgressSink()
    sink.emit(
        ProgressEvent(
            job_id=job_id,
            phase="starting",
            message="connecting to NotebookLM",
            total=total,
        )
    )
    try:
        client = MCPClient(
            command or DEFAULT_COMMAND,
            {**DEFAULT_ENV, **(env or {})},
            request_timeout=request_timeout,
        )
        try:
            _with_heartbeat(
                client.__enter__,
                sink=sink,
                job_id=job_id,
                phase="starting",
                message="connecting to NotebookLM",
                done=count,
                total=total,
                interval=heartbeat_interval,
            )
            sink.emit(
                ProgressEvent(
                    job_id=job_id,
                    phase="checking",
                    message="checking MCP tools",
                    total=total,
                )
            )
            tools = _with_heartbeat(
                client.list_tools,
                sink=sink,
                job_id=job_id,
                phase="checking",
                message="checking MCP tools",
                done=count,
                total=total,
                interval=heartbeat_interval,
            )
            names = {item.get("name") for item in tools.get("tools", [])}
            if "add_source" not in names:
                available = ", ".join(sorted(name for name in names if isinstance(name, str)))
                detail = f" Available tools: {available}." if available else ""
                message = "NotebookLM MCP server does not expose add_source." + detail
                sink.emit(
                    ProgressEvent(
                        job_id=job_id,
                        phase="failed",
                        message="NotebookLM MCP server does not expose add_source",
                        done=count,
                        total=total,
                        error=message,
                    )
                )
                raise NotebookLMSyncError(message)
            for chunk in items:
                title = f"{title_prefix}{chunk.rel_path} [{chunk.index}/{chunk.total}]"
                sink.emit(
                    ProgressEvent(
                        job_id=job_id,
                        phase="uploading",
                        message=f"uploading {title}",
                        done=count,
                        total=total,
                        rel_path=chunk.rel_path,
                    )
                )
                try:
                    _with_heartbeat(
                        lambda: client.call_tool(
                            "add_source",
                            {
                                "type": "text",
                                "title": title,
                                "content": chunk.text,
                                **({"notebook_id": notebook_id} if notebook_id else {}),
                                **({"notebook_url": notebook_url} if notebook_url else {}),
                            },
                        ),
                        sink=sink,
                        job_id=job_id,
                        phase="uploading",
                        message=f"uploading {title}",
                        done=count,
                        total=total,
                        rel_path=chunk.rel_path,
                        interval=heartbeat_interval,
                    )
                except MCPError as exc:
                    error = (
                        f"Failed uploading {chunk.rel_path} "
                        f"chunk {chunk.index}/{chunk.total}: {exc}"
                    )
                    sink.emit(
                        ProgressEvent(
                            job_id=job_id,
                            phase="failed",
                            message=f"failed uploading {title}",
                            done=count,
                            total=total,
                            rel_path=chunk.rel_path,
                            error=str(exc),
                        )
                    )
                    raise NotebookLMSyncError(error) from exc
                count += 1
                sink.emit(
                    ProgressEvent(
                        job_id=job_id,
                        phase="uploaded",
                        message=f"uploaded {title}",
                        done=count,
                        total=total,
                        rel_path=chunk.rel_path,
                    )
                )
        finally:
            client.__exit__(None, None, None)
    except MCPError as exc:
        sink.emit(
            ProgressEvent(
                job_id=job_id,
                phase="failed",
                message="NotebookLM MCP connection failed",
                done=count,
                total=total,
                error=str(exc),
            )
        )
        raise NotebookLMSyncError(f"NotebookLM MCP connection failed: {exc}") from exc
    sink.emit(
        ProgressEvent(
            job_id=job_id,
            phase="complete",
            message=f"uploaded {count} chunks",
            done=count,
            total=total,
        )
    )
    return count


def _with_heartbeat(
    action: Callable[[], T],
    *,
    sink: ProgressSink,
    job_id: str,
    phase: str,
    message: str,
    done: int,
    total: int,
    interval: float,
    rel_path: str | None = None,
) -> T:
    if interval <= 0:
        return action()

    result_queue: Queue[tuple[bool, Any]] = Queue(maxsize=1)

    def run() -> None:
        try:
            result_queue.put((True, action()))
        except BaseException as exc:
            result_queue.put((False, exc))

    thread = Thread(target=run, daemon=True)
    thread.start()
    while True:
        try:
            ok, value = result_queue.get(timeout=interval)
        except Empty:
            sink.emit(
                ProgressEvent(
                    job_id=job_id,
                    phase=phase,
                    message=message,
                    done=done,
                    total=total,
                    rel_path=rel_path,
                )
            )
            continue
        thread.join(timeout=0)
        if ok:
            return value
        raise value
