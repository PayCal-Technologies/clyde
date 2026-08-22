package clyde

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzSplitTextPreservesUTF8(f *testing.F) {
	f.Add("alpha\nbeta\n", 5)
	f.Add("ééééé", 3)
	f.Fuzz(func(t *testing.T, text string, limit int) {
		if limit < 1 || limit > 4096 {
			limit = 128
		}
		chunks := splitText(text, limit)
		if strings.Join(chunks, "") != text {
			t.Fatalf("text changed after split")
		}
		if utf8.ValidString(text) {
			for _, chunk := range chunks {
				if !utf8.ValidString(chunk) {
					t.Fatalf("invalid UTF-8 chunk")
				}
			}
		}
	})
}

func FuzzJSONRPCEnvelopeDecode(f *testing.F) {
	f.Add(`{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`)
	f.Add(`{"jsonrpc":"2.0","id":1,"error":{"message":"no"}}`)
	f.Fuzz(func(t *testing.T, data string) {
		var decoded struct {
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal([]byte(data), &decoded)
	})
}

func FuzzGlobMatching(f *testing.F) {
	f.Add("internal/clyde/cli.go", "*.go")
	f.Add("node_modules/a.js", "node_modules/**")
	f.Fuzz(func(t *testing.T, rel, pattern string) {
		_ = matchesAny(filepathSlash(rel), []string{filepathSlash(pattern)})
	})
}

func FuzzSecretDetection(f *testing.F) {
	f.Add("API_KEY='abcdefghijklmnopqrstuvwxyz123456'")
	f.Add("package main\n")
	f.Fuzz(func(t *testing.T, text string) {
		_ = looksSecret(text)
	})
}

func filepathSlash(value string) string {
	return strings.ReplaceAll(value, "\\", "/")
}
