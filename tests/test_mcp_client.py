import sys

from clyde.mcp_client import MCPClient


def test_mcp_client_reads_content_length_frames(tmp_path) -> None:
    server = tmp_path / "server.py"
    server.write_text(
        """
import json
import sys


def read_message():
    headers = {}
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


while True:
    message = read_message()
    method = message.get("method")
    if method == "initialize":
        send({"jsonrpc": "2.0", "id": message["id"], "result": {"server": "fake"}})
    elif method == "tools/list":
        send({"jsonrpc": "2.0", "method": "note", "params": {}})
        send({"jsonrpc": "2.0", "id": message["id"], "result": {"tools": [{"name": "add_source"}]}})
    elif method == "tools/call":
        send({"jsonrpc": "2.0", "id": message["id"], "result": {"ok": True}})
""".strip()
        + "\n"
    )

    with MCPClient([sys.executable, str(server)], {}, request_timeout=2) as client:
        assert client.list_tools()["tools"][0]["name"] == "add_source"
        assert client.call_tool("add_source", {"content": "hello"}) == {"ok": True}


def test_mcp_client_falls_back_to_newline_json(tmp_path) -> None:
    server = tmp_path / "server.py"
    server.write_text(
        """
import json
import sys


for line in sys.stdin:
    try:
        message = json.loads(line)
    except json.JSONDecodeError:
        raise SystemExit(2)
    method = message.get("method")
    if method == "initialize":
        print(json.dumps({"jsonrpc": "2.0", "id": message["id"], "result": {}}), flush=True)
    elif method == "tools/list":
        print(json.dumps({"jsonrpc": "2.0", "id": message["id"], "result": {"tools": []}}), flush=True)
""".strip()
        + "\n"
    )

    with MCPClient([sys.executable, str(server)], {}, request_timeout=2) as client:
        assert client.list_tools() == {"tools": []}
