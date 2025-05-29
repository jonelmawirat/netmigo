package main

import (
    "fmt"
    "log"
    "time"
    "log/slog"
    "github.com/jonelmawirat/netmigo/netmigo"
    "github.com/jonelmawirat/netmigo/logger"
)

func main() {

    loggerConfig := logger.Config{
        Level: slog.LevelDebug,
        Format: "json",
    }

    logger := logger.NewLogger(loggerConfig)



    iosxrCfg := &netmigo.DeviceConfig{
        IP:                "sandbox-iosxr-1.cisco.com",
        Port:              "22",
        Username:          "admin",
        Password:          "C1sco12345",
        KeyPath:           "",
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
    }




    device, err := netmigo.NewDevice(logger,netmigo.CISCO_IOSXR)
    if err != nil {

        log.Fatalf("Failed to create device: %v", err)
    }

    if err := device.Connect(iosxrCfg); err != nil {
        log.Fatalf("Connect failed: %v", err)
    }
    defer device.Disconnect()



    iosxrDev, ok := device.(*netmigo.Iosxr)
    if !ok {
        log.Fatal("Device is not iOSXR type!")
    }


    outputFile, err := iosxrDev.Execute("term len 0\nshow loggin")
    if err != nil {
        log.Fatalf("Execute failed: %v", err)
    }

    fmt.Println("Output File:", outputFile)


}


