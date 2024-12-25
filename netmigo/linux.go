package netmigo

import (
    "github.com/jonelmawirat/netmigo/logger"
)

type Linux struct {
    BaseDevice
}

func (d *Linux) Connect(cfg *DeviceConfig) error {
    logger.Log.Info("Connecting to Linux device", "host", cfg.IP)
    return d.connectBase(cfg)
}

func (d *Linux) Execute(command string) (string, error) {
    logger.Log.Info("Executing command on Linux", "command", command)
    return d.interactiveExecute(command, 10)
}

func (d *Linux) Download(remoteFilePath, localFilePath string) error {
    logger.Log.Info("Downloading file from Linux",
        "remotePath", remoteFilePath,
        "localPath", localFilePath,
    )
    return d.scpDownload(remoteFilePath, localFilePath)
}

func (d *Linux) Disconnect() {
    logger.Log.Info("Disconnecting Linux device")
    d.disconnectBase()
}


