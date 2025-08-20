package repository

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "log/slog"
    "os"
    "path/filepath"
    "strings"
    "time"

    "golang.org/x/crypto/ssh"
)

const outputDirName = "ssh_command_outputs"

func ExecutorInteractiveExecute(client *ssh.Client, logger *slog.Logger, command string, firstByteTimeout, inactivityTimeout time.Duration) (string, error) {
    if client == nil {
        return "", errors.New("ssh client is nil; not connected")
    }
    session, err := client.NewSession()
    if err != nil {
        logger.Error("Failed to create SSH session", "error", err)
        return "", fmt.Errorf("failed to create session: %w", err)
    }
    defer session.Close()

    modes := ssh.TerminalModes{
        ssh.ECHO:          0,
        ssh.TTY_OP_ISPEED: 14400,
        ssh.TTY_OP_OSPEED: 14400,
    }
    logger.Debug("Requesting PTY")
    if err := session.RequestPty("vt100", 80, 40, modes); err != nil {
        logger.Error("Failed to request pseudo terminal", "error", err)
        return "", fmt.Errorf("failed to request pseudo terminal: %w", err)
    }

    stdinPipe, err := session.StdinPipe()
    if err != nil {
        logger.Error("Failed to obtain stdin pipe", "error", err)
        return "", fmt.Errorf("failed to obtain stdin pipe: %w", err)
    }
    stdoutPipe, err := session.StdoutPipe()
    if err != nil {
        logger.Error("Failed to obtain stdout pipe", "error", err)
        return "", fmt.Errorf("failed to obtain stdout pipe: %w", err)
    }

    logger.Debug("Starting interactive shell")
    if err := session.Shell(); err != nil {
        logger.Error("Failed to start shell", "error", err)
        return "", fmt.Errorf("failed to start shell: %w", err)
    }

    logger.Debug("Initializing first-byte timer", "duration", firstByteTimeout)
    timer := time.NewTimer(firstByteTimeout)
    defer func() {
        logger.Debug("Stopping inactivity timer (deferred)")
        timer.Stop()
    }()

    done := make(chan error, 1)
    if err := os.MkdirAll(outputDirName, 0755); err != nil {
        logger.Error("Failed to create output directory", "directory", outputDirName, "error", err)
        return "", fmt.Errorf("failed to create output directory %s: %w", outputDirName, err)
    }
    outputFileName := fmt.Sprintf("cmd_output_%s.txt", time.Now().Format("20060102150405.000000000"))
    outputFilePath := filepath.Join(outputDirName, outputFileName)
    outputFile, err := os.Create(outputFilePath)
    if err != nil {
        logger.Error("Failed to create output file", "path", outputFilePath, "error", err)
        return "", fmt.Errorf("failed to create output file %s: %w", outputFilePath, err)
    }
    defer outputFile.Close()

    go func() {
        reader := bufio.NewReader(stdoutPipe)
        for {
            line, errRead := reader.ReadString('\n')
            if len(line) > 0 {
                logger.Debug("Read line from device", "line", strings.TrimSpace(line))
                if _, werr := outputFile.WriteString(line); werr != nil {
                    done <- fmt.Errorf("error writing to output file: %w", werr)
                    return
                }
                logger.Debug("Attempting to stop inactivity timer due to new data")
                if !timer.Stop() {
                    logger.Debug("Inactivity timer already fired or stopped, attempting to drain channel")
                    select {
                    case <-timer.C:
                        logger.Debug("Inactivity timer channel drained")
                    default:
                        logger.Debug("Inactivity timer channel was empty")
                    }
                } else {
                    logger.Debug("Inactivity timer stopped successfully")
                }
                logger.Debug("Resetting inactivity timer", "duration", inactivityTimeout)
                timer.Reset(inactivityTimeout)
            }
            if errRead != nil {
                if errRead == io.EOF {
                    logger.Debug("Reached EOF on stdout")
                    done <- nil
                } else {
                    logger.Error("Error reading stdout", "error", errRead)
                    done <- fmt.Errorf("error reading stdout: %w", errRead)
                }
                return
            }
        }
    }()

    _, _ = stdinPipe.Write([]byte("\n"))
    time.Sleep(1 * time.Second)

    logger.Debug("Sending command", "command", command)
    if n, err := stdinPipe.Write([]byte(command + "\n")); err != nil {
        logger.Error("Failed to send command", "error", err)
        return "", fmt.Errorf("failed to send command: %w", err)
    } else {
        logger.Debug("Command write successful", "bytesWritten", n)
    }

    select {
    case <-timer.C:
        logger.Info("Inactivity timer expired, assuming command output is complete.")
    case err := <-done:
        if err != nil {
            logger.Error("Goroutine error", "error", err)
            return "", err
        }
        logger.Debug("Goroutine finished successfully (EOF received).")
    }

    logger.Debug("Sending exit command")
    if n, err := stdinPipe.Write([]byte("exit\n")); err != nil {
        logger.Warn("Failed to send exit command", "error", err)
    } else {
        logger.Debug("Exit write successful", "bytesWritten", n)
    }

    if err := stdinPipe.Close(); err != nil {
        logger.Error("Failed to close stdin pipe", "error", err)
    }

    logger.Debug("Waiting for session to complete")
    if err := session.Wait(); err != nil {
        logger.Warn("Session wait completed with error (often expected after exit/timeout)", "error", err)
    }

    logger.Info("Command execution complete", "outputFile", outputFilePath)
    return outputFilePath, nil
}

