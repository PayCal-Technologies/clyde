from __future__ import annotations

from collections.abc import Iterable

from .mcp_client import MCPClient
from .models import ChunkRecord

DEFAULT_COMMAND = ["npx", "-y", "notebooklm-mcp@2.0.0"]
DEFAULT_ENV = {
    "NOTEBOOKLM_TRANSPORT": "stdio",
    "NOTEBOOKLM_PROFILE": "all",
    "NOTEBOOKLM_DISABLED_TOOLS": "cleanup_data,re_auth,remove_notebook,update_notebook",
}


def sync_chunks(
    chunks: Iterable[ChunkRecord],
    *,
    notebook_id: str,
    command: list[str] | None = None,
    env: dict[str, str] | None = None,
) -> int:
    count = 0
    with MCPClient(command or DEFAULT_COMMAND, {**DEFAULT_ENV, **(env or {})}) as client:
        tools = client.list_tools()
        names = {item.get("name") for item in tools.get("tools", [])}
        if "add_source" not in names:
            raise RuntimeError("NotebookLM MCP server does not expose add_source")
        for chunk in chunks:
            title = f"{chunk.rel_path} [{chunk.index}/{chunk.total}]"
            client.call_tool(
                "add_source",
                {
                    "notebook_id": notebook_id,
                    "type": "text",
                    "title": title,
                    "content": chunk.text,
                },
            )
            count += 1
    return count
