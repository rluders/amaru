package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestRedactBuiltins(t *testing.T) {
	r := DefaultRedactor()
	// Fixtures are built from parts so no literal in the repository looks like a real credential.
	gh := "ghp_" + strings.Repeat("a1", 20)
	pat := "github_pat_" + strings.Repeat("Ab1", 10)
	aws := "AKIA" + strings.Repeat("Q7", 8)
	sk := "sk-" + strings.Repeat("x9", 15)
	slack := "xoxb-" + strings.Repeat("1", 12)
	jwt := "eyJ" + strings.Repeat("a", 10) + "." + strings.Repeat("b", 10) + "." + strings.Repeat("c", 10)
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIabc\ndef\n-----END RSA PRIVATE KEY-----"
	for name, tc := range map[string]struct{ in, secret string }{
		"github token":     {"cloning with " + gh + " failed", gh},
		"github fine pat":  {"auth " + pat, pat},
		"aws key id":       {"key " + aws + " denied", aws},
		"sk key":           {"ANTHROPIC " + sk, sk},
		"slack token":      {"hook " + slack, slack},
		"jwt":              {"got " + jwt + " back", jwt},
		"private key":      {"before\n" + pem + "\nafter", "MIIabc"},
		"cut private key":  {"x -----BEGIN PRIVATE KEY-----\nMIIabc\nstill going", "MIIabc"},
		"bearer":           {"Authorization: Bearer abcdefghijklmnop1234", "abcdefghijklmnop1234"},
		"url credentials":  {"git clone https://bob:hunter22hunter@example.com/r.git", "hunter22hunter"},
		"labelled value":   {`password = "correct horse battery"`, "correct horse battery"},
		"labelled json":    {`{"api_key": "abcdef123456"}`, "abcdef123456"},
		"labelled unquote": {"TOKEN=abcdefgh1234 next", "abcdefgh1234"},
	} {
		t.Run(name, func(t *testing.T) {
			out, n := r.Redact(tc.in)
			if strings.Contains(out, tc.secret) {
				t.Errorf("secret survived: %q", out)
			}
			if n == 0 || !strings.Contains(out, RedactionMarker) {
				t.Errorf("n = %d, out = %q", n, out)
			}
		})
	}
}

func TestRedactKeepsOrdinaryText(t *testing.T) {
	r := DefaultRedactor()
	for _, s := range []string{
		"exit status 1",
		"the token expired", // a short value after a label is not a secret
		"password: short",
		"see https://example.com/a/b?c=d for details",
		"sk-short",
		"",
	} {
		if out, n := r.Redact(s); out != s || n != 0 {
			t.Errorf("Redact(%q) = %q, %d; want unchanged", s, out, n)
		}
	}
}

func TestRedactKeepsContext(t *testing.T) {
	out, _ := DefaultRedactor().Redact("git clone https://bob:hunter22hunter@example.com/r.git failed")
	if want := "git clone https://" + RedactionMarker + "@example.com/r.git failed"; out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestRedactIsIdempotent(t *testing.T) {
	r := DefaultRedactor()
	once, n1 := r.Redact("token=abcdefgh1234 and password: abcdefgh5678")
	twice, n2 := r.Redact(once)
	if once != twice || n1 != 2 || n2 != 0 {
		t.Errorf("once=%q (%d) twice=%q (%d)", once, n1, twice, n2)
	}
}

func TestRedactCountsEveryMatch(t *testing.T) {
	gh := "ghp_" + strings.Repeat("z", 36)
	if _, n := DefaultRedactor().Redact(gh + " " + gh); n != 2 {
		t.Errorf("n = %d, want 2", n)
	}
}

func TestRedactExtraPatterns(t *testing.T) {
	r, err := NewRedactor(`ACME-[0-9]{6}`)
	if err != nil {
		t.Fatal(err)
	}
	if out, n := r.Redact("id ACME-123456 ok"); out != "id "+RedactionMarker+" ok" || n != 1 {
		t.Errorf("got %q, %d", out, n)
	}
	// the built-ins still apply
	if _, n := r.Redact("AKIA" + strings.Repeat("Q7", 8)); n != 1 {
		t.Errorf("built-in lost: n = %d", n)
	}
}

func TestNewRedactorRefusesBadPatterns(t *testing.T) {
	many := make([]string, MaxRedactionPatterns+1)
	for i := range many {
		many[i] = "x"
	}
	for name, ps := range map[string][]string{
		"empty":       {""},
		"empty match": {"a*"},
		"bad syntax":  {"("},
		"too long":    {strings.Repeat("a", MaxRedactionPatternLen+1)},
		"too many":    many,
	} {
		if _, err := NewRedactor(ps...); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestZeroRedactorRedactsNothing(t *testing.T) {
	if out, n := (Redactor{}).Redact("AKIA" + strings.Repeat("Q7", 8)); n != 0 || out == "" {
		t.Errorf("zero value changed text: %q, %d", out, n)
	}
}
