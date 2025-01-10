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
        IP:                "sbx-nxos-mgmt.cisco.com",
        Port:              "22",
        Username:          "admin",
        Password:          "Admin_1234!",
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
    }
 
    
    device, err := netmigo.NewDevice(logger,netmigo.CISCO_NXOS)
    if err != nil {
        
        log.Fatalf("Failed to create device: %v", err)
    }
     
    if err := device.Connect(config); err != nil {
        log.Fatalf("Connect failed: %v", err)
    }
    defer device.Disconnect()

        
    
    nxosDev, ok := device.(*netmigo.Nxos)
    if !ok {
        log.Fatal("Device is not NXOS type!")
    }
 
    
    outputFile, err := nxosDev.Execute("term len 0\nshow run")
    if err != nil {
        log.Fatalf("Execute failed: %v", err)
    }

    fmt.Println("Output File:", outputFile)
 
}

