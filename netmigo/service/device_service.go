package service

import "github.com/jonelmawirat/netmigo/netmigo/config"

type DeviceService interface {
    Connect(cfg *config.DeviceConfig) error
    Execute(command string) (string, error)
	ExecuteMultiple(commands []string) ([]string, error)
    Download(remoteFilePath, localFilePath string) error
    Disconnect()
}
