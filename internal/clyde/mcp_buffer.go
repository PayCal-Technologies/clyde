package clyde

import (
	"bytes"
	"sync"
)

type lockedLimitedBuffer struct {
	Limit     int
	buf       bytes.Buffer
	Truncated bool
	mu        sync.Mutex
}

func (b *lockedLimitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
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

func (b *lockedLimitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	value := b.buf.String()
	if b.Truncated {
		value += "\n[stderr truncated]"
	}
	return value
}
