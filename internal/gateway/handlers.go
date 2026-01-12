package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// App holds the application state
type App struct {
	config         *Config
	cache          *Cache
	upstreamClient *UpstreamClient
}

// NewApp creates a new application instance
func NewApp(config *Config) (*App, error) {
	upstreamClient, err := NewUpstreamClient(config)
	if err != nil {
		return nil, err
	}

	cache := NewCache(config.GetCacheTTL())

	return &App{
		config:         config,
		cache:          cache,
		upstreamClient: upstreamClient,
	}, nil
}

// HandleOIDCDiscovery handles the /.well-known/openid-configuration endpoint
func (a *App) HandleOIDCDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	path := "/.well-known/openid-configuration"
	a.handleCachedEndpoint(w, r, path)
}

// HandleJWKS handles the /openid/v1/jwks endpoint
func (a *App) HandleJWKS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	path := "/openid/v1/jwks"
	a.handleCachedEndpoint(w, r, path)
}

// handleCachedEndpoint is a common handler for cached endpoints
func (a *App) handleCachedEndpoint(w http.ResponseWriter, r *http.Request, path string) {
	start := time.Now()
	var cacheHit bool
	var statusCode int

	defer func() {
		duration := time.Since(start)
		log.Printf("path=%s status=%d cache_hit=%v duration=%v", path, statusCode, cacheHit, duration)
	}()

	// Check cache first
	if cached, found := a.cache.Get(path); found {
		cacheHit = true
		statusCode = http.StatusOK
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", a.config.CacheTTLSeconds))
		w.WriteHeader(statusCode)
		w.Write(cached)
		return
	}

	// Cache miss - fetch from upstream
	cacheHit = false
	upstreamStart := time.Now()
	body, err := a.upstreamClient.Fetch(path)
	upstreamDuration := time.Since(upstreamStart)

	if err != nil {
		log.Printf("upstream_error: path=%s error=%v duration=%v", path, err, upstreamDuration)
		statusCode = http.StatusBadGateway
		http.Error(w, "Bad Gateway", statusCode)
		return
	}

	// Process the response
	var processedBody []byte
	if a.config.PrettyPrintJSON {
		// Parse and pretty-print JSON
		var jsonData interface{}
		if err := json.Unmarshal(body, &jsonData); err != nil {
			log.Printf("json_parse_error: path=%s error=%v", path, err)
			statusCode = http.StatusBadGateway
			http.Error(w, "Bad Gateway", statusCode)
			return
		}

		prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
		if err != nil {
			log.Printf("json_marshal_error: path=%s error=%v", path, err)
			statusCode = http.StatusInternalServerError
			http.Error(w, "Internal Server Error", statusCode)
			return
		}
		processedBody = prettyJSON
	} else {
		processedBody = body
	}

	// Store in cache
	a.cache.Set(path, processedBody)

	// Return response
	statusCode = http.StatusOK
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", a.config.CacheTTLSeconds))
	w.WriteHeader(statusCode)
	w.Write(processedBody)

	log.Printf("upstream_fetch: path=%s duration=%v", path, upstreamDuration)
}

// HandleHealthz handles the /healthz endpoint
// Liveness probe - fetches and caches both OIDC endpoints
func (a *App) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := a.populateCache(); err != nil {
		log.Printf("health check failed: %v", err)
		http.Error(w, "Service Unhealthy", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// HandleReadyz handles the /readyz endpoint
// Readiness probe - fetches and caches both OIDC endpoints
func (a *App) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := a.populateCache(); err != nil {
		log.Printf("readiness check failed: %v", err)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// HandleNotFound handles all other paths
func (a *App) HandleNotFound(w http.ResponseWriter, r *http.Request) {
	log.Printf("path=%s status=404 method=%s", r.URL.Path, r.Method)
	http.Error(w, "Not Found", http.StatusNotFound)
}

// populateCache fetches and caches both OIDC endpoints
func (a *App) populateCache() error {
	if a.upstreamClient == nil {
		return fmt.Errorf("upstream client not configured")
	}

	paths := []string{
		"/.well-known/openid-configuration",
		"/openid/v1/jwks",
	}

	for _, path := range paths {
		body, err := a.upstreamClient.Fetch(path)
		if err != nil {
			return err
		}
		a.cache.Set(path, body)
	}

	return nil
}
