# netmigo

A Go-based SSH utility library that simplifies common tasks such as:

1. **Connecting** to network devices or Linux hosts over SSH.  
2. **Executing** commands interactively, capturing the output to a local file.  
3. **SCP downloading** of files from remote devices.  
4. **Jump Server** support (SSH proxy) for more complex connectivity scenarios.  
5. Flexible logging with [slog](https://pkg.go.dev/log/slog)

---

## Features

- **Easy SSH** connections with retry logic, timeouts, and password/key-based authentication.  
- **Jump Server** (SSH proxy) support: connect to remote devices via an intermediary server.  
- **Device Abstraction**: uniform `Device` interface for multiple platforms.  
  - **Current Supported Devices**: `CISCO_IOSXR`, `CISCO_IOSXE`, `LINUX`,`CISCO_NXOS`.
- **Interactive Command Execution**: Reads all command output until EOF or timeout, automatically capturing into a temporary file.  
- **Flexible Logging**: Uses [slog](https://pkg.go.dev/log/slog)

---

## Getting Started

### Installation

```bash
go get github.com/jonelmawirat/netmigo
```

### Prerequisites

- Go 1.20+  
- (Optional) An SSH key pair if you're using key-based authentication.  
- A network device or Linux host reachable over SSH (optionally via a jump server).  

---

## Usage Examples

Below are some quick examples of how you can use netmigo for connecting to devices, with or without a jump server.

For **Cisco IOS XR** and **Cisco IOS XE**, see sample files at:
- `samples/iosxr/main.go`
- `samples/iosxe/main.go`

### 1. Connecting To Device Without a Jump Server

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/jonelmawirat/netmigo/netmigo"
    "github.com/jonelmawirat/netmigo/logger"
    "log/slog"
)

func main() {

    loggerConfig := logger.Config{
        Level:  slog.LevelDebug,
        Format: "json",
    }
    slogLogger := logger.NewLogger(loggerConfig)

    iosxrCfg := &netmigo.DeviceConfig{
        IP:                "1.2.3.4",
        Port:              "22",
        Username:          "admin",
        Password:          "my-secret-pass",
        KeyPath:           "", 
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
    }

    device, err := netmigo.NewDevice(slogLogger, netmigo.CISCO_IOSXR)
    if err != nil {
        log.Fatalf("Failed to create device: %v", err)
    }

    if err := device.Connect(iosxrCfg); err != nil {
        log.Fatalf("Connect failed: %v", err)
    }
    defer device.Disconnect()

    outputFilePath, err := device.Execute("show version")
    if err != nil {
        log.Fatalf("Command execution failed: %v", err)
    }

    fmt.Println("Captured command output in:", outputFilePath)
}
```

### 2. Connecting Through a Jump Server

Below is an example where you have a **jump server** (i.e., a bastion host) that you must SSH into first, before connecting to the target device.

```go
package main

import (
    "fmt"
    "log"
    "time"
    
    "github.com/jonelmawirat/netmigo/netmigo"
    "github.com/jonelmawirat/netmigo/logger"
    "log/slog"
)

func main() {

    loggerConfig := logger.Config{
        Level:  slog.LevelDebug,
        Format: "json",
    }
    slogLogger := logger.NewLogger(loggerConfig)

    jumpServerCfg := &netmigo.DeviceConfig{
        IP:                "10.10.10.1",
        Port:              "22",
        Username:          "jumpserver_user",
        Password:          "",
        KeyPath:           "/path/to/jumpserver_key", 
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
    }

    targetCfg := &netmigo.DeviceConfig{
        IP:                "10.10.10.2",
        Port:              "22",
        Username:          "admin",
        Password:          "target_password",
        KeyPath:           "", 
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
        JumpServer:        jumpServerCfg, 
    }

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
```

### 3. Sending Multiple Commands

```
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

```

---

## Changing the Logger

By design, **netmigo** receives a `*slog.Logger` instance. That means you can use **any** logging handler that implements [slog](https://pkg.go.dev/log/slog). For example:

```go
import (
    "log/slog"
    "os"

    "github.com/jonelmawirat/netmigo/netmigo"
)

textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
})
myCustomLogger := slog.New(textHandler)

device, err := netmigo.NewDevice(myCustomLogger, netmigo.LINUX)
```

Or you can replace slog with your own logger by wrapping your logger to satisfy the slog interface or bridging them (e.g., bridging Zap to slog, etc.).

---

## Project Structure

- **main.go**  
  Example usage of netmigo (connecting to devices, executing commands).
- **logger.go**  
  Sample logger setup using [slog](https://pkg.go.dev/log/slog).
- **netmigo/\***  
  Core library code. Notable files:
  - `base_device.go` — Shared SSH logic (connect, interactive execution, scp download).  
  - `iosxr.go`, `iosxe.go`, `linux.go` — Platform-specific devices implementing `Device`.  
  - `connect.go` — Actual connection logic (direct or jump server).  
  - `device.go` — Device interface definition and `NewDevice` factory.
- **samples/iosxr/main.go**  
  Example usage for Cisco IOS XR.
- **samples/iosxe/main.go**  
  Example usage for Cisco IOS XE.

---

## Contributing

1. Fork the project.  
2. Create your feature branch (`git checkout -b feature/new-stuff`).  
3. Commit your changes (`git commit -am 'Add some new stuff'`).  
4. Push to the branch (`git push origin feature/new-stuff`).  
5. Create a new Pull Request.  

---

## Support

If you run into any issues, please [open an issue](https://github.com/jonelmawirat/netmigo/issues).
