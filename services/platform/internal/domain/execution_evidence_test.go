package domain

import (
	"errors"
	"testing"
	"time"
)

func evidenceInput(t *testing.T) ExecutionEvidenceInput {
	t.Helper()
	at := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	ev, err := NewEvidence(EvidenceItem{Kind: "execution", ID: "e1"})
	if err != nil {
		t.Fatal(err)
	}
	ap, err := NewApproval(at, "e1.a", "p1", ApprovalGate{Kind: GatePolicyRequired, ExecutionID: "e1", StepID: "e1.b", Capability: "git.push"},
		Principal{Kind: PrincipalHuman, ID: "u1", Role: RoleContributor}, ev)
	if err != nil {
		t.Fatal(err)
	}
	return ExecutionEvidenceInput{
		Execution: Execution{ID: "e1", State: ExecSucceeded, CreatedAt: at},
		Steps: []StepExecution{
			{ID: "e1.a", ExecutionID: "e1", State: StepSucceeded, InputsJSON: []byte(`{"token":"s3cret"}`)},
			{ID: "e1.b", ExecutionID: "e1", State: StepSucceeded},
		},
		Events:    []ExecutionEvent{{ExecutionID: "e1", Seq: 1, Kind: EventStepLeased, StepID: "e1.a", At: at}},
		Approvals: []Approval{ap},
		Audit:     []AuditEvent{{Seq: 3, At: at, Kind: AuditApprovalRequested, Actor: "u1", ActorKind: ActorHuman, TargetType: "approval", TargetID: "e1.a"}},
		Contexts:  []StepContext{{ExecutionID: "e1", StepID: "e1.a", Format: "amaru-context/1", Hash: "h", Document: "doc", CreatedAt: at}},
		Artifacts: []ArtifactVersion{
			{ArtifactID: "z", Version: 1, ExecutionID: "e1", StepID: "e1.a"},
			{ArtifactID: "a", Version: 2, ExecutionID: "e1", StepID: "e1.a"},
			{ArtifactID: "a", Version: 1, ExecutionID: "e1", StepID: "e1.a"},
			{ArtifactID: "m", Version: 1, ExecutionID: "e1", StepID: "e1.b"},
		},
	}
}

func TestBuildExecutionEvidenceGroupsEverythingUnderItsStep(t *testing.T) {
	b, err := BuildExecutionEvidence(evidenceInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Steps) != 2 || b.Steps[0].ID != "e1.a" || b.Steps[1].ID != "e1.b" {
		t.Fatalf("steps keep creation order: %+v", b.Steps)
	}
	a := b.Steps[0]
	if a.Context == nil || a.Context.Document != "doc" || b.Steps[1].Context != nil {
		t.Errorf("context belongs to e1.a only: %+v / %+v", a.Context, b.Steps[1].Context)
	}
	var got []string
	for _, v := range a.Artifacts {
		got = append(got, v.ArtifactID+string(rune('0'+v.Version)))
	}
	if len(got) != 3 || got[0] != "a1" || got[1] != "a2" || got[2] != "z1" || len(b.Steps[1].Artifacts) != 1 {
		t.Errorf("artifacts ordered by id then version, under their own step: %v", got)
	}
	if !b.Final || len(b.Events) != 1 || len(b.Approvals) != 1 || len(b.Audit) != 1 {
		t.Errorf("final %v events %d approvals %d audit %d", b.Final, len(b.Events), len(b.Approvals), len(b.Audit))
	}
}

func TestBuildExecutionEvidenceDoesNotCopyStepInputs(t *testing.T) {
	b, err := BuildExecutionEvidence(evidenceInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if b.Steps[0].InputsHash != ContentHash([]byte(`{"token":"s3cret"}`)) || b.Steps[1].InputsHash != "" {
		t.Errorf("inputs hash %q / %q", b.Steps[0].InputsHash, b.Steps[1].InputsHash)
	}
}

func TestBuildExecutionEvidenceListsWhatItCannotHold(t *testing.T) {
	b, _ := BuildExecutionEvidence(evidenceInput(t))
	want := map[string]bool{GapCheck: true, GapDiff: true, GapLog: true, GapUsage: true, GapWorkspace: true}
	for _, g := range b.Gaps {
		if !want[g.Kind] || g.Reason == "" {
			t.Errorf("unexpected gap %+v", g)
		}
		delete(want, g.Kind)
	}
	if len(want) != 0 {
		t.Errorf("missing gaps %v", want)
	}
}

func TestBuildExecutionEvidenceIsFinalOnlyWhenTheExecutionIs(t *testing.T) {
	in := evidenceInput(t)
	in.Execution.State = ExecRunning
	b, err := BuildExecutionEvidence(in)
	if err != nil || b.Final {
		t.Fatalf("final %v, %v", b.Final, err)
	}
}

func TestBuildExecutionEvidenceDigestIsStableAndSeesEveryPart(t *testing.T) {
	base, _ := BuildExecutionEvidence(evidenceInput(t))
	again, _ := BuildExecutionEvidence(evidenceInput(t))
	if base.Digest == "" || base.Digest != again.Digest {
		t.Fatalf("digest %q vs %q", base.Digest, again.Digest)
	}
	for name, edit := range map[string]func(*ExecutionEvidenceInput){
		"execution": func(in *ExecutionEvidenceInput) { in.Execution.State = ExecFailed },
		"step":      func(in *ExecutionEvidenceInput) { in.Steps[0].Reason = "x" },
		"event":     func(in *ExecutionEvidenceInput) { in.Events[0].Reason = "x" },
		"approval":  func(in *ExecutionEvidenceInput) { in.Approvals[0].VoidReason = "x" },
		"audit":     func(in *ExecutionEvidenceInput) { in.Audit[0].Reason = "x" },
		"context":   func(in *ExecutionEvidenceInput) { in.Contexts[0].Document = "other" },
		"artifact":  func(in *ExecutionEvidenceInput) { in.Artifacts[0].Hash = "sha256:x" },
	} {
		in := evidenceInput(t)
		edit(&in)
		b, err := BuildExecutionEvidence(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if b.Digest == base.Digest {
			t.Errorf("a change to the %s did not change the digest", name)
		}
	}
}

func TestBuildExecutionEvidenceRefusesPartsOfAnotherRun(t *testing.T) {
	for name, edit := range map[string]func(*ExecutionEvidenceInput){
		"no execution id":   func(in *ExecutionEvidenceInput) { in.Execution.ID = "" },
		"foreign step":      func(in *ExecutionEvidenceInput) { in.Steps[1].ExecutionID = "e2" },
		"foreign event":     func(in *ExecutionEvidenceInput) { in.Events[0].ExecutionID = "e2" },
		"foreign approval":  func(in *ExecutionEvidenceInput) { in.Approvals[0].Gate.ExecutionID = "e2" },
		"foreign audit":     func(in *ExecutionEvidenceInput) { in.Audit[0].TargetID = "e2.a" },
		"audit of a grant":  func(in *ExecutionEvidenceInput) { in.Audit[0].TargetType = "grant" },
		"context no step":   func(in *ExecutionEvidenceInput) { in.Contexts[0].StepID = "nope" },
		"context other run": func(in *ExecutionEvidenceInput) { in.Contexts[0].ExecutionID = "e2" },
		"artifact no step":  func(in *ExecutionEvidenceInput) { in.Artifacts[3].StepID = "nope" },
		"artifact other":    func(in *ExecutionEvidenceInput) { in.Artifacts[0].ExecutionID = "e2" },
	} {
		in := evidenceInput(t)
		edit(&in)
		if _, err := BuildExecutionEvidence(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
