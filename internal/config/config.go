package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	// Instance
	Domain       string
	InstanceName string
	InstanceDesc string

	// Ports
	SSHPort  int
	HTTPPort int

	// Database
	DatabaseDSN string

	// Redis
	RedisURL string

	// Security
	AdminPasswordHash string
	SessionSecret     string

	// Timeouts
	SSHIdleTimeout   time.Duration
	HTTPReadTimeout  time.Duration
	HTTPWriteTimeout time.Duration

	// Limits
	MaxConnections int
	MaxPostLength  int
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetEnvPrefix("")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	v.SetDefault("DOMAIN", "localhost")
	v.SetDefault("INSTANCE_NAME", "CLIverse")
	v.SetDefault("INSTANCE_DESC", "A CLIverse Fediverse instance")
	v.SetDefault("SSH_PORT", 6969)
	v.SetDefault("HTTP_PORT", 8080)
	v.SetDefault("DATABASE_DSN", "postgres://cliverse:cliverse@localhost:5432/cliverse?sslmode=disable")
	v.SetDefault("REDIS_URL", "redis://localhost:6379/0")
	v.SetDefault("SESSION_SECRET", "changeme-please-use-a-long-random-string")
	v.SetDefault("SSH_IDLE_TIMEOUT", "30m")
	v.SetDefault("HTTP_READ_TIMEOUT", "30s")
	v.SetDefault("HTTP_WRITE_TIMEOUT", "30s")
	v.SetDefault("MAX_CONNECTIONS", 1000)
	v.SetDefault("MAX_POST_LENGTH", 500)

	idleTimeout, err := time.ParseDuration(v.GetString("SSH_IDLE_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("invalid SSH_IDLE_TIMEOUT: %w", err)
	}

	readTimeout, err := time.ParseDuration(v.GetString("HTTP_READ_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP_READ_TIMEOUT: %w", err)
	}

	writeTimeout, err := time.ParseDuration(v.GetString("HTTP_WRITE_TIMEOUT"))
	if err != nil {
		return nil, fmt.Errorf("invalid HTTP_WRITE_TIMEOUT: %w", err)
	}

	return &Config{
		Domain:            v.GetString("DOMAIN"),
		InstanceName:      v.GetString("INSTANCE_NAME"),
		InstanceDesc:      v.GetString("INSTANCE_DESC"),
		SSHPort:           v.GetInt("SSH_PORT"),
		HTTPPort:          v.GetInt("HTTP_PORT"),
		DatabaseDSN:       v.GetString("DATABASE_DSN"),
		RedisURL:          v.GetString("REDIS_URL"),
		AdminPasswordHash: v.GetString("ADMIN_PASSWORD_HASH"),
		SessionSecret:     v.GetString("SESSION_SECRET"),
		SSHIdleTimeout:    idleTimeout,
		HTTPReadTimeout:   readTimeout,
		HTTPWriteTimeout:  writeTimeout,
		MaxConnections:    v.GetInt("MAX_CONNECTIONS"),
		MaxPostLength:     v.GetInt("MAX_POST_LENGTH"),
	}, nil
}
