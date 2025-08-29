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

    commands := []string{
        "terminal length 0",
		"ping 8.8.8.8",
        "show version",
		"ping 9.9.9.9",
        "show run",
		"ping 5.5.5.5",
        "show logging",
    }

    outputFiles, err := device.ExecuteMultiple(
        commands,
        netmigo.WithTimeout(3*time.Second),
    )
    if err != nil {
        log.Fatalf("ExecuteMultiple failed: %v", err)
    }

    fmt.Println("Execution successful. Output files:")
    for i, file := range outputFiles {
        fmt.Printf("Output for command '%s' is in file: %s\n", commands[i], file)
    }
}
