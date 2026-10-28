// Package inproc is the in-process transport between the coordinator and the embedded Worker
// (ADR-0029: "the embedded Worker in compact installs uses the same messages through an
// in-process transport"). Pair returns the two ends of one ordered channel; nothing else is
// shared between them.
package inproc

import (
	"context"
	"sync"

	"github.com/rluders/amaru/services/platform/internal/ports"
)

// buffer is how many messages may be in flight per direction before Send blocks.
const buffer = 64

type shared struct {
	done chan struct{}
	once sync.Once
}

type end[Out, In any] struct {
	out chan Out
	in  chan In
	sh  *shared
}

// Pair returns the Platform's end and the Worker's end of a new link.
func Pair() (ports.PlatformLink, ports.WorkerLink) {
	toWorker := make(chan ports.PlatformMessage, buffer)
	toPlatform := make(chan ports.WorkerMessage, buffer)
	sh := &shared{done: make(chan struct{})}
	return &end[ports.PlatformMessage, ports.WorkerMessage]{out: toWorker, in: toPlatform, sh: sh},
		&end[ports.WorkerMessage, ports.PlatformMessage]{out: toPlatform, in: toWorker, sh: sh}
}

func (e *end[Out, In]) Send(ctx context.Context, m Out) error {
	select {
	case <-e.sh.done:
		return ports.ErrLinkClosed
	default:
	}
	select {
	case e.out <- m:
		return nil
	case <-e.sh.done:
		return ports.ErrLinkClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *end[Out, In]) Receive(ctx context.Context) (In, error) {
	var zero In
	// Messages already sent are delivered before the close is reported.
	select {
	case m := <-e.in:
		return m, nil
	default:
	}
	select {
	case m := <-e.in:
		return m, nil
	case <-e.sh.done:
		select {
		case m := <-e.in:
			return m, nil
		default:
			return zero, ports.ErrLinkClosed
		}
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func (e *end[Out, In]) Close() error {
	e.sh.once.Do(func() { close(e.sh.done) })
	return nil
}
