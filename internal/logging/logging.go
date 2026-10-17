// Package logging builds the application slog logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"
)

// New returns a logger writing to w at the given level ("debug", "info",
// "warn", "error") in the given format ("json" or "text").
func New(w io.Writer, level, format string) (*slog.Logger, error) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("log level: %w", err)
	}
	opts := &slog.HandlerOptions{Level: lvl}
	switch format {
	case "json":
		return slog.New(slog.NewJSONHandler(w, opts)), nil
	case "text":
		return slog.New(slog.NewTextHandler(w, opts)), nil
	default:
		return nil, fmt.Errorf("log format %q must be json or text", format)
	}
}
