package coordinator

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
)

// Every mutating command takes an idempotency key (FEAT-EXE-005). The Platform stores the
// command's outcome with the change it made, in one write. A second request with the same
// command name and key does not run again: it returns the stored outcome, and only the outcome
// the first request produced, not the state the Execution has reached since. A key presented with
// a different request is refused (domain.ErrKeyReused), because the key names one request.
//
// Commands that exist: execution.submit. Worker messages are not commands in this sense; the
// fencing token already makes a repeated Result a duplicate (ADR-0044). The commands that come
// later (cancel, approve) use the same record and say so in their own change.

// CmdSubmit is the name of Submit in command records.
const CmdSubmit = "execution.submit"

// StepSpec describes one Step of a submitted Execution. Name is unique within the request; the
// Step's id is derived from the Execution id and the name.
type StepSpec struct {
	Name       string `json:"name"`
	Capability string `json:"capability"`
	Inputs     []byte `json:"inputs,omitempty"`
	Idempotent bool   `json:"idempotent,omitempty"`
}

// SubmitRequest creates one Execution. Key is the caller's idempotency key and is required.
type SubmitRequest struct {
	Key string
	// ProjectID is required: every Execution belongs to a Project, and the per-Project limit
	// counts in it. EnvironmentID is optional. Both are opaque ids; neither is checked against a
	// store yet because Projects and Environments are not looked up here.
	ProjectID     string
	EnvironmentID string
	Steps         []StepSpec
}

// Submitted is the outcome of Submit: the Execution and its Steps as created.
type Submitted struct {
	Execution domain.Execution       `json:"execution"`
	Steps     []domain.StepExecution `json:"steps"`
}

// Submit creates an Execution whose Steps are all queued, and wakes Serve. It returns replayed
// true, with the original outcome, when Key was used before with the same Steps.
//
// Until the Workflow graph exists (#138) every Step is queued at once; Workflow order, Approval
// gates and Policy will decide which Steps start queued.
func (c *Coordinator) Submit(ctx context.Context, req SubmitRequest) (Submitted, bool, error) {
	if err := validateSubmit(req); err != nil {
		return Submitted{}, false, err
	}
	fp, err := fingerprint(struct {
		Project, Environment string
		Steps                []StepSpec
	}{req.ProjectID, req.EnvironmentID, req.Steps})
	if err != nil {
		return Submitted{}, false, err
	}
	// Two passes: the second only runs when another request with the same key committed between
	// our lookup and our write, and then the lookup finds it.
	for range 2 {
		if out, found, err := replay[Submitted](ctx, c, CmdSubmit, req.Key, fp); err != nil || found {
			return out, found, err
		}
		now := c.now()
		id := c.newExecutionID()
		// The store creates every row at version 1, so the outcome can be recorded before the write.
		out := Submitted{Execution: domain.Execution{ID: id, Version: 1, ProjectID: req.ProjectID, EnvironmentID: req.EnvironmentID, State: domain.ExecQueued, CreatedAt: now}}
		for _, sp := range req.Steps {
			st := domain.StepExecution{
				ID: id + "." + sp.Name, ExecutionID: id, Version: 1, State: domain.StepPending,
				Capability: sp.Capability, InputsJSON: sp.Inputs, Idempotent: sp.Idempotent,
			}
			if err := st.Plan(); err != nil {
				return Submitted{}, false, err
			}
			out.Steps = append(out.Steps, st)
		}
		result, err := json.Marshal(out)
		if err != nil {
			return Submitted{}, false, fmt.Errorf("encode outcome: %w", err)
		}
		_, _, err = c.store.CreateOnce(ctx, domain.CommandRecord{Name: CmdSubmit, Key: req.Key, Fingerprint: fp, Result: result, At: now}, out.Execution, out.Steps)
		if errors.Is(err, domain.ErrDuplicateCommand) {
			continue
		}
		if err != nil {
			return Submitted{}, false, err
		}
		c.Wake()
		return out, false, nil
	}
	return Submitted{}, false, fmt.Errorf("submit %q: command record vanished after a duplicate", req.Key)
}

// replay returns the stored outcome of the command and true when (name, key) was recorded, or
// domain.ErrKeyReused when it was recorded for a different request.
func replay[T any](ctx context.Context, c *Coordinator, name, key, fp string) (out T, found bool, err error) {
	rec, err := c.store.Command(ctx, name, key)
	if errors.Is(err, domain.ErrNotFound) {
		return out, false, nil
	}
	if err != nil {
		return out, false, err
	}
	if rec.Fingerprint != fp {
		return out, false, fmt.Errorf("%w: %s %q", domain.ErrKeyReused, name, key)
	}
	if err := json.Unmarshal(rec.Result, &out); err != nil {
		return out, false, fmt.Errorf("decode stored outcome of %s %q: %w", name, key, err)
	}
	return out, true, nil
}

func validateSubmit(req SubmitRequest) error {
	if req.Key == "" {
		return fmt.Errorf("%w: submit needs an idempotency key", domain.ErrInvalid)
	}
	if req.ProjectID == "" {
		return fmt.Errorf("%w: submit needs a project id", domain.ErrInvalid)
	}
	if len(req.Steps) == 0 {
		return fmt.Errorf("%w: submit needs at least one step", domain.ErrInvalid)
	}
	seen := map[string]bool{}
	for _, sp := range req.Steps {
		if sp.Name == "" || sp.Capability == "" || seen[sp.Name] {
			return fmt.Errorf("%w: step %q needs a unique name and a capability", domain.ErrInvalid, sp.Name)
		}
		seen[sp.Name] = true
	}
	return nil
}

func fingerprint(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("fingerprint request: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
