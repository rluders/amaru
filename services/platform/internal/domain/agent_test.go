package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func okAgent() Agent {
	return Agent{
		ID: "godot-developer", Name: "Godot developer", Role: "Developer", Instructions: "You are the Godot developer.",
		RequestedCapabilities: []Capability{"code.test"}, ProviderPreference: []string{"claude", "codex"},
		Model: "opus", Status: AgentActive,
	}
}

func TestAgentValidate(t *testing.T) {
	if err := okAgent().Validate(); err != nil {
		t.Fatalf("valid agent rejected: %v", err)
	}
	bad := map[string]func(*Agent){
		"empty id":               func(a *Agent) { a.ID = "" },
		"uppercase id":           func(a *Agent) { a.ID = "Godot Dev" },
		"short id":               func(a *Agent) { a.ID = "a" },
		"blank name":             func(a *Agent) { a.Name = "  " },
		"long name":              func(a *Agent) { a.Name = strings.Repeat("a", MaxTitle+1) },
		"long role":              func(a *Agent) { a.Role = strings.Repeat("a", MaxAgentRole+1) },
		"no instructions":        func(a *Agent) { a.Instructions = "" },
		"long instructions":      func(a *Agent) { a.Instructions = strings.Repeat("a", MaxAgentInstructions+1) },
		"bad status":             func(a *Agent) { a.Status = "archived" },
		"flag-like model":        func(a *Agent) { a.Model = "--evil" },
		"long model":             func(a *Agent) { a.Model = strings.Repeat("a", MaxAgentModel+1) },
		"empty per-model":        func(a *Agent) { a.Models = map[string]string{"claude": ""} },
		"bad per-model id":       func(a *Agent) { a.Models = map[string]string{"Claude": "opus"} },
		"bad capability":         func(a *Agent) { a.RequestedCapabilities = []Capability{"Nope"} },
		"duplicate capability":   func(a *Agent) { a.RequestedCapabilities = []Capability{"code.test", "code.test"} },
		"duplicate preference":   func(a *Agent) { a.ProviderPreference = []string{"claude", "claude"} },
		"bad preference":         func(a *Agent) { a.ProviderPreference = []string{"Claude"} },
		"commands without write": func(a *Agent) { a.WorktreeCommands = []string{"go test"} },
		"denied command":         func(a *Agent) { a.WorktreeWrite, a.WorktreeCommands = true, []string{"git push"} },
		"shell syntax":           func(a *Agent) { a.WorktreeWrite, a.WorktreeCommands = true, []string{"go test; rm -rf"} },
	}
	for name, mutate := range bad {
		a := okAgent()
		mutate(&a)
		if err := a.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	// Counts above the contract limits.
	a := okAgent()
	a.ProviderPreference = make([]string, MaxProviderPreference+1)
	for i := range a.ProviderPreference {
		a.ProviderPreference[i] = "p" + string(rune('a'+i))
	}
	if err := a.Validate(); !errors.Is(err, ErrInvalid) {
		t.Errorf("too many preferences: %v", err)
	}
	a = okAgent()
	a.WorktreeWrite, a.WorktreeCommands = true, []string{"go test", "go vet", "make check"}
	if err := a.Validate(); err != nil {
		t.Errorf("allowed commands rejected: %v", err)
	}
}

func TestSlugFromNameAndModelFor(t *testing.T) {
	for in, want := range map[string]string{"Godot Developer": "godot-developer", "  QA!! ": "qa", "***": ""} {
		if got := SlugFromName(in); got != want {
			t.Errorf("SlugFromName(%q) = %q, want %q", in, got, want)
		}
	}
	if got := SlugFromName(strings.Repeat("a", 100)); len(got) != 64 || !ValidAgentID(got) {
		t.Errorf("long slug: %q", got)
	}
	a := okAgent()
	a.Models = map[string]string{"codex": "gpt-5"}
	if a.ModelFor("codex") != "gpt-5" || a.ModelFor("claude") != "opus" {
		t.Errorf("ModelFor: codex=%q claude=%q", a.ModelFor("codex"), a.ModelFor("claude"))
	}
}

func TestEditsAreDetectedAndVersioned(t *testing.T) {
	h, err := NewAgentHistory(okAgent(), t0)
	if err != nil {
		t.Fatal(err)
	}
	if v := h.Head(); v.Revision != 1 || v.Hash == "" || !v.Definition.CreatedAt.Equal(t0) {
		t.Fatalf("first version: %+v", v)
	}
	t1 := t0.Add(time.Hour)
	name := "Godot lead"
	v, changed, err := h.Update(1, AgentPatch{Name: &name}, t1)
	if err != nil || !changed || v.Revision != 2 {
		t.Fatalf("edit: %+v changed=%v err=%v", v, changed, err)
	}
	if v.Hash == h.Versions()[0].Hash {
		t.Error("an edit must change the content hash")
	}
	if !v.Definition.CreatedAt.Equal(t0) || !v.Definition.UpdatedAt.Equal(t1) {
		t.Errorf("timestamps: created %v updated %v", v.Definition.CreatedAt, v.Definition.UpdatedAt)
	}
	// Saving the same content records nothing and does not move UpdatedAt of the head.
	again, changed, err := h.Update(2, AgentPatch{Name: &name}, t1.Add(time.Hour))
	if err != nil || changed || again.Revision != 2 || !again.Definition.UpdatedAt.Equal(t1) {
		t.Errorf("no-op edit: %+v changed=%v err=%v", again, changed, err)
	}
	if n := len(h.Versions()); n != 2 {
		t.Errorf("no-op edit recorded a revision: %d versions", n)
	}
	// Reordering a list is an edit: provider preference is ordered.
	order := []string{"codex", "claude"}
	if _, changed, err = h.Update(2, AgentPatch{ProviderPreference: &order}, t1); err != nil || !changed {
		t.Errorf("reordering preference: changed=%v err=%v", changed, err)
	}
	// Whole-definition edits (a changed file) go through the same path.
	next := okAgent()
	next.Instructions = "New instructions."
	if v, changed, err = h.Replace(3, next, t1); err != nil || !changed || v.Revision != 4 {
		t.Errorf("replace: %+v changed=%v err=%v", v, changed, err)
	}
	other := okAgent()
	other.ID = "someone-else"
	if _, _, err = h.Replace(4, other, t1); !errors.Is(err, ErrInvalid) {
		t.Errorf("id change: want ErrInvalid, got %v", err)
	}
	if _, _, err = h.Update(4, AgentPatch{}, t1); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty patch: want ErrInvalid, got %v", err)
	}
	bad := ""
	if _, _, err = h.Update(4, AgentPatch{Name: &bad}, t1); !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid edit: want ErrInvalid, got %v", err)
	}
	if h.Head().Revision != 4 {
		t.Errorf("rejected edits moved the head to %d", h.Head().Revision)
	}
}

