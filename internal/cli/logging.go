package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

type simpleSlogHandler struct {
	out   io.Writer
	level slog.Level
	mu    sync.Mutex
}

func (h *simpleSlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *simpleSlogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	var prefix string
	if !r.Time.IsZero() {
		prefix = r.Time.Format("2006-01-02 15:04:05") + " "
	}
	var attrs strings.Builder
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&attrs, " %s=%v", a.Key, a.Value.Any())
		return true
	})
	_, err := fmt.Fprintf(h.out, "%s[%s]: %s%s\n",
		prefix,
		strings.ToLower(r.Level.String()),
		strings.TrimRight(r.Message, "\n"),
		attrs.String(),
	)
	return err
}

// No-op: binary doesn't use slog.With(); attrs would be silently dropped.
func (h *simpleSlogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *simpleSlogHandler) WithGroup(_ string) slog.Handler      { return h }

func setupLogger() {
	level := slog.LevelInfo
	if verboseFlag {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(&simpleSlogHandler{out: os.Stderr, level: level}))
}