func ExecutorScpDownload(client *ssh.Client, logger *slog.Logger, remoteFilePath, localFilePath string) error {
    if client == nil {
        return errors.New("ssh client is nil; not connected")
    }
    logger.Info("Starting SCP Download", "remoteFile", remoteFilePath, "localFile", localFilePath)
    session, err := client.NewSession()
    if err != nil {
        return fmt.Errorf("failed to create SSH session: %w", err)
    }
    defer session.Close()

    scpCmd := fmt.Sprintf("scp -f %s", remoteFilePath)
    logger.Debug("Running SCP command", "command", scpCmd)
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
            logger.Error("SCP STDERR", "line", scanner.Text())
        }
        if serr := scanner.Err(); serr != nil {
            logger.Error("SCP stderr scanner error", "error", serr)
        }
    }()

    if _, err := writer.Write([]byte("\x00")); err != nil {
        return fmt.Errorf("failed to send readiness signal: %w", err)
    }

    var fileMode int
    var fileSize int64
    var fileName string

    buf := make([]byte, 1)
    header := ""
    for {
        _, err = reader.Read(buf)
        if err != nil {
            return fmt.Errorf("failed to read SCP metadata start: %w", err)
        }
        if buf[0] == 'C' {
            header += string(buf[0])
            break
        }
    }

    lineReader := bufio.NewReader(reader)
    headerRest, err := lineReader.ReadString('\n')
    if err != nil {
        return fmt.Errorf("failed to read SCP metadata line: %w", err)
    }
    header += headerRest
    logger.Debug("Raw SCP metadata", "metadata", header)

    _, err = fmt.Sscanf(header, "C%o %d %s\n", &fileMode, &fileSize, &fileName)
    if err != nil {
        return fmt.Errorf("failed to parse file metadata from '%s': %w", header, err)
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
    copied, err := io.CopyN(localFile, lineReader, fileSize)
    if err != nil {
        return fmt.Errorf("failed to copy remote file content (copied %d bytes): %w", copied, err)
    }
    duration := time.Since(start)
    logger.Info("File transfer content complete", "duration", duration, "bytesCopied", copied)

    if _, err := writer.Write([]byte("\x00")); err != nil {
        logger.Warn("Failed to send final ack for content, file might be complete", "error", err)
    }
    _ = writer.Close()

    if err := session.Wait(); err != nil {
        logger.Warn("SCP session wait returned an error", "error", err)
    }

    logger.Info("SCP Download likely successful", "localFile", localFilePath)
    if err := os.Chmod(localFilePath, os.FileMode(fileMode)); err != nil {
        logger.Warn("Failed to set file mode", "error", err, "mode", fileMode)
    }

    return nil
}

