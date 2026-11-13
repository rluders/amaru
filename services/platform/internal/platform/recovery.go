package platform

import (
	"context"
	"errors"
	"time"

	"github.com/rluders/amaru/services/platform/internal/coordinator"
	"github.com/rluders/amaru/services/platform/internal/store/postgres"
)

// Recovery constructs the shared Coordinator after storage acquisition and runs
// its existing restart reconciliation before execution or public admission.
// Register after Schema and messaging; construct a fresh owner per process run.
type Recovery struct {
	storage     *Storage
	options     []coordinator.Option
	ready       chan struct{}
	initErr     error
	coordinator *coordinator.Coordinator
}

func NewRecovery(storage *Storage, options ...coordinator.Option) (*Recovery, error) {
	if storage == nil {
		return nil, errors.New("recovery requires storage")
	}
	return &Recovery{storage: storage, options: append([]coordinator.Option(nil), options...), ready: make(chan struct{}), initErr: errors.New("recovery initialization did not complete")}, nil
}

func (*Recovery) Name() string { return "recovery" }

func (r *Recovery) Start(ctx context.Context) error {
	defer close(r.ready)
	ctx, cancel := context.WithDeadline(ctx, r.storage.deadline)
	defer cancel()
	db, err := r.storage.DB(ctx)
	if err == nil {
		r.coordinator = coordinator.New(postgres.NewExecutionStore(db), time.Now, r.options...)
		_, err = r.coordinator.Reconcile(ctx)
	}
	if err == nil {
		err = ctx.Err()
	}
	r.initErr = err
	return err
}

func (r *Recovery) WaitReady(ctx context.Context) error {
	select {
	case <-r.ready:
		if err := ctx.Err(); err != nil {
			return err
		}
		return r.initErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Coordinator withholds the instance until reconciliation completes successfully.
// Downstream owners must use this same instance and join before Storage.Stop.
func (r *Recovery) Coordinator(ctx context.Context) (*coordinator.Coordinator, error) {
	if err := r.WaitReady(ctx); err != nil {
		return nil, err
	}
	return r.coordinator, nil
}

// Stop joins reconciliation; it never retries it or undoes committed state.
func (r *Recovery) Stop(ctx context.Context) error {
	select {
	case <-r.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
