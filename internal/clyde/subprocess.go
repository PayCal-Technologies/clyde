package clyde

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"
)

const (
	maxCommandStderrBytes = 64 * 1024
	maxCommandOutputBytes = 16 * 1024 * 1024
)

func runCommand(ctx context.Context, command, args []string, timeout time.Duration) ([]byte, error) {
	if len(command) == 0 {
		return nil, errf("empty command")
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, command[0], append(command[1:], args...)...)
	var stdout, stderr limitedBuffer
	stdout.Limit = maxCommandOutputBytes
	stderr.Limit = maxCommandStderrBytes
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if runCtx.Err() == context.DeadlineExceeded {
		return nil, errf("timed out running %s after %s", commandSummary(command, args), timeout)
	}
	if stdout.Truncated {
		return nil, errf("command output too large from %s", commandSummary(command, args))
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		if stderr.Truncated {
			detail += " [stderr truncated]"
		}
		return nil, errf("%s", detail)
	}
	return bytes.TrimSpace(stdout.Bytes()), nil
}

type limitedBuffer struct {
	Limit     int
	buf       bytes.Buffer
	Truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Limit <= 0 {
		b.Truncated = true
		return len(p), nil
	}
	remaining := b.Limit - b.buf.Len()
	if remaining <= 0 {
		b.Truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.buf.Write(p[:remaining])
		b.Truncated = true
		return len(p), nil
	}
	_, _ = b.buf.Write(p)
	return len(p), nil
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

func (b *limitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func commandSummary(command, args []string) string {
	parts := append([]string{}, command...)
	for i := 0; i < len(args); i++ {
		parts = append(parts, args[i])
		if args[i] == "--text" && i+1 < len(args) {
			parts = append(parts, "[redacted]")
			i++
		}
	}
	return strings.Join(parts, " ")
}
