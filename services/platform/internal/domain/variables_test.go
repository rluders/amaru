package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestSubstitute(t *testing.T) {
	vars := map[string]string{"a": "1", "b": "{{a}}", "empty": ""}
	for _, tc := range []struct{ in, want string }{
		{"plain", "plain"},
		{"x {{a}} y", "x 1 y"},
		{"{{a}}{{a}}", "11"},
		{"[{{empty}}]", "[]"},
		{"{{b}}", "{{a}}"}, // a value is never expanded again
		{"héllo {{a}} ✓", "héllo 1 ✓"},
	} {
		got, err := Substitute(tc.in, vars)
		if err != nil || got != tc.want {
			t.Errorf("Substitute(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestSubstituteRefuses(t *testing.T) {
	vars := map[string]string{"a": strings.Repeat("x", MaxVarValue)}
	for name, in := range map[string]string{
		"unknown":     "{{nope}}",
		"unclosed":    "x {{a",
		"space":       "{{ a }}",
		"over limit":  "{{a}}{{a}}{{a}}",
		"huge expand": strings.Repeat("{{a}}", 1000),
	} {
		if _, err := Substitute(in, vars); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	if _, err := Substitute("{{a}}{{a}}", vars); err != nil {
		t.Errorf("exactly at the limit must pass: %v", err)
	}
}

func TestResolve(t *testing.T) {
	w := Workflow{ID: "wf", Variables: []WorkflowVariable{{Name: "intent", Value: "default"}, {Name: "n", Value: "1"}}}
	got, err := w.Resolve(map[string]string{"intent": "mine"})
	if err != nil || got["intent"] != "mine" || got["n"] != "1" {
		t.Fatalf("Resolve = %v, %v", got, err)
	}
	if _, err := w.Resolve(map[string]string{"intnet": "x"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("undeclared override: %v", err)
	}
	if _, err := w.Resolve(map[string]string{"intent": strings.Repeat("é", MaxVarValue+1)}); !errors.Is(err, ErrInvalid) {
		t.Errorf("long override: %v", err)
	}
	whole := strings.Repeat("é", MaxIntent)
	if got, err := w.Resolve(map[string]string{"intent": whole}); err != nil || got["intent"] != whole {
		t.Errorf("a whole Work Item intent must pass unchanged: %v", err)
	}
}
