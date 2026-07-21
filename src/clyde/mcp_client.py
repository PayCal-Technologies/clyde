from __future__ import annotations

import json
import os
import selectors
import subprocess
import threading
import time
from dataclasses import dataclass
from typing import Any, Literal


class MCPError(RuntimeError):
    pass


@dataclass
class MCPClient:
    command: list[str]
    env: dict[str, str]
    request_timeout: float = 30.0
    framing: Literal["auto", "content-length", "newline"] = "auto"

    def __enter__(self) -> "MCPClient":
        self._fallback_attempted = False
        self._start_process()
        try:
            self._initialize()
        except MCPError:
            if self.framing != "auto" or self._fallback_attempted:
                raise
            self._fallback_attempted = True
            self._stop_process()
            self._start_process(framing="newline")
            try:
                self._initialize()
            except MCPError:
                self._stop_process()
                raise
        return self

    def _start_process(self, *, framing: Literal["content-length", "newline"] | None = None) -> None:
        merged_env = {**os.environ, **self.env}
        self._next_id = 1
        self._framing = framing or ("content-length" if self.framing == "auto" else self.framing)
        self._stdout_buffer = bytearray()
        try:
            self._proc = subprocess.Popen(
                self.command,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env=merged_env,
            )
        except OSError as exc:
            raise MCPError(f"Failed to start MCP command {self.command!r}: {exc}") from exc
        self._stderr: list[bytes] = []
        self._stderr_thread = threading.Thread(target=self._drain_stderr, daemon=True)
        self._stderr_thread.start()

    def _initialize(self) -> None:
        self._request(
            "initialize",
            {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "clyde", "version": "0.1.0"},
            },
        )
        self._notify("notifications/initialized", {})

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        self._stop_process()

    def _stop_process(self) -> None:
        if getattr(self, "_proc", None) is not None:
            if self._proc.poll() is not None:
                return
            self._proc.terminate()
            try:
                self._proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._proc.kill()
                self._proc.wait(timeout=5)

    def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        return self._request("tools/call", {"name": name, "arguments": arguments})

    def list_tools(self) -> Any:
        return self._request("tools/list", {})

    def _request(self, method: str, params: dict[str, Any]) -> Any:
        request_id = self._next_id
        self._next_id += 1
        self._send({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params})
        deadline = time.monotonic() + self.request_timeout
        while True:
            message = self._read_message(deadline)
            if message.get("id") != request_id:
                continue
            if "error" in message:
                raise MCPError(f"MCP {method} failed: {json.dumps(message['error'])}")
            return message.get("result")

    def _notify(self, method: str, params: dict[str, Any]) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params})

    def _send(self, message: dict[str, Any]) -> None:
        if self._proc.stdin is None:
            raise MCPError("MCP process stdin is closed")
        payload = json.dumps(message, separators=(",", ":")).encode()
        if self._framing == "content-length":
            header = f"Content-Length: {len(payload)}\r\n\r\n".encode()
            self._proc.stdin.write(header + payload)
        else:
            self._proc.stdin.write(payload + b"\n")
        self._proc.stdin.flush()

    def _read_message(self, deadline: float) -> dict[str, Any]:
        if self._proc.stdout is None:
            raise MCPError("MCP process stdout is closed")

        while True:
            message = self._try_parse_message()
            if message is not None:
                return message

            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise MCPError(
                    "Timed out waiting for MCP response "
                    f"after {self.request_timeout:g}s. {self._stderr_summary()}"
                )
            self._read_stdout_chunk(timeout=remaining)

    def _try_parse_message(self) -> dict[str, Any] | None:
        if not self._stdout_buffer:
            return None

        if self._stdout_buffer.startswith(b"Content-Length:"):
            header_end = self._stdout_buffer.find(b"\r\n\r\n")
            separator_len = 4
            if header_end < 0:
                header_end = self._stdout_buffer.find(b"\n\n")
                separator_len = 2
            if header_end < 0:
                return None
            header = self._stdout_buffer[:header_end].decode("ascii", errors="replace")
            content_length = self._parse_content_length(header)
            body_start = header_end + separator_len
            body_end = body_start + content_length
            if len(self._stdout_buffer) < body_end:
                return None
            payload = bytes(self._stdout_buffer[body_start:body_end])
            del self._stdout_buffer[:body_end]
            return self._decode_json_payload(payload)

        newline = self._stdout_buffer.find(b"\n")
        if newline < 0:
            return None
        payload = bytes(self._stdout_buffer[:newline]).strip()
        del self._stdout_buffer[: newline + 1]
        if not payload:
            return None
        return self._decode_json_payload(payload)

    def _parse_content_length(self, header: str) -> int:
        for line in header.splitlines():
            name, _, value = line.partition(":")
            if name.lower() == "content-length":
                try:
                    length = int(value.strip())
                except ValueError as exc:
                    raise MCPError(f"Invalid MCP Content-Length header: {line!r}") from exc
                if length < 0:
                    raise MCPError(f"Invalid MCP Content-Length header: {line!r}")
                return length
        raise MCPError("MCP frame is missing Content-Length header")

    def _decode_json_payload(self, payload: bytes) -> dict[str, Any]:
        try:
            message = json.loads(payload.decode())
        except (UnicodeDecodeError, json.JSONDecodeError) as exc:
            preview = payload[:200].decode(errors="replace")
            raise MCPError(f"Invalid MCP JSON response: {preview!r}") from exc
        if not isinstance(message, dict):
            raise MCPError(
                "Invalid MCP JSON-RPC message: "
                f"expected object, got {type(message).__name__}"
            )
        return message

    def _read_stdout_chunk(self, *, timeout: float) -> None:
        selector = selectors.DefaultSelector()
        selector.register(self._proc.stdout, selectors.EVENT_READ)
        try:
            events = selector.select(timeout)
        finally:
            selector.close()

        if not events:
            if self._proc.poll() is not None:
                raise MCPError(f"MCP process exited before responding. {self._stderr_summary()}")
            return

        chunk = os.read(self._proc.stdout.fileno(), 4096)
        if not chunk:
            raise MCPError(f"MCP process exited before responding. {self._stderr_summary()}")
        self._stdout_buffer.extend(chunk)

    def _stderr_summary(self) -> str:
        stderr = b"".join(self._stderr[-20:]).decode(errors="replace").strip()
        if not stderr:
            return "No stderr was captured."
        return f"Recent stderr:\n{stderr}"

    def _drain_stderr(self) -> None:
        if self._proc.stderr is None:
            return
        for line in self._proc.stderr:
            self._stderr.append(line)
