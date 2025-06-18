package repository

import (
    "log/slog"

    "golang.org/x/crypto/ssh"

    "github.com/jonelmawirat/netmigo/netmigo/config"
)

type SSHRepository interface {
    Connect(cfg config.DeviceConfig) (*ssh.Client, error)
    Disconnect(client *ssh.Client, jumpCfg *config.DeviceConfig)
    InteractiveExecute(client *ssh.Client, command string, opts ...ExecuteOption) (string, error)
    InteractiveExecuteMultiple(client *ssh.Client, commands []string, opts ...ExecuteOption) ([]string, error)
    ScpDownload(client *ssh.Client, remoteFilePath, localFilePath string) error
}

type sshRepositoryImpl struct {
    logger *slog.Logger
}

func NewSSHRepository(logger *slog.Logger) SSHRepository {
    return &sshRepositoryImpl{logger: logger}
}

func (r *sshRepositoryImpl) Connect(cfg config.DeviceConfig) (*ssh.Client, error) {
    return connectToTarget(cfg)
}

func (r *sshRepositoryImpl) Disconnect(client *ssh.Client, jumpCfg *config.DeviceConfig) {
    if client != nil {
        r.logger.Info("Closing SSH connection to target device")
        client.Close()
    }
    if jumpCfg != nil {
        r.logger.Info("Releasing jump server client", "jumpserver", jumpCfg.IP)
        ReleaseJumpClient(jumpCfg)
    }
}

func (r *sshRepositoryImpl) InteractiveExecute(client *ssh.Client, command string, opts ...ExecuteOption) (string, error) {
    options := NewExecuteOptions(opts...)
    timeoutSeconds := int(options.Timeout.Seconds())
    return ExecutorInteractiveExecute(client, r.logger, command, timeoutSeconds)
}

func (r *sshRepositoryImpl) InteractiveExecuteMultiple(client *ssh.Client, commands []string, opts ...ExecuteOption) ([]string, error) {
    options := NewExecuteOptions(opts...)
    timeoutSeconds := int(options.Timeout.Seconds())
    return ExecutorInteractiveExecuteMultiple(client, r.logger, commands, timeoutSeconds)
}

func (r *sshRepositoryImpl) ScpDownload(client *ssh.Client, remoteFilePath, localFilePath string) error {
    return ExecutorScpDownload(client, r.logger, remoteFilePath, localFilePath)
}
