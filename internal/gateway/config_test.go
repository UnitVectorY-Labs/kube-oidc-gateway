package gateway

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	t.Run("Default values", func(t *testing.T) {
		// Clear env vars
		os.Clearenv()

		config := LoadConfig()

		if config.ListenAddr != "0.0.0.0" {
			t.Errorf("Expected ListenAddr 0.0.0.0, got %s", config.ListenAddr)
		}
		if config.ListenPort != "8080" {
			t.Errorf("Expected ListenPort 8080, got %s", config.ListenPort)
		}
		if config.UpstreamHost != "https://kubernetes.default.svc" {
			t.Errorf("Expected UpstreamHost https://kubernetes.default.svc, got %s", config.UpstreamHost)
		}
		if config.UpstreamTimeoutSeconds != 5 {
			t.Errorf("Expected UpstreamTimeoutSeconds 5, got %d", config.UpstreamTimeoutSeconds)
		}
		if config.CacheTTLSeconds != 60 {
			t.Errorf("Expected CacheTTLSeconds 60, got %d", config.CacheTTLSeconds)
		}
		if !config.PrettyPrintJSON {
			t.Error("Expected PrettyPrintJSON to be true by default")
		}
	})

	t.Run("Custom environment values", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("LISTEN_ADDR", "127.0.0.1")
		os.Setenv("LISTEN_PORT", "9090")
		os.Setenv("UPSTREAM_HOST", "https://custom-api-server")
		os.Setenv("UPSTREAM_TIMEOUT_SECONDS", "10")
		os.Setenv("CACHE_TTL_SECONDS", "120")
		os.Setenv("PRETTY_PRINT_JSON", "false")
		os.Setenv("LOG_LEVEL", "debug")

		config := LoadConfig()

		if config.ListenAddr != "127.0.0.1" {
			t.Errorf("Expected ListenAddr 127.0.0.1, got %s", config.ListenAddr)
		}
		if config.ListenPort != "9090" {
			t.Errorf("Expected ListenPort 9090, got %s", config.ListenPort)
		}
		if config.UpstreamHost != "https://custom-api-server" {
			t.Errorf("Expected custom UpstreamHost, got %s", config.UpstreamHost)
		}
		if config.UpstreamTimeoutSeconds != 10 {
			t.Errorf("Expected UpstreamTimeoutSeconds 10, got %d", config.UpstreamTimeoutSeconds)
		}
		if config.CacheTTLSeconds != 120 {
			t.Errorf("Expected CacheTTLSeconds 120, got %d", config.CacheTTLSeconds)
		}
		if config.PrettyPrintJSON {
			t.Error("Expected PrettyPrintJSON to be false")
		}
		if config.LogLevel != "debug" {
			t.Errorf("Expected LogLevel debug, got %s", config.LogLevel)
		}
	})

	t.Run("Duration conversions", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("CACHE_TTL_SECONDS", "120")
		os.Setenv("UPSTREAM_TIMEOUT_SECONDS", "10")

		config := LoadConfig()

		if config.GetCacheTTL() != 120*time.Second {
			t.Errorf("Expected cache TTL 120s, got %v", config.GetCacheTTL())
		}
		if config.GetUpstreamTimeout() != 10*time.Second {
			t.Errorf("Expected upstream timeout 10s, got %v", config.GetUpstreamTimeout())
		}
	})

	t.Run("Invalid integer falls back to default", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("CACHE_TTL_SECONDS", "invalid")
		os.Setenv("UPSTREAM_TIMEOUT_SECONDS", "not-a-number")

		config := LoadConfig()

		if config.CacheTTLSeconds != 60 {
			t.Errorf("Expected default CacheTTLSeconds 60, got %d", config.CacheTTLSeconds)
		}
		if config.UpstreamTimeoutSeconds != 5 {
			t.Errorf("Expected default UpstreamTimeoutSeconds 5, got %d", config.UpstreamTimeoutSeconds)
		}
	})

	t.Run("Invalid boolean falls back to default", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("PRETTY_PRINT_JSON", "not-a-bool")

		config := LoadConfig()

		if !config.PrettyPrintJSON {
			t.Error("Expected default PrettyPrintJSON to be true")
		}
	})
}
