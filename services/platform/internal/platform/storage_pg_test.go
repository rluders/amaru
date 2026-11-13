//go:build pg

package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/rluders/amaru/services/platform/internal/domain"
	"github.com/rluders/amaru/services/platform/internal/store/migrations"
	"github.com/rluders/amaru/services/platform/internal/store/postgres"
)

var databaseSequence atomic.Int64

func TestPostgresStorageSchemaLifecycle(t *testing.T) {
	dsn := os.Getenv("AMARU_TEST_PG")
	if dsn == "" {
		t.Skip("AMARU_TEST_PG is not set")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid disposable PostgreSQL configuration")
	}
	admin := sql.OpenDB(stdlib.GetConnector(*config))
	defer admin.Close()
	name := fmt.Sprintf("amaru_lifecycle_%d_%d", os.Getpid(), databaseSequence.Add(1))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	identifier := pgx.Identifier{name}.Sanitize()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal("could not create disposable database")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(cleanup, "DROP DATABASE "+identifier+" WITH (FORCE)"); err != nil {
			t.Error("could not drop disposable database")
		}
	}()
	config.Database = name
	wantFailure := errors.New("later startup stage failed")
	project := domain.Project{Key: "id:01ARZ3NDEKTSV4RRFFQ69G5FAV", Name: "retained", CreatedAt: time.Now().UTC()}
	for round := 0; round < 3; round++ {
		s, err := NewStorage(stdlib.GetConnector(*config), time.Now().Add(15*time.Second))
		if err != nil {
			t.Fatal(err)
		}
		schema, _ := NewSchema(s)
		process, stop := context.WithCancel(ctx)
		reached := make(chan struct{})
		l := newLane()
		l.AddStage(s)
		l.AddStage(schema)
		l.AddStage(dependent{ready: func(ctx context.Context) error {
			db, err := s.DB(ctx)
			if err != nil {
				return err
			}
			dir, err := migrations.OpenDir(migrations.Embedded())
			if err != nil {
				return err
			}
			status, err := migrations.NewRunner(db, dir, time.Second).Status(ctx)
			if err != nil {
				return err
			}
			if !status.UpToDate() || status.Current != dir.Latest() {
				return errors.New("dependent admitted before all migrations completed")
			}
			store := postgres.NewProjectStore(db)
			if round == 0 {
				if err := store.Create(ctx, project); err != nil {
					return err
				}
			} else if got, err := store.Get(ctx, project.Key); err != nil || got.Name != project.Name {
				return errors.New("restart lost existing Project")
			}
			close(reached)
			if round == 1 {
				return wantFailure
			}
			return nil
		}, stop: func(ctx context.Context) error {
			db, err := s.DB(ctx)
			if err != nil {
				return err
			}
			// The database remains usable after all Start contexts are cancelled.
			return db.PingContext(ctx)
		}})
		done := make(chan error, 1)
		go func() { done <- l.Run(process) }()
		select {
		case <-reached:
		case <-done:
			stop()
			t.Fatal("PostgreSQL startup failed before dependent readiness")
		case <-ctx.Done():
			stop()
			t.Fatal("PostgreSQL lifecycle validation deadline exceeded")
		}
		if round != 1 {
			stop()
		}
		err = result(t, done)
		stop()
		if (round == 1 && !errors.Is(err, wantFailure)) || (round != 1 && err != nil) {
			t.Fatal("unexpected PostgreSQL lifecycle result")
		}
		if s.db.Stats().OpenConnections != 0 {
			t.Fatal("storage owner retained connections after Stop")
		}
		if err := admin.PingContext(ctx); err != nil {
			t.Fatal("operator-owned database server was stopped")
		}
	}
}
