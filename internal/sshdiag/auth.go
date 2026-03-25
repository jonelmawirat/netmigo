package sshdiag

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type AuthMode string

const (
	AuthModeAuto                AuthMode = "auto"
	AuthModePassword            AuthMode = "password"
	AuthModeKeyboardInteractive AuthMode = "keyboard-interactive"
	AuthModeKey                 AuthMode = "key"
)

const (
	defaultPort              = "22"
	defaultConnectionTimeout = 10 * time.Second
	defaultRetries           = 3
)

type EndpointConfig struct {
	Label             string
	Host              string
	Port              string
	Username          string
	Password          string
	KeyPath           string
	KeyPassphrase     string
	AuthMode          AuthMode
	ConnectionTimeout time.Duration
	Retries           int
}

type authPlan struct {
	Mode     AuthMode
	Methods  []ssh.AuthMethod
	SetupErr error
}

func (c EndpointConfig) withDefaults() EndpointConfig {
	cfg := c
	cfg.Port = strings.TrimSpace(cfg.Port)
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	cfg.AuthMode = normalizeAuthMode(cfg.AuthMode)
	if cfg.ConnectionTimeout <= 0 {
		cfg.ConnectionTimeout = defaultConnectionTimeout
	}
	if cfg.Retries < 1 {
		cfg.Retries = defaultRetries
	}
	if strings.TrimSpace(cfg.Label) == "" {
		cfg.Label = "target"
	}
	return cfg
}

func (c EndpointConfig) Address() string {
	cfg := c.withDefaults()
	return fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
}

func (c EndpointConfig) Validate() error {
	cfg := c.withDefaults()
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("%s host is required", cfg.Label)
	}
	if strings.TrimSpace(cfg.Username) == "" {
		return fmt.Errorf("%s username is required", cfg.Label)
	}

	switch cfg.AuthMode {
	case AuthModeAuto:
		if cfg.Password == "" && cfg.KeyPath == "" {
			return fmt.Errorf("%s requires password or key path when auth mode is auto", cfg.Label)
		}
	case AuthModePassword, AuthModeKeyboardInteractive:
		if cfg.Password == "" {
			return fmt.Errorf("%s password is required for auth mode %q", cfg.Label, cfg.AuthMode)
		}
	case AuthModeKey:
		if cfg.KeyPath == "" {
			return fmt.Errorf("%s key path is required for auth mode %q", cfg.Label, cfg.AuthMode)
		}
	default:
		return fmt.Errorf("%s auth mode %q is not supported", cfg.Label, cfg.AuthMode)
	}

	return nil
}

func normalizeAuthMode(mode AuthMode) AuthMode {
	normalized := AuthMode(strings.TrimSpace(strings.ToLower(string(mode))))
	switch normalized {
	case "", AuthModeAuto:
		return AuthModeAuto
	case AuthModePassword:
		return AuthModePassword
	case AuthModeKeyboardInteractive:
		return AuthModeKeyboardInteractive
	case AuthModeKey:
		return AuthModeKey
	default:
		return normalized
	}
}

func buildAuthPlans(cfg EndpointConfig, logger *slog.Logger) ([]authPlan, error) {
	cfg = cfg.withDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	switch cfg.AuthMode {
	case AuthModeAuto:
		var plans []authPlan
		if cfg.KeyPath != "" {
			plans = append(plans, newKeyPlan(cfg))
		}
		if cfg.Password != "" {
			plans = append(plans,
				authPlan{Mode: AuthModeKeyboardInteractive, Methods: []ssh.AuthMethod{keyboardInteractiveAuth(cfg.Label, cfg.Password, logger)}},
				authPlan{Mode: AuthModePassword, Methods: []ssh.AuthMethod{ssh.Password(cfg.Password)}},
			)
		}
		return plans, nil
	case AuthModePassword:
		return []authPlan{{Mode: AuthModePassword, Methods: []ssh.AuthMethod{ssh.Password(cfg.Password)}}}, nil
	case AuthModeKeyboardInteractive:
		return []authPlan{{Mode: AuthModeKeyboardInteractive, Methods: []ssh.AuthMethod{keyboardInteractiveAuth(cfg.Label, cfg.Password, logger)}}}, nil
	case AuthModeKey:
		return []authPlan{newKeyPlan(cfg)}, nil
	default:
		return nil, fmt.Errorf("%s auth mode %q is not supported", cfg.Label, cfg.AuthMode)
	}
}

func newKeyPlan(cfg EndpointConfig) authPlan {
	method, err := publicKeyAuthMethod(cfg.KeyPath, cfg.KeyPassphrase)
	if err != nil {
		return authPlan{
			Mode:     AuthModeKey,
			SetupErr: fmt.Errorf("%s key auth setup failed: %w", cfg.Label, err),
		}
	}
	return authPlan{
		Mode:    AuthModeKey,
		Methods: []ssh.AuthMethod{method},
	}
}

func keyboardInteractiveAuth(label, secret string, logger *slog.Logger) ssh.AuthMethod {
	return ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		if logger != nil {
			logger.Debug("Received keyboard-interactive challenge",
				"endpoint", label,
				"user", user,
				"instruction", instruction,
				"promptCount", len(questions),
			)
		}

		answers := make([]string, len(questions))
		for i := range questions {
			answers[i] = secret
		}
		return answers, nil
	})
}

func publicKeyAuthMethod(path, passphrase string) (ssh.AuthMethod, error) {
	signer, err := loadSignerFromFile(path, passphrase)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func loadSignerFromFile(path, passphrase string) (ssh.Signer, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read key file %q: %w", path, err)
	}

	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("parse private key with passphrase: %w", err)
		}
		return signer, nil
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		var passphraseErr *ssh.PassphraseMissingError
		if errors.As(err, &passphraseErr) {
			return nil, errors.New("private key requires a passphrase; provide --key-passphrase")
		}
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	return signer, nil
}
