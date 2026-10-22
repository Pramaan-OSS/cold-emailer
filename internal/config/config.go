package config

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pramaan-oss/cold-email/internal/infrastructure/parameterstore"
	"github.com/pramaan-oss/cold-email/internal/infrastructure/secrets"
)

type Config struct {
	Env                 string // "dev" or "prod"
	RemoteConfigEnabled bool
	params              *parameterstore.ParameterStore
	secrets             *secrets.SecretsManagerClient
}

// NewConfig creates a new config instance with env-first loading and optional AWS fallback.
// AWS clients are only initialized if AWS_CONFIG_ENABLED=true.
func NewConfig(ctx context.Context) (*Config, error) {
	return &Config{Env: getEnvOrDefault("APP_ENV", "dev")}, nil
}

// Load creates a Config with pre-initialized AWS clients (legacy compatibility).
// Deprecated: Use NewConfig for new code.
func Load(params *parameterstore.ParameterStore, secrets *secrets.SecretsManagerClient) *Config {
	env := getEnvOrDefault("APP_ENV", "dev")

	return &Config{
		Env:                 env,
		RemoteConfigEnabled: true, // Legacy remote configuration is optional
		params:              params,
		secrets:             secrets,
	}
}

// GetKeyID returns the full AWS parameter/secret path with environment prefix.
func (c *Config) GetKeyID(s string) string {
	return "/coldemail/" + c.Env + "/" + s
}

// getEnvOrDefault returns the environment variable value or a default if not set.
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// GetString retrieves a configuration value, checking env var first, then AWS SSM if enabled.
func (c *Config) GetString(ctx context.Context, envKey, parameterKey string) (string, error) {
	if val := os.Getenv(envKey); val != "" {
		return val, nil
	}
	return "", fmt.Errorf("config %s not found (env: %s, parameter: %s)", envKey, envKey, parameterKey)
}

// GetStringRaw retrieves a configuration value from AWS SSM using the raw key (no environment prefix).
func (c *Config) GetStringRaw(ctx context.Context, envKey, parameterKey string) (string, error) {
	if val := os.Getenv(envKey); val != "" {
		return val, nil
	}
	return "", fmt.Errorf("config %s not found (env: %s, parameter: %s)", envKey, envKey, parameterKey)
}

// GetSecret retrieves a secret value, checking env var first, then AWS Secrets Manager if enabled.
func (c *Config) GetSecret(ctx context.Context, envKey, parameterKey string) (string, error) {
	if val := os.Getenv(envKey); val != "" {
		return val, nil
	}
	return "", fmt.Errorf("secret %s not found (env: %s, parameter: %s)", envKey, envKey, parameterKey)
}

// GetSecretRaw retrieves a secret value from AWS Secrets Manager using the raw key (no environment prefix).
func (c *Config) GetSecretRaw(ctx context.Context, envKey, parameterKey string) (string, error) {
	if val := os.Getenv(envKey); val != "" {
		return val, nil
	}
	return "", fmt.Errorf("secret %s not found (env: %s, parameter: %s)", envKey, envKey, parameterKey)
}

// GetStringOptional retrieves an optional configuration value with a default fallback.
func (c *Config) GetStringOptional(ctx context.Context, envKey, parameterKey, defaultVal string) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	if c.RemoteConfigEnabled && c.params != nil {
		if val, err := c.params.Get(ctx, c.GetKeyID(parameterKey)); err == nil && val != "" {
			return val
		}
	}
	return defaultVal
}

// GetBoolOptional retrieves an optional boolean configuration value, parsed
// with strconv.ParseBool ("1", "t", "true", "0", "false", ...). An unset or
// unparseable value returns the default.
func (c *Config) GetBoolOptional(ctx context.Context, envKey, parameterKey string, defaultVal bool) bool {
	raw := c.GetStringOptional(ctx, envKey, parameterKey, "")
	if raw == "" {
		return defaultVal
	}
	if b, err := strconv.ParseBool(strings.TrimSpace(raw)); err == nil {
		return b
	}
	return defaultVal
}

// GetBoolPtr retrieves an optional boolean, returning nil when the value is
// unset (rather than a default), so a caller can distinguish "not configured"
// from an explicit false and fall back to its own default.
func (c *Config) GetBoolPtr(ctx context.Context, envKey, parameterKey string) *bool {
	raw := c.GetStringOptional(ctx, envKey, parameterKey, "")
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	b, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	return &b
}

// GetStringOptionalRaw retrieves an optional configuration value using raw AWS key.
func (c *Config) GetStringOptionalRaw(ctx context.Context, envKey, parameterKey, defaultVal string) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	if c.RemoteConfigEnabled && c.params != nil {
		if val, err := c.params.Get(ctx, parameterKey); err == nil && val != "" {
			return val
		}
	}
	return defaultVal
}

// GetSecretOptional retrieves an optional secret value with a default fallback.
func (c *Config) GetSecretOptional(ctx context.Context, envKey, parameterKey, defaultVal string) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	if c.RemoteConfigEnabled && c.secrets != nil {
		if val, err := c.secrets.Get(ctx, c.GetKeyID(parameterKey)); err == nil && val != "" {
			return val
		}
	}
	return defaultVal
}

// GetSecretOptionalRaw retrieves an optional secret value using raw AWS key.
func (c *Config) GetSecretOptionalRaw(ctx context.Context, envKey, parameterKey, defaultVal string) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	if c.RemoteConfigEnabled && c.secrets != nil {
		if val, err := c.secrets.Get(ctx, parameterKey); err == nil && val != "" {
			return val
		}
	}
	return defaultVal
}
