package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	rootA = "0123456789abcdef0123456789abcdef01234567"
	ulidA = "01J9ZK3Q4R5S6T7V8W9XABCDEF"
)

func TestNormalizeRemote(t *testing.T) {
	cases := map[string]string{
		"https://GitHub.com/acme/app.git":          "https://github.com/acme/app",
		"https://github.com/acme/app/":             "https://github.com/acme/app",
		"https://user:tok@github.com/acme/app.git": "https://github.com/acme/app",
		"HTTPS://github.com/acme/App":              "https://github.com/acme/App", // path case is kept
		"ssh://git@Host.example/acme/app.git":      "ssh://host.example/acme/app",
		"git@github.com:acme/app.git":              "ssh://github.com/acme/app",
		"  https://github.com/acme/app  ":          "https://github.com/acme/app",
	}
	for in, want := range cases {
		got, err := NormalizeRemote(in)
		if err != nil || got != want {
			t.Errorf("NormalizeRemote(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "acme/app", "https://github.com", "https://github.com/.git", "/srv/git/app"} {
		if _, err := NormalizeRemote(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("NormalizeRemote(%q) = %v; want ErrInvalid", bad, err)
		}
	}
}

func TestProjectKeyIgnoresCloneSpelling(t *testing.T) {
	a, err := NewProjectKey("https://github.com/acme/app.git", rootA)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewProjectKey("https://user:secret@GITHUB.com/acme/app/", rootA)
	if a != b {
		t.Fatalf("keys differ: %q vs %q", a, b)
	}
	if strings.Contains(string(b), "secret") {
		t.Fatalf("key carries a credential: %q", b)
	}
	other, _ := NewProjectKey("https://github.com/acme/app", strings.Repeat("a", 40))
	if other == a {
		t.Fatal("different root commit gave the same key")
	}
}

func TestProjectKeyRejectsBadInput(t *testing.T) {
	if _, err := NewProjectKey("https://github.com/acme/app", "abc"); !errors.Is(err, ErrInvalid) {
		t.Errorf("short commit: got %v", err)
	}
	if _, err := NewProjectKey("https://github.com/acme/app", strings.ToUpper(rootA)); !errors.Is(err, ErrInvalid) {
		t.Errorf("uppercase commit: got %v", err)
	}
	if _, err := NewLocalProjectKey("not-a-ulid"); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad ulid: got %v", err)
	}
	k, err := NewLocalProjectKey(ulidA)
	if err != nil || !k.Valid() {
		t.Errorf("local key: %q %v", k, err)
	}
}

func TestProjectKeyValid(t *testing.T) {
	good, _ := NewProjectKey("https://github.com/acme/app", rootA)
	if !good.Valid() {
		t.Fatalf("derived key %q is not valid", good)
	}
	for _, k := range []ProjectKey{"", "id:", "id:short", "https://github.com/acme/app", "/home/me/app#" + ProjectKey(rootA),
		"https://GitHub.com/acme/app#" + ProjectKey(rootA)} { // not in normalized form
		if k.Valid() {
			t.Errorf("key %q reported valid", k)
		}
	}
}

func TestProjectValidate(t *testing.T) {
	k, _ := NewProjectKey("https://github.com/acme/app", rootA)
	ok := Project{Key: k, Name: "App", CreatedAt: time.Now()}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]Project{
		"no key":       {Name: "App", CreatedAt: ok.CreatedAt},
		"empty name":   {Key: k, CreatedAt: ok.CreatedAt},
		"padded name":  {Key: k, Name: " App", CreatedAt: ok.CreatedAt},
		"long name":    {Key: k, Name: strings.Repeat("x", MaxProjectName+1), CreatedAt: ok.CreatedAt},
		"no timestamp": {Key: k, Name: "App"},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: got %v, want ErrInvalid", name, err)
		}
	}
}
