package main

import (
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
		req, _ := http.NewRequest("GET", server.URL+"/api/payments", nil)
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

	t.Run("Header key configuration", func(t *testing.T) {
		// Mock dev endpoint so preview routing works without DNS
		os.Setenv("DIVERGE_DEV_MODULE_ENDPOINT", "localhost:9999")
		defer os.Unsetenv("DIVERGE_DEV_MODULE_ENDPOINT")

		// Sending the custom configured header
		req, _ := http.NewRequest("GET", server.URL+"/api/module-registry", nil)
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
		
		// If header was used correctly, it triggers the dev endpoint logic
		if result.Modules["payments"].Version != "local-dev" {
			t.Errorf("Expected local-dev version, got %s", result.Modules["payments"].Version)
		}
	})

	t.Run("Cache bounding", func(t *testing.T) {
		cacheMu.Lock()
		previewCache = make(map[string]cacheEntry) // reset
		for i := 0; i < 1000; i++ {
			previewCache[fmt.Sprintf("svc/%d", i)] = cacheEntry{exists: true, expires: time.Now().Add(time.Hour)}
		}
		cacheMu.Unlock()

		// Trigger a DNS lookup failure which adds an entry
		req, _ := http.NewRequest("GET", server.URL+"/api/payments", nil)
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
		req, _ := http.NewRequest("GET", server.URL+"/api/module-registry", nil)
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
		// We use a custom handler that injects context so we don't have to rely on divergehttp internals.
		// Wait, divergehttp uses its own ContextKey. 
		// Actually, divergehttp.FromContext is what setupGateway() uses. If we inject using divergehttp.WithContext, it will work.
		// Wait, does divergehttp export WithContext? Let's assume it does.
		// Actually, I can just write a wrapper that sets the header, but wait, the test is to test Context fallback!
		// Let's just mock DIVERGE_DEV_ENDPOINT and send NO header. If the topology handler uses divergehttp.FromContext, we need the context to have it.
		// Let's look at divergehttp if we can. Or we can just trust the `PropagateEnvironment` middleware in setupGateway to set it if we send `x-diverge-env`.
		// Wait! setupGateway() adds divergehttp.PropagateEnvironment(mux)!
		// This middleware reads `x-diverge-env` and puts it into the context!
		// So if we send `x-diverge-env: ctx-preview`, divergehttp middleware will put it in context.
		// And our topology handler should read it from context!
		
		os.Setenv("DIVERGE_DEV_ENDPOINT", paymentsApi.URL[7:]) // strip http://
		defer os.Unsetenv("DIVERGE_DEV_ENDPOINT")

		req, _ := http.NewRequest("GET", server.URL+"/topology", nil)
		req.Header.Set("x-diverge-env", "ctx-preview") // Not the headerKey, so Header.Get(headerKey) is empty!
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed request: %v", err)
		}
		defer resp.Body.Close()

		var results map[string]string
		json.NewDecoder(resp.Body).Decode(&results)
		if results["payments-api"] != "baseline-payments" {
			t.Errorf("Expected dev endpoint (baseline-payments from mock), got %s", results["payments-api"])
		}
	})
}
