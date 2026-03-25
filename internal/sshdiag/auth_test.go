package sshdiag

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestBuildAuthPlansAutoOrdersKeyThenKeyboardInteractiveThenPassword(t *testing.T) {
	keyPath := writePrivateKeyFile(t, false, "")

	plans, err := buildAuthPlans(EndpointConfig{
		Label:    "target",
		Host:     "10.0.0.1",
		Username: "tester",
		Password: "secret",
		KeyPath:  keyPath,
		AuthMode: AuthModeAuto,
	}, nil)
	if err != nil {
		t.Fatalf("buildAuthPlans returned error: %v", err)
	}

	got := planModes(plans)
	want := []AuthMode{AuthModeKey, AuthModeKeyboardInteractive, AuthModePassword}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected auth plan order: got %v want %v", got, want)
	}
}

func TestBuildAuthPlansAutoWithPasswordOnlyPrefersKeyboardInteractiveThenPassword(t *testing.T) {
	plans, err := buildAuthPlans(EndpointConfig{
		Label:    "target",
		Host:     "10.0.0.1",
		Username: "tester",
		Password: "secret",
		AuthMode: AuthModeAuto,
	}, nil)
	if err != nil {
		t.Fatalf("buildAuthPlans returned error: %v", err)
	}

	got := planModes(plans)
	want := []AuthMode{AuthModeKeyboardInteractive, AuthModePassword}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected auth plan order: got %v want %v", got, want)
	}
}

func TestBuildAuthPlansPasswordModeRequiresPassword(t *testing.T) {
	_, err := buildAuthPlans(EndpointConfig{
		Label:    "target",
		Host:     "10.0.0.1",
		Username: "tester",
		AuthMode: AuthModePassword,
	}, nil)
	if err == nil {
		t.Fatal("expected missing password error")
	}
	if !strings.Contains(err.Error(), "password is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildAuthPlansSupportsEncryptedKeyWithPassphrase(t *testing.T) {
	keyPath := writePrivateKeyFile(t, true, "hunter2")

	plans, err := buildAuthPlans(EndpointConfig{
		Label:         "target",
		Host:          "10.0.0.1",
		Username:      "tester",
		KeyPath:       keyPath,
		KeyPassphrase: "hunter2",
		AuthMode:      AuthModeKey,
	}, nil)
	if err != nil {
		t.Fatalf("buildAuthPlans returned error: %v", err)
	}

	if len(plans) != 1 {
		t.Fatalf("expected 1 auth plan, got %d", len(plans))
	}
	if plans[0].SetupErr != nil {
		t.Fatalf("expected key plan to be ready, got setup error: %v", plans[0].SetupErr)
	}
}

func TestBuildAuthPlansReportsHelpfulErrorForEncryptedKeyWithoutPassphrase(t *testing.T) {
	keyPath := writePrivateKeyFile(t, true, "hunter2")

	plans, err := buildAuthPlans(EndpointConfig{
		Label:    "target",
		Host:     "10.0.0.1",
		Username: "tester",
		KeyPath:  keyPath,
		AuthMode: AuthModeKey,
	}, nil)
	if err != nil {
		t.Fatalf("buildAuthPlans returned unexpected error: %v", err)
	}

	if len(plans) != 1 {
		t.Fatalf("expected 1 auth plan, got %d", len(plans))
	}
	if plans[0].SetupErr == nil {
		t.Fatal("expected key setup error for missing passphrase")
	}
	if !strings.Contains(plans[0].SetupErr.Error(), "passphrase") {
		t.Fatalf("unexpected setup error: %v", plans[0].SetupErr)
	}
}

func TestKeyboardInteractiveAuthRepeatsSecretForEveryPrompt(t *testing.T) {
	method := keyboardInteractiveAuth("target", "super-secret", nil)
	challenge, ok := method.(ssh.KeyboardInteractiveChallenge)
	if !ok {
		t.Fatalf("expected keyboard-interactive challenge, got %T", method)
	}

	answers, err := challenge("tester", "prompt", []string{"Password:", "OTP:"}, []bool{false, false})
	if err != nil {
		t.Fatalf("keyboardInteractiveAuth returned error: %v", err)
	}

	want := []string{"super-secret", "super-secret"}
	if !reflect.DeepEqual(answers, want) {
		t.Fatalf("unexpected answers: got %v want %v", answers, want)
	}
}

func planModes(plans []authPlan) []AuthMode {
	modes := make([]AuthMode, 0, len(plans))
	for _, plan := range plans {
		modes = append(modes, plan.Mode)
	}
	return modes
}

func writePrivateKeyFile(t *testing.T, encrypted bool, passphrase string) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	privateKey := x509.MarshalPKCS1PrivateKey(key)
	var block *pem.Block
	if encrypted {
		block, err = x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY", privateKey, []byte(passphrase), x509.PEMCipherAES256)
		if err != nil {
			t.Fatalf("failed to encrypt pem block: %v", err)
		}
	} else {
		block = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKey}
	}

	path := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatalf("failed to write key file: %v", err)
	}
	return path
}
