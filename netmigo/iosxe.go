package netmigo


type Iosxe struct {
    BaseDevice
}

func (d *Iosxe) Connect(cfg *DeviceConfig) error {
    d.BaseDevice.logger.Info("Connecting to IOSXE device", "host", cfg.IP)
    return d.connectBase(cfg)
}

func (d *Iosxe) Execute(command string) (string, error) {
    d.BaseDevice.logger.Info("Executing command on IOSXE", "command", command)
    return d.interactiveExecute(command, 10) 
}

func (d *Iosxe) Download(remoteFilePath, localFilePath string) error {
    d.BaseDevice.logger.Info("Downloading file from IOSXE",
        "remotePath", remoteFilePath,
        "localPath", localFilePath,
    )
    return d.scpDownload(remoteFilePath, localFilePath)
}

func (d *Iosxe) Disconnect() {
    d.BaseDevice.logger.Info("Disconnecting IOSXE device")
    d.disconnectBase()
}

func (d *Iosxe) ExecuteMultiple(commands []string) ([]string, error) {
    d.BaseDevice.logger.Info("Executing multiple commands on iOSXR", "commandsCount", len(commands))
    return d.interactiveExecuteMultiple(commands, 2)
}
