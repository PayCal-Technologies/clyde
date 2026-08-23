package clyde

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

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
				value := strings.TrimSpace(line[len("content-length:"):])
				n, err := strconv.Atoi(value)
				if err != nil {
					ch <- result{err: errf("invalid MCP Content-Length header: %q", value)}
					return
				}
				length = n
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
		if length > maxMCPFrameBytes {
			ch <- result{err: errf("MCP frame is too large: %d bytes", length)}
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
