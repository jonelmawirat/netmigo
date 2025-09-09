# netmigo

Inspired by the simplicity and power of Python's Netmiko library, `netmigo` is a Go-native SSH utility designed for network engineers who are transitioning to or working with Go. It aims to simplify common network automation tasks by providing a clean, intuitive, and robust API for interacting with network devices.

If you come from a background of using Netmiko and are looking for a similar experience in Go, you're in the right place. `netmigo` handles the complexities of interactive SSH sessions, jump servers, and command output collection, allowing you to focus on the automation logic itself.

### Core Features

*   **Simplified SSH Connections**: Handles password/key authentication, connection timeouts, and retry logic.
*   **Built-in Jump Server Support**: Natively connects to target devices through an SSH bastion or jump host.
*   **Platform Abstraction**: Provides a consistent `DeviceService` interface for all supported platforms, including `CISCO_IOSXR`, `CISCO_IOSXE`, `CISCO_NXOS`, and `LINUX`.
*   **Reliable Command Execution**: Intelligently captures command output to local files, with mechanisms to determine when a command has finished running.
*   **Flexible Logging**: Uses Go's standard structured logging library (`slog`) for easy integration into any logging pipeline.


## Get Started Right Away with Cisco Sandbox

Here is a complete, runnable example that connects to a public Cisco IOS-XR sandbox device, executes a list of commands, and prints the paths to the files containing the output. You can save this as `main.go` and run it directly.

**filename: `main.go`**
```go
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
	// 1. Configure the logger
	loggerConfig := logger.Config{
		Level:  slog.LevelInfo, // Use slog.LevelDebug for more verbose output
		Format: "text",
	}
	slogLogger := logger.NewLogger(loggerConfig)

	// 2. Configure the target device connection details
	iosxrCfg := netmigo.NewDeviceConfig(
		"sandbox-iosxr-1.cisco.com",
		netmigo.WithUsername("admin"),
		netmigo.WithPassword("C1sco12345"),
		netmigo.WithConnectionTimeout(15*time.Second),
	)

	// 3. Create a new device object using the factory
	device, err := netmigo.NewDevice(slogLogger, netmigo.CISCO_IOSXR)
	if err != nil {
		log.Fatalf("Failed to create device: %v", err)
	}

	// 4. Connect to the device and defer disconnection
	slogLogger.Info("Connecting to device...", "host", iosxrCfg.IP)
	if err := device.Connect(iosxrCfg); err != nil {
		log.Fatalf("Connect failed: %v", err)
	}
	defer device.Disconnect()
	slogLogger.Info("Connection successful")

	// 5. Define commands and execute them
	commands := []string{
		"show version",
		"show platform",
		"show install active summary",
	}

	slogLogger.Info("Executing commands...", "count", len(commands))
	// Use an inactivity timeout of 3 seconds for collecting command output
	outputFiles, err := device.ExecuteMultiple(
		commands,
		netmigo.WithTimeout(3*time.Second),
	)
	if err != nil {
		log.Fatalf("ExecuteMultiple failed: %v", err)
	}

	// 6. Print the results
	fmt.Println("\n--- Execution Complete ---")
	for i, file := range outputFiles {
		fmt.Printf("Output for command '%s' is in file: %s\n", commands[i], file)
	}
}
```

---

## Understanding Key Concepts

### `WithConnectionTimeout` Explained

The `WithConnectionTimeout` option configures the maximum time the library will wait to **establish a successful SSH connection and complete the handshake**. It is not a timeout for command execution.

*   **What it covers**: The initial TCP dial, the SSH handshake, and authentication.
*   **What it does NOT cover**: The time it takes for a command like `show run` to execute and display its output.

If the target device is unreachable or the SSH handshake fails to complete within this duration, the `Connect()` method will return a timeout error.

### How Netmigo Knows a Command is Finished

Capturing the complete output of a command in an interactive shell is complex because there is no single, universal signal that a command is "done." `netmigo` uses two different, robust strategies to handle this.

#### 1. For Single Commands (`Execute` method)

