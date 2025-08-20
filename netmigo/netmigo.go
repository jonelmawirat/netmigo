package netmigo

import (
    "log/slog"
    "time"

    "github.com/jonelmawirat/netmigo/netmigo/config"
    "github.com/jonelmawirat/netmigo/netmigo/factory"
    "github.com/jonelmawirat/netmigo/netmigo/repository"
    "github.com/jonelmawirat/netmigo/netmigo/service"
)

type DeviceConfig = config.DeviceConfig
type DeviceConfigOption = config.DeviceConfigOption

var (
    NewDeviceConfig       = config.NewDeviceConfig
    WithUsername          = config.WithUsername
    WithPassword          = config.WithPassword
    WithKeyPath           = config.WithKeyPath
    WithPort              = config.WithPort
    WithJumpServer        = config.WithJumpServer
    WithMaxRetry          = config.WithMaxRetry
    WithConnectionTimeout = config.WithConnectionTimeout
)

type ExecuteOption = repository.ExecuteOption

func WithTimeout(d time.Duration) ExecuteOption {
    return repository.WithTimeout(d)
}

func WithFirstByteTimeout(d time.Duration) ExecuteOption {
    return repository.WithFirstByteTimeout(d)
}

const (
    CISCO_IOSXR = config.CISCO_IOSXR
    CISCO_IOSXE = config.CISCO_IOSXE
    CISCO_NXOS  = config.CISCO_NXOS
    LINUX       = config.LINUX
)

type Device = service.DeviceService

type Iosxr = service.IosxrDeviceService
type Linux = service.LinuxDeviceService

func NewDevice(logger *slog.Logger, platform config.Platform) (Device, error) {
    return factory.NewDevice(logger, platform)
}
