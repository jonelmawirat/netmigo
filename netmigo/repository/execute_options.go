package repository

import "time"

type ExecuteOptions struct {
    Timeout          time.Duration
    FirstByteTimeout time.Duration
}

type ExecuteOption func(*ExecuteOptions)

func NewExecuteOptions(opts ...ExecuteOption) *ExecuteOptions {
    options := &ExecuteOptions{
        Timeout:          10 * time.Second,
        FirstByteTimeout: 300 * time.Second,
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

func WithFirstByteTimeout(d time.Duration) ExecuteOption {
    return func(o *ExecuteOptions) {
        o.FirstByteTimeout = d
    }
}
