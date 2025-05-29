package netmigo

import (
    "log/slog"

    "github.com/jonelmawirat/netmigo/netmigo/config"
    "github.com/jonelmawirat/netmigo/netmigo/factory"
    "github.com/jonelmawirat/netmigo/netmigo/service"
)

type DeviceConfig = config.DeviceConfig

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
