package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any env vars that might interfere.
	for _, key := range []string{"DOMAIN", "SSH_PORT", "HTTP_PORT", "MAX_POST_LENGTH", "DATABASE_DSN"} {
		os.Unsetenv(key)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.Domain != "localhost" {
		t.Errorf("expected Domain=localhost, got %q", cfg.Domain)
	}
	if cfg.SSHPort != 6969 {
		t.Errorf("expected SSHPort=6969, got %d", cfg.SSHPort)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("expected HTTPPort=8080, got %d", cfg.HTTPPort)
	}
	if cfg.MaxPostLength != 500 {
		t.Errorf("expected MaxPostLength=500, got %d", cfg.MaxPostLength)
	}
	if cfg.InstanceName != "CLIverse" {
		t.Errorf("expected InstanceName=CLIverse, got %q", cfg.InstanceName)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	os.Setenv("DOMAIN", "example.com")
	os.Setenv("SSH_PORT", "2222")
	os.Setenv("MAX_POST_LENGTH", "1000")
	defer func() {
		os.Unsetenv("DOMAIN")
		os.Unsetenv("SSH_PORT")
		os.Unsetenv("MAX_POST_LENGTH")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	if cfg.Domain != "example.com" {
		t.Errorf("expected Domain=example.com, got %q", cfg.Domain)
	}
	if cfg.SSHPort != 2222 {
		t.Errorf("expected SSHPort=2222, got %d", cfg.SSHPort)
	}
	if cfg.MaxPostLength != 1000 {
		t.Errorf("expected MaxPostLength=1000, got %d", cfg.MaxPostLength)
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	cfg := &Config{
		Domain:        "example.com",
		DatabaseDSN:   "postgres://localhost/test",
		SessionSecret: "test-secret",
		SSHPort:       0,
		HTTPPort:      8080,
		MaxPostLength: 500,
	}
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for SSHPort=0")
	}

	cfg.SSHPort = 6969
	cfg.HTTPPort = 99999
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for HTTPPort=99999")
	}
}

func TestValidateRejectsEmptyDomain(t *testing.T) {
	cfg := &Config{
		Domain:        "",
		DatabaseDSN:   "postgres://localhost/test",
		SessionSecret: "test-secret",
		SSHPort:       6969,
		HTTPPort:      8080,
		MaxPostLength: 500,
	}
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for empty Domain")
	}
}

func TestValidateRejectsZeroPostLength(t *testing.T) {
	cfg := &Config{
		Domain:        "example.com",
		DatabaseDSN:   "postgres://localhost/test",
		SessionSecret: "test-secret",
		SSHPort:       6969,
		HTTPPort:      8080,
		MaxPostLength: 0,
	}
	if err := cfg.validate(); err == nil {
		t.Error("expected validation error for MaxPostLength=0")
	}
}

func TestValidateAcceptsGoodConfig(t *testing.T) {
	cfg := &Config{
		Domain:        "example.com",
		DatabaseDSN:   "postgres://localhost/test",
		SessionSecret: "a-good-secret",
		SSHPort:       6969,
		HTTPPort:      8080,
		MaxPostLength: 500,
	}
	if err := cfg.validate(); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}