func ExecutorInteractiveExecuteMultiple(client *ssh.Client, logger *slog.Logger, commands []string, firstByteTimeout, inactivityTimeout time.Duration) ([]string, error) {
    if client == nil {
        return nil, errors.New("ssh client is nil; not connected")
    }
    session, err := client.NewSession()
    if err != nil {
        logger.Error("Failed to create SSH session for multiple commands", "error", err)
        return nil, fmt.Errorf("failed to create session: %w", err)
    }
    defer session.Close()

    modes := ssh.TerminalModes{
        ssh.ECHO:          0,
        ssh.TTY_OP_ISPEED: 14400,
        ssh.TTY_OP_OSPEED: 14400,
    }
    logger.Debug("Requesting PTY for multiple commands")
    if err := session.RequestPty("vt100", 80, 40, modes); err != nil {
        logger.Error("Failed to request pseudo terminal", "error", err)
        return nil, fmt.Errorf("failed to request pseudo terminal: %w", err)
    }

    stdinPipe, err := session.StdinPipe()
    if err != nil {
        logger.Error("Failed to obtain stdin pipe", "error", err)
        return nil, fmt.Errorf("failed to obtain stdin pipe: %w", err)
    }
    stdoutPipe, err := session.StdoutPipe()
    if err != nil {
        logger.Error("Failed to obtain stdout pipe", "error", err)
        return nil, fmt.Errorf("failed to obtain stdout pipe: %w", err)
    }

    logger.Debug("Starting interactive shell for multiple commands")
    if err := session.Shell(); err != nil {
        logger.Error("Failed to start shell", "error", err)
        return nil, fmt.Errorf("failed to start shell: %w", err)
    }

    _, _ = stdinPipe.Write([]byte("\n"))
    time.Sleep(500 * time.Millisecond)

    outputChannel := make(chan string, 100)
    errorChannel := make(chan error, 1)
    var outputFiles []string
    if err := os.MkdirAll(outputDirName, 0755); err != nil {
        logger.Error("Failed to create output directory for multiple commands", "directory", outputDirName, "error", err)
        return nil, fmt.Errorf("failed to create output directory %s: %w", outputDirName, err)
    }

    go func() {
        defer close(outputChannel)
        defer close(errorChannel)
        reader := bufio.NewReader(stdoutPipe)
        for {
            line, readErr := reader.ReadString('\n')
            if len(line) > 0 {
                outputChannel <- line
            }
            if readErr != nil {
                if readErr == io.EOF {
                    logger.Debug("Reader Goroutine: EOF reached on stdout (multiple commands)")
                } else {
                    logger.Error("Reader Goroutine: Error reading stdout (multiple commands)", "error", readErr)
                    select {
                    case errorChannel <- fmt.Errorf("error reading stdout: %w", readErr):
                    default:
                        logger.Warn("Reader Goroutine: Error channel full or closed, could not send read error", "error", readErr)
                    }
                }
                return
            }
        }
    }()

    initialDrainDuration := 2 * time.Second
    logger.Debug("Initializing initial drain timer", "duration", initialDrainDuration)
    initialDrainTimer := time.NewTimer(initialDrainDuration)
    func() {
        logger.Debug("Starting initial output drain loop")
        draining := true
        for draining {
            select {
            case line, ok := <-outputChannel:
                if !ok {
                    logger.Debug("Initial drain: outputChannel closed, exiting drain loop.")
                    draining = false
                    break
                }
                logger.Debug("Initial drain: discarded line", "line", strings.TrimSpace(line))
            case <-initialDrainTimer.C:
                logger.Debug("Initial drain timer expired")
                draining = false
                break
            case errReader, ok := <-errorChannel:
                if ok && errReader != nil {
                    logger.Error("Initial drain: Error from reader goroutine", "error", errReader)
                } else if !ok {
                    logger.Debug("Initial drain: Error channel closed during drain.")
                }
                draining = false
                break
            }
        }
    }()
    logger.Debug("Attempting to stop initial drain timer")
    if !initialDrainTimer.Stop() {
        logger.Debug("Initial drain timer already fired or stopped, attempting to drain channel")
        select {
        case <-initialDrainTimer.C:
            logger.Debug("Initial drain timer channel drained")
        default:
            logger.Debug("Initial drain timer channel was empty")
        }
    } else {
        logger.Debug("Initial drain timer stopped successfully")
    }

    for idx, cmd := range commands {
        sentinel := fmt.Sprintf("__CMD_DONE_%d__", idx)
        logger.Info("Sending command (multiple)", "command", cmd, "index", idx)
        if _, err := stdinPipe.Write([]byte(cmd + "\n")); err != nil {
            logger.Error("Failed to send command in multiple execution", "command", cmd, "error", err)
            return outputFiles, fmt.Errorf("failed to send command %q: %w", cmd, err)
        }

        if _, err := stdinPipe.Write([]byte("! " + sentinel + "\n")); err != nil {
            logger.Error("Failed to send sentinel command", "command", cmd, "sentinel", sentinel, "error", err)
            return outputFiles, fmt.Errorf("failed to send sentinel for %q: %w", cmd, err)
        }

        currentCmdOutput := ""
        logger.Debug("Initializing command output timer", "command", cmd, "duration", firstByteTimeout)
        cmdOutputTimer := time.NewTimer(firstByteTimeout)
        collecting := true
        var sentinelSeenInOutput bool = false
    COLLECT_LOOP:
        for collecting {
            select {
            case line, ok := <-outputChannel:
                if !ok {
                    logger.Warn("Output channel closed while collecting for command, exiting loop.", "command", cmd)
                    collecting = false
                    break COLLECT_LOOP
                }
                logger.Debug("Collecting output for command", "command", cmd, "line", strings.TrimSpace(line))
                if !sentinelSeenInOutput && strings.Contains(line, sentinel) {
                    logger.Info("Sentinel string detected in output line", "command", cmd, "sentinel", sentinel, "line", strings.TrimSpace(line))
                    sentinelSeenInOutput = true
                } else {
                    currentCmdOutput += line
                }
                logger.Debug("Attempting to stop command output timer (due to new data)", "command", cmd)
                if !cmdOutputTimer.Stop() {
                    logger.Debug("Command output timer already fired or stopped (new data path), attempting to drain", "command", cmd)
                    select {
                    case <-cmdOutputTimer.C:
                        logger.Debug("Command output timer channel drained (new data path)", "command", cmd)
                    default:
                        logger.Debug("Command output timer channel was empty (new data path)", "command", cmd)
                    }
                } else {
                    logger.Debug("Command output timer stopped successfully (new data path)", "command", cmd)
                }
                logger.Debug("Resetting command output timer (due to new data)", "command", cmd, "duration", inactivityTimeout)
                cmdOutputTimer.Reset(inactivityTimeout)
            case <-cmdOutputTimer.C:
                logger.Warn("Command output timer EXPIRED for command", "command", cmd, "timeout", firstByteTimeout, "sentinelSeen", sentinelSeenInOutput)
                if sentinelSeenInOutput {
                    logger.Info("Timer expired after sentinel was seen. Output collection for command considered complete.", "command", cmd)
                } else {
                    logger.Warn("Timer expired BEFORE sentinel was seen. Command output might be incomplete or command hung.", "command", cmd)
                }
                collecting = false
            case errReader, ok := <-errorChannel:
                if ok && errReader != nil {
                    logger.Error("Error from reader goroutine during command execution", "command", cmd, "error", errReader)
                } else if !ok {
                    logger.Debug("Error channel closed during command execution collection.", "command", cmd)
                }
                collecting = false
                logger.Debug("Attempting to stop command output timer due to error/EOF from reader", "command", cmd)
                if !cmdOutputTimer.Stop() {
                    select {
                    case <-cmdOutputTimer.C:
                    default:
                    }
                }
            }
        }
        logger.Debug("Attempting to stop command output timer (post-collection)", "command", cmd)
        if !cmdOutputTimer.Stop() {
            logger.Debug("Command output timer (post-collection) already fired or stopped, attempting to drain", "command", cmd)
            select {
            case <-cmdOutputTimer.C:
                logger.Debug("Command output timer (post-collection) channel drained", "command", cmd)
            default:
                logger.Debug("Command output timer (post-collection) channel was empty", "command", cmd)
            }
        } else {
            logger.Debug("Command output timer (post-collection) stopped successfully", "command", cmd)
        }

        outputFileName := fmt.Sprintf("cmd_multi_output_%d_%s.txt", idx, time.Now().Format("20060102150405.000000000"))
        outputFilePath := filepath.Join(outputDirName, outputFileName)
        outputFile, err := os.Create(outputFilePath)
        if err != nil {
            logger.Error("Failed to create output file for multiple command output", "command", cmd, "path", outputFilePath, "error", err)
            return outputFiles, fmt.Errorf("failed to create output file for %q: %w", cmd, err)
        }
        if _, err := outputFile.WriteString(currentCmdOutput); err != nil {
            logger.Error("Failed to write to output file for multiple command output", "command", cmd, "path", outputFilePath, "error", err)
            outputFile.Close()
            return outputFiles, fmt.Errorf("failed to write to output file for %q: %w", cmd, err)
        }
        outputFile.Close()
        logger.Info("Command output saved (multiple)", "command", cmd, "file", outputFilePath, "sentinelSeen", sentinelSeenInOutput)
        outputFiles = append(outputFiles, outputFilePath)
    }

    logger.Debug("Sending exit command after multiple commands")
    if _, err := stdinPipe.Write([]byte("exit\n")); err != nil {
        logger.Error("Failed to send exit command (multiple)", "error", err)
    }
    if err := stdinPipe.Close(); err != nil {
        logger.Warn("Failed to close stdin pipe (multiple)", "error", err)
    }

    finalWaitDuration := 3 * time.Second
    logger.Debug("Initializing final wait timer for reader goroutine to complete", "duration", finalWaitDuration)
    finalWaitTimer := time.NewTimer(finalWaitDuration)
    defer func() {
        logger.Debug("Stopping final wait timer (deferred)")
        finalWaitTimer.Stop()
    }()

    select {
    case errReader, ok := <-errorChannel:
        if ok && errReader != nil {
            logger.Error("Final error from reader goroutine after all commands", "error", errReader)
        } else if !ok {
            logger.Debug("Reader goroutine's error channel already closed upon final wait.")
        }
    default:
        logger.Debug("Waiting for reader goroutine to complete or final timeout.")
        select {
        case errReader, ok := <-errorChannel:
            if ok && errReader != nil {
                logger.Error("Final error from reader goroutine after all commands (during wait)", "error", errReader)
            } else if !ok {
                logger.Info("Reader goroutine completed (error channel closed) during final wait.")
            }
        case <-finalWaitTimer.C:
            logger.Warn("Timeout waiting for reader goroutine to finish after exit command and stdin close.")
        }
    }

    logger.Debug("Waiting for SSH session (multiple commands) to complete")
    if err := session.Wait(); err != nil {
        logger.Warn("SSH session wait (multiple commands) completed with error (often expected after exit/EOF)", "error", err)
    }

    logger.Info("All commands execution complete in single shell (multiple)", "count", len(commands))
    return outputFiles, nil
}
