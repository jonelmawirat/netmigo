package repository

import "time"

type ExecuteOptions struct {
    Timeout time.Duration
}

type ExecuteOption func(*ExecuteOptions)

func NewExecuteOptions(opts ...ExecuteOption) *ExecuteOptions {
    options := &ExecuteOptions{
        Timeout: 10 * time.Second,
    }

    for _, opt := range opts {
        opt(options)
    }

    return options
}

func WithTimeout(d time.Duration) ExecuteOption {
    return func(o *ExecuteOptions) {
        o.Timeout = d
    }
}
