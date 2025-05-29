
package service

import (
    "errors"
    "log/slog"

    "golang.org/x/crypto/ssh"

    "github.com/jonelmawirat/netmigo/netmigo/config"
    "github.com/jonelmawirat/netmigo/netmigo/repository"
)

type LinuxDeviceService struct {
    repo   repository.SSHRepository
    logger *slog.Logger
    client *ssh.Client
    devCfg config.DeviceConfig
}

func NewLinuxDeviceService(repo repository.SSHRepository, logger *slog.Logger) *LinuxDeviceService {
    return &LinuxDeviceService{repo: repo, logger: logger}
}


func (s *LinuxDeviceService) Connect(cfg *config.DeviceConfig) error {
    s.logger.Info("Connecting to Linux device service", "host", cfg.IP)
    s.devCfg = *cfg
    client, err := s.repo.Connect(*cfg)
    if err != nil {
        return err
    }
    s.client = client
    return nil
}

func (s *LinuxDeviceService) Disconnect() {
    s.logger.Info("Disconnecting Linux device service")
    if s.client != nil {
        s.repo.Disconnect(s.client)
        s.client = nil
    }
}

func (s *LinuxDeviceService) Execute(command string) (string, error) {
    s.logger.Info("Executing command on Linux service", "command", command)
    if s.client == nil {
        return "", errors.New("not connected (LinuxDeviceService)")
    }
    return s.repo.InteractiveExecute(s.client, command, 10)
}

func (s *LinuxDeviceService) Download(remoteFilePath, localFilePath string) error {
    s.logger.Info("Downloading file from Linux service",
        "remotePath", remoteFilePath,
        "localPath", localFilePath,
    )
    if s.client == nil {
        return errors.New("not connected (LinuxDeviceService)")
    }
    return s.repo.ScpDownload(s.client, remoteFilePath, localFilePath)
}