When you execute a single command, `netmigo` primarily relies on an **inactivity timer**.

*   The `WithTimeout` option (e.g., `netmigo.WithTimeout(3*time.Second)`) sets this timer.
*   The process is as follows:
    1.  The command is sent to the device.
    2.  `netmigo` starts reading the output.
    3.  Each time a new chunk of data arrives, it resets the inactivity timer.
    4.  If the timer expires (e.g., 3 seconds pass with **no new output** from the device), `netmigo` assumes the command has finished printing its output and concludes the collection.
*   This method is effective because network devices typically stream command output continuously. A pause long enough to trigger the timeout usually signifies the end of the output.

#### 2. For Multiple Commands (`ExecuteMultiple` method)

For executing a series of commands in a single session, relying solely on a timer can be unreliable. Instead, `netmigo` uses a more deterministic **sentinel-based approach**.

*   The process for each command in the list is:
    1.  `netmigo` sends the user's command (e.g., `show version`).
    2.  Immediately after, it sends a unique, hidden "sentinel" command (e.g., `echo !__CMD_DONE_0__`).
    3.  `netmigo` then reads all output, collecting everything that comes before the sentinel string.
    4.  When it sees the sentinel echoed back in the output stream, it knows the preceding command must have completed. It then moves on to the next command in the list.
*   An inactivity timer is still used as a fallback mechanism, but the primary method is this sentinel detection, which is faster and more reliable for batch command execution.


## Netmigo Integration Guide for Developers

This guide outlines the correct architectural patterns and best practices for integrating the `netmigo` library into your applications. Following these principles will ensure your code is robust, maintainable, and aligned with the library's design.

### 1. Core Philosophy: Abstraction Through Interfaces

The `netmigo` library is designed around a core principle: **interact with devices through a common interface, not concrete types.** This allows the library to manage platform-specific details internally while providing a consistent API for all supported devices.

The three most important components you will interact with are:

1.  **Device Factory (`netmigo.NewDevice`)**: The entry point for creating a new device connection object.
2.  **Configuration (`netmigo.DeviceConfig`)**: A struct used to define all connection parameters, such as IP, credentials, and jump servers.
3.  **The Service Interface (`netmigo.DeviceService`)**: The interface that defines all device operations (`Connect`, `Execute`, `Disconnect`, etc.). **This is the object you will work with after creating a device**.

### 2. The Golden Rule: Do Not Assert Concrete Types

The most common integration error is attempting to down-cast the `DeviceService` interface returned by the factory to a specific, concrete type (e.g., `*netmigo.Iosxr`, `*service.IosxrDeviceService`). **This is an anti-pattern and will break your code.**

The factory returns an object that fulfills the `netmigo.DeviceService` interface. You must store and use this interface directly.

#### **INCORRECT** Integration:
This approach attempts to assert a concrete type that is not part of the public API and is incorrect. This will always fail.
```go
// WRONG: This code will fail.
device, err := netmigo.NewDevice(logger, netmigo.CISCO_IOSXR)
if err != nil {
    // handle error
}

// This type assertion is incorrect and will fail.
iosxrDevice, ok := device.(*netmigo.Iosxr)
if !ok {
    log.Fatal("device is not IOSXR type") // This error will always be triggered.
}
iosxrDevice.Execute("show version")
```

#### **CORRECT** Integration:
This approach correctly uses the `DeviceService` interface provided by the factory.
```go
// CORRECT: Store and use the interface directly.
device, err := netmigo.NewDevice(logger, netmigo.CISCO_IOSXR)
if err != nil {
    // handle error
}
// The 'device' variable is of type netmigo.DeviceService.
// Call its methods directly.
outputFile, err := device.Execute("show version")
```

### 3. Standard Integration Workflow

Follow these steps for a successful and clean integration.

#### Step 1: Configure the Logger
The library accepts a `*slog.Logger`. You can configure its level and format as needed.
```go
loggerConfig := logger.Config{
    Level:  slog.LevelInfo, // Or slog.LevelDebug for more verbosity
    Format: "json",
}
slogLogger := logger.NewLogger(loggerConfig)
```

