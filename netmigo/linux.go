package netmigo


type Linux struct {
    BaseDevice
}

func (d *Linux) Connect(cfg *DeviceConfig) error {
    d.BaseDevice.logger.Info("Connecting to Linux device", "host", cfg.IP)
    return d.connectBase(cfg)
}

func (d *Linux) Execute(command string) (string, error) {
    d.BaseDevice.logger.Info("Executing command on Linux", "command", command)
    return d.interactiveExecute(command, 10)
}

func (d *Linux) Download(remoteFilePath, localFilePath string) error {
    d.BaseDevice.logger.Info("Downloading file from Linux",
        "remotePath", remoteFilePath,
        "localPath", localFilePath,
    )
    return d.scpDownload(remoteFilePath, localFilePath)
}

func (d *Linux) Disconnect() {
    d.BaseDevice.logger.Info("Disconnecting Linux device")
    d.disconnectBase()
}


