package service

import (
    "github.com/jonelmawirat/netmigo/netmigo/config"
    "github.com/jonelmawirat/netmigo/netmigo/repository"
)

type DeviceService interface {
    Connect(cfg *config.DeviceConfig) error
    Execute(command string, opts ...repository.ExecuteOption) (string, error)
    ExecuteMultiple(commands []string, opts ...repository.ExecuteOption) ([]string, error)
    Download(remoteFilePath, localFilePath string) error
    Disconnect()
}
