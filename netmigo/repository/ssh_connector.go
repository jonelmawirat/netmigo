package repository

import (
    "errors"
    "fmt"
    "net"
    "os"
    "time"

    "github.com/jonelmawirat/netmigo/netmigo/config"
    "golang.org/x/crypto/ssh"
)

func connectToTarget(cfg config.DeviceConfig) (*ssh.Client, error) {
    if cfg.JumpServer != nil {
        jumpClient, err := connectToTarget(*cfg.JumpServer)
        if err != nil {
            return nil, fmt.Errorf("failed to connect to jump server: %w", err)
        }
        return connectThroughJumpServer(jumpClient, cfg)
    }
    return connectDirectly(cfg)
}

func connectDirectly(cfg config.DeviceConfig) (*ssh.Client, error) {
    authMethods, err := getAuthMethods(&cfg)
    if err != nil {
        return nil, err
    }
    sshConfig := &ssh.ClientConfig{
        User:            cfg.Username,
        Auth:            authMethods,
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         cfg.ConnectionTimeout,
    }
    address := fmt.Sprintf("%s:%s", cfg.IP, cfg.Port)
    maxRetries := cfg.MaxRetry
    if maxRetries < 1 {
        maxRetries = 1
    }
    var dialErr error
    for i := 0; i < maxRetries; i++ {
        client, err := ssh.Dial("tcp", address, sshConfig)
        if err == nil {
            return client, nil
        }
        dialErr = err
        if i < maxRetries-1 {
            time.Sleep(1 * time.Second)
        }
    }
    return nil, fmt.Errorf("failed to connect to %s after %d attempts: %w", address, maxRetries, dialErr)
}

func connectThroughJumpServer(jumpClient *ssh.Client, cfg config.DeviceConfig) (*ssh.Client, error) {
    address := fmt.Sprintf("%s:%s", cfg.IP, cfg.Port)

    connChan := make(chan net.Conn, 1)
    errChan := make(chan error, 1)

    go func() {
        c, e := jumpClient.Dial("tcp", address)
        if e != nil {
            errChan <- e
            return
        }
        connChan <- c
    }()

    var netConn net.Conn
    var timeoutChan <-chan time.Time
    if cfg.ConnectionTimeout > 0 {
        timeoutChan = time.After(cfg.ConnectionTimeout)
    }

    select {
    case conn := <-connChan:
        netConn = conn
    case err := <-errChan:
        return nil, fmt.Errorf("jump server dial error: %w", err)
    case <-timeoutChan:
        return nil, fmt.Errorf("timed out connecting to target %s via jump server after %s", address, cfg.ConnectionTimeout)
    }

    authMethods, err := getAuthMethods(&cfg)
    if err != nil {
        netConn.Close()
        return nil, err
    }

    sshConfig := &ssh.ClientConfig{
        User:            cfg.Username,
        Auth:            authMethods,
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         cfg.ConnectionTimeout,
    }

    clientConn, chans, reqs, err := ssh.NewClientConn(netConn, address, sshConfig)
    if err != nil {
        netConn.Close()
        return nil, fmt.Errorf("new client conn error: %w", err)
    }

    return ssh.NewClient(clientConn, chans, reqs), nil
}

func getAuthMethods(cfg *config.DeviceConfig) ([]ssh.AuthMethod, error) {
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
