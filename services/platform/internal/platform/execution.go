package platform

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/ports"
	"github.com/rluders/amaru/services/platform/internal/worker/embedded"
	"github.com/rluders/amaru/services/platform/internal/worker/inproc"
)

// Session owns the embedded protocol link and the recovered Coordinator's loop.
// Register after recovery/background stages, before its EmbeddedWorker owner.
type Session struct {
	recovery *Recovery
	workerID string
	platform ports.PlatformLink
	worker   ports.WorkerLink
	ready    chan struct{}
	done     chan struct{}
	once     sync.Once
	initErr  error
}

func NewSession(recovery *Recovery, workerID string) (*Session, error) {
	if recovery == nil || strings.TrimSpace(workerID) == "" {
		return nil, errors.New("session requires recovery and a Worker identity")
	}
	return &Session{recovery: recovery, workerID: workerID, ready: make(chan struct{}), done: make(chan struct{})}, nil
}

func (*Session) Name() string { return "coordinator-session" }

func (s *Session) publish(err error) {
	s.once.Do(func() { s.initErr = err; close(s.ready) })
}

func (s *Session) Start(ctx context.Context) error {
	defer close(s.done)
	defer s.publish(errors.New("session initialization did not complete"))
	c, err := s.recovery.Coordinator(ctx)
	if err != nil {
		s.publish(err)
		return err
	}
	s.platform, s.worker = inproc.Pair()
	err = c.Serve(ctx, sessionLink{PlatformLink: s.platform, owner: s}, s.workerID)
	if err == nil && ctx.Err() == nil {
		return errors.New("Coordinator session exited unexpectedly")
	}
	return err
}

func (s *Session) WaitReady(ctx context.Context) error {
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

func (s *Session) Stop(ctx context.Context) error {
	select {
	case <-s.done:
		if s.platform != nil {
			return s.platform.Close()
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type sessionLink struct {
	ports.PlatformLink
	owner *Session
}

func (l sessionLink) Receive(ctx context.Context) (ports.WorkerMessage, error) {
	// The transport exists and the receive pump has entered, without an Offer.
	l.owner.publish(ctx.Err())
	return l.PlatformLink.Receive(ctx)
}

// EmbeddedWorker owns cancel-and-join of the existing Worker.Run, not Step state.
type EmbeddedWorker struct {
	session *Session
	worker  embedded.Worker
	ready   chan struct{}
	done    chan struct{}
	once    sync.Once
	initErr error
}

func NewEmbeddedWorker(session *Session, worker embedded.Worker) (*EmbeddedWorker, error) {
	if session == nil || worker.Runner == nil {
		return nil, errors.New("embedded Worker requires session and Runner")
	}
	return &EmbeddedWorker{session: session, worker: worker, ready: make(chan struct{}), done: make(chan struct{})}, nil
}

func (*EmbeddedWorker) Name() string { return "embedded-worker" }

func (w *EmbeddedWorker) publish(err error) {
	w.once.Do(func() { w.initErr = err; close(w.ready) })
}

func (w *EmbeddedWorker) Start(ctx context.Context) error {
	defer close(w.done)
	defer w.publish(errors.New("embedded Worker initialization did not complete"))
	if err := w.session.WaitReady(ctx); err != nil {
		w.publish(err)
		return err
	}
	err := w.worker.Run(ctx, workerLink{WorkerLink: w.session.worker, owner: w})
	if err == nil && ctx.Err() == nil {
		return errors.New("embedded Worker exited unexpectedly")
	}
	return err
}

func (w *EmbeddedWorker) WaitReady(ctx context.Context) error {
	select {
	case <-w.ready:
		if err := ctx.Err(); err != nil {
			return err
		}
		return w.initErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *EmbeddedWorker) Stop(ctx context.Context) error {
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type workerLink struct {
	ports.WorkerLink
	owner *EmbeddedWorker
}

func (l workerLink) Send(ctx context.Context, message ports.WorkerMessage) error {
	err := l.WorkerLink.Send(ctx, message)
	// Worker.Run's first send is its initial Offer, not an assignment/result.
	if _, ok := message.(ports.Offer); ok {
		if err == nil {
			err = ctx.Err()
		}
		l.owner.publish(err)
	}
	return err
}
