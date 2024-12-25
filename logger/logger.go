package logger

import (
    "log/slog"
    "os"
)


var Log *slog.Logger

func init() {
    Log = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelDebug, 
    }))
}
