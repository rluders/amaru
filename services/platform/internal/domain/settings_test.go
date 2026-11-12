package domain

import (
	"errors"
	"testing"
)

func TestParseUpdateSettings(t *testing.T) {
	for _, raw := range []string{`{}`, `null`, `[]`, `{"importClaudeSkills":null}`, `{"importClaudeSkills":"true"}`, `{"importClaudeSkills":1}`, `{"ImportClaudeSkills":true}`, `{"importClaudeSkills":true,"project":"other"}`, `{"unknown":true}`, `{"importClaudeSkills":true} {}`} {
		if _, err := ParseUpdateSettings([]byte(raw)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", raw, err)
		}
	}
	for raw, want := range map[string]bool{`{"importClaudeSkills":true}`: true, `{"importClaudeSkills":false}`: false} {
		p, err := ParseUpdateSettings([]byte(raw))
		if err != nil || p.Validate() != nil || *p.ImportClaudeSkills != want {
			t.Fatalf("%s: %v", raw, err)
		}
	}
}
