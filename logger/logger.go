package logger

import (
	"github.com/DevloperAmanSingh/watcher/env"
	"github.com/lmittmann/tint"
	"log/slog"
	"os"
)

func New() *slog.Logger {
	if env.FetchString("APP_ENV") == "dev" {
		return slog.New(tint.NewHandler(os.Stdout, &tint.Options{
			Level:     slog.LevelDebug,
			AddSource: true,
		}))
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}
