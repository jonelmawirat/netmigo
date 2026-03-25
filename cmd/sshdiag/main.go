package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jonelmawirat/netmigo/internal/sshdiag"
)

func main() {
	cfg, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "flag error: %v\n", err)
		os.Exit(2)
	}

	logger, closeLogWriter, err := newLogger(cfg.logFilePath, cfg.logFormat, cfg.logLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger error: %v\n", err)
		os.Exit(2)
	}
	defer closeLogWriter()

	logger.Info("Starting SSH diagnostic probe",
		"target", cfg.probe.Target.Address(),
		"targetAuthMode", cfg.probe.Target.AuthMode,
		"usedJump", cfg.probe.Jump != nil,
		"command", cfg.probe.Command,
	)

	result, runErr := sshdiag.Run(cfg.probe, logger)
	fmt.Println(result.JSON())
	if runErr != nil {
		logger.Error("SSH diagnostic probe failed", "error", runErr)
		os.Exit(1)
	}

	logger.Info("SSH diagnostic probe completed successfully")
}

type cliConfig struct {
	probe       sshdiag.ProbeConfig
	logFormat   string
	logLevel    string
	logFilePath string
}

func parseFlags() (cliConfig, error) {
	var cfg cliConfig

	host := flag.String("host", "", "target host or IP")
	port := flag.String("port", "22", "target SSH port")
	username := flag.String("username", "", "target username")
	password := flag.String("password", "", "target password or keyboard-interactive secret")
	keyPath := flag.String("key-path", "", "target private key path")
	keyPassphrase := flag.String("key-passphrase", "", "target private key passphrase")
	authMode := flag.String("auth-mode", string(sshdiag.AuthModeAuto), "target auth mode: auto|password|keyboard-interactive|key")

	jumpHost := flag.String("jump-host", "", "optional jump host or IP")
	jumpPort := flag.String("jump-port", "22", "jump host SSH port")
	jumpUsername := flag.String("jump-username", "", "jump host username")
	jumpPassword := flag.String("jump-password", "", "jump host password or keyboard-interactive secret")
	jumpKeyPath := flag.String("jump-key-path", "", "jump host private key path")
	jumpKeyPassphrase := flag.String("jump-key-passphrase", "", "jump host private key passphrase")
	jumpAuthMode := flag.String("jump-auth-mode", string(sshdiag.AuthModeAuto), "jump host auth mode: auto|password|keyboard-interactive|key")

	timeout := flag.Duration("timeout", 10*time.Second, "SSH connection timeout per attempt")
	retries := flag.Int("retries", 3, "SSH connection retries per auth mode")
	command := flag.String("command", "", "optional post-auth command probe")
	commandTimeout := flag.Duration("command-timeout", 5*time.Second, "inactivity timeout for the optional command probe")
	firstByteTimeout := flag.Duration("command-first-byte-timeout", 30*time.Second, "first-byte timeout for the optional command probe")
	logFormat := flag.String("log-format", "text", "log format: text|json")
	logLevel := flag.String("log-level", "debug", "log level: debug|info|warn|error")
	logFile := flag.String("log-file", "", "optional log file path")

	flag.Parse()

	cfg.logFormat = strings.ToLower(strings.TrimSpace(*logFormat))
	cfg.logLevel = strings.ToLower(strings.TrimSpace(*logLevel))
	cfg.logFilePath = strings.TrimSpace(*logFile)
	cfg.probe = sshdiag.ProbeConfig{
		Target: sshdiag.EndpointConfig{
			Label:             "target",
			Host:              strings.TrimSpace(*host),
			Port:              strings.TrimSpace(*port),
			Username:          strings.TrimSpace(*username),
			Password:          *password,
			KeyPath:           strings.TrimSpace(*keyPath),
			KeyPassphrase:     *keyPassphrase,
			AuthMode:          sshdiag.AuthMode(strings.TrimSpace(*authMode)),
			ConnectionTimeout: *timeout,
			Retries:           *retries,
		},
		Command:            *command,
		CommandTimeout:     *commandTimeout,
		CommandFirstByteTT: *firstByteTimeout,
	}

	if strings.TrimSpace(*jumpHost) != "" {
		cfg.probe.Jump = &sshdiag.EndpointConfig{
			Label:             "jump",
			Host:              strings.TrimSpace(*jumpHost),
			Port:              strings.TrimSpace(*jumpPort),
			Username:          strings.TrimSpace(*jumpUsername),
			Password:          *jumpPassword,
			KeyPath:           strings.TrimSpace(*jumpKeyPath),
			KeyPassphrase:     *jumpKeyPassphrase,
			AuthMode:          sshdiag.AuthMode(strings.TrimSpace(*jumpAuthMode)),
			ConnectionTimeout: *timeout,
			Retries:           *retries,
		}
	}

	if cfg.probe.Target.Host == "" {
		return cliConfig{}, fmt.Errorf("--host is required")
	}
	if cfg.probe.Target.Username == "" {
		return cliConfig{}, fmt.Errorf("--username is required")
	}
	if cfg.probe.Jump != nil && cfg.probe.Jump.Username == "" {
		return cliConfig{}, fmt.Errorf("--jump-username is required when --jump-host is set")
	}

	return cfg, nil
}

func newLogger(logFilePath, format, level string) (*slog.Logger, func(), error) {
	var writer io.Writer = os.Stdout
	closeWriter := func() {}
	if logFilePath != "" {
		if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
			return nil, nil, fmt.Errorf("create log directory: %w", err)
		}
		file, err := os.Create(logFilePath)
		if err != nil {
			return nil, nil, fmt.Errorf("create log file: %w", err)
		}
		writer = io.MultiWriter(os.Stdout, file)
		closeWriter = func() {
			_ = file.Close()
		}
	}

	levelVar, err := parseLogLevel(level)
	if err != nil {
		closeWriter()
		return nil, nil, err
	}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: levelVar})
	case "", "text":
		handler = slog.NewTextHandler(writer, &slog.HandlerOptions{Level: levelVar})
	default:
		closeWriter()
		return nil, nil, fmt.Errorf("unsupported log format %q", format)
	}
	return slog.New(handler), closeWriter, nil
}

func parseLogLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unsupported log level %q", level)
	}
}
