package taskmaster

import (
	"log/slog"
	"os"
)

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
