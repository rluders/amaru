package domain

import (
	"errors"
	"testing"
	"time"
)

func testApproval(t *testing.T) Approval {
	t.Helper()
	ev, err := NewEvidence(EvidenceItem{Kind: "execution", ID: "e1"}, EvidenceItem{Kind: "step_execution", ID: "e1.s"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewApproval(t0, "a1", "p1", ApprovalGate{Kind: GatePolicyRequired, ExecutionID: "e1", StepID: "e1.s", Capability: "git.push"},
		Principal{Kind: PrincipalHuman, ID: "u1", Role: RoleContributor}, ev)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestEvidenceDigestIsOrderIndependentAndContentSensitive(t *testing.T) {
	a, _ := NewEvidence(EvidenceItem{Kind: "commit", Hash: "x"}, EvidenceItem{Kind: "execution", ID: "e1"})
	b, _ := NewEvidence(EvidenceItem{Kind: "execution", ID: "e1"}, EvidenceItem{Kind: "commit", Hash: "x"})
	if a.Digest != b.Digest || a.Digest == "" {
		t.Fatalf("same items, different digest: %q %q", a.Digest, b.Digest)
	}
	for name, items := range map[string][]EvidenceItem{
		"another hash": {{Kind: "commit", Hash: "y"}, {Kind: "execution", ID: "e1"}},
		"another id":   {{Kind: "commit", Hash: "x"}, {Kind: "execution", ID: "e2"}},
		"another kind": {{Kind: "diff", Hash: "x"}, {Kind: "execution", ID: "e1"}},
		"fewer items":  {{Kind: "execution", ID: "e1"}},
		// A value cannot be moved between fields to forge the same digest.
		"shifted fields": {{Kind: "commit", ID: "x"}, {Kind: "execution", ID: "e1"}},
	} {
		c, err := NewEvidence(items...)
		if err != nil || c.Digest == a.Digest {
			t.Errorf("%s: digest unchanged or error: %v", name, err)
		}
	}
	for name, items := range map[string][]EvidenceItem{
		"none": nil, "no kind": {{ID: "x"}}, "no id or hash": {{Kind: "commit"}},
	} {
		if _, err := NewEvidence(items...); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestApprovalDecision(t *testing.T) {
	maintainer := Principal{Kind: PrincipalHuman, ID: "m1", Role: RoleMaintainer}
	now := t0.Add(time.Hour)

	a := testApproval(t)
	if err := a.Decide(now, maintainer, OutcomeApproved, "looks right", a.Evidence.Digest); err != nil {
		t.Fatal(err)
	}
	d := a.Decision
	if a.State != ApprovalApproved || d == nil || d.Approver != "m1" || d.ApproverRole != RoleMaintainer || !d.DecidedAt.Equal(now) || d.EvidenceDigest != a.Evidence.Digest || d.Rationale != "looks right" {
		t.Fatalf("%+v %+v", a, d)
	}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	// Final: no second decision, no void.
	if err := a.Decide(now, maintainer, OutcomeRejected, "no", a.Evidence.Digest); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("second decision: %v", err)
	}
	if err := a.Void(now, "x"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("void after a decision: %v", err)
	}

	for name, by := range map[string]Principal{
		"viewer":      {Kind: PrincipalHuman, ID: "u", Role: RoleViewer},
		"contributor": {Kind: PrincipalHuman, ID: "u", Role: RoleContributor},
		"no role":     {Kind: PrincipalHuman, ID: "u"},
		"no id":       {Kind: PrincipalHuman, Role: RoleOwner},
		"agent":       {Kind: PrincipalAgent, ID: "a", Role: RoleOwner},
		"trigger":     {Kind: PrincipalTrigger, ID: "t", Role: RoleOwner},
		"plugin":      {Kind: PrincipalPlugin, ID: "p", Role: RoleOwner},
	} {
		b := testApproval(t)
		if err := b.Decide(now, by, OutcomeApproved, "", b.Evidence.Digest); !errors.Is(err, ErrNotPermitted) || b.State != ApprovalRequested || b.Decision != nil {
			t.Errorf("%s: %v (state %s)", name, err, b.State)
		}
	}
	for _, role := range []Role{RoleMaintainer, RoleAdmin, RoleOwner} {
		b := testApproval(t)
		if err := b.Decide(now, Principal{Kind: PrincipalHuman, ID: "u", Role: role}, OutcomeApproved, "", b.Evidence.Digest); err != nil {
			t.Errorf("%s: %v", role, err)
		}
	}

	b := testApproval(t)
	if err := b.Decide(now, maintainer, OutcomeRejected, "  ", b.Evidence.Digest); !errors.Is(err, ErrReasonRequired) {
		t.Errorf("reject without a rationale: %v", err)
	}
	if err := b.Decide(now, maintainer, "maybe", "", b.Evidence.Digest); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown outcome: %v", err)
	}
	if err := b.Decide(now, maintainer, OutcomeApproved, "", "sha256:other"); !errors.Is(err, ErrEvidenceChanged) {
		t.Errorf("other evidence: %v", err)
	}
	if b.State != ApprovalRequested {
		t.Errorf("a refused decision changed the state to %s", b.State)
	}
	if err := b.Decide(now, maintainer, OutcomeRejected, "wrong branch", b.Evidence.Digest); err != nil || b.State != ApprovalRejected {
		t.Errorf("reject: %v (%s)", err, b.State)
	}
}

func TestApprovalVoid(t *testing.T) {
	a := testApproval(t)
	if err := a.Void(t0, ""); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("void without a reason: %v", err)
	}
	if err := a.Void(t0, "execution_cancelled"); err != nil || a.State != ApprovalVoided || a.Validate() != nil {
		t.Fatalf("void: %v %+v", err, a)
	}
	maintainer := Principal{Kind: PrincipalHuman, ID: "m1", Role: RoleMaintainer}
	if err := a.Decide(t0, maintainer, OutcomeApproved, "", a.Evidence.Digest); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("decide after void: %v", err)
	}
}

func TestApprovalValidate(t *testing.T) {
	for name, mod := range map[string]func(*Approval){
		"no id":             func(a *Approval) { a.ID = "" },
		"no project":        func(a *Approval) { a.ProjectID = "" },
		"unknown gate":      func(a *Approval) { a.Gate.Kind = "step_approval" },
		"no step":           func(a *Approval) { a.Gate.StepID = "" },
		"no requester":      func(a *Approval) { a.Requester.ID = "" },
		"low minimum role":  func(a *Approval) { a.MinRole = RoleContributor },
		"no evidence":       func(a *Approval) { a.Evidence = Evidence{} },
		"approved no body":  func(a *Approval) { a.State = ApprovalApproved },
		"requested decided": func(a *Approval) { a.Decision = &ApprovalDecision{} },
		"unknown state":     func(a *Approval) { a.State = "pending" },
	} {
		a := testApproval(t)
		mod(&a)
		if err := a.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestApprovalAuditEvent(t *testing.T) {
	a := testApproval(t)
	ev, err := ApprovalAuditEvent(nil, a)
	if err != nil || ev.Kind != AuditApprovalRequested || ev.ActorKind != ActorPolicy || ev.TargetType != "approval" || ev.TargetID != "a1" || ev.ProjectID != "p1" || !ev.At.Equal(t0) || ev.Reason == "" {
		t.Fatalf("requested: %+v, %v", ev, err)
	}
	maintainer := Principal{Kind: PrincipalHuman, ID: "m1", Role: RoleMaintainer}
	approved, rejected, voided := a, a, a
	_ = approved.Decide(t0.Add(time.Minute), maintainer, OutcomeApproved, "fine", a.Evidence.Digest)
	_ = rejected.Decide(t0.Add(time.Minute), maintainer, OutcomeRejected, "no", a.Evidence.Digest)
	_ = voided.Void(t0.Add(time.Minute), "execution_cancelled")
	for kind, next := range map[string]Approval{AuditApprovalApproved: approved, AuditApprovalRejected: rejected, AuditApprovalVoided: voided} {
		ev, err := ApprovalAuditEvent(&a, next)
		if err != nil || ev.Kind != kind || ev.Reason == "" || !ev.At.Equal(t0.Add(time.Minute)) {
			t.Errorf("%s: %+v, %v", kind, ev, err)
		}
	}
	if ev, _ := ApprovalAuditEvent(&a, approved); ev.Actor != "m1" || ev.ActorKind != ActorHuman {
		t.Errorf("approved actor %+v", ev)
	}
	if ev, _ := ApprovalAuditEvent(&a, voided); ev.ActorKind != ActorPlatform {
		t.Errorf("voided actor %+v", ev)
	}
	// Not lifecycle steps: no change, a decided Approval moving again, a new Approval already decided.
	if _, err := ApprovalAuditEvent(&a, a); !errors.Is(err, ErrInvalid) {
		t.Errorf("requested to requested: %v", err)
	}
	if _, err := ApprovalAuditEvent(&approved, rejected); !errors.Is(err, ErrInvalid) {
		t.Errorf("approved to rejected: %v", err)
	}
	if _, err := ApprovalAuditEvent(nil, approved); !errors.Is(err, ErrInvalid) {
		t.Errorf("new but decided: %v", err)
	}
}
