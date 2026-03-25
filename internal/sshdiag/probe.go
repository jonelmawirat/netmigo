package sshdiag

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/jonelmawirat/netmigo/netmigo/repository"
	"golang.org/x/crypto/ssh"
)

const (
	defaultCommandTimeout      = 5 * time.Second
	defaultCommandFirstByteTTL = 30 * time.Second
)

type ProbeConfig struct {
	Target             EndpointConfig
	Jump               *EndpointConfig
	Command            string
	CommandTimeout     time.Duration
	CommandFirstByteTT time.Duration
}

type AttemptResult struct {
	Mode     AuthMode `json:"mode"`
	Try      int      `json:"try"`
	Stage    string   `json:"stage"`
	Success  bool     `json:"success"`
	Duration string   `json:"duration,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type EndpointResult struct {
	Label          string          `json:"label"`
	Address        string          `json:"address"`
	Username       string          `json:"username"`
	RequestedMode  AuthMode        `json:"requested_mode"`
	AttemptedModes []AuthMode      `json:"attempted_modes"`
	Attempts       []AttemptResult `json:"attempts"`
	SuccessfulMode AuthMode        `json:"successful_mode,omitempty"`
	Error          string          `json:"error,omitempty"`
}

type CommandResult struct {
	Command    string `json:"command"`
	OutputFile string `json:"output_file,omitempty"`
	Error      string `json:"error,omitempty"`
}

type Result struct {
	StartedAt    time.Time       `json:"started_at"`
	FinishedAt   time.Time       `json:"finished_at"`
	Duration     string          `json:"duration"`
	Success      bool            `json:"success"`
	FailureStage string          `json:"failure_stage,omitempty"`
	Error        string          `json:"error,omitempty"`
	UsedJump     bool            `json:"used_jump"`
	Jump         *EndpointResult `json:"jump,omitempty"`
	Target       EndpointResult  `json:"target"`
	Command      *CommandResult  `json:"command,omitempty"`
}

func (r Result) JSON() string {
	payload, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Sprintf("{\"success\":false,\"error\":%q}", err.Error())
	}
	return string(payload)
}

func Run(cfg ProbeConfig, logger *slog.Logger) (*Result, error) {
	started := time.Now()
	result := &Result{
		StartedAt: started,
		UsedJump:  cfg.Jump != nil,
	}

	targetCfg := cfg.Target.withDefaults()
	targetCfg.Label = "target"
	if err := targetCfg.Validate(); err != nil {
		return failResult(result, "validation", err), err
	}

	var jumpCfg *EndpointConfig
	if cfg.Jump != nil {
		j := cfg.Jump.withDefaults()
		j.Label = "jump"
		if err := j.Validate(); err != nil {
			return failResult(result, "validation", err), err
		}
		jumpCfg = &j
	}

	var jumpClient *ssh.Client
	if jumpCfg != nil {
		jumpResult, client, err := connectEndpoint(*jumpCfg, nil, logger)
		result.Jump = jumpResult
		if err != nil {
			return failResult(result, "jump_connect", err), err
		}
		jumpClient = client
		defer jumpClient.Close()
	}

	targetResult, targetClient, err := connectEndpoint(targetCfg, jumpClient, logger)
	result.Target = *targetResult
	if err != nil {
		return failResult(result, "target_connect", err), err
	}
	defer targetClient.Close()

	if cfg.Command != "" {
		commandResult := &CommandResult{Command: cfg.Command}
		result.Command = commandResult

		commandTimeout := cfg.CommandTimeout
		if commandTimeout <= 0 {
			commandTimeout = defaultCommandTimeout
		}
		firstByteTTL := cfg.CommandFirstByteTT
		if firstByteTTL <= 0 {
			firstByteTTL = defaultCommandFirstByteTTL
		}

		logger.Info("Running post-auth command probe",
			"command", cfg.Command,
			"timeout", commandTimeout,
			"firstByteTimeout", firstByteTTL,
		)

		outputFile, err := repository.ExecutorInteractiveExecute(targetClient, logger, cfg.Command, firstByteTTL, commandTimeout)
		if err != nil {
			commandResult.Error = err.Error()
			return failResult(result, "command_probe", err), err
		}
		commandResult.OutputFile = outputFile
	}

	result.Success = true
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt).String()
	return result, nil
}

func failResult(result *Result, stage string, err error) *Result {
	result.Success = false
	result.FailureStage = stage
	result.Error = err.Error()
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt).String()
	return result
}

func connectEndpoint(cfg EndpointConfig, jumpClient *ssh.Client, logger *slog.Logger) (*EndpointResult, *ssh.Client, error) {
	cfg = cfg.withDefaults()
	result := &EndpointResult{
		Label:         cfg.Label,
		Address:       cfg.Address(),
		Username:      cfg.Username,
		RequestedMode: cfg.AuthMode,
	}

	plans, err := buildAuthPlans(cfg, logger)
	if err != nil {
		result.Error = err.Error()
		return result, nil, err
	}

	for _, plan := range plans {
		result.AttemptedModes = append(result.AttemptedModes, plan.Mode)
		if plan.SetupErr != nil {
			result.Attempts = append(result.Attempts, AttemptResult{
				Mode:    plan.Mode,
				Try:     1,
				Stage:   "auth_setup",
				Success: false,
				Error:   plan.SetupErr.Error(),
			})
			logger.Warn("Skipping auth mode because setup failed",
				"endpoint", cfg.Label,
				"mode", plan.Mode,
				"error", plan.SetupErr,
			)
			continue
		}

		for attempt := 1; attempt <= cfg.Retries; attempt++ {
			logger.Info("Dialing SSH endpoint",
				"endpoint", cfg.Label,
				"address", cfg.Address(),
				"mode", plan.Mode,
				"attempt", attempt,
				"maxAttempts", cfg.Retries,
				"viaJump", jumpClient != nil,
			)

			started := time.Now()
			client, err := dialWithAuth(cfg, plan.Methods, jumpClient)
			duration := time.Since(started)
			attemptResult := AttemptResult{
				Mode:     plan.Mode,
				Try:      attempt,
				Stage:    "connect",
				Duration: duration.String(),
			}
			if err == nil {
				attemptResult.Success = true
				result.Attempts = append(result.Attempts, attemptResult)
				result.SuccessfulMode = plan.Mode
				return result, client, nil
			}

			attemptResult.Success = false
			attemptResult.Error = err.Error()
			result.Attempts = append(result.Attempts, attemptResult)
			result.Error = err.Error()

			logger.Warn("SSH dial failed",
				"endpoint", cfg.Label,
				"address", cfg.Address(),
				"mode", plan.Mode,
				"attempt", attempt,
				"error", err,
			)

			if attempt < cfg.Retries {
				time.Sleep(time.Second)
			}
		}
	}

	return result, nil, fmt.Errorf("%s connection failed after %d auth mode(s)", cfg.Label, len(plans))
}

func dialWithAuth(cfg EndpointConfig, methods []ssh.AuthMethod, jumpClient *ssh.Client) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            methods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.ConnectionTimeout,
	}

	address := cfg.Address()
	if jumpClient == nil {
		return ssh.Dial("tcp", address, sshConfig)
	}

	connChan := make(chan net.Conn, 1)
	errChan := make(chan error, 1)

	go func() {
		conn, err := jumpClient.Dial("tcp", address)
		if err != nil {
			errChan <- err
			return
		}
		connChan <- conn
	}()

	var timeout <-chan time.Time
	if cfg.ConnectionTimeout > 0 {
		timeout = time.After(cfg.ConnectionTimeout)
	}

	var netConn net.Conn
	select {
	case conn := <-connChan:
		netConn = conn
	case err := <-errChan:
		return nil, fmt.Errorf("jump server dial error: %w", err)
	case <-timeout:
		return nil, fmt.Errorf("timed out connecting to %s via jump server after %s", address, cfg.ConnectionTimeout)
	}

	clientConn, chans, reqs, err := ssh.NewClientConn(netConn, address, sshConfig)
	if err != nil {
		netConn.Close()
		return nil, fmt.Errorf("new client conn error: %w", err)
	}

	return ssh.NewClient(clientConn, chans, reqs), nil
}
