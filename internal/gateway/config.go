package gateway

import (
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	ListenAddr             string
	ListenPort             string
	UpstreamHost           string
	UpstreamTimeoutSeconds int
	CacheTTLSeconds        int
	PrettyPrintJSON        bool
	SATokenPath            string
	SACACertPath           string
}

// LoadConfig loads configuration from environment variables with safe defaults
func LoadConfig() *Config {
	return &Config{
		ListenAddr:             getEnv("LISTEN_ADDR", "0.0.0.0"),
		ListenPort:             getEnv("LISTEN_PORT", "8080"),
		UpstreamHost:           getEnv("UPSTREAM_HOST", "https://kubernetes.default.svc"),
		UpstreamTimeoutSeconds: getEnvAsInt("UPSTREAM_TIMEOUT_SECONDS", 5),
		CacheTTLSeconds:        getEnvAsInt("CACHE_TTL_SECONDS", 60),
		PrettyPrintJSON:        getEnvAsBool("PRETTY_PRINT_JSON", true),
		SATokenPath:            getEnv("SA_TOKEN_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/token"),
		SACACertPath:           getEnv("SA_CA_CERT_PATH", "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"),
	}
}

// GetCacheTTL returns the cache TTL as a duration
func (c *Config) GetCacheTTL() time.Duration {
	return time.Duration(c.CacheTTLSeconds) * time.Second
}

// GetUpstreamTimeout returns the upstream timeout as a duration
func (c *Config) GetUpstreamTimeout() time.Duration {
	return time.Duration(c.UpstreamTimeoutSeconds) * time.Second
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
