package clyde

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMCPReadMessageAcceptsLowercaseContentLength(t *testing.T) {
	body := `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`
	client := &MCPClient{
		reader:  bufio.NewReader(strings.NewReader(fmt.Sprintf("content-length: %d\r\n\r\n%s", len(body), body))),
		framing: "content-length",
	}

	msg, err := client.readMessage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if numberInt(msg["id"]) != 1 {
		t.Fatalf("unexpected message: %#v", msg)
	}
}

func TestMCPReadMessageRejectsOversizedFrame(t *testing.T) {
	client := &MCPClient{
		reader:  bufio.NewReader(strings.NewReader("Content-Length: 16777217\r\n\r\n{}")),
		framing: "content-length",
	}

	_, err := client.readMessage(context.Background())

	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMCPReadMessageRejectsOversizedHeaderLine(t *testing.T) {
	client := &MCPClient{
		reader:  bufio.NewReader(strings.NewReader(strings.Repeat("x", maxMCPHeaderLineBytes+1) + "\n\r\n{}")),
		framing: "content-length",
	}

	_, err := client.readMessage(context.Background())

	if err == nil || !strings.Contains(err.Error(), "line is too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMCPReadMessageRejectsOversizedNewlineFrame(t *testing.T) {
	client := &MCPClient{
		reader:  bufio.NewReader(strings.NewReader(strings.Repeat("x", maxMCPFrameBytes+1) + "\n")),
		framing: "newline",
	}

	_, err := client.readMessage(context.Background())

	if err == nil || !strings.Contains(err.Error(), "line is too large") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMCPBaseEnvDoesNotForwardCredentials(t *testing.T) {
	t.Setenv("PATH", os.Getenv("PATH"))
	t.Setenv("GITHUB_TOKEN", "secret")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")

	env := strings.Join(mcpBaseEnv(), "\n")

	if strings.Contains(env, "GITHUB_TOKEN") || strings.Contains(env, "AWS_SECRET_ACCESS_KEY") {
		t.Fatalf("credential env leaked: %s", env)
	}
	if !strings.Contains(env, "PATH=") {
		t.Fatalf("expected PATH in env: %s", env)
	}
}

func TestMCPRequestsAreSingleFlight(t *testing.T) {
	body1 := `{"jsonrpc":"2.0","id":1,"result":{"ok":"one"}}`
	body2 := `{"jsonrpc":"2.0","id":2,"result":{"ok":"two"}}`
	client := &MCPClient{
		stdin:   &lockedWriteCloser{},
		reader:  bufio.NewReader(strings.NewReader(body1 + "\n" + body2 + "\n")),
		framing: "newline",
		nextID:  1,
		Timeout: time.Second,
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Request(context.Background(), "test", map[string]any{})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLockedLimitedBufferBoundsStderr(t *testing.T) {
	var buf lockedLimitedBuffer
	buf.Limit = 4
	if _, err := buf.Write([]byte("abcdef")); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "abcd") || !strings.Contains(got, "truncated") || strings.Contains(got, "ef") {
		t.Fatalf("unexpected buffer: %q", got)
	}
}

type lockedWriteCloser struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *lockedWriteCloser) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *lockedWriteCloser) Close() error {
	return nil
}

var _ io.WriteCloser = (*lockedWriteCloser)(nil)
