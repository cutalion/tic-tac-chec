package observability

import (
	"context"
	"errors"
	"log/slog"
)

// fanout dispatches each log record to every underlying handler. Record.Clone
// is used so each handler receives an independent copy (slog's contract).
type fanout struct {
	handlers []slog.Handler
}

func newFanout(handlers ...slog.Handler) slog.Handler {
	return &fanout{handlers: handlers}
}

func (f *fanout) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *fanout) Handle(ctx context.Context, r slog.Record) error {
	var err error
	for _, h := range f.handlers {
		err = errors.Join(err, h.Handle(ctx, r.Clone()))
	}
	return err
}

func (f *fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		out[i] = h.WithAttrs(attrs)
	}
	return &fanout{handlers: out}
}

func (f *fanout) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		out[i] = h.WithGroup(name)
	}
	return &fanout{handlers: out}
}
