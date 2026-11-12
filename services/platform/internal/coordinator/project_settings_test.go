package coordinator_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/ports/fakes"
)

func TestProjectSettingsAccess(t *testing.T) {
	ctx := context.Background()
	projects := fakes.NewProjectStore()
	a := domain.ProjectKey("ssh://github.com/acme/a#0123456789abcdef0123456789abcdef01234567")
	b := domain.ProjectKey("ssh://github.com/acme/b#0123456789abcdef0123456789abcdef01234567")
	foreign := domain.ProjectKey("ssh://github.com/acme/foreign#0123456789abcdef0123456789abcdef01234567")
	for _, key := range []domain.ProjectKey{a, b} {
		if err := projects.Create(ctx, domain.Project{Key: key, Name: "same", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	owner := domain.Principal{Kind: domain.PrincipalHuman, ID: "owner", Role: domain.RoleOwner}
	denied := false
	api, err := coordinator.NewProjectAccess(coordinator.New(fakes.NewExecutionStore(), time.Now), projects, func(context.Context) (domain.Principal, error) {
		if denied {
			return domain.Principal{}, domain.ErrNotPermitted
		}
		return owner, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	value := true
	patch := domain.UpdateSettings{ImportClaudeSkills: &value}
	for range 2 {
		got, err := api.UpdateSettings(ctx, a, patch, projects)
		if err != nil || !got.ImportClaudeSkills {
			t.Fatalf("update: %+v %v", got, err)
		}
	}
	for _, key := range []domain.ProjectKey{foreign, "invalid"} {
		if _, err := api.Settings(ctx, key, projects); err != domain.ErrNotFound {
			t.Fatalf("foreign read: %v", err)
		}
		if _, err := api.UpdateSettings(ctx, key, patch, projects); err != domain.ErrNotFound {
			t.Fatalf("foreign write: %v", err)
		}
	}
	if _, err := api.UpdateSettings(ctx, a, domain.UpdateSettings{}, projects); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("empty: %v", err)
	}
	denied = true
	value = false
	if _, err := api.UpdateSettings(ctx, a, patch, projects); err != domain.ErrNotPermitted {
		t.Fatalf("revoked write: %v", err)
	}
	if _, err := api.Settings(ctx, a, projects); err != domain.ErrNotPermitted {
		t.Fatalf("revoked read: %v", err)
	}
	denied = false
	owner.Role = domain.RoleContributor
	if _, err := api.UpdateSettings(ctx, a, patch, projects); err != domain.ErrNotPermitted {
		t.Fatalf("nonowner: %v", err)
	}
	first, err := projects.Settings(ctx, a)
	if err != nil || !first.ImportClaudeSkills {
		t.Fatalf("rejected mutation: %+v %v", first, err)
	}
	second, err := projects.Settings(ctx, b)
	if err != nil || second.ImportClaudeSkills {
		t.Fatalf("foreign mutation: %+v %v", second, err)
	}
}
