package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func headVersion(t *testing.T) (*AgentHistory, AgentVersion) {
	t.Helper()
	h, err := NewAgentHistory(Agent{
		ID: "reviewer", Name: "Reviewer", Instructions: "Review.", Status: AgentActive,
		ProviderPreference: []string{"claude", "codex"}, Model: "opus", Models: map[string]string{"codex": "gpt-5"},
		WorktreeWrite: true, WorktreeCommands: []string{"go test"},
	}, t0)
	if err != nil {
		t.Fatal(err)
	}
	return h, h.Head()
}

var skillA = SkillUse{Name: "style-guide", Version: "2", Hash: HashBody("be tidy")}

func TestResolveRecordsVersionAndSkills(t *testing.T) {
	_, v := headVersion(t)
	r, err := v.Resolve("codex", ResolutionPreference, IsolationStrict, []SkillUse{skillA})
	if err != nil {
		t.Fatal(err)
	}
	if r.Revision != v.Revision || r.Hash != v.Hash {
		t.Errorf("revision/hash %d %s, want %d %s", r.Revision, r.Hash, v.Revision, v.Hash)
	}
	if r.Model != "gpt-5" || r.ProviderID != "codex" || len(r.Skills) != 1 || r.Skills[0] != skillA || !r.WorktreeWrite {
		t.Errorf("snapshot %+v", r)
	}
}

func TestResolveRefusals(t *testing.T) {
	h, v := headVersion(t)
	if err := h.Delete(1, t0); err != nil {
		t.Fatal(err)
	}
	tomb := h.Head()
	long := make([]SkillUse, MaxStepSkills+1)
	cases := map[string]struct {
		v      AgentVersion
		prov   string
		res    Resolution
		iso    ToolIsolation
		skills []SkillUse
		want   error
	}{
		"deleted":        {tomb, "claude", ResolutionDefault, "", nil, ErrDeleted},
		"bad provider":   {v, "Claude!", ResolutionDefault, "", nil, ErrInvalid},
		"bad resolution": {v, "claude", "guess", "", nil, ErrInvalid},
		"bad isolation":  {v, "claude", ResolutionDefault, "loose", nil, ErrInvalid},
		"too many":       {v, "claude", ResolutionDefault, "", long, ErrInvalid},
		"short hash":     {v, "claude", ResolutionDefault, "", []SkillUse{{"a-skill", "1", "abc"}}, ErrInvalid},
		"no version":     {v, "claude", ResolutionDefault, "", []SkillUse{{"a-skill", "", skillA.Hash}}, ErrInvalid},
		"bad name":       {v, "claude", ResolutionDefault, "", []SkillUse{{"Bad Name", "1", skillA.Hash}}, ErrInvalid},
		"duplicate":      {v, "claude", ResolutionDefault, "", []SkillUse{skillA, skillA}, ErrInvalid},
	}
	for name, c := range cases {
		if _, err := c.v.Resolve(c.prov, c.res, c.iso, c.skills); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", name, c.want, err)
		}
	}
}

// A snapshot is a copy: editing the Agent afterwards, or the caller's slices, must not change it.
func TestSnapshotSurvivesLaterEditsAndAliasing(t *testing.T) {
	h, v := headVersion(t)
	skills := []SkillUse{skillA}
	r, err := v.Resolve("claude", ResolutionPreference, "", skills)
	if err != nil {
		t.Fatal(err)
	}
	skills[0].Version = "changed"
	name := "Renamed"
	if _, _, err := h.Update(1, AgentPatch{Name: &name}, t0); err != nil {
		t.Fatal(err)
	}
	if r.Name != "Reviewer" || r.Revision != 1 || r.Skills[0].Version != "2" {
		t.Errorf("snapshot followed later changes: %+v", r)
	}
	if h.Head().Revision != 2 || h.Head().Hash == r.Hash {
		t.Error("history should have moved on while the snapshot stayed at revision 1")
	}
	if got, ok := h.Version(r.Revision); !ok || got.Hash != r.Hash {
		t.Error("the snapshot's (id, revision) must resolve to the exact version it came from")
	}
}

func TestBindAgentOnceWhilePending(t *testing.T) {
	_, v := headVersion(t)
	r, _ := v.Resolve("claude", ResolutionDefault, "", []SkillUse{skillA})
	s := StepExecution{ID: "s1", ExecutionID: "e1", State: StepPending}
	if err := s.BindAgent(r); err != nil {
		t.Fatal(err)
	}
	r.Skills[0].Name = "mutated"
	if s.Agent.Skills[0].Name != "style-guide" {
		t.Error("BindAgent aliased the caller's slice")
	}
	if err := s.BindAgent(r); !errors.Is(err, ErrConflict) {
		t.Errorf("second bind: %v", err)
	}
	if err := s.Plan(); err != nil {
		t.Fatal(err)
	}
	if s.Agent == nil || s.Agent.Revision != 1 {
		t.Error("snapshot lost across Plan")
	}
	q := StepExecution{ID: "s2", State: StepQueued}
	if err := q.BindAgent(r); !errors.Is(err, ErrInvalidTransition) || q.Agent != nil {
		t.Errorf("bind after pending: %v", err)
	}
}

func TestHashBody(t *testing.T) {
	if HashBody("") != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Error("empty-body hash is not SHA-256")
	}
	if HashBody("a") == HashBody("b") || !sha256HexRe.MatchString(HashBody("a")) {
		t.Error("hash must differ per body and be 64 lowercase hex")
	}
}

func TestResolvedAgentContractFixturesAgree(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "packages", "contracts", "fixtures", "api", "v1", "ResolvedAgent")
	decode := func(name string) (ResolvedAgent, error) {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		var r ResolvedAgent
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.DisallowUnknownFields()
		return r, dec.Decode(&r)
	}
	for _, name := range []string{"valid.json", "valid-minimal.json"} {
		r, err := decode(name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !r.Resolution.Valid() || !r.ToolIsolation.Valid() || r.Revision < 1 || !sha256HexRe.MatchString(r.Hash) {
			t.Errorf("%s: %+v", name, r)
		}
		for _, s := range r.Skills {
			if err := s.validate(); err != nil {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
	// Shapes the schema rejects that the types can also see.
	if _, err := decode("invalid-skill-extra-field.json"); err == nil {
		t.Error("unknown skill field must not decode")
	}
	r, _ := decode("invalid-skill-hash.json")
	if len(r.Skills) != 1 || r.Skills[0].validate() == nil {
		t.Error("short skill hash must fail validation")
	}
	if r, _ := decode("invalid-revision-zero.json"); r.Revision != 0 {
		t.Error("fixture should carry revision 0")
	}
}
