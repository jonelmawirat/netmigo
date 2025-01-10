package netmigo


type Nxos struct {
    BaseDevice
}

func (d *Nxos) Connect(cfg *DeviceConfig) error {
    d.BaseDevice.logger.Info("Connecting to NXOS device", "host", cfg.IP)
    return d.connectBase(cfg)
}

func (d *Nxos) Execute(command string) (string, error) {
    d.BaseDevice.logger.Info("Executing command on NXOS", "command", command)
    return d.interactiveExecute(command, 10) 
}

func (d *Nxos) Download(remoteFilePath, localFilePath string) error {
    d.BaseDevice.logger.Info("Downloading file from NXOS",
        "remotePath", remoteFilePath,
        "localPath", localFilePath,
    )
    return d.scpDownload(remoteFilePath, localFilePath)
}

func (d *Nxos) Disconnect() {
    d.BaseDevice.logger.Info("Disconnecting NXOS device")
    d.disconnectBase()
}

func (d *Nxos) ExecuteMultiple(commands []string) ([]string, error) {
    d.BaseDevice.logger.Info("Executing multiple commands on NXOS", "commandsCount", len(commands))
    return d.interactiveExecuteMultiple(commands, 2)
}
