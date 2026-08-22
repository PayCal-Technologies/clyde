package clyde

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
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
