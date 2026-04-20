// Package logging exposes a default [*slog.Logger] used across the
// exporter. Migrated from logrus in LIM-1038 — logrus has been in
// maintenance mode since 2022 and stdlib log/slog is the Go 1.21+
// structured-logging default.
//
// Callers who want to inject a custom logger (different handler,
// different level, JSON output, etc.) should construct their own
// [*slog.Logger] and thread it through instead of relying on NewLogger.
package logging

import (
	"log/slog"
	"os"
)

// Default configuration for the package-level logger each sub-package
// constructs via NewLogger. Tuned for operator readability: source
// attribution, Info level, text handler. Callers that want JSON or a
// different level should construct their own *slog.Logger rather than
// mutating these.
const (
	defaultLevel = slog.LevelInfo
)

// NewLogger returns a *slog.Logger with source attribution enabled and
// the default level (Info). Stderr is the destination so metrics on
// stdout (if anyone ever wires a text-exposition endpoint) stays clean.
//
// Replaces the v0.x logrus-based Logger. Existing callers that used
// Infof/Warnf/Errorf should migrate to slog's structured API:
//
//	log.Info("something happened", "key", value)   // was: log.Infof
//	log.Warn("trouble", "err", err)                // was: log.Warnf
//	log.Error("broken", "err", err)                // was: log.Errorf
func NewLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
		Level:     defaultLevel,
	}))
}
