# netmigo

`netmigo` is a Go SSH library for network automation. It is inspired by the ergonomics of Python's Netmiko, but exposes a Go-first API for creating interactive SSH sessions, traversing jump hosts, collecting command output into local files, and integrating with standard `slog` logging.

Use it when you want a small Go package that handles SSH session setup and interactive command collection while your application owns the higher-level workflow.

## Quick Start

### Requirements

- Go `1.23.3` or newer
- An SSH-reachable target device or host
- Optional jump/bastion host credentials if your network path requires one

### Install

Add the module to your project:

```bash
go get github.com/jonelmawirat/netmigo@latest
```

The primary import paths are:

```go
import (
    "github.com/jonelmawirat/netmigo/logger"
    "github.com/jonelmawirat/netmigo/netmigo"
)
```

### Smallest Working Example

This example connects to an IOS-XR device, runs one command, and prints the output file path returned by the library.

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
    slogLogger := logger.NewLogger(logger.Config{
        Level:  slog.LevelInfo,
        Format: "text",
    })

    cfg := netmigo.NewDeviceConfig(
        "sandbox-iosxr-1.cisco.com",
        netmigo.WithUsername("admin"),
        netmigo.WithPassword("C1sco12345"),
        netmigo.WithConnectionTimeout(15*time.Second),
    )

    device, err := netmigo.NewDevice(slogLogger, netmigo.CISCO_IOSXR)
    if err != nil {
        log.Fatalf("create device: %v", err)
    }

    if err := device.Connect(cfg); err != nil {
        log.Fatalf("connect: %v", err)
    }
    defer device.Disconnect()

    outputFile, err := device.Execute(
        "show version",
        netmigo.WithTimeout(3*time.Second),
    )
    if err != nil {
        log.Fatalf("execute: %v", err)
    }

    fmt.Println("Command output saved to:", outputFile)
}
```

More runnable examples live in:

- `sample/iosxr`
- `sample/iosxr_multiple_commands`
- `sample/jump_server`

## Supported Platforms And Limitations

`netmigo.NewDevice` currently constructs device services for:

- `netmigo.CISCO_IOSXR`
- `netmigo.LINUX`

Important limitation:

- `netmigo.CISCO_IOSXE` and `netmigo.CISCO_NXOS` constants are exported, but `netmigo.NewDevice(...)` currently returns `unsupported platform in factory` for those values. Do not document or depend on IOS-XE or NX-OS support until the factory and services are implemented.

## Public API Quick Reference

Core setup:

- `netmigo.NewDeviceConfig(ip, opts...)`
- `netmigo.WithUsername(...)`
- `netmigo.WithPassword(...)`
- `netmigo.WithKeyPath(...)`
- `netmigo.WithPort(...)`
- `netmigo.WithJumpServer(...)`
- `netmigo.WithMaxRetry(...)`
- `netmigo.WithConnectionTimeout(...)`

Device creation:

- `netmigo.NewDevice(logger, platform)`

Returned interface:

- `Connect(cfg *netmigo.DeviceConfig) error`
- `Execute(command string, opts ...netmigo.ExecuteOption) (string, error)`
- `ExecuteMultiple(commands []string, opts ...netmigo.ExecuteOption) ([]string, error)`
- `Download(remoteFilePath, localFilePath string) error`
- `Disconnect()`

Command execution options:

- `netmigo.WithTimeout(...)`
- `netmigo.WithFirstByteTimeout(...)`

## Connection And Command Timing

### `WithConnectionTimeout`

`WithConnectionTimeout` applies to connection establishment only:

- TCP dial
- SSH handshake
- authentication

It does not control how long a command is allowed to run after a session is established.

### `WithTimeout`

`WithTimeout` is the inactivity timeout used while reading interactive command output. If the command stops producing output long enough to hit the timeout, `netmigo` treats the command as complete.

### `WithFirstByteTimeout`

`WithFirstByteTimeout` limits how long the library will wait for the first byte of output from a command. This is useful when the remote command might hang before producing any data.

### How Completion Is Detected

- `Execute(...)` primarily uses inactivity-based completion.
- `ExecuteMultiple(...)` uses sentinel markers between commands, with timeout-based fallback behavior.

That split keeps single-command usage simple while making multi-command sessions more deterministic.

## Integration Guidance

### Prefer The Interface, Not Concrete Types

`netmigo.NewDevice(...)` returns the public `netmigo.Device` interface. Keep and use that interface directly.

Incorrect:

```go
device, err := netmigo.NewDevice(logger, netmigo.CISCO_IOSXR)
if err != nil {
    // handle error
}

