package domain

import (
	"fmt"
	"regexp"
)

// RedactionMarker replaces every secret the Redactor finds.
const RedactionMarker = "[REDACTED]"

// Limits on configured redaction patterns, so an Organization setting cannot make every
// stored string expensive to scan.
const (
	MaxRedactionPatterns   = 50
	MaxRedactionPatternLen = 512
)

type redactRule struct {
	re   *regexp.Regexp
	repl string
}

// builtinRedactions are the known secret formats of FEAT-ART-003. They are deliberately
// conservative about shape (a fixed prefix or a labelled value) so ordinary prose is left alone,
// and they are NOT a complete list: a secret in a format nobody listed is stored as it came
// (DATA_ARCHITECTURE §5 names redaction completeness as the risk). Order matters: a long block
// goes before the short patterns that could match inside it.
var builtinRedactions = []redactRule{
	// A private key block, closed or cut off before its end line.
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), RedactionMarker},
	{regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*`), RedactionMarker},
	// Credentials in a URL: scheme://user:password@host keeps the scheme and host.
	{regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)[^\s/:@]+:[^\s/@]+@`), "${1}" + RedactionMarker + "@"},
	// A labelled value: password=..., "api_key": "...", token: ... (value of 8 or more characters).
	{regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_-]?key|access[_-]?key|private[_-]?key)(["']?\s*[=:]\s*)("[^"]{8,}"|'[^']{8,}'|[^\s"',;]{8,})`), "${1}${2}" + RedactionMarker},
	{regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`), "Bearer " + RedactionMarker},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,255}\b`), RedactionMarker},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{22,255}\b`), RedactionMarker},
	{regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`), RedactionMarker},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`), RedactionMarker},
	{regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`), RedactionMarker},
	{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`), RedactionMarker},
}

// Redactor removes known secret patterns from text before the Platform stores it. It is pure and
// safe for concurrent use. The zero value redacts nothing; use NewRedactor.
type Redactor struct {
	rules []redactRule
}

// NewRedactor returns a Redactor with the built-in patterns and the Organization's extra
// patterns (Go RE2 syntax, so a pattern cannot take exponential time). A pattern that is empty,
// too long, does not compile or matches the empty string is refused with ErrInvalid.
func NewRedactor(extra ...string) (Redactor, error) {
	if len(extra) > MaxRedactionPatterns {
		return Redactor{}, invalid("redaction: at most %d extra patterns", MaxRedactionPatterns)
	}
	rules := append([]redactRule(nil), builtinRedactions...)
	for _, p := range extra {
		if p == "" || len(p) > MaxRedactionPatternLen {
			return Redactor{}, invalid("redaction: a pattern is 1 to %d bytes", MaxRedactionPatternLen)
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return Redactor{}, invalid("redaction: pattern %q: %v", p, err)
		}
		if re.MatchString("") {
			return Redactor{}, invalid("redaction: pattern %q matches the empty string", p)
		}
		rules = append(rules, redactRule{re, RedactionMarker})
	}
	return Redactor{rules: rules}, nil
}

// DefaultRedactor is a Redactor with the built-in patterns only.
func DefaultRedactor() Redactor {
	r, err := NewRedactor()
	if err != nil {
		panic(fmt.Sprintf("domain: built-in redaction patterns: %v", err))
	}
	return r
}

// Redact returns s with every match replaced and the number of replacements made. The count
// lets the caller surface that something was removed.
func (r Redactor) Redact(s string) (string, int) {
	n := 0
	for _, rule := range r.rules {
		s = rule.re.ReplaceAllStringFunc(s, func(m string) string {
			out := rule.re.ReplaceAllString(m, rule.repl)
			if out != m { // a value already redacted is not counted again
				n++
			}
			return out
		})
	}
	return s, n
}
