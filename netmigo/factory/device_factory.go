package factory

import (
    "errors"
    "log/slog"

    "github.com/jonelmawirat/netmigo/netmigo/config"
    "github.com/jonelmawirat/netmigo/netmigo/repository"
    "github.com/jonelmawirat/netmigo/netmigo/service"
)

func NewDevice(logger *slog.Logger, platform config.Platform) (service.DeviceService, error) {
    repo := repository.NewSSHRepository(logger)

    switch platform {
    case config.CISCO_IOSXR:
        return service.NewIosxrDeviceService(repo, logger), nil
    case config.LINUX:
        return service.NewLinuxDeviceService(repo, logger), nil
    default:
        return nil, errors.New("unsupported platform in factory")
    }
}
