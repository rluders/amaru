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
	// Steps are the Steps to run when no Workflow is given. Exactly one of Steps and Workflow is set.
	Steps []StepSpec
	// Workflow is the definition to run. Submit pins a snapshot of it to the Execution
	// (FEAT-WFL-002) and creates one Step per Workflow step, in Workflow order. Every step must
	// name a Capability; agent and approval steps have none yet, so they are refused until the
	// Agent and Approval Steps exist. Revision is the Git commit id the definition was read at,
	// or empty if it was not committed.
	Workflow *domain.Workflow
	Revision string
	// Variables overrides the Workflow's declared variables for this Execution, for example the
	// intent of a Work Item (FEAT-WFL-003). Each name must be declared by the Workflow and each
	// value is at most domain.MaxVarValue characters; a value is refused, never cut. Placeholders
	// in step inputs are replaced before the Steps are created, so a Step's inputs are final.
	// Without a Workflow it must be empty.
	Variables map[string]string
}

// Submitted is the outcome of Submit: the Execution and its Steps as created.
type Submitted struct {
	Execution domain.Execution       `json:"execution"`
	Steps     []domain.StepExecution `json:"steps"`
}

// Submit creates an Execution whose Steps are all queued, and wakes Serve. It returns replayed
// true, with the original outcome, when Key was used before with the same Steps.
//
// Every Step is queued at once, even when a Workflow gives them an order; edges, Approval gates
// and Policy will decide which Steps start queued.
func (c *Coordinator) Submit(ctx context.Context, req SubmitRequest) (Submitted, bool, error) {
	pin, specs, err := plan(req)
	if err != nil {
		return Submitted{}, false, err
	}
	fp, err := fingerprint(struct {
		Project, Environment string
		Steps                []StepSpec
		Definition           *domain.DefinitionPin
		Variables            map[string]string
	}{req.ProjectID, req.EnvironmentID, specs, pin, req.Variables})
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
		out := Submitted{Execution: domain.Execution{ID: id, Version: 1, ProjectID: req.ProjectID, EnvironmentID: req.EnvironmentID, State: domain.ExecQueued, Definition: pin, CreatedAt: now}}
		for _, sp := range specs {
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

// plan returns the pin and the Steps a request creates: the Workflow's, in its order, or the
// caller's.
func plan(req SubmitRequest) (*domain.DefinitionPin, []StepSpec, error) {
	if req.Key == "" {
		return nil, nil, fmt.Errorf("%w: submit needs an idempotency key", domain.ErrInvalid)
	}
	if req.ProjectID == "" {
		return nil, nil, fmt.Errorf("%w: submit needs a project id", domain.ErrInvalid)
	}
	var pin *domain.DefinitionPin
	specs := req.Steps
	switch {
	case req.Workflow != nil && len(req.Steps) > 0:
		return nil, nil, fmt.Errorf("%w: submit takes a workflow or steps, not both", domain.ErrInvalid)
	case req.Workflow != nil:
		p, err := domain.Pin(*req.Workflow, req.Revision)
		if err != nil {
			return nil, nil, err
		}
		pin = &p
		if specs, err = workflowSpecs(*req.Workflow, req.Variables); err != nil {
			return nil, nil, err
		}
	case req.Revision != "":
		return nil, nil, fmt.Errorf("%w: a revision needs a workflow", domain.ErrInvalid)
	case len(req.Variables) > 0:
		return nil, nil, fmt.Errorf("%w: variables need a workflow", domain.ErrInvalid)
	}
	if len(specs) == 0 {
		return nil, nil, fmt.Errorf("%w: submit needs at least one step", domain.ErrInvalid)
	}
	seen := map[string]bool{}
	for _, sp := range specs {
		if sp.Name == "" || sp.Capability == "" || seen[sp.Name] {
			return nil, nil, fmt.Errorf("%w: step %q needs a unique name and a capability", domain.ErrInvalid, sp.Name)
		}
		seen[sp.Name] = true
	}
	return pin, specs, nil
}

func workflowSpecs(w domain.Workflow, overrides map[string]string) ([]StepSpec, error) {
	order, err := w.Order()
	if err != nil {
		return nil, err
	}
	vars, err := w.Resolve(overrides)
	if err != nil {
		return nil, err
	}
	specs := make([]StepSpec, 0, len(order))
	for _, i := range order {
		st := w.Steps[i]
		if st.Capability == "" {
			return nil, fmt.Errorf("%w: workflow step %s (%s) names no capability and cannot run yet", domain.ErrInvalid, st.ID, st.Type)
		}
		sp := StepSpec{Name: st.ID, Capability: string(st.Capability)}
		if len(st.Input) > 0 {
			resolved := make(map[string]string, len(st.Input))
			for k, v := range st.Input {
				if resolved[k], err = domain.Substitute(v, vars); err != nil {
					return nil, fmt.Errorf("input %s of step %s: %w", k, st.ID, err)
				}
			}
			in, err := json.Marshal(resolved)
			if err != nil {
				return nil, fmt.Errorf("encode inputs of step %s: %w", st.ID, err)
			}
			sp.Inputs = in
		}
		specs = append(specs, sp)
	}
	return specs, nil
}

// Definition returns the Workflow the Execution was submitted with, read from the snapshot pinned
// to it and not from any file, so editing the definition never changes what a running or past
// Execution runs and a restarted Platform resumes with the same definition. It returns
// domain.ErrNotFound when the Execution has no pinned definition.
func (c *Coordinator) Definition(ctx context.Context, executionID string) (domain.Workflow, error) {
	e, _, err := c.store.Get(ctx, executionID)
	if err != nil {
		return domain.Workflow{}, err
	}
	if e.Definition == nil {
		return domain.Workflow{}, fmt.Errorf("%w: execution %s has no pinned definition", domain.ErrNotFound, executionID)
	}
	return e.Definition.Workflow()
}

func fingerprint(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("fingerprint request: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
