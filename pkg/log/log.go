package log

import (
	"context"
	"io"
	"log/slog"
)

// MultiStreamHandler slog handler for directing different
type MultiStreamHandler struct {
	level       slog.Level
	lowHandler  slog.Handler
	highHandler slog.Handler
}

// New returns new MultiStreamHandler
func New(outLow, outHigh io.Writer, format string, opts *slog.HandlerOptions) *MultiStreamHandler {
	if format == "json" {
		return &MultiStreamHandler{
			level:       opts.Level.Level(),
			lowHandler:  slog.NewJSONHandler(outLow, opts),
			highHandler: slog.NewJSONHandler(outHigh, opts),
		}
	}
	return &MultiStreamHandler{
		level:       opts.Level.Level(),
		lowHandler:  slog.NewTextHandler(outLow, opts),
		highHandler: slog.NewTextHandler(outHigh, opts),
	}
}

// Enabled check if log level is enabled
func (m *MultiStreamHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= m.level.Level()
}

// Handle pass to Low or High handlers based on log level
func (m *MultiStreamHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level <= slog.LevelInfo.Level() {
		return m.lowHandler.Handle(ctx, r)
	}
	return m.highHandler.Handle(ctx, r)
}

func (m *MultiStreamHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Not used within the code
	panic("Not implemented")
}

func (m *MultiStreamHandler) WithGroup(string) slog.Handler {
	// Not used within the code
	panic("Not implemented")
}
