package coordinator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Coordinator reconciles Executions at start and hands their Steps to Workers.
type Coordinator struct {
	store          ports.ExecutionStore
	now            func() time.Time
	leasePeriod    time.Duration
	newAttemptID   func() string
	newExecutionID func() string
	newApprovalID  func() string
	limits         Limits
	timeouts       domain.Timeouts
	grants         ports.GrantStore // nil: no Grants exist, only rules allow
	trust          ports.TrustStore // nil: no Project is trusted, so no Step starts
	rules          []domain.PolicyRule
	assignMu       sync.Mutex // one lease decision at a time: count, check and lease are one step

	mu     sync.Mutex
	wakeCh chan struct{}
	// cancels maps an attempt to the Worker session that holds it, so Cancel can reach it.
	cancels map[string]chan<- ports.Cancel
}

// Option configures a Coordinator.
type Option func(*Coordinator)

// WithLeasePeriod sets how long a Lease lasts without a renewal. The default is 30 seconds.
func WithLeasePeriod(d time.Duration) Option { return func(c *Coordinator) { c.leasePeriod = d } }

// WithAttemptIDs replaces the attempt id generator (random by default); tests use a counter.
// WithTimeouts sets the Step timeouts: the defaults per kind of Step and the Organization maximum
// an override may reach (FEAT-WFL-004). It panics on limits that are not positive or whose
// defaults exceed the maximum, because that is a start-up mistake and not a runtime condition.
func WithTimeouts(t domain.Timeouts) Option {
	if err := t.Validate(); err != nil {
		panic(err)
	}
	return func(c *Coordinator) { c.timeouts = t }
}

func WithAttemptIDs(f func() string) Option { return func(c *Coordinator) { c.newAttemptID = f } }

// WithExecutionIDs replaces the Execution id generator (random by default); tests use a counter.
func WithExecutionIDs(f func() string) Option { return func(c *Coordinator) { c.newExecutionID = f } }

// WithApprovalIDs replaces the Approval id generator (random by default); tests use a counter.
func WithApprovalIDs(f func() string) Option { return func(c *Coordinator) { c.newApprovalID = f } }

// New returns a Coordinator. now is the clock; the Platform passes time.Now, tests a fixed one.
func New(store ports.ExecutionStore, now func() time.Time, opts ...Option) *Coordinator {
	c := &Coordinator{store: store, now: now, leasePeriod: 30 * time.Second, newAttemptID: randomID, newExecutionID: randomID, newApprovalID: randomID, limits: DefaultLimits, timeouts: domain.DefaultTimeouts, rules: domain.DefaultRules(), wakeCh: make(chan struct{}), cancels: map[string]chan<- ports.Cancel{}}
	for _, o := range opts {
		o(c)
	}
	return c
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // the system random source failing is not recoverable
	}
	return hex.EncodeToString(b[:])
}

// ReconcileReport counts what Reconcile did, per Step.
type ReconcileReport struct {
	Executions  int // open Executions examined
	Kept        int // Steps left waiting (rule 1)
	Resumed     int // Steps queued again to re-attach to a resumable remote job (rule 2)
	Retried     int // Steps queued again for a new Lease (rule 3, and Leases that ended before the Step started)
	Interrupted int // Steps ended interrupted (rule 4)
	Waiting     int // Executions moved to waiting because a Step now needs a person
}

// Reconcile applies the restart rule of ADR-0042 to every open Execution. Run it once at start,
// before any Worker is leased a Step. It is safe to run again: a Step it has settled is not
// touched twice, and a failed run leaves each Execution either fully reconciled or untouched,
// because each Execution is one atomic write.
//
// It never runs a Step. A Step it re-queues waits for a Lease like any queued Step.
func (c *Coordinator) Reconcile(ctx context.Context) (ReconcileReport, error) {
	ids, err := c.store.Open(ctx)
	if err != nil {
		return ReconcileReport{}, fmt.Errorf("reconcile: list open executions: %w", err)
	}
	var rep ReconcileReport
	for _, id := range ids {
		if err := c.reconcileOne(ctx, id, &rep); err != nil {
			return rep, fmt.Errorf("reconcile: execution %s: %w", id, err)
		}
		if err := c.settleCancel(ctx, id); err != nil {
			return rep, fmt.Errorf("reconcile: execution %s: settle cancel: %w", id, err)
		}
		rep.Executions++
	}
	return rep, nil
}

func (c *Coordinator) reconcileOne(ctx context.Context, id string, rep *ReconcileReport) error {
	exec, steps, err := c.store.Get(ctx, id)
	if err != nil {
		return err
	}
	now := c.now()
	var change ports.ExecutionChange
	var kept, resumed, retried, interrupted int
	for _, st := range steps {
		if st.State.Terminal() {
			continue
		}
		action, err := st.Reconcile(now, domain.ReasonPlatformRestarted)
		if err != nil {
			return fmt.Errorf("step %s: %w", st.ID, err)
		}
		switch action {
		case domain.RestartKept:
			kept++
			continue
		case domain.RestartResumed:
			resumed++
		case domain.RestartRetried:
			retried++
		case domain.RestartInterrupted:
			interrupted++
		}
		change.Steps = append(change.Steps, st)
		change.Events = append(change.Events, domain.ExecutionEvent{
			Kind: eventKind(action), StepID: st.ID, Reason: st.Reason, At: now,
		})
	}

	// An Execution that has a Step waiting for a person waits too; it does not fail (ADR-0042 amendment).
	waiting := false
	if interrupted > 0 && exec.State == domain.ExecRunning {
		if err := exec.Wait(); err != nil {
			return err
		}
		change.Execution = &exec
		change.Events = append(change.Events, domain.ExecutionEvent{Kind: domain.EventExecWaiting, Reason: domain.ReasonPlatformRestarted, At: now})
		waiting = true
	}

	if len(change.Steps) > 0 || change.Execution != nil {
		if _, err := c.store.Commit(ctx, change); err != nil {
			if errors.Is(err, domain.ErrConflict) {
				return fmt.Errorf("another writer changed the execution during reconcile: %w", err)
			}
			return err
		}
	}
	rep.Kept += kept
	rep.Resumed += resumed
	rep.Retried += retried
	rep.Interrupted += interrupted
	if waiting {
		rep.Waiting++
	}
	return nil
}

func eventKind(a domain.RestartAction) string {
	switch a {
	case domain.RestartResumed:
		return domain.EventStepResumed
	case domain.RestartRetried:
		return domain.EventStepRetried
	default:
		return domain.EventStepInterrupted
	}
}
