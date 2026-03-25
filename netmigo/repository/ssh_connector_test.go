package repository

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonelmawirat/netmigo/netmigo/config"
	"golang.org/x/crypto/ssh"
)

type fakeNetConn struct {
	closed chan struct{}
}

func (c *fakeNetConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (c *fakeNetConn) Write(b []byte) (int, error)        { return len(b), nil }
func (c *fakeNetConn) Close() error                       { closeOnce(c.closed); return nil }
func (c *fakeNetConn) LocalAddr() net.Addr                { return fakeAddr("local") }
func (c *fakeNetConn) RemoteAddr() net.Addr               { return fakeAddr("remote") }
func (c *fakeNetConn) SetDeadline(_ time.Time) error      { return nil }
func (c *fakeNetConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *fakeNetConn) SetWriteDeadline(_ time.Time) error { return nil }

type fakeAddr string

func (a fakeAddr) Network() string { return "tcp" }
func (a fakeAddr) String() string  { return string(a) }

func closeOnce(ch chan struct{}) {
	select {
	case <-ch:
	default:
		close(ch)
	}
}

func TestConnectDirectlyStopsRetryingAuthErrors(t *testing.T) {
	originalDial := sshDialFunc
	originalSleep := sleepFunc
	t.Cleanup(func() {
		sshDialFunc = originalDial
		sleepFunc = originalSleep
	})

	attempts := 0
	sleepCalls := 0
	sshDialFunc = func(network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
		attempts++
		return nil, errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password], no supported methods remain")
	}
	sleepFunc = func(time.Duration) {
		sleepCalls++
	}

	_, err := connectDirectly(config.DeviceConfig{
		IP:                "10.0.0.1",
		Port:              "22",
		Username:          "user",
		Password:          "pass",
		MaxRetry:          5,
		ConnectionTimeout: time.Second,
	})
	if err == nil {
		t.Fatal("connectDirectly returned nil error")
	}
	if attempts != 1 {
		t.Fatalf("attempt count = %d, want 1", attempts)
	}
	if sleepCalls != 0 {
		t.Fatalf("sleep calls = %d, want 0", sleepCalls)
	}
	if !strings.Contains(err.Error(), "after 1 attempt") {
		t.Fatalf("error = %q, want single-attempt wording", err.Error())
	}
}

func TestConnectDirectlyRetriesTransientErrors(t *testing.T) {
	originalDial := sshDialFunc
	originalSleep := sleepFunc
	t.Cleanup(func() {
		sshDialFunc = originalDial
		sleepFunc = originalSleep
	})

	attempts := 0
	sleepCalls := 0
	sshDialFunc = func(network, addr string, cfg *ssh.ClientConfig) (*ssh.Client, error) {
		attempts++
		if attempts < 3 {
			return nil, errors.New("connection reset by peer")
		}
		return &ssh.Client{}, nil
	}
	sleepFunc = func(time.Duration) {
		sleepCalls++
	}

	client, err := connectDirectly(config.DeviceConfig{
		IP:                "10.0.0.1",
		Port:              "22",
		Username:          "user",
		Password:          "pass",
		MaxRetry:          5,
		ConnectionTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("connectDirectly returned error: %v", err)
	}
	if client == nil {
		t.Fatal("connectDirectly returned nil client")
	}
	if attempts != 3 {
		t.Fatalf("attempt count = %d, want 3", attempts)
	}
	if sleepCalls != 2 {
		t.Fatalf("sleep calls = %d, want 2", sleepCalls)
	}
}

func TestConnectToTargetReleasesJumpClientOnFailedTargetConnect(t *testing.T) {
	originalGetJumpClient := getJumpClientFunc
	originalConnectThroughJump := connectThroughJumpFunc
	originalReleaseJump := releaseJumpClientFunc
	t.Cleanup(func() {
		getJumpClientFunc = originalGetJumpClient
		connectThroughJumpFunc = originalConnectThroughJump
		releaseJumpClientFunc = originalReleaseJump
	})

	var releaseCalls atomic.Int32
	getJumpClientFunc = func(cfg *config.DeviceConfig) (*ssh.Client, error) {
		return &ssh.Client{}, nil
	}
	connectThroughJumpFunc = func(client *ssh.Client, cfg config.DeviceConfig) (*ssh.Client, error) {
		return nil, errors.New("target auth failed")
	}
	releaseJumpClientFunc = func(cfg *config.DeviceConfig) {
		releaseCalls.Add(1)
	}

	_, err := connectToTarget(config.DeviceConfig{
		IP:                "10.0.0.1",
		Port:              "22",
		Username:          "user",
		Password:          "pass",
		ConnectionTimeout: time.Second,
		JumpServer: &config.DeviceConfig{
			IP:       "10.0.0.2",
			Port:     "22",
			Username: "jump",
			Password: "pass",
		},
	})
	if err == nil {
		t.Fatal("connectToTarget returned nil error")
	}
	if releaseCalls.Load() != 1 {
		t.Fatalf("release calls = %d, want 1", releaseCalls.Load())
	}
}

func TestDialConnWithTimeoutClosesLateConnection(t *testing.T) {
	conn := &fakeNetConn{closed: make(chan struct{})}
	releaseDial := make(chan struct{})

	_, err := dialConnWithTimeout(func() (net.Conn, error) {
		<-releaseDial
		return conn, nil
	}, 10*time.Millisecond)
	if !errors.Is(err, errJumpDialTimedOut) {
		t.Fatalf("dialConnWithTimeout error = %v, want timeout", err)
	}

	close(releaseDial)

	select {
	case <-conn.closed:
	case <-time.After(time.Second):
		t.Fatal("late connection was not closed after timeout")
	}
}