#### Step 2: Create the Device Configuration
Use the `netmigo.NewDeviceConfig` function along with the `With...` functional options to build your device configuration. This provides sane defaults for port, timeout, and retries.

```go
// For a direct connection
targetCfg := netmigo.NewDeviceConfig(
    "192.168.1.1",
    netmigo.WithUsername("admin"),
    netmigo.WithPassword("C1sco12345"),
    netmigo.WithConnectionTimeout(15*time.Second),
)

// For a connection via a jump server
jumpServerCfg := netmigo.NewDeviceConfig(
    "jumpserver.example.com",
    netmigo.WithUsername("jumpuser"),
    netmigo.WithKeyPath("/path/to/ssh/key"),
)

targetWithJumpCfg := netmigo.NewDeviceConfig(
    "10.0.0.1",
    netmigo.WithUsername("admin"),
    netmigo.WithPassword("target_password"),
    netmigo.WithJumpServer(jumpServerCfg),
)
```

#### Step 3: Instantiate the Device
Use the `netmigo.NewDevice` factory to create your device object. You will get a `netmigo.DeviceService` interface back.

```go
device, err := netmigo.NewDevice(slogLogger, netmigo.CISCO_IOSXR)
if err != nil {
    log.Fatalf("Failed to create device: %v", err)
}
```

#### Step 4: Connect and Disconnect
Call the `Connect()` method on the interface. It's a best practice to `defer` the `Disconnect()` call immediately after a successful connection.

```go
if err := device.Connect(targetCfg); err != nil {
    log.Fatalf("Connect failed: %v", err)
}
defer device.Disconnect()
```

#### Step 5: Execute Commands
Use the `Execute()` or `ExecuteMultiple()` methods on the interface to run commands on the device.

```go
// Execute a single command
outputFile, err := device.Execute("show version")
if err != nil {
    log.Fatalf("Command execution failed: %v", err)
}
fmt.Printf("Output for 'show version' is in: %s\n", outputFile)


// Execute multiple commands
commands := []string{"terminal length 0", "show run", "show logging"}
outputFiles, err := device.ExecuteMultiple(commands)
if err != nil {
    log.Fatalf("ExecuteMultiple failed: %v", err)
}
for i, file := range outputFiles {
    fmt.Printf("Output for command '%s' is in file: %s\n", commands[i], file)
}
```

### 4. Advanced Topic: Concurrent Execution
Your application uses a pattern of running multiple command sets in parallel. The `netmigo` library is safe for this, provided you create a new device connection for each concurrent goroutine.

The logic in your `ExecuteMutiple` function is a good example of this pattern. Each goroutine should be responsible for the full `Connect` -> `Execute` -> `Disconnect` lifecycle.

Here is a simplified, correct implementation of that pattern:

```go
func runCommandsConcurrently(commandsList []string, deviceConfig *netmigo.DeviceConfig, logger *slog.Logger, maxConcurrent int) error {
    var wg sync.WaitGroup

    // A channel to limit concurrency
    sem := make(chan struct{}, maxConcurrent)

    for i, command := range commandsList {
        wg.Add(1)
        sem <- struct{}{} // Acquire a slot

        go func(cmd string, idx int) {
            defer wg.Done()
            defer func() { <-sem }() // Release the slot

            log.Printf("Starting execution for command %d: %s", idx, cmd)

            // Each goroutine gets its own device instance
            device, err := netmigo.NewDevice(logger, netmigo.CISCO_IOSXR)
            if err != nil {
                log.Printf("Error creating device for command %d: %v", idx, err)
                return
            }

            if err := device.Connect(deviceConfig); err != nil {
                log.Printf("Error connecting for command %d: %v", idx, err)
                return
            }
            defer device.Disconnect()

            outputFile, err := device.Execute(cmd)
            if err != nil {
                log.Printf("Error executing command %d: %v", idx, err)
                return
            }
            log.Printf("Success for command %d. Output in %s", idx, outputFile)
        }(command, i)
    }

    wg.Wait()
    return nil
}
```
