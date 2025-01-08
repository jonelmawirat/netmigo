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
    
 
    
    iosxeCfg := &netmigo.DeviceConfig{
        IP:                "devnetsandboxiosxe.cisco.com",
        Port:              "22",
        Username:          "admin",
        Password:          "C1sco12345",
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
    }

 
    
    
    device, err := netmigo.NewDevice(logger,netmigo.CISCO_IOSXE)
    if err != nil {
        
        log.Fatalf("Failed to create device: %v", err)
    }
     
    if err := device.Connect(iosxeCfg); err != nil {
        log.Fatalf("Connect failed: %v", err)
    }
    defer device.Disconnect()

        
    
    iosxrDev, ok := device.(*netmigo.Iosxe)
    if !ok {
        log.Fatal("Device is not iOSXE type!")
    }
 
    
    commands := []string{
        "term len 0",
        "show logging",
        "show version",
        "show ip interface brief",
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


