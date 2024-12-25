# **Netmigo: SSH & SCP for Network Devices**

**Netmigo** is a Go library for connecting to **network devices** (e.g., Cisco iOSXR, Linux) via SSH, executing commands, and performing SCP downloads. It supports:

- **Jump/bastion hosts** (nested SSH connections)  
- **Interactive shell mode** for commands  
- **Logging** with Go’s `slog`  
- **Keyboard-interactive or password** authentication  
- **Time-based timeouts and concurrency** (if needed)

## **Features**

1. **Connect** to a device directly or through a jump server.  
2. **Execute** commands in **interactive shell** mode:
   - Perfect for network devices that expect a shell-based prompt.  
3. **SCP Download** files from the device.  
4. **Logging** with `slog` for debug, info, and error messages.

---

## **Installation**

```bash
go get github.com/jonelmawirat/netmigo
```

---

## **Basic Usage**

### 1. **Import the Library**

In your Go code:

```go
import (
    "fmt"
    "log"
    // Adjust your paths to match where netmigo is located
    "github.com/jonelmawirat/netmigo/logger"
    "github.com/jonelmawirat/netmigo/netmigo"
)
```

### 2. **Create a DeviceConfig**

A `DeviceConfig` defines how to connect to the device, including IP, port, username/password, optional key-based auth, and an optional jump server config.

```go
cfg := &netmigo.DeviceConfig{
    IP:                "1.2.3.4",
    Port:              "22",
    Username:          "admin",
    Password:          "secretpassword",
    KeyPath:           "",     // if using SSH keys instead
    MaxRetry:          3,      // number of connection attempts
    ConnectionTimeout: 5 * time.Second,
    JumpServer:        nil,    // set if using a jump host
}
```

### 3. **Create a Device**

```go
device, err := netmigo.NewDevice(netmigo.CISCO_IOSXR)
if err != nil {
    log.Fatalf("Failed to create iOSXR device: %v", err)
}
```

### 4. **Connect & Execute a Command**

```go
// Connect to the iOSXR device
if err := device.Connect(cfg); err != nil {
    log.Fatalf("Connection failed: %v", err)
}
defer device.Disconnect()

// Execute a command in interactive shell
outputFile, err := device.Execute("show version")
if err != nil {
    log.Fatalf("Command execution failed: %v", err)
}

fmt.Println("Output stored in:", outputFile)
```

The `Execute(...)` method always runs in **interactive shell mode**, returning the path to a **temporary file** containing the command’s output. You can open that file to read or parse the output.

---

## **Examples**

### **A. Connecting to iOSXR Without a Jump Server**

```go
package main

import (
    "fmt"
    "log"
    "os"
    "time"

    "github.com/jonelmawirat/netmigo/logger"
    "github.com/jonelmawirat/netmigo/netmigo"
)

func main() {
    logger.Log.Info("Starting example: direct iOSXR connection")

    // 1) Define the direct DeviceConfig (no jump server)
    iosxrCfg := &netmigo.DeviceConfig{
        IP:                "10.0.0.50",   // iOSXR device IP
        Port:              "22",
        Username:          "admin",
        Password:          "mypassword",
        KeyPath:           "",
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
        JumpServer:        nil, // No jump server
    }

    // 2) Create an iOSXR device
    device, err := netmigo.NewDevice(netmigo.CISCO_IOSXR)
    if err != nil {
        log.Fatalf("Failed to create iOSXR device: %v", err)
    }

    // 3) Connect
    if err := device.Connect(iosxrCfg); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer device.Disconnect()

    // 4) Execute a command
    outputFile, err := device.Execute("show version")
    if err != nil {
        log.Fatalf("Command execution failed: %v", err)
    }

    logger.Log.Info("Command executed", "outputFile", outputFile)

    // Optionally read the file content
    data, _ := os.ReadFile(outputFile)
    fmt.Println("Command Output:\n", string(data))
}
```

### **B. Connecting to iOSXR **with** a Jump Server**

```go
package main

import (
    "fmt"
    "log"
    "os"
    "time"

    "github.com/jonelmawirat/netmigo/logger"
    "github.com/jonelmawirat/netmigo/netmigo"
)

func main() {
    logger.Log.Info("Starting example: iOSXR connection via jump server")

    // 1) Define the jump server config (typically a Linux box or similar)
    jumpServerCfg := &netmigo.DeviceConfig{
        IP:                "192.168.10.5",
        Port:              "22",
        Username:          "jumpuser",
        Password:          "jumppassword",
        KeyPath:           "",
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
    }

    // 2) Define the final iOSXR device config, referencing the jump server
    iosxrCfg := &netmigo.DeviceConfig{
        IP:                "10.0.0.50",
        Port:              "22",
        Username:          "admin",
        Password:          "mypassword",
        KeyPath:           "",
        MaxRetry:          3,
        ConnectionTimeout: 5 * time.Second,
        JumpServer:        jumpServerCfg, // link to jump server
    }

    // 3) Create an iOSXR device
    device, err := netmigo.NewDevice(netmigo.CISCO_IOSXR)
    if err != nil {
        log.Fatalf("Failed to create iOSXR device: %v", err)
    }

    // 4) Connect (the library will first connect to jump server, then iOSXR)
    if err := device.Connect(iosxrCfg); err != nil {
        log.Fatalf("Failed to connect via jump server: %v", err)
    }
    defer device.Disconnect()

    // 5) Execute a command
    outputFile, err := device.Execute("term len 0\nshow version")
    if err != nil {
        log.Fatalf("Command execution failed: %v", err)
    }

    logger.Log.Info("Command executed", "outputFile", outputFile)

    // Optionally read the file content
    data, _ := os.ReadFile(outputFile)
    fmt.Println("Command Output:\n", string(data))
}
```

