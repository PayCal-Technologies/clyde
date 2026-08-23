package clyde

import (
	"strings"
	"testing"
)

func TestValidateRepoRelPathAcceptsPlainUnicode(t *testing.T) {
	if err := validateRepoRelPath("docs/cafe-é.txt"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateRepoRelPathRejectsUnsafeForms(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "empty", path: "", want: "empty"},
		{name: "absolute slash", path: "/tmp/file", want: "relative"},
		{name: "windows volume", path: "C:/tmp/file", want: "relative"},
		{name: "backslash", path: `dir\file`, want: "slash separators"},
		{name: "dotdot", path: "dir/../file", want: "unsafe component"},
		{name: "empty component", path: "dir//file", want: "unsafe component"},
		{name: "control", path: "dir/\x1bfile", want: "control character"},
		{name: "bidi", path: "dir/\u202efile", want: "bidirectional control character"},
		{name: "nul", path: "dir/\x00file", want: "NUL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepoRelPath(tt.path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
		})
	}
}
