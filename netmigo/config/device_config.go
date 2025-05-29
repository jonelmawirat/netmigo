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

type Platform int

const (
    CISCO_IOSXR Platform = iota
    CISCO_IOSXE
    CISCO_NXOS
    LINUX
)
