package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/UnitVectorY-Labs/kube-oidc-gateway/internal/gateway"
)

func main() {
	// Load configuration
	config := gateway.LoadConfig()

	// Set up logging
	log.SetFlags(log.LstdFlags | log.LUTC)
	log.Printf("Starting kube-oidc-gateway")
	log.Printf("Config: listen=%s:%s upstream=%s cache_ttl=%ds pretty_print=%v",
		config.ListenAddr, config.ListenPort, config.UpstreamHost,
		config.CacheTTLSeconds, config.PrettyPrintJSON)

	// Create application
	app, err := gateway.NewApp(config)
	if err != nil {
		log.Printf("Failed to initialize application: %v", err)
		os.Exit(1)
	}

	// Set up HTTP routes
	mux := http.NewServeMux()

	// OIDC endpoints
	mux.HandleFunc("/.well-known/openid-configuration", app.HandleOIDCDiscovery)
	mux.HandleFunc("/openid/v1/jwks", app.HandleJWKS)

	// Health endpoints
	mux.HandleFunc("/healthz", app.HandleHealthz)
	mux.HandleFunc("/readyz", app.HandleReadyz)

	// Catch-all for 404
	mux.HandleFunc("/", app.HandleNotFound)

	// Start server
	addr := fmt.Sprintf("%s:%s", config.ListenAddr, config.ListenPort)
	log.Printf("Listening on %s", addr)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}
