package clyde

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCommandSummaryRedactsTextPayload(t *testing.T) {
	got := commandSummary([]string{"nlm"}, []string{"source", "add", "nb", "--text", "secret source text", "--title", "app.go"})

	if strings.Contains(got, "secret source text") {
		t.Fatalf("summary leaked text payload: %s", got)
	}
	if !strings.Contains(got, "--text [redacted]") {
		t.Fatalf("summary did not mark redaction: %s", got)
	}
}

func TestRunCommandRedactsTextPayloadOnTimeout(t *testing.T) {
	_, err := runCommand(context.Background(), []string{"sh", "-c", "sleep 1"}, []string{"--text", "secret source text"}, time.Nanosecond)

	if err == nil {
		t.Fatalf("expected timeout")
	}
	if strings.Contains(err.Error(), "secret source text") {
		t.Fatalf("timeout error leaked text payload: %v", err)
	}
}

func TestLimitedBufferTruncates(t *testing.T) {
	var buf limitedBuffer
	buf.Limit = 4
	n, err := buf.Write([]byte("abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 6 || !buf.Truncated || buf.String() != "abcd" {
		t.Fatalf("unexpected buffer state n=%d truncated=%v value=%q", n, buf.Truncated, buf.String())
	}
}