iosxrDevice, ok := device.(*netmigo.Iosxr)
if !ok {
    log.Fatal("device is not IOSXR type")
}
```

Correct:

```go
device, err := netmigo.NewDevice(logger, netmigo.CISCO_IOSXR)
if err != nil {
    // handle error
}

outputFile, err := device.Execute("show version")
```

### Standard Workflow

1. Build a logger. The repo includes `logger.NewLogger(...)`, but any `*slog.Logger` works.
2. Build a `DeviceConfig` with credentials, timeout, and optional jump host.
3. Create a device with `netmigo.NewDevice(...)`.
4. Call `Connect(...)`, then `defer Disconnect()`.
5. Run `Execute(...)`, `ExecuteMultiple(...)`, or `Download(...)`.

### Concurrent Usage

The safe pattern for concurrency is one SSH connection per goroutine. Each worker should create its own device, connect, execute work, and disconnect. Do not share one connected device instance across multiple goroutines.

## SSH Diagnostic Probe

When you need to troubleshoot authentication or jump-host behavior without running a larger application flow, use the bundled `sshdiag` CLI.

Show the canonical flags:

```bash
go run ./cmd/sshdiag --help
```

Build locally:

```bash
go build -o ./bin/sshdiag ./cmd/sshdiag
```

Build a Linux amd64 artifact for a remote server:

```bash
GOOS=linux GOARCH=amd64 go build -o ./bin/sshdiag-linux-amd64 ./cmd/sshdiag
```

Supported auth modes:

- `auto`
- `password`
- `keyboard-interactive`
- `key`

In `auto` mode the probe tries key auth first when a key path is provided, then `keyboard-interactive`, then `password`.

Direct password example:

```bash
./bin/sshdiag \
  --host 10.205.142.62 \
  --username t-rbgunawan \
  --password 'your-secret' \
  --auth-mode keyboard-interactive \
  --log-level debug
```

Direct key example:

```bash
./bin/sshdiag \
  --host 10.205.142.62 \
  --username t-rbgunawan \
  --key-path /path/to/id_rsa \
  --key-passphrase 'optional-passphrase' \
  --auth-mode key \
  --log-level debug
```

Jump-host example:

```bash
./bin/sshdiag \
  --host 10.205.142.62 \
  --username t-rbgunawan \
  --password 'target-secret' \
  --auth-mode auto \
  --jump-host 10.174.6.11 \
  --jump-username t-rbgunawan \
  --jump-password 'jump-secret' \
  --jump-auth-mode auto \
  --log-level debug \
  --log-file ./sshdiag.log
```

If you add `--command 'show version'`, the probe will run one post-auth interactive command and include the generated output file path in the final JSON summary. The JSON summary is printed even when the probe fails so it can be copied into troubleshooting notes.

## Developer Validation

Run these checks from the repository root:

```bash
go test ./...
go run ./cmd/sshdiag --help
```

Then verify the shipped examples still match the repo:

- `sample/iosxr/main.go`
- `sample/iosxr_multiple_commands/main.go`
- `sample/jump_server/main.go`

## Maintainer Release Checklist

Before creating a release tag:

1. Run `go test ./...`.
2. Run `go run ./cmd/sshdiag --help`.
3. Check `git status --short` and exclude generated binaries under `bin/` unless you intentionally want to ship them.
4. Commit the documentation and code changes you actually want in the release.
5. Create an annotated tag for the next version, for example `v1.3.10`.
6. Push the branch and the tag to `origin`.

Example:

```bash
git add readme.md
git commit -m "docs: improve netmigo onboarding"
git tag -a v1.3.10 -m "v1.3.10"
git push origin main
git push origin v1.3.10
```
