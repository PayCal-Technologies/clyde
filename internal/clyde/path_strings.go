package clyde

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func validateRepoRelPath(rel string) error {
	if rel == "" {
		return errf("path must not be empty")
	}
	if !utf8.ValidString(rel) {
		return errf("path must be valid UTF-8")
	}
	if strings.ContainsRune(rel, 0) {
		return errf("path must not contain NUL bytes")
	}
	if strings.Contains(rel, "\\") {
		return errf("path must use slash separators")
	}
	if strings.HasPrefix(rel, "/") || filepath.IsAbs(rel) || hasWindowsVolumePrefix(rel) {
		return errf("path must be relative")
	}
	if rel == "." {
		return errf("path must not be .")
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return errf("path contains unsafe component %q", part)
		}
	}
	if reason := suspiciousPathStringReason(rel); reason != "" {
		return errf("path contains %s", reason)
	}
	return nil
}

func suspiciousPathStringReason(value string) string {
	for _, r := range value {
		if isPathControlRune(r) {
			return "control character"
		}
		if isBidiControlRune(r) {
			return "bidirectional control character"
		}
	}
	return ""
}

func isPathControlRune(r rune) bool {
	return r < 0x20 || (r >= 0x7f && r <= 0x9f)
}

func isBidiControlRune(r rune) bool {
	switch r {
	case '\u061c', '\u200e', '\u200f',
		'\u202a', '\u202b', '\u202c', '\u202d', '\u202e',
		'\u2066', '\u2067', '\u2068', '\u2069':
		return true
	default:
		return false
	}
}

func hasWindowsVolumePrefix(value string) bool {
	if len(value) < 2 || value[1] != ':' {
		return false
	}
	c := value[0]
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
