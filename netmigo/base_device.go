package netmigo

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)


type BaseDevice struct {
    client *ssh.Client
    logger *slog.Logger
}


func (b *BaseDevice) connectBase(cfg *DeviceConfig) error {
    c, err := connectToTarget(cfg)
    if err != nil {
        return err
    }
    b.client = c
    return nil
}


func (b *BaseDevice) disconnectBase() {
    if b.client != nil {
        b.logger.Info("Closing SSH connection")
        b.client.Close()
    }
}


func (b *BaseDevice) interactiveExecute(command string, timeoutSeconds int) (string, error) {
    if b.client == nil {
        return "", errors.New("ssh client is nil; not connected")
    }

    session, err := b.client.NewSession()
    if err != nil {
        b.logger.Error("Failed to create SSH session", "error", err)
        return "", fmt.Errorf("failed to create session: %w", err)
    }
    defer session.Close()

    
    modes := ssh.TerminalModes{
        ssh.ECHO:          0,
        ssh.TTY_OP_ISPEED: 14400,
        ssh.TTY_OP_OSPEED: 14400,
    }
    b.logger.Debug("Requesting PTY")
    if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
        b.logger.Error("Failed to request pseudo terminal", "error", err)
        return "", fmt.Errorf("failed to request pseudo terminal: %w", err)
    }

    
    stdinPipe, err := session.StdinPipe()
    if err != nil {
        b.logger.Error("Failed to obtain stdin pipe", "error", err)
        return "", fmt.Errorf("failed to obtain stdin pipe: %w", err)
    }
    stdoutPipe, err := session.StdoutPipe()
    if err != nil {
        b.logger.Error("Failed to obtain stdout pipe", "error", err)
        return "", fmt.Errorf("failed to obtain stdout pipe: %w", err)
    }

    b.logger.Debug("Starting interactive shell")
    if err := session.Shell(); err != nil {
        b.logger.Error("Failed to start shell", "error", err)
        return "", fmt.Errorf("failed to start shell: %w", err)
    }

    _, _ = stdinPipe.Write([]byte("\n"))
    time.Sleep(3 * time.Second)

    b.logger.Debug("Sending command", "command", command)
    if n, err := stdinPipe.Write([]byte(command + "\n")); err != nil {
        b.logger.Error("Failed to send command", "error", err)
        return "", fmt.Errorf("failed to send command: %w", err)
    } else {
        b.logger.Debug("Command write successful", "bytesWritten", n)
    }

    b.logger.Debug("Sending exit command")
    if n, err := stdinPipe.Write([]byte("exit\n")); err != nil {
        b.logger.Error("Failed to send exit command", "error", err)
        return "", fmt.Errorf("failed to send exit command: %w", err)
    } else {
        b.logger.Debug("Exit write successful", "bytesWritten", n)
    }

    if err := stdinPipe.Close(); err != nil {
        b.logger.Error("Failed to close stdin pipe", "error", err)
        return "", fmt.Errorf("failed to close stdin pipe: %w", err)
    }

    
    timeout := time.Duration(timeoutSeconds) * time.Second
    timer := time.NewTimer(timeout)
    defer timer.Stop()

    done := make(chan error, 1)

    
    tempFile, err := os.CreateTemp("", "cmd_output_*.txt")
    if err != nil {
        b.logger.Error("Failed to create temp file", "error", err)
        return "", fmt.Errorf("failed to create temp file: %w", err)
    }
    defer tempFile.Close()

    go func() {
        reader := bufio.NewReader(stdoutPipe)
        for {
            line, err := reader.ReadString('\n')
            if len(line) > 0 {
                
                b.logger.Debug("Read line from device", "line", line)

                
                if _, werr := tempFile.WriteString(line); werr != nil {
                    done <- fmt.Errorf("error writing to temp file: %w", werr)
                    return
                }
                
                if !timer.Stop() {
                    <-timer.C
                }
                timer.Reset(timeout)
            }
            if err != nil {
                
                if err == io.EOF {
                    b.logger.Debug("Reached EOF on stdout")
                    done <- nil
                } else {
                    b.logger.Error("Error reading stdout", "error", err)
                    done <- fmt.Errorf("error reading stdout: %w", err)
                }
                return
            }
        }
    }()

    
    select {
    case <-timer.C:
        b.logger.Error("Timeout: no data received", "timeoutSeconds", timeoutSeconds)
        return "", fmt.Errorf("timeout: no data received for %d seconds", timeoutSeconds)

    case err := <-done:
        if err != nil {
            b.logger.Error("Goroutine error", "error", err)
            return "", err
        }
    }

    
    b.logger.Debug("Waiting for session to complete")
    if err := session.Wait(); err != nil {
        b.logger.Error("Failed to wait for session", "error", err)
        return "", fmt.Errorf("failed to wait for session: %w", err)
    }

    b.logger.Info("Command execution complete", "outputFile", tempFile.Name())
    return tempFile.Name(), nil
}


