package clyde

import (
	"bufio"
	"context"
	"fmt"
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
