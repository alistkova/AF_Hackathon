package log

import (
	"log/slog"
	"os"
	"strings"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
)

func New(service string) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(config.String("LOG_LEVEL", "info"), "debug") {
		level = slog.LevelDebug
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler).With("service", service)
}
