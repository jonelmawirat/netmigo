package netmigo

import (
	"errors"
	"log/slog"
	"time"
)


type Device interface {
    Connect(*DeviceConfig) error
    Execute(string) (string, error)
    Download(string, string) error
    Disconnect()
}


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
    LINUX
)


func NewDevice(logger *slog.Logger, platform Platform) (Device, error) {
    switch platform {
    case CISCO_IOSXR:
        return &Iosxr{
            BaseDevice: BaseDevice{
                logger: logger,
            },
        }, nil
    case LINUX:
        return &Linux{
            BaseDevice: BaseDevice{
                logger: logger,
            },
        }, nil
    default:
        return nil, errors.New("unsupported platform")
    }
}

