package service

import (
    "errors"
    "log/slog"

    "golang.org/x/crypto/ssh"

    "github.com/jonelmawirat/netmigo/netmigo/config"
    "github.com/jonelmawirat/netmigo/netmigo/repository"
)

type IosxrDeviceService struct {
    repo   repository.SSHRepository
    logger *slog.Logger
    client *ssh.Client
    devCfg config.DeviceConfig
}

func NewIosxrDeviceService(repo repository.SSHRepository, logger *slog.Logger) *IosxrDeviceService {
    return &IosxrDeviceService{repo: repo, logger: logger}
}

func (s *IosxrDeviceService) Connect(cfg *config.DeviceConfig) error {
    s.logger.Info("Connecting to iOSXR device service", "host", cfg.IP)
    s.devCfg = *cfg
    client, err := s.repo.Connect(*cfg)
    if err != nil {
        return err
    }
    s.client = client
    return nil
}

func (s *IosxrDeviceService) Disconnect() {
    s.logger.Info("Disconnecting iOSXR device service")
    if s.client != nil {
        s.repo.Disconnect(s.client)
        s.client = nil
    }
}

func (s *IosxrDeviceService) Execute(command string, opts ...repository.ExecuteOption) (string, error) {
    s.logger.Info("Executing command on iOSXR service", "command", command)
    if s.client == nil {
        return "", errors.New("not connected (IosxrDeviceService)")
    }
    return s.repo.InteractiveExecute(s.client, command, opts...)
}

func (s *IosxrDeviceService) Download(remoteFilePath, localFilePath string) error {
    s.logger.Info("Downloading file from iOSXR service",
        "remotePath", remoteFilePath,
        "localPath", localFilePath,
    )
    if s.client == nil {
        return errors.New("not connected (IosxrDeviceService)")
    }
    return s.repo.ScpDownload(s.client, remoteFilePath, localFilePath)
}

func (s *IosxrDeviceService) ExecuteMultiple(commands []string, opts ...repository.ExecuteOption) ([]string, error) {
    s.logger.Info("Executing multiple commands on iOSXR service", "commandsCount", len(commands))
    if s.client == nil {
        return nil, errors.New("not connected (IosxrDeviceService ExecuteMultiple)")
    }
    return s.repo.InteractiveExecuteMultiple(s.client, commands, opts...)
}
