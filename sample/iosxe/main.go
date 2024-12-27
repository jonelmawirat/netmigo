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
    
    config := &netmigo.DeviceConfig{
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
     
    if err := device.Connect(config); err != nil {
        log.Fatalf("Connect failed: %v", err)
    }
    defer device.Disconnect()

        
    
    iosxeDev, ok := device.(*netmigo.Iosxe)
    if !ok {
        log.Fatal("Device is not IOSXE type!")
    }
 
    
    outputFile, err := iosxeDev.Execute("term len 0\nshow loggin")
    if err != nil {
        log.Fatalf("Execute failed: %v", err)
    }

    fmt.Println("Output File:", outputFile)
    
    
}