func (b *BaseDevice) scpDownload(remoteFilePath, localFilePath string) error {
    if b.client == nil {
        return errors.New("ssh client is nil; not connected")
    }

    b.logger.Info("Starting SCP Download",
        "remoteFile", remoteFilePath,
        "localFile", localFilePath,
    )

    session, err := b.client.NewSession()
    if err != nil {
        return fmt.Errorf("failed to create SSH session: %w", err)
    }
    defer session.Close()

    scpCmd := fmt.Sprintf("scp -f %s", remoteFilePath)
    b.logger.Debug("Running SCP command", "command", scpCmd)

    writer, err := session.StdinPipe()
    if err != nil {
        return fmt.Errorf("failed to create stdin pipe: %w", err)
    }
    reader, err := session.StdoutPipe()
    if err != nil {
        return fmt.Errorf("failed to create stdout pipe: %w", err)
    }
    stderrPipe, err := session.StderrPipe()
    if err != nil {
        return fmt.Errorf("failed to create stderr pipe: %w", err)
    }

    if err := session.Start(scpCmd); err != nil {
        return fmt.Errorf("failed to start SCP session: %w", err)
    }

    
    go func() {
        scanner := bufio.NewScanner(stderrPipe)
        for scanner.Scan() {
            b.logger.Error("SCP STDERR", "line", scanner.Text())
        }
        if serr := scanner.Err(); serr != nil {
            b.logger.Error("SCP stderr scanner error", "error", serr)
        }
    }()

    
    if _, err := writer.Write([]byte("\x00")); err != nil {
        return fmt.Errorf("failed to send readiness signal: %w", err)
    }

    
    var fileMode int
    var fileSize int64
    var fileName string

    buf := make([]byte, 512)
    n, err := reader.Read(buf)
    if err != nil {
        return fmt.Errorf("failed to read SCP metadata: %w", err)
    }
    b.logger.Debug("Raw SCP metadata", "metadata", string(buf[:n]))

    _, err = fmt.Sscanf(string(buf[:n]), "C%o %d %s\n", &fileMode, &fileSize, &fileName)
    if err != nil {
        return fmt.Errorf("failed to parse file metadata: %w", err)
    }

    
    if _, err := writer.Write([]byte("\x00")); err != nil {
        return fmt.Errorf("failed to confirm file creation: %w", err)
    }

    localFile, err := os.Create(localFilePath)
    if err != nil {
        return fmt.Errorf("failed to create local file: %w", err)
    }
    defer localFile.Close()

    start := time.Now()
    if _, err := io.CopyN(localFile, reader, fileSize); err != nil {
        return fmt.Errorf("failed to copy remote file content: %w", err)
    }
    duration := time.Since(start)
    b.logger.Info("File transfer complete", "duration", duration)

    
    if _, err := reader.Read(make([]byte, 1)); err != nil && err != io.EOF {
        return fmt.Errorf("failed to read end-of-stream: %w", err)
    }

    _ = writer.Close()
    if err := session.Wait(); err != nil {
        return fmt.Errorf("SCP session did not exit cleanly: %w", err)
    }

    b.logger.Info("SCP Download successful",
        "localFile", localFilePath,
    )
    return nil
}


