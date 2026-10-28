package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestContentHash(t *testing.T) {
	// SHA-256 of the empty input.
	if got, want := ContentHash(nil), "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	if !ValidContentHash(ContentHash([]byte("x"))) || ValidContentHash("sha256:ABC") || ValidContentHash("e3b0") {
		t.Error("ValidContentHash")
	}
}

func TestOutputPath(t *testing.T) {
	ok := map[string]string{"a.txt": "a.txt", "sub/a.txt": "sub/a.txt", "./a.txt": "a.txt", "sub//a.txt": "sub/a.txt", "sub/./a": "sub/a"}
	for in, want := range ok {
		if got, err := OutputPath(in); err != nil || got != want {
			t.Errorf("%q: got %q %v, want %q", in, got, err, want)
		}
	}
	// A ".." element is refused even when it stays inside: the rule is simple to audit.
	bad := []string{"", ".", "a/b/../c", "..", "../a", "a/../../b", "/etc/passwd", "a\\b", "a\x00b", "a/..", "./..", strings.Repeat("a", MaxOutputPathLen+1), "\xff"}
	for _, in := range bad {
		if _, err := OutputPath(in); !errors.Is(err, ErrOutsideOutput) {
			t.Errorf("%q: got %v, want outside the output location", in, err)
		}
	}
}
