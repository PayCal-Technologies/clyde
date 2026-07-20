from __future__ import annotations

import json
import os
import subprocess
import threading
from dataclasses import dataclass
from typing import Any


class MCPError(RuntimeError):
    pass


@dataclass
class MCPClient:
    command: list[str]
    env: dict[str, str]

    def __enter__(self) -> "MCPClient":
        merged_env = {**os.environ, **self.env}
        self._next_id = 1
        self._proc = subprocess.Popen(
            self.command,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            env=merged_env,
        )
        self._stderr: list[str] = []
        self._stderr_thread = threading.Thread(target=self._drain_stderr, daemon=True)
        self._stderr_thread.start()
        self._request(
            "initialize",
            {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "clyde", "version": "0.1.0"},
            },
        )
        self._notify("notifications/initialized", {})
        return self

    def __exit__(self, exc_type: object, exc: object, traceback: object) -> None:
        if getattr(self, "_proc", None) is not None:
            self._proc.terminate()
            try:
                self._proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._proc.kill()

    def call_tool(self, name: str, arguments: dict[str, Any]) -> Any:
        return self._request("tools/call", {"name": name, "arguments": arguments})

    def list_tools(self) -> Any:
        return self._request("tools/list", {})

    def _request(self, method: str, params: dict[str, Any]) -> Any:
        request_id = self._next_id
        self._next_id += 1
        self._send({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params})
        while True:
            line = self._readline()
            message = json.loads(line)
            if message.get("id") != request_id:
                continue
            if "error" in message:
                raise MCPError(json.dumps(message["error"]))
            return message.get("result")

    def _notify(self, method: str, params: dict[str, Any]) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params})

    def _send(self, message: dict[str, Any]) -> None:
        if self._proc.stdin is None:
            raise MCPError("MCP process stdin is closed")
        self._proc.stdin.write(json.dumps(message) + "\n")
        self._proc.stdin.flush()

    def _readline(self) -> str:
        if self._proc.stdout is None:
            raise MCPError("MCP process stdout is closed")
        line = self._proc.stdout.readline()
        if not line:
            stderr = "".join(self._stderr[-20:])
            raise MCPError(f"MCP process exited before responding. stderr:\n{stderr}")
        return line

    def _drain_stderr(self) -> None:
        if self._proc.stderr is None:
            return
        for line in self._proc.stderr:
            self._stderr.append(line)