func (b *BaseDevice) interactiveExecuteMultiple(commands []string, timeoutSeconds int) ([]string, error) {
    if b.client == nil {
        return nil, errors.New("ssh client is nil; not connected")
    }

    session, err := b.client.NewSession()
    if err != nil {
        b.logger.Error("Failed to create SSH session", "error", err)
        return nil, fmt.Errorf("failed to create session: %w", err)
    }
    defer session.Close()

    
    modes := ssh.TerminalModes{
        ssh.ECHO:          0,
        ssh.TTY_OP_ISPEED: 14400,
        ssh.TTY_OP_OSPEED: 14400,
    }
    b.logger.Debug("Requesting PTY for multiple commands")
    if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
        b.logger.Error("Failed to request pseudo terminal", "error", err)
        return nil, fmt.Errorf("failed to request pseudo terminal: %w", err)
    }

    
    stdinPipe, err := session.StdinPipe()
    if err != nil {
        b.logger.Error("Failed to obtain stdin pipe", "error", err)
        return nil, fmt.Errorf("failed to obtain stdin pipe: %w", err)
    }
    stdoutPipe, err := session.StdoutPipe()
    if err != nil {
        b.logger.Error("Failed to obtain stdout pipe", "error", err)
        return nil, fmt.Errorf("failed to obtain stdout pipe: %w", err)
    }

    
    b.logger.Debug("Starting interactive shell for multiple commands")
    if err := session.Shell(); err != nil {
        b.logger.Error("Failed to start shell", "error", err)
        return nil, fmt.Errorf("failed to start shell: %w", err)
    }

    
    reader := bufio.NewReader(stdoutPipe)

    
    linesCh := make(chan string, 1000)
    doneCh := make(chan error, 1)

    
    go func() {
        defer close(linesCh)
        for {
            line, err := reader.ReadString('\n')
            if len(line) > 0 {
                linesCh <- line
            }
            if err != nil {
                if err == io.EOF {
                    b.logger.Debug("Reached EOF on stdout in multiple commands")
                    doneCh <- nil
                } else {
                    b.logger.Error("Error reading stdout in multiple commands", "error", err)
                    doneCh <- fmt.Errorf("error reading stdout: %w", err)
                }
                return
            }
        }
    }()

    
    _, _ = stdinPipe.Write([]byte("\n"))
    time.Sleep(1 * time.Second)

    var outputFiles []string

    
    for _, cmd := range commands {
        b.logger.Info("Sending command", "command", cmd)
        if _, err := stdinPipe.Write([]byte(cmd + "\n")); err != nil {
            b.logger.Error("Failed to send command in multiple commands", "error", err)
            return nil, fmt.Errorf("failed to send command %q: %w", cmd, err)
        }

        
        outputBuf := make([]string, 0)
        timeout := time.NewTimer(time.Duration(timeoutSeconds) * time.Second)

    COLLECT_LOOP:
        for {
            select {
            case line := <-linesCh:
                if line == "" {
                    
                    break COLLECT_LOOP
                }
                
                outputBuf = append(outputBuf, line)
                
                if !timeout.Stop() {
                    <-timeout.C
                }
                timeout.Reset(time.Duration(timeoutSeconds) * time.Second)
            case <-timeout.C:
                
                break COLLECT_LOOP
            }
        }

        
        tempFile, err := os.CreateTemp("", "cmd_output_*.txt")
        if err != nil {
            b.logger.Error("Failed to create temp file", "error", err)
            return nil, fmt.Errorf("failed to create temp file for command %q: %w", cmd, err)
        }
        for _, line := range outputBuf {
            if _, werr := tempFile.WriteString(line); werr != nil {
                tempFile.Close()
                return nil, fmt.Errorf("error writing to temp file: %w", werr)
            }
        }
        tempFile.Close()

        b.logger.Info("Command output saved", "command", cmd, "outputFile", tempFile.Name())
        outputFiles = append(outputFiles, tempFile.Name())
    }

    
    b.logger.Debug("Sending exit command after multiple commands")
    if _, err := stdinPipe.Write([]byte("exit\n")); err != nil {
        b.logger.Error("Failed to send exit command in multiple commands", "error", err)
        return nil, fmt.Errorf("failed to send exit command: %w", err)
    }
    _ = stdinPipe.Close()

    
    select {
    case err := <-doneCh:
        if err != nil {
            b.logger.Error("Error from reading goroutine", "error", err)
            return nil, err
        }
    case <-time.After(5 * time.Second):
        b.logger.Warn("Waited 5s after sending exit, force closing session")
    }

    b.logger.Debug("Waiting for session to complete (multiple commands)")
    if err := session.Wait(); err != nil {
        b.logger.Error("Failed to wait for session (multiple commands)", "error", err)
        return nil, fmt.Errorf("failed to wait for session: %w", err)
    }

    b.logger.Info("All commands execution complete in single shell", "count", len(commands))
    return outputFiles, nil
}

