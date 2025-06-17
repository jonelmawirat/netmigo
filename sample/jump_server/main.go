package main

import (
    "fmt"
    "log"
    "log/slog"
    "time"

    "github.com/jonelmawirat/netmigo/logger"
    "github.com/jonelmawirat/netmigo/netmigo"
)

func main() {
    loggerConfig := logger.Config{
        Level:  slog.LevelDebug,
        Format: "json",
    }
    slogLogger := logger.NewLogger(loggerConfig)

    jumpServerCfg := netmigo.NewDeviceConfig(
        "10.10.10.1",
        netmigo.WithUsername("jumpserver_user"),
        netmigo.WithKeyPath("/path/to/jumpserver_key"),
        netmigo.WithConnectionTimeout(5*time.Second),
    )

    targetCfg := netmigo.NewDeviceConfig(
        "10.10.10.2",
        netmigo.WithUsername("admin"),
        netmigo.WithPassword("target_password"),
        netmigo.WithConnectionTimeout(5*time.Second),
        netmigo.WithJumpServer(jumpServerCfg),
    )

    device, err := netmigo.NewDevice(slogLogger, netmigo.CISCO_IOSXR)
    if err != nil {
        log.Fatalf("Failed to create device: %v", err)
    }

    if err := device.Connect(targetCfg); err != nil {
        log.Fatalf("Connect failed: %v", err)
    }
    defer device.Disconnect()

    outputFilePath, err := device.Execute("show logging")
    if err != nil {
        log.Fatalf("Command execution failed: %v", err)
    }

    fmt.Println("Captured logging output in:", outputFilePath)
}
