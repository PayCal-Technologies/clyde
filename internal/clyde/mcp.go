package clyde

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MCPClient struct {
	Command []string
	Env     map[string]string
	Timeout time.Duration

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	reader  *bufio.Reader
	nextID  int
	framing string
	stderr  bytes.Buffer
	mu      sync.Mutex
}

func NewMCPClient(command []string, env map[string]string, timeout time.Duration) *MCPClient {
	return &MCPClient{Command: command, Env: env, Timeout: timeout, nextID: 1, framing: "content-length"}
}

func (c *MCPClient) Start(ctx context.Context) error {
	if err := c.start(ctx, "content-length"); err != nil {
		return err
	}
	if err := c.initialize(ctx); err != nil {
		_ = c.Close()
		if err2 := c.start(ctx, "newline"); err2 != nil {
			return err
		}
		if err3 := c.initialize(ctx); err3 != nil {
			_ = c.Close()
			return err3
		}
	}
	return nil
}

func (c *MCPClient) start(ctx context.Context, framing string) error {
	if len(c.Command) == 0 {
		return errf("empty MCP command")
	}
	cmd := exec.CommandContext(ctx, c.Command[0], c.Command[1:]...)
	cmd.Env = os.Environ()
	for key, value := range c.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return errf("failed to start MCP command %q: %w", strings.Join(c.Command, " "), err)
	}
	c.cmd = cmd
	c.stdin = stdin
	c.reader = bufio.NewReader(stdout)
	c.framing = framing
	go func() { _, _ = io.Copy(&c.stderr, stderr) }()
	return nil
}

func (c *MCPClient) initialize(ctx context.Context) error {
	_, err := c.Request(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "clyde", "version": productVersion},
	})
	if err != nil {
		return err
	}
	return c.Notify("notifications/initialized", map[string]any{})
}

func (c *MCPClient) Close() error {
	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}
	_ = c.cmd.Process.Kill()
	_, _ = c.cmd.Process.Wait()
	return nil
}

func (c *MCPClient) ListTools(ctx context.Context) (map[string]any, error) {
	return c.Request(ctx, "tools/list", map[string]any{})
}

func (c *MCPClient) CallTool(ctx context.Context, name string, arguments map[string]any) (map[string]any, error) {
	return c.Request(ctx, "tools/call", map[string]any{"name": name, "arguments": arguments})
}

func (c *MCPClient) Request(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	c.mu.Lock()
	id := c.nextID
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
		msg, err := c.readMessage(ctx)
		if err != nil {
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

func (c *MCPClient) send(msg map[string]any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	if c.framing == "newline" {
		_, err = c.stdin.Write(append(data, '\n'))
		return err
	}
	_, err = c.stdin.Write(append([]byte("Content-Length: "+strconv.Itoa(len(data))+"\r\n\r\n"), data...))
	return err
}

func (c *MCPClient) readMessage(ctx context.Context) (map[string]any, error) {
	type result struct {
		msg map[string]any
		err error
	}
	ch := make(chan result, 1)
	go func() {
		if c.framing == "newline" {
			line, err := c.reader.ReadBytes('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			var msg map[string]any
			ch <- result{msg: msg, err: json.Unmarshal(bytes.TrimSpace(line), &msg)}
			return
		}
		header, err := c.reader.ReadString('\n')
		if err != nil {
			ch <- result{err: err}
			return
		}
		for strings.TrimSpace(header) == "" {
			header, err = c.reader.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
		}
		length := 0
		for {
			line := strings.TrimSpace(header)
			if strings.HasPrefix(strings.ToLower(line), "content-length:") {
				length, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")))
			}
			next, err := c.reader.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			if strings.TrimSpace(next) == "" {
				break
			}
			header = next
		}
		if length <= 0 {
			ch <- result{err: errf("MCP frame is missing Content-Length header")}
			return
		}
		body := make([]byte, length)
		if _, err := io.ReadFull(c.reader, body); err != nil {
			ch <- result{err: err}
			return
		}
		var msg map[string]any
		ch <- result{msg: msg, err: json.Unmarshal(body, &msg)}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.msg, res.err
	}
}
