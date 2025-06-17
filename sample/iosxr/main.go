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
    logger := logger.NewLogger(loggerConfig)

    iosxrCfg := netmigo.NewDeviceConfig(
        "sandbox-iosxr-1.cisco.com",
        netmigo.WithUsername("admin"),
        netmigo.WithPassword("C1sco12345"),
        netmigo.WithConnectionTimeout(15*time.Second),
    )

    device, err := netmigo.NewDevice(logger, netmigo.CISCO_IOSXR)
    if err != nil {
        log.Fatalf("Failed to create device: %v", err)
    }

    if err := device.Connect(iosxrCfg); err != nil {
        log.Fatalf("Connect failed: %v", err)
    }
    defer device.Disconnect()

    command := "terminal length 0\nshow logging"

    outputFile, err := device.Execute(
        command,
        netmigo.WithTimeout(5*time.Second),
    )
    if err != nil {
        log.Fatalf("Execute failed: %v", err)
    }

    fmt.Printf("Execution successful. Combined output is in file: %s\n", outputFile)
}
