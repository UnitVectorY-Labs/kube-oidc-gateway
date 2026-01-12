package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// Create HTTP server with timeouts
	addr := fmt.Sprintf("%s:%s", config.ListenAddr, config.ListenPort)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("Listening on %s", addr)
		serverErrors <- server.ListenAndServe()
	}()

	// Listen for shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Block until a signal is received or server error
	select {
	case err := <-serverErrors:
		log.Printf("Server error: %v", err)
		os.Exit(1)
	case sig := <-shutdown:
		log.Printf("Received shutdown signal: %v. Starting graceful shutdown...", sig)

		// Give outstanding requests a deadline for completion
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Perform graceful shutdown
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			// Force close
			if err := server.Close(); err != nil {
				log.Printf("Failed to close server: %v", err)
			}
			os.Exit(1)
		}

		log.Printf("Graceful shutdown completed")
	}
}
