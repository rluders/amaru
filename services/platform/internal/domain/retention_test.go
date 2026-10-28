package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultRetentionIsValid(t *testing.T) {
	p, err := NewRetentionPolicy(nil)
	if err != nil {
		t.Fatal(err)
	}
	if p[ClassLogs] != Days(30) || p[ClassIntermediateArtifact] != Days(90) || !p[ClassAudit].Indefinite || !p[ClassReferencedArtifact].Indefinite {
		t.Errorf("defaults = %+v", p)
	}
}

func TestRetentionIsConfigurablePerClass(t *testing.T) {
	p, err := NewRetentionPolicy(map[DataClass]Retention{ClassLogs: Days(7), ClassIntermediateArtifact: Days(10)})
	if err != nil {
		t.Fatal(err)
	}
	if p[ClassLogs] != Days(7) || p[ClassIntermediateArtifact] != Days(10) {
		t.Errorf("set = %+v", p)
	}
	if p[ClassAudit] != Forever {
		t.Error("an unset class keeps its default")
	}
}

func TestRetentionRefusals(t *testing.T) {
	for name, set := range map[string]map[DataClass]Retention{
		"unknown class":               {"nope": Days(1)},
		"referenced has no setting":   {ClassReferencedArtifact: Days(1)},
		"zero":                        {ClassLogs: {}},
		"negative":                    {ClassLogs: {Keep: -time.Hour}},
		"audit shorter than artifact": {ClassAudit: Days(30)},
		"artifact outlives audit":     {ClassAudit: Days(100), ClassIntermediateArtifact: Forever},
	} {
		if _, err := NewRetentionPolicy(set); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	// a finite audit retention at least as long as the artifact's is fine
	if _, err := NewRetentionPolicy(map[DataClass]Retention{ClassAudit: Days(90)}); err != nil {
		t.Errorf("audit equal to artifact: %v", err)
	}
}

func TestExpired(t *testing.T) {
	p := DefaultRetention()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		class DataClass
		age   time.Duration
		ref   bool
		want  bool
	}{
		{"logs inside window", ClassLogs, 29 * 24 * time.Hour, false, false},
		{"logs at the edge", ClassLogs, 30 * 24 * time.Hour, false, true},
		{"logs past window", ClassLogs, 31 * 24 * time.Hour, false, true},
		{"intermediate past window", ClassIntermediateArtifact, 91 * 24 * time.Hour, false, true},
		{"intermediate still referenced", ClassIntermediateArtifact, 400 * 24 * time.Hour, true, false},
		{"referenced class never", ClassReferencedArtifact, 4000 * 24 * time.Hour, false, false},
		{"audit never", ClassAudit, 4000 * 24 * time.Hour, false, false},
	} {
		got, err := p.Expired(tc.class, t0, t0.Add(tc.age), tc.ref)
		if err != nil || got != tc.want {
			t.Errorf("%s: got %v, %v; want %v", tc.name, got, err, tc.want)
		}
	}
	if _, err := p.Expired("nope", t0, t0, false); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown class: %v", err)
	}
}
