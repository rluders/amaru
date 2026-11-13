package attach

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/workflowdef"
)

// WorkflowReader reads committed definitions using Attacher's hardened, bounded Git process
// path. It captures one active clone from the same Instance store used for attach; public
// callers must enter through ProjectAccess before lookup or Git IO.
type WorkflowReader struct {
	Git Attacher
}

var _ ports.ProjectWorkflowSource = WorkflowReader{}

// Workflow loads only a regular blob in the selected repository's history. It checks the
// clone's current identity and the requested commit's root before reading definition bytes.
// Errors deliberately omit Git diagnostics and parser values from untrusted repository content.
func (r WorkflowReader) Workflow(ctx context.Context, project domain.ProjectKey, id, revision string) (domain.Workflow, error) {
	if err := ctx.Err(); err != nil {
		return domain.Workflow{}, err
	}
	if r.Git.Projects == nil || r.Git.Runner == nil || r.Git.Resolver == nil {
		return domain.Workflow{}, fmt.Errorf("%w: Workflow reader needs Git, Projects and clone lookup", domain.ErrInvalid)
	}
	if !domain.ValidWorkflowID(id) || !domain.ValidDefinitionRevision(revision) {
		return domain.Workflow{}, domain.ErrInvalid
	}
	if !project.Valid() {
		return domain.Workflow{}, domain.ErrNotFound
	}
	p, err := r.Git.Projects.Get(ctx, project)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && p.Key != project) {
		return domain.Workflow{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Workflow{}, err
	}
	clone, err := r.Git.Projects.ActiveClone(ctx, project)
	if ctx.Err() != nil {
		return domain.Workflow{}, ctx.Err()
	}
	if err != nil || clone.ProjectKey != project || !filepath.IsAbs(clone.Root) {
		return domain.Workflow{}, domain.ErrNotFound
	}
	dir := clone.Root
	top, err := r.Git.one(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil || filepath.Clean(top) != filepath.Clean(dir) {
		return domain.Workflow{}, domain.ErrNotFound
	}
	if shallow, err := r.Git.one(ctx, dir, "rev-parse", "--is-shallow-repository"); err != nil || shallow != "false" {
		return domain.Workflow{}, domain.ErrInvalid
	}
	cfg, err := r.Git.committedConfig(ctx, dir)
	if err != nil {
		return domain.Workflow{}, domain.ErrInvalid
	}
	key, err := r.Git.key(ctx, dir, cfg)
	if err != nil || key != project {
		return domain.Workflow{}, domain.ErrNotFound
	}
	kind, err := r.Git.one(ctx, dir, "cat-file", "-t", revision)
	if err != nil || kind != "commit" {
		return domain.Workflow{}, domain.ErrNotFound
	}
	root, err := r.Git.rootCommit(ctx, dir)
	if err != nil {
		return domain.Workflow{}, domain.ErrInvalid
	}
	roots, err := r.Git.one(ctx, dir, "rev-list", "--max-parents=0", revision)
	if err != nil || len(strings.Fields(roots)) == 0 || slices.Min(strings.Fields(roots)) != root {
		return domain.Workflow{}, domain.ErrNotFound
	}
	path := ".amaru/workflows/" + id + ".yaml"
	listed, err := r.Git.one(ctx, dir, "ls-tree", revision, "--", path)
	if err != nil || (!strings.HasPrefix(listed, "100644 blob ") && !strings.HasPrefix(listed, "100755 blob ")) {
		return domain.Workflow{}, domain.ErrNotFound
	}
	data, err := r.Git.run(ctx, dir, "cat-file", "blob", revision+":"+path)
	if err != nil {
		return domain.Workflow{}, domain.ErrInvalid
	}
	w, err := workflowdef.Parse(data)
	if err != nil || w.ID != id {
		return domain.Workflow{}, domain.ErrInvalid
	}
	return w, nil
}