// Two editors read revision 1; the second to save must not overwrite the first (F-7).
func TestLostUpdateIsRejected(t *testing.T) {
	h, _ := NewAgentHistory(okAgent(), t0)
	a, b := "first", "second"
	if _, _, err := h.Update(1, AgentPatch{Role: &a}, t0); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.Update(1, AgentPatch{Role: &b}, t0); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale edit: want ErrConflict, got %v", err)
	}
	if got := h.Head().Definition.Role; got != "first" {
		t.Errorf("role = %q, want the first editor's value", got)
	}
}

func TestDeleteNeverRewritesHistory(t *testing.T) {
	h, _ := NewAgentHistory(okAgent(), t0)
	n := "renamed"
	h.Update(1, AgentPatch{Name: &n}, t0)
	before, _ := json.Marshal(h.Versions())
	if err := h.Delete(1, t0); !errors.Is(err, ErrConflict) {
		t.Errorf("stale delete: want ErrConflict, got %v", err)
	}
	if err := h.Delete(2, t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if at, gone := h.Deleted(); !gone || !at.Equal(t0.Add(time.Hour)) {
		t.Errorf("deleted = %v, %v", at, gone)
	}
	after, _ := json.Marshal(h.Versions())
	if !bytes.Equal(before, after) {
		t.Error("deleting changed recorded versions")
	}
	if v, ok := h.Version(1); !ok || v.Definition.Name != "Godot developer" {
		t.Errorf("version 1 after delete: %+v %v", v, ok)
	}
	if _, _, err := h.Update(2, AgentPatch{Name: &n}, t0); !errors.Is(err, ErrDeleted) {
		t.Errorf("edit after delete: want ErrDeleted, got %v", err)
	}
	if err := h.Delete(2, t0); !errors.Is(err, ErrDeleted) {
		t.Errorf("second delete: want ErrDeleted, got %v", err)
	}
	if _, ok := h.Version(3); ok {
		t.Error("Version(3) should not exist")
	}
}

func TestHistoryDoesNotAliasCallers(t *testing.T) {
	src := okAgent()
	h, _ := NewAgentHistory(src, t0)
	src.ProviderPreference[0] = "mutated"
	head := h.Head()
	head.Definition.ProviderPreference[0] = "mutated"
	head.Definition.RequestedCapabilities[0] = "mutated.cap"
	if got := h.Head().Definition; got.ProviderPreference[0] != "claude" || got.RequestedCapabilities[0] != "code.test" {
		t.Errorf("history was altered through a copy: %+v", got)
	}
	pref := []string{"codex"}
	p := AgentPatch{ProviderPreference: &pref}
	h.Update(1, p, t0)
	pref[0] = "mutated"
	if got := h.Head().Definition.ProviderPreference[0]; got != "codex" {
		t.Errorf("patch slice aliased into history: %q", got)
	}
}

func TestHashIgnoresTimestampsAndNilVersusEmpty(t *testing.T) {
	a, b := okAgent(), okAgent()
	b.CreatedAt, b.UpdatedAt = t0, t0.Add(time.Hour)
	if HashAgent(a) != HashAgent(b) {
		t.Error("timestamps must not change the hash")
	}
	a.RequestedCapabilities, b.RequestedCapabilities = nil, []Capability{}
	if HashAgent(a) != HashAgent(b) {
		t.Error("nil and empty lists must hash alike")
	}
	b.Status = AgentDisabled
	if HashAgent(a) == HashAgent(b) {
		t.Error("status is part of the definition")
	}
}

func TestAgentContractFixturesAgree(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "packages", "contracts", "fixtures", "api", "v1")
	decode := func(rel string, v any) error {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		return dec.Decode(v)
	}
	for _, rel := range []string{"Agent/valid.json", "Agent/valid-minimal.json"} {
		var a Agent
		if err := decode(rel, &a); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if err := a.Validate(); err != nil {
			t.Errorf("%s: %v", rel, err)
		}
	}
	for _, rel := range []string{"Agent/invalid-id.json", "Agent/invalid-model.json", "Agent/invalid-status.json", "Agent/invalid-duplicate-preference.json"} {
		var a Agent
		if err := decode(rel, &a); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if err := a.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", rel, err)
		}
	}
	var list []Agent
	if err := decode("AgentList/valid.json", &list); err != nil || len(list) == 0 {
		t.Fatalf("AgentList/valid.json: %v (%d)", err, len(list))
	}
	var p AgentPatch
	if err := decode("UpdateAgent/valid.json", &p); err != nil || p.Empty() {
		t.Errorf("UpdateAgent/valid.json: %v empty=%v", err, p.Empty())
	}
	var empty AgentPatch
	if err := decode("UpdateAgent/invalid-empty.json", &empty); err != nil || !empty.Empty() {
		t.Errorf("UpdateAgent/invalid-empty.json: %v empty=%v", err, empty.Empty())
	}
	// The contract does not accept an id in an update; neither does the patch.
	var withID AgentPatch
	if err := decode("UpdateAgent/invalid-id.json", &withID); err == nil {
		t.Error("UpdateAgent/invalid-id.json decoded; the id must not be editable")
	}
}
