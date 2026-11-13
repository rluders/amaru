// Package platform owns concrete process resources outside the Platform core.
package platform

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"time"

	"github.com/rluders/amaru/services/platform/internal/store/migrations"
)

// Storage owns only its connection pool, never the operator's PostgreSQL server.
// Construct a fresh owner for each process run. The composition root supplies a
// pgx stdlib connector and one absolute startup deadline shared by all stages.
type Storage struct {
	connector driver.Connector
	deadline  time.Time
	ready     chan struct{}
	initErr   error
	db        *sql.DB
	stopOnce  sync.Once
	stopped   chan struct{}
	stopErr   error
}

func NewStorage(connector driver.Connector, startupDeadline time.Time) (*Storage, error) {
	if connector == nil || startupDeadline.IsZero() {
		return nil, errors.New("storage requires a connector and startup deadline")
	}
	return &Storage{connector: connector, deadline: startupDeadline, ready: make(chan struct{}), stopped: make(chan struct{}), initErr: errors.New("storage initialization did not complete")}, nil
}

func (*Storage) Name() string { return "storage" }

func (s *Storage) initialize(ctx context.Context) error {
	defer close(s.ready) // also releases rollback after an initializer panic
	ctx, cancel := context.WithDeadline(ctx, s.deadline)
	defer cancel()
	s.db = sql.OpenDB(s.connector)
	s.initErr = s.db.PingContext(ctx)
	if s.initErr == nil {
		s.initErr = ctx.Err()
	}
	return s.initErr
}

func (s *Storage) Start(ctx context.Context) error {
	if err := s.initialize(ctx); err != nil {
		return err
	}
	<-ctx.Done()
	// Lane cancels every Start before Stop. Dependencies stay usable for joins.
	return nil
}

func (s *Storage) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		if err := ctx.Err(); err != nil {
			return err
		}
		return s.initErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DB is available only after connectivity succeeds. Dependents must join before
// Storage.Stop; a successful startup check is not a continuing health guarantee.
func (s *Storage) DB(ctx context.Context) (*sql.DB, error) {
	if err := s.WaitReady(ctx); err != nil {
		return nil, err
	}
	return s.db, nil
}

func (s *Storage) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() {
		go func() {
			defer close(s.stopped)
			defer func() {
				if recover() != nil {
					s.stopErr = errors.New("storage cleanup panicked")
				}
			}()
			<-s.ready
			if s.db != nil {
				s.stopErr = s.db.Close()
			}
		}()
	})
	select {
	case <-s.stopped:
		return s.stopErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Schema applies the existing embedded migrations once, after Storage is ready.
// It owns no pool; Stop joins Apply (including its advisory-lock release).
type Schema struct {
	storage *Storage
	ready   chan struct{}
	initErr error
}

func NewSchema(storage *Storage) (*Schema, error) {
	if storage == nil {
		return nil, errors.New("schema requires storage")
	}
	return &Schema{storage: storage, ready: make(chan struct{}), initErr: errors.New("schema initialization did not complete")}, nil
}

func (*Schema) Name() string { return "schema" }

func (s *Schema) Start(ctx context.Context) error {
	defer close(s.ready)
	ctx, cancel := context.WithDeadline(ctx, s.storage.deadline)
	defer cancel()
	db, err := s.storage.DB(ctx)
	if err == nil {
		var dir *migrations.Dir
		dir, err = migrations.OpenDir(migrations.Embedded())
		if err == nil {
			_, err = migrations.NewRunner(db, dir, 5*time.Second).Apply(ctx)
		}
	}
	if err == nil {
		err = ctx.Err()
	}
	s.initErr = err
	return err
}

func (s *Schema) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		if err := ctx.Err(); err != nil {
			return err
		}
		return s.initErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Schema) Stop(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
