package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestValidBranch(t *testing.T) {
	good := []string{"main", "amaru/wi-001-x", "feat/19-a_b.c", "release-1.2", "a", "x/y/z"}
	for _, b := range good {
		if !ValidBranch(b) {
			t.Errorf("ValidBranch(%q) = false, want true", b)
		}
	}
	bad := []string{
		"", "@", "-x", "--force", "/x", "x/", "x.", "x.lock", "a/b.lock", "a..b", "a//b", "a@{b",
		".hidden", "a/.hidden", "a b", "a\tb", "a\x00b", "a~b", "a^b", "a:b", "a?b", "a*b", "a[b", `a\b`,
		"café", "x\x7f", strings.Repeat("a", MaxBranchLen+1),
	}
	for _, b := range bad {
		if ValidBranch(b) {
			t.Errorf("ValidBranch(%q) = true, want false", b)
		}
	}
	if !ValidBranch(strings.Repeat("a", MaxBranchLen)) {
		t.Error("a branch of the maximum length is valid")
	}
}

func TestValidWorktreeKey(t *testing.T) {
	for _, k := range []string{"WI-001", "a", "ENG_12", strings.Repeat("a", 64)} {
		if !ValidWorktreeKey(k) {
			t.Errorf("ValidWorktreeKey(%q) = false", k)
		}
	}
	for _, k := range []string{"", "-a", "_a", ".", "..", "../x", "a/b", `a\b`, "a b", "a.b", strings.Repeat("a", 65)} {
		if ValidWorktreeKey(k) {
			t.Errorf("ValidWorktreeKey(%q) = true", k)
		}
	}
}

func TestWorkspaceValidate(t *testing.T) {
	ok := Workspace{Key: "WI-1", Branch: "amaru/wi-1", Base: "main"}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, w := range []Workspace{
		{Key: "../x", Branch: "b", Base: "main"},
		{Key: "WI-1", Branch: "-b", Base: "main"},
		{Key: "WI-1", Branch: "b", Base: "a..b"},
	} {
		if err := w.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("Validate(%+v) = %v, want ErrInvalid", w, err)
		}
	}
}
