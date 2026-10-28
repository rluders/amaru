package domain

import (
	"strings"
	"unicode/utf8"
)

// MaxResolvedValue bounds a Step input or prompt after its {{variable}} placeholders are replaced
// (FEAT-WFL-003). It is two full variables, so a Work Item intent (MaxIntent) travels whole into a
// value that also carries text around it, and the expansion of many placeholders stays bounded.
const MaxResolvedValue = 2 * MaxVarValue

// Resolve returns the values a run of the Workflow uses: each declared variable's default,
// replaced by the override of the same name. An override for a name the Workflow does not declare
// is refused, so a misspelled name cannot silently do nothing, and a value over MaxVarValue is
// refused rather than cut (F-15: a Work Item intent reached the agent truncated).
func (w Workflow) Resolve(overrides map[string]string) (map[string]string, error) {
	vars := make(map[string]string, len(w.Variables))
	for _, v := range w.Variables {
		vars[v.Name] = v.Value
	}
	for name, value := range overrides {
		if _, declared := vars[name]; !declared {
			return nil, invalid("variable %q is not declared by workflow %s", name, w.ID)
		}
		if utf8.RuneCountInString(value) > MaxVarValue {
			return nil, invalid("variable %s is at most %d characters", name, MaxVarValue)
		}
		vars[name] = value
	}
	return vars, nil
}

// Substitute replaces every {{name}} in text with vars[name] in one pass: a value that itself
// contains {{x}} is inserted as is and never expanded again, so a value cannot pull in another
// variable. A placeholder that names no variable, or is not closed, is an error; there is no
// escape for a literal "{{" yet. The result is at most MaxResolvedValue characters.
func Substitute(text string, vars map[string]string) (string, error) {
	if !strings.Contains(text, "{{") {
		return text, nil
	}
	var b strings.Builder
	rest := text
	for {
		open := strings.Index(rest, "{{")
		if open < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:open])
		rest = rest[open+2:]
		end := strings.Index(rest, "}}")
		if end < 0 {
			return "", invalid("a {{ placeholder is not closed")
		}
		name := rest[:end]
		value, ok := vars[name]
		if !ok {
			return "", invalid("placeholder {{%s}} names no variable", name)
		}
		b.WriteString(value)
		rest = rest[end+2:]
		if b.Len() > 4*MaxResolvedValue {
			// Bytes, not characters: stop early before building something huge.
			return "", invalid("value is over %d characters after substitution", MaxResolvedValue)
		}
	}
	out := b.String()
	if utf8.RuneCountInString(out) > MaxResolvedValue {
		return "", invalid("value is over %d characters after substitution", MaxResolvedValue)
	}
	return out, nil
}
