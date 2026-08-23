package clyde

import (
	"bufio"
	"context"
	"io"
	"os/exec"
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
	stderr  lockedLimitedBuffer
	mu      sync.Mutex
}

const maxMCPFrameBytes = 16 * 1024 * 1024

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
