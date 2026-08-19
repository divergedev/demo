package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestGateway(t *testing.T) {
	// Set up mock backends
	paymentsApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(map[string]string{"version": "baseline-payments"})
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("payments-baseline"))
	}))
	defer paymentsApi.Close()

	// Separate preview backend for dev endpoint routing tests
	previewApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(map[string]string{"version": "preview-payments"})
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("payments-preview"))
	}))
	defer previewApi.Close()

	paymentsModule := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(map[string]string{"version": "baseline-module"})
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("module-baseline"))
	}))
	defer paymentsModule.Close()

	accountsApi := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(map[string]string{"version": "baseline-accounts"})
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("accounts-baseline"))
	}))
	defer accountsApi.Close()

	webApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(map[string]string{"version": "baseline-web"})
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("web-baseline"))
	}))
	defer webApp.Close()

	// Set env vars
	os.Setenv("PAYMENTS_API_URL", paymentsApi.URL)
	os.Setenv("ACCOUNTS_API_URL", accountsApi.URL)
	os.Setenv("PAYMENTS_MODULE_URL", paymentsModule.URL)
	os.Setenv("WEB_APP_URL", webApp.URL)
	os.Setenv("DIVERGE_HEADER_KEY", "x-test-preview")
	os.Setenv("PORT", "8080")
	defer os.Clearenv()

	handler := setupGateway()
	server := httptest.NewServer(handler)
	defer server.Close()

	t.Run("resolveTarget fallback (no header)", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), "GET", server.URL+"/api/payments", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		buf, _ := io.ReadAll(resp.Body)
		if string(buf) != "payments-baseline" {
			t.Errorf("Expected payments-baseline, got %s", string(buf))
		}
	})

	t.Run("Header key configuration - module registry", func(t *testing.T) {
		// Mock dev endpoint so preview routing works without DNS
		os.Setenv("DIVERGE_DEV_MODULE_ENDPOINT", "localhost:9999")
		defer os.Unsetenv("DIVERGE_DEV_MODULE_ENDPOINT")

		req, _ := http.NewRequestWithContext(t.Context(), "GET", server.URL+"/api/module-registry", nil)
		req.Header.Set("x-test-preview", "some-preview-id")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		var result struct {
			Modules map[string]struct {
				Version string `json:"version"`
			} `json:"modules"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Modules["payments"].Version != "local-dev" {
			t.Errorf("Expected local-dev version, got %s", result.Modules["payments"].Version)
		}
	})

	t.Run("Header key configuration - API proxy routing", func(t *testing.T) {
		// Set dev endpoint to preview backend
		os.Setenv("DIVERGE_DEV_ENDPOINT", previewApi.Listener.Addr().String())
		os.Setenv("DIVERGE_DEV_PREVIEW_ID", "fraud-detection")
		defer os.Unsetenv("DIVERGE_DEV_ENDPOINT")
		defer os.Unsetenv("DIVERGE_DEV_PREVIEW_ID")

		req, _ := http.NewRequestWithContext(t.Context(), "GET", server.URL+"/api/payments", nil)
		req.Header.Set("x-test-preview", "fraud-detection")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		buf, _ := io.ReadAll(resp.Body)
		if string(buf) != "payments-preview" {
			t.Errorf("Expected payments-preview, got %s", string(buf))
		}
	})

	t.Run("Cache bounding", func(t *testing.T) {
		// Clean up dev endpoint env from prior test
		os.Unsetenv("DIVERGE_DEV_ENDPOINT")
		os.Unsetenv("DIVERGE_DEV_PREVIEW_ID")

		cacheMu.Lock()
		previewCache = make(map[string]cacheEntry) // reset
		for i := 0; i < 1000; i++ {
			previewCache[fmt.Sprintf("svc/%d", i)] = cacheEntry{exists: true, expires: time.Now().Add(time.Hour)}
		}
		cacheMu.Unlock()

		// Trigger a DNS lookup failure which adds an entry
		req, _ := http.NewRequestWithContext(t.Context(), "GET", server.URL+"/api/payments", nil)
		req.Header.Set("x-test-preview", "trigger-bound")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		cacheMu.RLock()
		defer cacheMu.RUnlock()
		if len(previewCache) > 1 {
			t.Errorf("Expected cache to be bounded (size 1), got %d", len(previewCache))
		}
	})

	t.Run("Module registry baseline", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(t.Context(), "GET", server.URL+"/api/module-registry", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		var result struct {
			Modules map[string]struct {
				Version string `json:"version"`
			} `json:"modules"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		if result.Modules["payments"].Version != "baseline" {
			t.Errorf("Expected baseline version, got %s", result.Modules["payments"].Version)
		}
	})

	t.Run("Topology handler - Context fallback", func(t *testing.T) {
		// Set dev endpoint to the distinct preview backend, no specific preview ID
		// (devID == "" means any preview routes to dev endpoint)
		os.Setenv("DIVERGE_DEV_ENDPOINT", previewApi.Listener.Addr().String())
		os.Unsetenv("DIVERGE_DEV_PREVIEW_ID")
		defer os.Unsetenv("DIVERGE_DEV_ENDPOINT")

		// Send x-preview-env (the SDK's DefaultHeaderKey, NOT our custom headerKey)
		// PropagateEnvironment middleware reads this and puts it in context,
		// topology handler picks it up via divergehttp.FromContext fallback
		req, _ := http.NewRequestWithContext(t.Context(), "GET", server.URL+"/topology", nil)
		req.Header.Set("x-preview-env", "ctx-preview")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		var results map[string]string
		json.NewDecoder(resp.Body).Decode(&results)
		// Preview backend returns "preview-payments", baseline returns "baseline-payments"
		if results["payments-api"] != "preview-payments" {
			t.Errorf("Expected preview-payments (from dev endpoint), got %s", results["payments-api"])
		}
	})
}

// TestGatewayContextHelper verifies t.Context() is available (Go 1.21+)
func TestGatewayContextHelper(t *testing.T) {
	ctx := t.Context()
	if ctx == nil {
		t.Fatal("t.Context() returned nil")
	}
	if ctx.Err() != nil {
		t.Fatal("context should not be cancelled")
	}
	_ = context.Background() // ensure context import is used
}
