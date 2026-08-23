package clyde

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
)

func (c *MCPClient) start(ctx context.Context, framing string) error {
	if len(c.Command) == 0 {
		return errf("empty MCP command")
	}
	cmd := exec.CommandContext(ctx, c.Command[0], c.Command[1:]...)
	cmd.Env = mcpBaseEnv()
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
	c.stderr = lockedLimitedBuffer{Limit: maxCommandStderrBytes}
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

func mcpBaseEnv() []string {
	allowed := []string{
		"PATH", "HOME", "USERPROFILE", "HOMEDRIVE", "HOMEPATH",
		"TMPDIR", "TEMP", "TMP",
		"LANG", "LC_ALL", "LC_CTYPE",
		"SystemRoot", "WINDIR", "COMSPEC", "PATHEXT",
	}
	env := make([]string, 0, len(allowed))
	for _, key := range allowed {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}
