package netmigo

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "os"
    "time"

    "golang.org/x/crypto/ssh"

    "github.com/jonelmawirat/netmigo/logger"
)


type BaseDevice struct {
    client *ssh.Client
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
        logger.Log.Info("Closing SSH connection")
        b.client.Close()
    }
}


func (b *BaseDevice) interactiveExecute(command string, timeoutSeconds int) (string, error) {
    if b.client == nil {
        return "", errors.New("ssh client is nil; not connected")
    }

    session, err := b.client.NewSession()
    if err != nil {
        logger.Log.Error("Failed to create SSH session", "error", err)
        return "", fmt.Errorf("failed to create session: %w", err)
    }
    defer session.Close()

    
    modes := ssh.TerminalModes{
        ssh.ECHO:          0,
        ssh.TTY_OP_ISPEED: 14400,
        ssh.TTY_OP_OSPEED: 14400,
    }
    logger.Log.Debug("Requesting PTY")
    if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
        logger.Log.Error("Failed to request pseudo terminal", "error", err)
        return "", fmt.Errorf("failed to request pseudo terminal: %w", err)
    }

    
    stdinPipe, err := session.StdinPipe()
    if err != nil {
        logger.Log.Error("Failed to obtain stdin pipe", "error", err)
        return "", fmt.Errorf("failed to obtain stdin pipe: %w", err)
    }
    stdoutPipe, err := session.StdoutPipe()
    if err != nil {
        logger.Log.Error("Failed to obtain stdout pipe", "error", err)
        return "", fmt.Errorf("failed to obtain stdout pipe: %w", err)
    }

    logger.Log.Debug("Starting interactive shell")
    if err := session.Shell(); err != nil {
        logger.Log.Error("Failed to start shell", "error", err)
        return "", fmt.Errorf("failed to start shell: %w", err)
    }

    _, _ = stdinPipe.Write([]byte("\n"))
    time.Sleep(3 * time.Second)

    logger.Log.Debug("Sending command", "command", command)
    if n, err := stdinPipe.Write([]byte(command + "\n")); err != nil {
        logger.Log.Error("Failed to send command", "error", err)
        return "", fmt.Errorf("failed to send command: %w", err)
    } else {
        logger.Log.Debug("Command write successful", "bytesWritten", n)
    }

    logger.Log.Debug("Sending exit command")
    if n, err := stdinPipe.Write([]byte("exit\n")); err != nil {
        logger.Log.Error("Failed to send exit command", "error", err)
        return "", fmt.Errorf("failed to send exit command: %w", err)
    } else {
        logger.Log.Debug("Exit write successful", "bytesWritten", n)
    }

    if err := stdinPipe.Close(); err != nil {
        logger.Log.Error("Failed to close stdin pipe", "error", err)
        return "", fmt.Errorf("failed to close stdin pipe: %w", err)
    }

    
    timeout := time.Duration(timeoutSeconds) * time.Second
    timer := time.NewTimer(timeout)
    defer timer.Stop()

    done := make(chan error, 1)

    
    tempFile, err := os.CreateTemp("", "cmd_output_*.txt")
    if err != nil {
        logger.Log.Error("Failed to create temp file", "error", err)
        return "", fmt.Errorf("failed to create temp file: %w", err)
    }
    defer tempFile.Close()

    go func() {
        reader := bufio.NewReader(stdoutPipe)
        for {
            line, err := reader.ReadString('\n')
            if len(line) > 0 {
                
                logger.Log.Debug("Read line from device", "line", line)

                
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
                    logger.Log.Debug("Reached EOF on stdout")
                    done <- nil
                } else {
                    logger.Log.Error("Error reading stdout", "error", err)
                    done <- fmt.Errorf("error reading stdout: %w", err)
                }
                return
            }
        }
    }()

    
    select {
    case <-timer.C:
        logger.Log.Error("Timeout: no data received", "timeoutSeconds", timeoutSeconds)
        return "", fmt.Errorf("timeout: no data received for %d seconds", timeoutSeconds)

    case err := <-done:
        if err != nil {
            logger.Log.Error("Goroutine error", "error", err)
            return "", err
        }
    }

    
    logger.Log.Debug("Waiting for session to complete")
    if err := session.Wait(); err != nil {
        logger.Log.Error("Failed to wait for session", "error", err)
        return "", fmt.Errorf("failed to wait for session: %w", err)
    }

    logger.Log.Info("Command execution complete", "outputFile", tempFile.Name())
    return tempFile.Name(), nil
}


func (b *BaseDevice) scpDownload(remoteFilePath, localFilePath string) error {
    if b.client == nil {
        return errors.New("ssh client is nil; not connected")
    }

    logger.Log.Info("Starting SCP Download",
        "remoteFile", remoteFilePath,
        "localFile", localFilePath,
    )

    session, err := b.client.NewSession()
    if err != nil {
        return fmt.Errorf("failed to create SSH session: %w", err)
    }
    defer session.Close()

    scpCmd := fmt.Sprintf("scp -f %s", remoteFilePath)
    logger.Log.Debug("Running SCP command", "command", scpCmd)

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
            logger.Log.Error("SCP STDERR", "line", scanner.Text())
        }
        if serr := scanner.Err(); serr != nil {
            logger.Log.Error("SCP stderr scanner error", "error", serr)
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
    logger.Log.Debug("Raw SCP metadata", "metadata", string(buf[:n]))

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
    logger.Log.Info("File transfer complete", "duration", duration)

    
    if _, err := reader.Read(make([]byte, 1)); err != nil && err != io.EOF {
        return fmt.Errorf("failed to read end-of-stream: %w", err)
    }

    _ = writer.Close()
    if err := session.Wait(); err != nil {
        return fmt.Errorf("SCP session did not exit cleanly: %w", err)
    }

    logger.Log.Info("SCP Download successful",
        "localFile", localFilePath,
    )
    return nil
}

