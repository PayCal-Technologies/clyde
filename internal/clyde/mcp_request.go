package clyde

import (
	"context"
	"encoding/json"
	"time"
)

func (c *MCPClient) ListTools(ctx context.Context) (map[string]any, error) {
	return c.Request(ctx, "tools/list", map[string]any{})
}

func (c *MCPClient) CallTool(ctx context.Context, name string, arguments map[string]any) (map[string]any, error) {
	return c.Request(ctx, "tools/call", map[string]any{"name": name, "arguments": arguments})
}

func (c *MCPClient) Request(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	c.mu.Lock()
	id := c.nextID
	if id == int(^uint(0)>>1) {
		c.mu.Unlock()
		return nil, errf("MCP request id overflow")
	}
	c.nextID++
	c.mu.Unlock()
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(c.Timeout)
	for {
		if time.Now().After(deadline) {
			return nil, errf("timed out waiting for MCP response after %s. Recent stderr:\n%s", c.Timeout, c.stderr.String())
		}
		readCtx, cancel := context.WithTimeout(ctx, time.Until(deadline))
		msg, err := c.readMessage(readCtx)
		readErr := readCtx.Err()
		cancel()
		if err != nil {
			if readErr != nil {
				_ = c.Close()
				return nil, errf("timed out waiting for MCP response after %s. Recent stderr:\n%s", c.Timeout, c.stderr.String())
			}
			return nil, err
		}
		if numberInt(msg["id"]) != id {
			continue
		}
		if msg["error"] != nil {
			data, _ := json.Marshal(msg["error"])
			return nil, errf("MCP %s failed: %s", method, data)
		}
		result, _ := msg["result"].(map[string]any)
		return result, nil
	}
}

func (c *MCPClient) Notify(method string, params map[string]any) error {
	return c.send(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
