package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// log is the application-wide structured logger.
// Initialized by initLogger(); safe to use from any goroutine.
var logger *slog.Logger

// initLogger opens (or creates) the log file and configures a JSON logger that
// writes to both the file and stderr. Returns a closer for the log file.
func initLogger() (close func(), err error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		// Fall back to stderr-only logging.
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
		return func() {}, nil
	}

	logDir := filepath.Join(configDir, "openwhisper")
	if err := os.MkdirAll(logDir, 0700); err != nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
		return func() {}, nil
	}

	logPath := filepath.Join(logDir, "openwhisper.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
		return func() {}, nil
	}

	w := io.MultiWriter(f, os.Stderr)
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	logger = slog.New(slog.NewJSONHandler(w, opts))
	slog.SetDefault(logger)

	logger.Info("logger initialized", "path", logPath)
	return func() { f.Close() }, nil
}
