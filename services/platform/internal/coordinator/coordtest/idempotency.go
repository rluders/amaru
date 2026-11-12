package coordtest

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// CommandStores supplies one authoritative store and a fresh adapter over its retained data.
type CommandStores struct {
	Executions ports.ExecutionStore
	Grants     ports.GrantStore
	Trust      ports.TrustStore
	Projects   ports.ProjectStore
	Reopen     func() CommandStores
}

// IdempotencySuite covers every caller-facing mutation, including commit/ack failure boundaries.
func IdempotencySuite(t *testing.T, factory func(*testing.T) CommandStores) {
	ctx := context.Background()
	owner := domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}
	project := domain.ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	names := []string{coordinator.CmdSubmit, coordinator.CmdCancel, coordinator.CmdDecideApproval, coordinator.CmdCreateGrant, coordinator.CmdRevokeGrant, coordinator.CmdTrustContent, coordinator.CmdRevokeTrust}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for _, mode := range []string{"sequential and restart", "concurrent", "before commit", "lost acknowledgement"} {
				t.Run(mode, func(t *testing.T) {
					stores := factory(t)
					if err := stores.Projects.Create(ctx, domain.Project{Key: project, Name: "app", CreatedAt: clock()}); err != nil {
						t.Fatal(err)
					}
					build := func(s CommandStores, now func() time.Time) *coordinator.Coordinator {
						return coordinator.New(s.Executions, now, coordinator.WithGrants(s.Grants), coordinator.WithTrust(s.Trust), coordinator.WithExecutionIDs(func() string { return "exec" }), coordinator.WithApprovalIDs(func() string { return "approval" }))
					}
					c := build(stores, clock)
					grant := domain.Grant{ID: "grant", Level: domain.GrantProject, ProjectID: string(project), Subject: domain.Subject{Kind: domain.SubjectWorkflow, ID: "tools"}, Capabilities: []domain.Capability{"mcp.call"}}
					if name == coordinator.CmdCreateGrant && mode != "before commit" {
						expiry := clock().Add(time.Hour)
						grant.ExpiresAt = &expiry
					}
					if _, err := c.TrustContent(ctx, "seed-trust", owner, project, domain.TrustProject, "", ""); err != nil {
						t.Fatal(err)
					}
					if name == coordinator.CmdRevokeGrant {
						if _, err := c.CreateGrant(ctx, "seed-grant", owner, grant); err != nil {
							t.Fatal(err)
						}
					}
					var submitted coordinator.Submitted
					if name == coordinator.CmdCancel || name == coordinator.CmdDecideApproval {
						var err error
						submitted, _, err = c.Submit(ctx, coordinator.SubmitRequest{Key: "seed-submit", By: owner, ProjectID: string(project), Steps: []coordinator.StepSpec{{Name: "push", Capability: "git.push"}}})
						if err != nil || len(submitted.Approvals) != 1 {
							t.Fatalf("seed: %v %+v", err, submitted)
						}
					}
					call := func(c *coordinator.Coordinator, key string, by domain.Principal, changed bool) ([]byte, error) {
						why := "requested"
						if changed {
							why = "changed"
						}
						var out any
						var err error
						switch name {
						case coordinator.CmdSubmit:
							out, _, err = c.Submit(ctx, coordinator.SubmitRequest{Key: key, By: by, ProjectID: string(project), Steps: []coordinator.StepSpec{{Name: why, Capability: "code.test"}}})
						case coordinator.CmdCancel:
							out, _, err = c.Cancel(ctx, coordinator.CancelRequest{Key: key, ExecutionID: submitted.Execution.ID, Reason: why, By: domain.Actor{Kind: domain.ActorKind(by.Kind)}, ActorID: by.ID})
						case coordinator.CmdDecideApproval:
							out, _, err = c.DecideApproval(ctx, coordinator.DecideRequest{Key: key, ApprovalID: submitted.Approvals[0].ID, By: by, Outcome: domain.OutcomeApproved, Rationale: why, EvidenceDigest: submitted.Approvals[0].Evidence.Digest})
						case coordinator.CmdCreateGrant:
							g := grant
							if changed {
								g.Capabilities = []domain.Capability{"git.push"}
							}
							out, err = c.CreateGrant(ctx, key, by, g)
						case coordinator.CmdRevokeGrant:
							out, err = c.RevokeGrant(ctx, key, by, grant.ID, why)
						case coordinator.CmdTrustContent:
							ref := "workflow"
							if changed {
								ref = "other"
							}
							out, err = c.TrustContent(ctx, key, by, project, domain.TrustWorkflow, ref, "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
						case coordinator.CmdRevokeTrust:
							out, err = c.RevokeTrust(ctx, key, by, project, domain.TrustProject, "", why)
						}
						if err != nil {
							return nil, err
						}
						return json.Marshal(out)
					}
					reader := func(s CommandStores) ports.CommandReader {
						switch name {
						case coordinator.CmdCreateGrant, coordinator.CmdRevokeGrant:
							return s.Grants
						case coordinator.CmdTrustContent, coordinator.CmdRevokeTrust:
							return s.Trust
						default:
							return s.Executions
						}
					}
					// Count the command's audit effect once; execution cancellation has its own Event log too.
					audit := func() int {
						var log ports.AuditLog = stores.Executions
						switch name {
						case coordinator.CmdCreateGrant, coordinator.CmdRevokeGrant:
							log = stores.Grants.(ports.AuditLog)
						case coordinator.CmdTrustContent, coordinator.CmdRevokeTrust:
							log = stores.Trust
						}
						rows, err := log.Audit(ctx, 0, 0)
						if err != nil {
							t.Fatal(err)
						}
						return len(rows)
					}
					baseline := audit()
					var first []byte
					switch mode {
					case "concurrent":
						const n = 16
						results := make([][]byte, n)
						errs := make([]error, n)
						var wg sync.WaitGroup
						start := make(chan struct{})
						for i := range n {
							wg.Add(1)
							go func() { defer wg.Done(); <-start; results[i], errs[i] = call(c, "key", owner, false) }()
						}
						close(start)
						wg.Wait()
						for i := range n {
							if errs[i] != nil {
								t.Fatalf("duplicate %d: %v", i, errs[i])
							}
							if string(results[i]) != string(results[0]) {
								t.Fatalf("duplicate %d differs", i)
							}
						}
						first = results[0]
					case "before commit", "lost acknowledgement":
						fault := &commandFault{after: mode == "lost acknowledgement"}
						broken := stores
						broken.Executions = faultExecutions{stores.Executions, fault}
						broken.Grants = faultGrants{stores.Grants, fault}
						broken.Trust = faultTrust{stores.Trust, fault}
						if _, err := call(build(broken, clock), "key", owner, false); !errors.Is(err, errCommandFault) {
							t.Fatalf("fault: %v", err)
						}
						rec, err := reader(stores).Command(ctx, name, "key")
						if fault.after {
							if err != nil {
								t.Fatal(err)
							}
							first = rec.Result
						} else {
							if !errors.Is(err, domain.ErrNotFound) || audit() != baseline {
								t.Fatalf("precommit failure left state: %v", err)
							}
							if name == coordinator.CmdCreateGrant {
								if _, err := stores.Grants.Get(ctx, grant.ID); !errors.Is(err, domain.ErrNotFound) {
									t.Fatalf("grant persisted: %v", err)
								}
							}
						}
					default:
						var err error
						first, err = call(c, "key", owner, false)
						if err != nil {
							t.Fatal(err)
						}
					}
					// Recreate adapters and coordinator, advance the clock: neither restart nor age frees keys.
					reopened := stores
					if stores.Reopen != nil {
						reopened = stores.Reopen()
					}
					restarted := build(reopened, func() time.Time { return clock().Add(365 * 24 * time.Hour) })
					again, err := call(restarted, "key", owner, false)
					if err != nil {
						t.Fatal(err)
					}
					if first == nil {
						first = again
					} else if string(first) != string(again) {
						t.Fatalf("original outcome changed\n%s\n%s", first, again)
					}
					wantAudit := baseline
					if name != coordinator.CmdSubmit {
						wantAudit++
					}
					if got := audit(); got != wantAudit {
						t.Fatalf("audit events %d, want %d", got, wantAudit)
					}
					evs, err := stores.Executions.Events(ctx, "exec")
					if name == coordinator.CmdCancel && (err != nil || len(evs) != 3) {
						t.Fatalf("cancel events: %v %+v", err, evs)
					}
					for range 2 {
						out, err := call(restarted, "key", owner, false)
						if err != nil || string(out) != string(first) {
							t.Fatalf("replay: %v", err)
						}
					}
					if _, err := call(restarted, "key", owner, true); !errors.Is(err, domain.ErrKeyReused) {
						t.Fatalf("payload reuse: %v", err)
					}
					other := owner
					other.ID = "other"
					if _, err := call(restarted, "key", other, false); !errors.Is(err, domain.ErrKeyReused) {
						t.Fatalf("actor reuse: %v", err)
					}
					other = domain.Principal{Kind: domain.PrincipalAgent, ID: "agent"}
					if out, err := call(restarted, "key", other, false); err == nil || len(out) != 0 {
						t.Fatalf("unauthorized replay: %s %v", out, err)
					}
					for _, invalidKey := range []string{"", strings.Repeat("k", 129), string([]byte{0xff})} {
						if _, err := call(restarted, invalidKey, owner, false); !errors.Is(err, domain.ErrInvalid) {
							t.Fatalf("invalid key: %v", err)
						}
					}
					if name == coordinator.CmdCancel {
						if _, _, err := restarted.Cancel(ctx, coordinator.CancelRequest{Key: "key", ExecutionID: submitted.Execution.ID, Reason: "requested", By: domain.Actor{Kind: domain.ActorHuman}}); !errors.Is(err, domain.ErrNotPermitted) {
							t.Fatalf("anonymous human replay: %v", err)
						}
					}
					if audit() != wantAudit {
						t.Fatal("replay or conflict wrote another audit event")
					}
					after, _ := stores.Executions.Events(ctx, "exec")
					if len(after) != len(evs) {
						t.Fatal("duplicate appended execution events")
					}
					rec, err := reader(reopened).Command(ctx, name, "key")
					if err != nil || string(rec.Result) != string(first) {
						t.Fatalf("durable result: %v", err)
					}
					// Later mutations do not replace the original outcome or let an old revoke undo renewal.
					var later error
					switch name {
					case coordinator.CmdCreateGrant:
						_, later = restarted.RevokeGrant(ctx, "key", owner, grant.ID, "withdrawn")
					case coordinator.CmdTrustContent:
						_, later = restarted.RevokeTrust(ctx, "key", owner, project, domain.TrustWorkflow, "workflow", "withdrawn")
					case coordinator.CmdRevokeTrust:
						_, later = restarted.TrustContent(ctx, "key", owner, project, domain.TrustProject, "", "")
					default:
						return
					}
					if later != nil {
						t.Fatal(later)
					}
					effects := audit()
					if out, err := call(restarted, "key", owner, false); err != nil || string(out) != string(first) {
						t.Fatalf("replay after later mutation: %v", err)
					}
					if audit() != effects {
						t.Fatal("replay undid a later mutation")
					}

				})
			}
		})
	}
}

