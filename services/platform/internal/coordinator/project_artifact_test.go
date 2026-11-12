package coordinator_test

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

type artifactIndexProbe struct {
	ports.ArtifactIndex
	read func(context.Context, string) ([]domain.ArtifactVersion, error)
}

func (p artifactIndexProbe) Versions(ctx context.Context, id string) ([]domain.ArtifactVersion, error) {
	return p.read(ctx, id)
}

type artifactBlobProbe struct {
	ports.BlobStore
	open func(context.Context, string) (io.ReadCloser, error)
}

func (p artifactBlobProbe) Open(ctx context.Context, hash string) (io.ReadCloser, error) {
	return p.open(ctx, hash)
}

type closeProbe struct {
	io.Reader
	closed bool
}

func (p *closeProbe) Close() error { p.closed = true; return nil }

func TestProjectArtifactAccess(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	a := domain.ProjectKey("ssh://example.com/a#" + strings.Repeat("a", 40))
	b := domain.ProjectKey("ssh://example.com/b#" + strings.Repeat("a", 40))
	projects, executions := fakes.NewProjectStore(), fakes.NewExecutionStore()
	index, blobs := fakes.NewArtifactIndex(), fakes.NewBlobStore()
	for id, project := range map[string]domain.ProjectKey{"exec-a": a, "exec-b": b} {
		if err := projects.Create(ctx, domain.Project{Key: project, Name: "same-name", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if _, _, err := executions.Create(ctx, domain.Execution{ID: id, ProjectID: string(project), State: domain.ExecQueued, CreatedAt: now}, nil); err != nil {
			t.Fatal(err)
		}
	}
	var revoked atomic.Bool
	api, err := coordinator.NewProjectAccess(coordinator.New(executions, func() time.Time { return now }), projects, func(context.Context) (domain.Principal, error) {
		if revoked.Load() {
			return domain.Principal{}, domain.ErrNotPermitted
		}
		return domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	record := func(id, execution, body string) domain.ArtifactVersion {
		t.Helper()
		hash, size, err := blobs.Put(ctx, strings.NewReader(body), 100)
		if err != nil {
			t.Fatal(err)
		}
		v, err := index.Record(ctx, domain.ArtifactVersion{ArtifactID: id, ExecutionID: execution, StepID: execution + ".step", AttemptID: body, Hash: hash, Size: size, Path: "out.txt", CreatedAt: now})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	first := record("report", "exec-a", "a-one")
	foreign := record("report", "exec-b", "b-two")
	third := record("report", "exec-a", "a-three")
	record("foreign-only", "exec-b", "private")
	record("orphan", "missing-execution", "orphan")
	before, _ := index.Versions(ctx, "report")

	t.Run("same Artifact ID retains only owned versions and content", func(t *testing.T) {
		for _, tc := range []struct {
			project domain.ProjectKey
			want    []domain.ArtifactVersion
		}{{a, []domain.ArtifactVersion{first, third}}, {b, []domain.ArtifactVersion{foreign}}} {
			got, err := api.ArtifactVersions(ctx, tc.project, "report", index)
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("versions: %+v %v", got, err)
			}
			for _, v := range got {
				r, err := api.ArtifactContent(ctx, tc.project, "report", v.Version, index, blobs)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(r)
				if closeErr := r.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
				if err != nil || domain.ContentHash(body) != v.Hash {
					t.Fatalf("content: %q %v", body, err)
				}
			}
		}
	})
	t.Run("foreign missing orphan and unknown version open no blob", func(t *testing.T) {
		for _, id := range []string{"foreign-only", "missing", "orphan"} {
			got, err := api.ArtifactVersions(ctx, a, id, index)
			if err != domain.ErrNotFound || got != nil {
				t.Fatalf("%s: %+v %v", id, got, err)
			}
		}
		for _, version := range []int64{foreign.Version, 999, 0, -1} {
			r, err := api.ArtifactContent(ctx, a, "report", version, index, nil)
			if err != domain.ErrNotFound || r != nil {
				t.Fatalf("v%d: %v %v", version, r, err)
			}
		}
	})

	t.Run("authorization precedes index IO", func(t *testing.T) {
		unread := artifactIndexProbe{read: func(context.Context, string) ([]domain.ArtifactVersion, error) {
			t.Fatal("unauthorized index read")
			return nil, nil
		}}
		if _, err := api.ArtifactVersions(ctx, domain.ProjectKey(""), "report", unread); err != domain.ErrNotFound {
			t.Fatal(err)
		}
		revoked.Store(true)
		defer revoked.Store(false)
		if r, err := api.ArtifactContent(ctx, a, "report", first.Version, unread, nil); err != domain.ErrNotPermitted || r != nil {
			t.Fatalf("%v %v", r, err)
		}
	})

	t.Run("revocation during index read withholds metadata", func(t *testing.T) {
		defer revoked.Store(false)
		probe := artifactIndexProbe{read: func(ctx context.Context, id string) ([]domain.ArtifactVersion, error) {
			revoked.Store(true)
			return index.Versions(ctx, id)
		}}
		got, err := api.ArtifactVersions(ctx, a, "report", probe)
		if err != domain.ErrNotPermitted || got != nil {
			t.Fatalf("%+v %v", got, err)
		}
	})

	t.Run("revocation during blob open closes unread content", func(t *testing.T) {
		defer revoked.Store(false)
		reader := &closeProbe{Reader: strings.NewReader("private")}
		probe := artifactBlobProbe{open: func(_ context.Context, hash string) (io.ReadCloser, error) {
			if hash != first.Hash {
				t.Fatal("wrong content hash")
			}
			revoked.Store(true)
			return reader, nil
		}}
		r, err := api.ArtifactContent(ctx, a, "report", first.Version, index, probe)
		if err != domain.ErrNotPermitted || r != nil || !reader.closed {
			t.Fatalf("%v %v closed=%v", r, err, reader.closed)
		}
	})

	t.Run("storage failures return no partial result", func(t *testing.T) {
		failed := errors.New("storage unavailable")
		probe := artifactIndexProbe{read: func(context.Context, string) ([]domain.ArtifactVersion, error) { return before, failed }}
		if got, err := api.ArtifactVersions(ctx, a, "report", probe); err != failed || got != nil {
			t.Fatalf("%+v %v", got, err)
		}
		blob := artifactBlobProbe{open: func(context.Context, string) (io.ReadCloser, error) { return nil, failed }}
		if r, err := api.ArtifactContent(ctx, a, "report", first.Version, index, blob); err != failed || r != nil {
			t.Fatalf("%v %v", r, err)
		}
	})

	t.Run("invalid trusted dependencies and selectors", func(t *testing.T) {
		if got, err := api.ArtifactVersions(ctx, a, "report", nil); err != domain.ErrInvalid || got != nil {
			t.Fatalf("%+v %v", got, err)
		}
		if got, err := api.ArtifactVersions(ctx, a, "", index); err != domain.ErrInvalid || got != nil {
			t.Fatalf("%+v %v", got, err)
		}
		if r, err := api.ArtifactContent(ctx, a, "report", first.Version, index, nil); err != domain.ErrInvalid || r != nil {
			t.Fatalf("%v %v", r, err)
		}
	})
	after, _ := index.Versions(ctx, "report")
	if !reflect.DeepEqual(before, after) {
		t.Fatal("reads changed Artifact history")
	}
}
