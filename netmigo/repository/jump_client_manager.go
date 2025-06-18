package repository

import (
    "fmt"
    "sync"

    "github.com/jonelmawirat/netmigo/netmigo/config"
    "golang.org/x/crypto/ssh"
)

type sharedJumpClient struct {
    client   *ssh.Client
    refCount int
    mu       sync.Mutex
}

type jumpClientManager struct {
    clients map[string]*sharedJumpClient
    mu      sync.Mutex
}

var manager = &jumpClientManager{
    clients: make(map[string]*sharedJumpClient),
}

func getJumpClient(cfg *config.DeviceConfig) (*ssh.Client, error) {
    if cfg == nil {
        return nil, nil
    }

    key := fmt.Sprintf("%s@%s:%s", cfg.Username, cfg.IP, cfg.Port)

    manager.mu.Lock()
    shared, exists := manager.clients[key]
    if !exists {
        client, err := connectDirectly(*cfg)
        if err != nil {
            manager.mu.Unlock()
            return nil, fmt.Errorf("jump client manager failed to connect: %w", err)
        }
        shared = &sharedJumpClient{
            client:   client,
            refCount: 0,
        }
        manager.clients[key] = shared
    }
    manager.mu.Unlock()

    shared.mu.Lock()
    defer shared.mu.Unlock()
    shared.refCount++

    return shared.client, nil
}

func ReleaseJumpClient(cfg *config.DeviceConfig) {
    if cfg == nil {
        return
    }

    key := fmt.Sprintf("%s@%s:%s", cfg.Username, cfg.IP, cfg.Port)

    manager.mu.Lock()
    defer manager.mu.Unlock()

    if shared, exists := manager.clients[key]; exists {
        shared.mu.Lock()
        shared.refCount--
        shouldClose := shared.refCount <= 0
        shared.mu.Unlock()

        if shouldClose {
            shared.client.Close()
            delete(manager.clients, key)
        }
    }
}
