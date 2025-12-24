package taskmaster

import (
	"log/slog"
	"os"
)

// SetLogger creates a log file from [Log] and sets as a default logger by using [pkg/log/slog.NewJSONHandler].
//
// This function should be called at the beginning of execution, so almost all activity is written in logfile.
func SetLogger(log Log) error {
	file, err := os.OpenFile(log.Path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		slog.Error("SetLogger", "error", err.Error())
		return err
	}

	logger := slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{Level: log.Level}))
	slog.SetDefault(logger)

	return nil
}
