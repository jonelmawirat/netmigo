package config

import "time"

type DeviceConfig struct {
    IP                string
    Username          string
    Password          string
    KeyPath           string
    Port              string
    JumpServer        *DeviceConfig
    MaxRetry          int
    ConnectionTimeout time.Duration
}

type DeviceConfigOption func(*DeviceConfig)

func NewDeviceConfig(ip string, opts ...DeviceConfigOption) *DeviceConfig {
    cfg := &DeviceConfig{
        IP:                ip,
        Port:              "22",
        MaxRetry:          3,
        ConnectionTimeout: 10 * time.Second,
    }

    for _, opt := range opts {
        opt(cfg)
    }

    return cfg
}

func WithUsername(username string) DeviceConfigOption {
    return func(c *DeviceConfig) {
        c.Username = username
    }
}

func WithPassword(password string) DeviceConfigOption {
    return func(c *DeviceConfig) {
        c.Password = password
    }
}

func WithKeyPath(keyPath string) DeviceConfigOption {
    return func(c *DeviceConfig) {
        c.KeyPath = keyPath
    }
}

func WithPort(port string) DeviceConfigOption {
    return func(c *DeviceConfig) {
        c.Port = port
    }
}

func WithJumpServer(jumpServer *DeviceConfig) DeviceConfigOption {
    return func(c *DeviceConfig) {
        c.JumpServer = jumpServer
    }
}

func WithMaxRetry(retries int) DeviceConfigOption {
    return func(c *DeviceConfig) {
        c.MaxRetry = retries
    }
}

func WithConnectionTimeout(timeout time.Duration) DeviceConfigOption {
    return func(c *DeviceConfig) {
        c.ConnectionTimeout = timeout
    }
}

type Platform int

const (
    CISCO_IOSXR Platform = iota
    CISCO_IOSXE
    CISCO_NXOS
    LINUX
)
