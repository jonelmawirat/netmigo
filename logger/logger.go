package logger

import (
    "log/slog"
    "os"
)

type Config struct {
    Level slog.Level
    Format string
}


func NewLogger(cfg Config) *slog.Logger {
    var handler slog.Handler
    switch cfg.Format {
    case "json":
        handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
            Level: cfg.Level,
        })
    default:
        handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
            Level: cfg.Level,
        })
    }
    return slog.New(handler)
}


