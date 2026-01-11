package gateway

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
)

// UpstreamClient handles requests to the Kubernetes API server
type UpstreamClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewUpstreamClient creates a new upstream client configured for in-cluster access
func NewUpstreamClient(config *Config) (*UpstreamClient, error) {
	// Read the service account token
	tokenBytes, err := os.ReadFile(config.SATokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read service account token: %w", err)
	}
	token := string(tokenBytes)

	// Read the CA certificate
	caCert, err := os.ReadFile(config.SACACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// Create a certificate pool and add the CA
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Create TLS config
	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	// Create HTTP client with timeout and TLS config
	httpClient := &http.Client{
		Timeout: config.GetUpstreamTimeout(),
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	return &UpstreamClient{
		httpClient: httpClient,
		baseURL:    config.UpstreamHost,
		token:      token,
	}, nil
}

// Fetch retrieves data from the upstream path
func (u *UpstreamClient) Fetch(path string) ([]byte, error) {
	url := u.baseURL + path

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authorization header with service account token
	req.Header.Set("Authorization", "Bearer "+u.token)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// HealthCheck performs a basic connectivity check to the upstream
func (u *UpstreamClient) HealthCheck() error {
	// Try to fetch the well-known configuration as a health check
	_, err := u.Fetch("/.well-known/openid-configuration")
	return err
}
