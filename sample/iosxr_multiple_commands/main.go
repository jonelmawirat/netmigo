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
 
    
    commands := []string{
        "term len 0",
        "show logging",
        "show version",
        "show interfaces brief",
        "show run",
    }

    outputFiles, err := iosxrDev.ExecuteMultiple(commands)
    if err != nil {
        log.Fatalf("ExecuteMultiple failed: %v", err)
    }

    for i, outFile := range outputFiles {
        fmt.Printf("Output for command %q saved in: %s\n", commands[i], outFile)
    }
    
    
}