var errCommandFault = errors.New("injected command failure")

type commandFault struct{ after bool }

func (f *commandFault) run(write func() error) error {
	if !f.after {
		return errCommandFault
	}
	if err := write(); err != nil {
		return err
	}
	return errCommandFault
}

type faultExecutions struct {
	ports.ExecutionStore
	f *commandFault
}

func (s faultExecutions) CreateOnce(ctx context.Context, r domain.CommandRecord, e domain.Execution, ss []domain.StepExecution, aa ...domain.Approval) (out domain.Execution, steps []domain.StepExecution, err error) {
	err = s.f.run(func() error {
		var e2 error
		out, steps, e2 = s.ExecutionStore.CreateOnce(ctx, r, e, ss, aa...)
		return e2
	})
	return
}
func (s faultExecutions) Commit(ctx context.Context, c ports.ExecutionChange) (out ports.ExecutionChange, err error) {
	err = s.f.run(func() error { var e error; out, e = s.ExecutionStore.Commit(ctx, c); return e })
	return
}

type faultGrants struct {
	ports.GrantStore
	f *commandFault
}

func (s faultGrants) CreateOnce(ctx context.Context, r domain.CommandRecord, g domain.Grant) (out domain.Grant, err error) {
	err = s.f.run(func() error { var e error; out, e = s.GrantStore.CreateOnce(ctx, r, g); return e })
	return
}
func (s faultGrants) UpdateOnce(ctx context.Context, r domain.CommandRecord, g domain.Grant) (out domain.Grant, err error) {
	err = s.f.run(func() error { var e error; out, e = s.GrantStore.UpdateOnce(ctx, r, g); return e })
	return
}

