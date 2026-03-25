package repository

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/jonelmawirat/netmigo/netmigo/config"
	"golang.org/x/crypto/ssh"
)

var (
	sshDialFunc                  = ssh.Dial
	sleepFunc                    = time.Sleep
	timeAfterFunc                = time.After
	getJumpClientFunc            = getJumpClient
	releaseJumpClientFunc        = ReleaseJumpClient
	connectThroughJumpFunc       = connectThroughJumpServer
	errJumpDialTimedOut    error = errors.New("jump server dial timed out")
)

func connectToTarget(cfg config.DeviceConfig) (*ssh.Client, error) {
	if cfg.JumpServer != nil {
		jumpClient, err := getJumpClientFunc(cfg.JumpServer)
		if err != nil {
			return nil, fmt.Errorf("failed to get jump server client: %w", err)
		}
		client, err := connectThroughJumpFunc(jumpClient, cfg)
		if err != nil {
			releaseJumpClientFunc(cfg.JumpServer)
			return nil, err
		}
		return client, nil
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
	attempts := 0
	for attempts < maxRetries {
		attempts++
		client, err := sshDialFunc("tcp", address, sshConfig)
		if err == nil {
			return client, nil
		}
		dialErr = err
		if !shouldRetrySSHConnectError(err) || attempts == maxRetries {
			break
		}
		sleepFunc(1 * time.Second)
	}
	return nil, fmt.Errorf("failed to connect to %s after %d %s: %w", address, attempts, attemptLabel(attempts), dialErr)
}

func connectThroughJumpServer(jumpClient *ssh.Client, cfg config.DeviceConfig) (*ssh.Client, error) {
	address := fmt.Sprintf("%s:%s", cfg.IP, cfg.Port)

	netConn, err := dialConnWithTimeout(func() (net.Conn, error) {
		return jumpClient.Dial("tcp", address)
	}, cfg.ConnectionTimeout)
	if err != nil {
		if errors.Is(err, errJumpDialTimedOut) {
			return nil, fmt.Errorf("timed out connecting to target %s via jump server after %s", address, cfg.ConnectionTimeout)
		}
		return nil, fmt.Errorf("jump server dial error: %w", err)
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

func dialConnWithTimeout(dial func() (net.Conn, error), timeout time.Duration) (net.Conn, error) {
	type dialResult struct {
		conn net.Conn
		err  error
	}

	results := make(chan dialResult, 1)
	done := make(chan struct{})
	defer close(done)

	go func() {
		conn, err := dial()
		result := dialResult{
			conn: conn,
			err:  err,
		}
		select {
		case results <- result:
		case <-done:
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()

	if timeout <= 0 {
		result := <-results
		return result.conn, result.err
	}

	select {
	case result := <-results:
		return result.conn, result.err
	case <-timeAfterFunc(timeout):
		return nil, fmt.Errorf("%w after %s", errJumpDialTimedOut, timeout)
	}
}

func shouldRetrySSHConnectError(err error) bool {
	return !isAuthFailureError(err)
}

func isAuthFailureError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	authPatterns := []string{
		"unable to authenticate",
		"no supported methods remain",
		"permission denied",
		"authentication failed",
	}
	for _, pattern := range authPatterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}
	return false
}

func attemptLabel(attempts int) string {
	if attempts == 1 {
		return "attempt"
	}
	return "attempts"
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
