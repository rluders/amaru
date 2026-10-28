package coordinator

import (
	"context"
	"fmt"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
)

// Context assembly (FEAT-SKL-002). AssembleContext builds the context of one Step from its Agent
// snapshot, the Skills it pinned and the task, with domain.AssembleContext, and records it as the
// Step's Evidence. The Skills are resolved again here with the same checks as at Submit (they must
// exist at the pinned hash and a Project Skill must still be trusted), so a context is never built
// from a Skill that may not run. The context of a Step is written once: building it again from the
// same inputs returns the stored record, and from other inputs returns domain.ErrConflict.
//
// Not implemented: nothing calls AssembleContext on its own yet. Agent Steps do not exist (a
// Workflow agent step is refused at Submit and no Step gets an Agent snapshot), so no Provider is
// handed the document. Knowledge is passed in by the caller; the Platform does not read Project
// files or search an index (FEAT-PRJ-007). The token budget is FEAT-SKL-004.

// WithContexts sets where assembled contexts are recorded.
func WithContexts(s ports.ContextStore) Option { return func(c *Coordinator) { c.contexts = s } }

// ContextRequest asks for the context of one Step.
type ContextRequest struct {
	ExecutionID, StepID string
	// Intent is the task, for example the Work Item's intent.
	Intent string
	// Knowledge is the Project documentation to include, in order.
	Knowledge []domain.KnowledgeItem
}

// AssembleContext builds and records the context of a Step that has not started: it must be
// queued or waiting for an Approval. A Skill that may not be used ends the call with a
// *domain.SkillError (see domain.IsSkillError) and records nothing.
func (c *Coordinator) AssembleContext(ctx context.Context, req ContextRequest) (domain.StepContext, error) {
	if c.contexts == nil {
		return domain.StepContext{}, fmt.Errorf("%w: contexts are not configured", domain.ErrInvalid)
	}
	e, steps, err := c.store.Get(ctx, req.ExecutionID)
	if err != nil {
		return domain.StepContext{}, err
	}
	var st *domain.StepExecution
	for i := range steps {
		if steps[i].ID == req.StepID {
			st = &steps[i]
		}
	}
	if st == nil {
		return domain.StepContext{}, fmt.Errorf("%w: step %s", domain.ErrNotFound, req.StepID)
	}
	if st.State != domain.StepQueued && st.State != domain.StepAwaitingApproval {
		return domain.StepContext{}, fmt.Errorf("%w: the context of step %s is built before it starts, not when it is %s", domain.ErrInvalidTransition, st.ID, st.State)
	}
	skills, err := c.resolveSkills(ctx, e.ProjectID, st.Skills)
	if err != nil {
		return domain.StepContext{}, err
	}
	built, err := domain.AssembleContext(domain.ContextInput{Agent: st.Agent, Intent: req.Intent, Skills: skills, Knowledge: req.Knowledge})
	if err != nil {
		return domain.StepContext{}, err
	}
	return c.contexts.Record(ctx, domain.StepContext{
		ExecutionID: e.ID, StepID: st.ID, Format: domain.ContextFormat, Hash: built.Hash,
		Document: built.Document, Items: built.Items, CreatedAt: c.now(),
	})
}
