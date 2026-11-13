package operational

import (
	"context"
	"log/slog"
)

// LifecycleHandler drops messages, source locations and attributes without
// resolving their values. Only fixed severity labels reach the bounded queue.
// The composition owner drains it through Logger.Write with trusted identity;
// slow or failed sinks must never block Lane callbacks. Keep the queue open
// until all lifecycle producers have stopped. A nil/full queue drops records.
func LifecycleHandler(queue chan<- Record) slog.Handler {
	return lifecycleHandler{queue: queue}
}

type lifecycleHandler struct{ queue chan<- Record }

func (lifecycleHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h lifecycleHandler) Handle(_ context.Context, r slog.Record) error {
	outcome := "debug"
	switch {
	case r.Level >= slog.LevelError:
		outcome = "error"
	case r.Level >= slog.LevelWarn:
		outcome = "warning"
	case r.Level >= slog.LevelInfo:
		outcome = "info"
	}
	select {
	case h.queue <- Record{Operation: "process.lifecycle", Outcome: outcome}:
	default:
	}
	return nil
}

func (h lifecycleHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h lifecycleHandler) WithGroup(string) slog.Handler      { return h }
