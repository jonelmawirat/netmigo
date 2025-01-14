package netmigo


type Iosxr struct {
    BaseDevice
}

func (d *Iosxr) Connect(cfg *DeviceConfig) error {
    d.BaseDevice.logger.Info("Connecting to iOSXR device", "host", cfg.IP)
    return d.connectBase(cfg)
}

func (d *Iosxr) Execute(command string) (string, error) {
    d.BaseDevice.logger.Info("Executing command on iOSXR", "command", command)
    return d.interactiveExecute(command, 30) 
}

func (d *Iosxr) Download(remoteFilePath, localFilePath string) error {
    d.BaseDevice.logger.Info("Downloading file from iOSXR",
        "remotePath", remoteFilePath,
        "localPath", localFilePath,
    )
    return d.scpDownload(remoteFilePath, localFilePath)
}

func (d *Iosxr) ExecuteMultiple(commands []string) ([]string, error) {
    d.BaseDevice.logger.Info("Executing multiple commands on iOSXR", "commandsCount", len(commands))
    return d.interactiveExecuteMultiple(commands, 30)
}

func (d *Iosxr) Disconnect() {
    d.BaseDevice.logger.Info("Disconnecting iOSXR device")
    d.disconnectBase()
}

