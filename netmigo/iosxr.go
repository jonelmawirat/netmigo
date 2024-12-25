package netmigo

import (
	"fmt"
	"sync"

	"github.com/jonelmawirat/netmigo/logger"
)

type Iosxr struct {
    BaseDevice
}

func (d *Iosxr) Connect(cfg *DeviceConfig) error {
    logger.Log.Info("Connecting to iOSXR device", "host", cfg.IP)
    return d.connectBase(cfg)
}

func (d *Iosxr) Execute(command string) (string, error) {
    logger.Log.Info("Executing command on iOSXR", "command", command)
    return d.interactiveExecute(command, 10) 
}

func (d *Iosxr) Download(remoteFilePath, localFilePath string) error {
    logger.Log.Info("Downloading file from iOSXR",
        "remotePath", remoteFilePath,
        "localPath", localFilePath,
    )
    return d.scpDownload(remoteFilePath, localFilePath)
}

func (d *Iosxr) Disconnect() {
    logger.Log.Info("Disconnecting iOSXR device")
    d.disconnectBase()
}



func (d *Iosxr) ExecuteMultiple(commands []string) ([]string, error) {
    if d.client == nil {
        return nil, fmt.Errorf("not connected")
    }
    logger.Log.Info("Executing multiple commands concurrently on iOSXR",
        "numCommands", len(commands),
    )

    var wg sync.WaitGroup
    results := make([]string, len(commands))
    errs := make([]error, len(commands))

    for i, cmd := range commands {
        wg.Add(1)
        go func(idx int, command string) {
            defer wg.Done()
            outFile, err := d.Execute(command)
            if err != nil {
                errs[idx] = err
                return
            }
            results[idx] = outFile
        }(i, cmd)
    }

    wg.Wait()

    
    for _, e := range errs {
        if e != nil {
            return results, fmt.Errorf("one or more commands failed: %v", errs)
        }
    }

    return results, nil
}

