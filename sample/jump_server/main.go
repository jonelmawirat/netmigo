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
        "babybear.chat",
        netmigo.WithUsername("root"),
        netmigo.WithKeyPath("/Users/jmawirat/.ssh/id_rsa"),
        netmigo.WithConnectionTimeout(5*time.Second),
    )

    targetCfg := netmigo.NewDeviceConfig(
        "sandbox-iosxr-1.cisco.com",
        netmigo.WithUsername("admin"),
        netmigo.WithPassword("C1sco12345"),
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

    commands := []string{
        "terminal length 0",
        "show logging",
		"show run",
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
