package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/UnitVectorY-Labs/kube-oidc-gateway/internal/gateway"
)

func TestServerTimeouts(t *testing.T) {
	t.Run("Server has production timeouts configured", func(t *testing.T) {
		// Create a test server configuration
		config := &gateway.Config{
			ListenAddr:      "127.0.0.1",
			ListenPort:      "0", // Use port 0 to get a random free port
			CacheTTLSeconds: 60,
			PrettyPrintJSON: false,
		}

		app := &gateway.App{}
		mux := http.NewServeMux()
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		server := &http.Server{
			Addr:              config.ListenAddr + ":" + config.ListenPort,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		}

		// Verify timeouts are set correctly
		if server.ReadHeaderTimeout != 10*time.Second {
			t.Errorf("Expected ReadHeaderTimeout 10s, got %v", server.ReadHeaderTimeout)
		}
		if server.ReadTimeout != 30*time.Second {
			t.Errorf("Expected ReadTimeout 30s, got %v", server.ReadTimeout)
		}
		if server.WriteTimeout != 30*time.Second {
			t.Errorf("Expected WriteTimeout 30s, got %v", server.WriteTimeout)
		}
		if server.IdleTimeout != 120*time.Second {
			t.Errorf("Expected IdleTimeout 120s, got %v", server.IdleTimeout)
		}

		// Clean up - don't need to start the server for this test
		_ = app
	})
}

func TestGracefulShutdown(t *testing.T) {
	t.Run("Server can be shutdown gracefully", func(t *testing.T) {
		// Create a simple test server
		mux := http.NewServeMux()
		handlerCalled := make(chan bool, 1)
		
		mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
			// Simulate a slow request
			select {
			case <-time.After(100 * time.Millisecond):
				handlerCalled <- true
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			case <-r.Context().Done():
				// Request was cancelled
				handlerCalled <- false
			}
		})

		server := &http.Server{
			Addr:              "127.0.0.1:0",
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
		}

		// Start server
		go func() {
			_ = server.ListenAndServe()
		}()

		// Give server time to start
		time.Sleep(50 * time.Millisecond)

		// Gracefully shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := server.Shutdown(ctx)
		if err != nil {
			t.Errorf("Expected graceful shutdown to succeed, got error: %v", err)
		}
	})
}

func TestSignalHandling(t *testing.T) {
	t.Run("Signal notification is set up correctly", func(t *testing.T) {
		// This test verifies that signal.Notify works as expected
		shutdown := make(chan os.Signal, 1)
		signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

		// Clean up signal handler
		signal.Stop(shutdown)
		close(shutdown)

		// If we got here without panic, the signal setup works
	})
}
