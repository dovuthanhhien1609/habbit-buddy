// Package logger provides a process-wide structured JSON logger built on log/slog.
// Import this package for its side-effect (init sets slog's default logger) or
// use L directly for explicit calls.
package logger

import (
	"log/slog"
	"os"
)

// L is the global structured logger. It writes JSON to stdout.
var L *slog.Logger

func init() {
	L = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(L)
}
