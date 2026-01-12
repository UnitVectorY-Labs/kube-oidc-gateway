package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlers(t *testing.T) {
	// Create a test app with mock upstream
	config := &Config{
		CacheTTLSeconds: 60,
		PrettyPrintJSON: true,
	}

	app := &App{
		config: config,
		cache:  NewCache(config.GetCacheTTL()),
	}

	t.Run("HandleHealthz returns 503 without upstream", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/healthz", nil)
		w := httptest.NewRecorder()

		app.HandleHealthz(w, req)

		// Without a mock upstream, healthz will fail
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503 without upstream, got %d", w.Code)
		}
	})

	t.Run("HandleHealthz rejects non-GET", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/healthz", nil)
		w := httptest.NewRecorder()

		app.HandleHealthz(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})

	t.Run("HandleReadyz returns 503 when upstream unavailable", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/readyz", nil)
		w := httptest.NewRecorder()

		app.HandleReadyz(w, req)

		// Without a mock upstream, readyz will fail
		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got %d", w.Code)
		}
	})

	t.Run("HandleNotFound returns 404", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/unknown-path", nil)
		w := httptest.NewRecorder()

		app.HandleNotFound(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("OIDC endpoints reject non-GET methods", func(t *testing.T) {
		tests := []struct {
			name    string
			handler func(http.ResponseWriter, *http.Request)
			path    string
		}{
			{"Discovery", app.HandleOIDCDiscovery, "/.well-known/openid-configuration"},
			{"JWKS", app.HandleJWKS, "/openid/v1/jwks"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("POST", tt.path, nil)
				w := httptest.NewRecorder()

				tt.handler(w, req)

				if w.Code != http.StatusMethodNotAllowed {
					t.Errorf("Expected status 405 for POST, got %d", w.Code)
				}
			})
		}
	})
}

func TestCacheIntegration(t *testing.T) {
	t.Run("Cache hit returns cached data", func(t *testing.T) {
		config := &Config{
			CacheTTLSeconds: 60,
			PrettyPrintJSON: false,
		}

		app := &App{
			config: config,
			cache:  NewCache(config.GetCacheTTL()),
		}

		// Pre-populate cache
		testData := []byte(`{"test": "cached"}`)
		app.cache.Set("/.well-known/openid-configuration", testData)

		req := httptest.NewRequest("GET", "/.well-known/openid-configuration", nil)
		w := httptest.NewRecorder()

		app.HandleOIDCDiscovery(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if w.Body.String() != string(testData) {
			t.Errorf("Expected cached data, got %s", w.Body.String())
		}
		if w.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
		}
	})

	t.Run("Cache response includes ETag header", func(t *testing.T) {
		config := &Config{
			CacheTTLSeconds: 60,
			PrettyPrintJSON: false,
		}

		app := &App{
			config: config,
			cache:  NewCache(config.GetCacheTTL()),
		}

		// Pre-populate cache
		testData := []byte(`{"test": "etag"}`)
		app.cache.Set("/.well-known/openid-configuration", testData)

		req := httptest.NewRequest("GET", "/.well-known/openid-configuration", nil)
		w := httptest.NewRecorder()

		app.HandleOIDCDiscovery(w, req)

		etag := w.Header().Get("ETag")
		if etag == "" {
			t.Error("Expected ETag header to be set")
		}
		// ETag should be in quoted format
		if len(etag) < 3 || etag[0] != '"' || etag[len(etag)-1] != '"' {
			t.Errorf("Expected ETag to be quoted, got %s", etag)
		}
	})

	t.Run("Same content produces same ETag", func(t *testing.T) {
		config := &Config{
			CacheTTLSeconds: 60,
			PrettyPrintJSON: false,
		}

		app := &App{
			config: config,
			cache:  NewCache(config.GetCacheTTL()),
		}

		testData := []byte(`{"test": "same"}`)
		app.cache.Set("/.well-known/openid-configuration", testData)

		req1 := httptest.NewRequest("GET", "/.well-known/openid-configuration", nil)
		w1 := httptest.NewRecorder()
		app.HandleOIDCDiscovery(w1, req1)
		etag1 := w1.Header().Get("ETag")

		req2 := httptest.NewRequest("GET", "/.well-known/openid-configuration", nil)
		w2 := httptest.NewRecorder()
		app.HandleOIDCDiscovery(w2, req2)
		etag2 := w2.Header().Get("ETag")

		if etag1 != etag2 {
			t.Errorf("Expected same ETag for same content, got %s and %s", etag1, etag2)
		}
	})
}
