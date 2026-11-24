package log

import (
	"io"
	"log/slog"
)

// NewSlogLogger returns a JSON `*slog.Logger` writing to w. It is used as echo v5's `Echo.Logger`
// (which is what `c.Logger()` returns in handlers), so echo/handler logs go to the same output as
// the bean logger. The message key is renamed to `message` to keep it consistent with the bean logger.
func NewSlogLogger(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.MessageKey {
				a.Key = "message"
			}
			return a
		},
	}))
}
