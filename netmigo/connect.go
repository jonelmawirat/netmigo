package netmigo

import (
    "errors"
    "fmt"
    "os"
    "time"

    "golang.org/x/crypto/ssh"

    "github.com/jonelmawirat/netmigo/logger" 
)

// connectToTarget decides if we need a jump server or a direct connection.
func connectToTarget(cfg *DeviceConfig) (*ssh.Client, error) {
    if cfg.JumpServer != nil {
        logger.Log.Debug("Connecting to jump server first",
            "jumpServerIP", cfg.JumpServer.IP,
        )
        jumpClient, err := connectToTarget(cfg.JumpServer)
        if err != nil {
            return nil, fmt.Errorf("failed to connect to jump server: %w", err)
        }
        return connectThroughJumpServer(jumpClient, cfg)
    }
    return connectDirectly(cfg)
}

// connectDirectly tries to SSH dial the device, retrying if needed.

func connectDirectly(cfg *DeviceConfig) (*ssh.Client, error) {
    // Step 1. Build your auth methods (password, public key, keyboard-interactive, etc.)
    authMethods, err := getAuthMethods(cfg)
    if err != nil {
        logger.Log.Error("Failed to get auth methods", "error", err, "host", cfg.IP)
        return nil, err
    }

    // Step 2. Build ssh.ClientConfig
    sshConfig := &ssh.ClientConfig{
        User:            cfg.Username,
        Auth:            authMethods,
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         cfg.ConnectionTimeout,
    }
    address := fmt.Sprintf("%s:%s", cfg.IP, cfg.Port)

    // Step 3. Attempt multiple retries
    maxRetries := cfg.MaxRetry
    if maxRetries < 1 {
        maxRetries = 1
    }
    var dialErr error

    for i := 0; i < maxRetries; i++ {
        logger.Log.Info("Attempting SSH dial (direct)",
            "address", address, 
            "attempt", i+1,
            "maxAttempts", maxRetries,
        )

        // Actual Dial call
        client, err := ssh.Dial("tcp", address, sshConfig)
        if err == nil {
            // Step 4. If no error => authenticated successfully
            logger.Log.Info("SSH dial successful - authentication complete",
                "address", address,
                "username", cfg.Username,
            )
            // Return the client => we are connected and authenticated
            return client, nil
        }

        // Otherwise, log that we failed
        dialErr = err
        logger.Log.Warn("SSH dial failed, retrying...",
            "error", err,
            "address", address,
            "attempt", i+1,
        )
        time.Sleep(time.Second)
    }

    // If we never succeeded
    logger.Log.Error("Failed to connect after retries",
        "address", address,
        "maxAttempts", maxRetries,
        "finalError", dialErr,
    )
    return nil, fmt.Errorf("failed to connect to %s after %d attempts: %w", address, maxRetries, dialErr)
}


// connectThroughJumpServer dials the final target via an existing jumpClient.

func connectThroughJumpServer(jumpClient *ssh.Client, cfg *DeviceConfig) (*ssh.Client, error) {
    address := fmt.Sprintf("%s:%s", cfg.IP, cfg.Port)
    logger.Log.Info("Dialing final target via jump server",
        "jumpServer", jumpClient.RemoteAddr(),
        "finalHost", address,
    )

    netConn, err := jumpClient.Dial("tcp", address)
    if err != nil {
        logger.Log.Error("Jump server dial error", "error", err, "finalHost", address)
        return nil, fmt.Errorf("jump server dial error: %w", err)
    }

    // Build new sshConfig for final device (like above)
    authMethods, err := getAuthMethods(cfg)
    if err != nil {
        logger.Log.Error("Failed to get auth methods for final device", "error", err)
        return nil, err
    }

    sshConfig := &ssh.ClientConfig{
        User:            cfg.Username,
        Auth:            authMethods,
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         cfg.ConnectionTimeout,
    }

    logger.Log.Info("Creating SSH client via jump server", 
        "finalAddress", address,
        "username", cfg.Username,
    )

    // ssh.NewClientConn + ssh.NewClient
    clientConn, chans, reqs, err := ssh.NewClientConn(netConn, address, sshConfig)
    if err != nil {
        logger.Log.Error("Failed to create client connection via jump server",
            "error", err,
            "finalHost", address,
        )
        return nil, fmt.Errorf("new client conn error: %w", err)
    }

    // If we reach here => authenticated to final device
    logger.Log.Info("Successfully authenticated to final device via jump server",
        "finalAddress", address,
    )

    return ssh.NewClient(clientConn, chans, reqs), nil
}


// getAuthMethods constructs a list of possible auth methods (key + password).
func getAuthMethods(cfg *DeviceConfig) ([]ssh.AuthMethod, error) {
    var methods []ssh.AuthMethod

    if cfg.KeyPath != "" {
        pk, err := publicKeyFile(cfg.KeyPath)
        if err != nil {
            return nil, err
        }
        methods = append(methods, pk)
    }

    if cfg.Password != "" {
        methods = append(methods, ssh.Password(cfg.Password))
    }

    if len(methods) == 0 {
        return nil, errors.New("no auth method provided (need KeyPath or Password)")
    }
    return methods, nil
}

// publicKeyFile reads a private key file and returns an AuthMethod.
func publicKeyFile(file string) (ssh.AuthMethod, error) {
    key, err := os.ReadFile(file)
    if err != nil {
        return nil, fmt.Errorf("error reading key file: %w", err)
    }
    signer, err := ssh.ParsePrivateKey(key)
    if err != nil {
        return nil, fmt.Errorf("error parsing private key: %w", err)
    }
    return ssh.PublicKeys(signer), nil
}