type faultTrust struct {
	ports.TrustStore
	f *commandFault
}

func (s faultTrust) SaveOnce(ctx context.Context, r domain.CommandRecord, tr domain.Trust) error {
	return s.f.run(func() error { return s.TrustStore.SaveOnce(ctx, r, tr) })
}

// CommandAtomicitySuite checks errors after a command record is inserted roll the record back too.
func CommandAtomicitySuite(t *testing.T, factory func(*testing.T) CommandStores) {
	s := factory(t)
	ctx := context.Background()
	owner := domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}
	project := domain.ProjectKey("ssh://github.com/acme/app#0123456789abcdef0123456789abcdef01234567")
	if err := s.Projects.Create(ctx, domain.Project{Key: project, Name: "app", CreatedAt: clock()}); err != nil {
		t.Fatal(err)
	}
	c := coordinator.New(s.Executions, clock, coordinator.WithGrants(s.Grants), coordinator.WithTrust(s.Trust))
	g, err := c.CreateGrant(ctx, "seed", owner, domain.Grant{ID: "g", Level: domain.GrantProject, ProjectID: string(project), Subject: domain.Subject{Kind: domain.SubjectWorkflow, ID: "w"}, Capabilities: []domain.Capability{"mcp.call"}})
	if err != nil {
		t.Fatal(err)
	}
	record := func(key string) domain.CommandRecord {
		return domain.CommandRecord{Name: "atomicity", Key: key, Fingerprint: "fingerprint", Result: []byte(`{}`), At: clock()}
	}
	noRecord := func(reader ports.CommandReader, key string) {
		t.Helper()
		if _, err := reader.Command(ctx, "atomicity", key); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("failed mutation retained record %s: %v", key, err)
		}
	}
	if _, err := s.Grants.CreateOnce(ctx, record("exists"), g); !errors.Is(err, domain.ErrExists) {
		t.Fatalf("create: %v", err)
	}
	noRecord(s.Grants, "exists")
	g.Version = 0
	if _, err := s.Grants.UpdateOnce(ctx, record("stale"), g); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("update: %v", err)
	}
	noRecord(s.Grants, "stale")
	old, err := c.TrustContent(ctx, "old", owner, project, domain.TrustWorkflow, "w", "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TrustContent(ctx, "new", owner, project, domain.TrustWorkflow, "w", "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"); err != nil {
		t.Fatal(err)
	}
	if err := old.Revoke(clock(), owner, "withdraw old trust"); err != nil {
		t.Fatal(err)
	}
	if err := s.Trust.SaveOnce(ctx, record("replaced"), old); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale trust: %v", err)
	}
	noRecord(s.Trust, "replaced")
	current, err := s.Trust.Get(ctx, project, domain.TrustWorkflow, "w")
	if err != nil || current.Hash == old.Hash || current.RevokedAt != nil {
		t.Fatalf("stale revocation changed renewal: %+v %v", current, err)
	}
}
