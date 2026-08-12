package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "baseline"
	}
	paymentsURL := os.Getenv("PAYMENTS_API_URL")
	if paymentsURL == "" {
		paymentsURL = "http://payments-api:8080"
	}
	accountsURL := os.Getenv("ACCOUNTS_API_URL")
	if accountsURL == "" {
		accountsURL = "http://accounts-api:8080"
	}
	paymentsModuleURL := os.Getenv("PAYMENTS_MODULE_URL")
	if paymentsModuleURL == "" {
		paymentsModuleURL = "http://payments-module:8080"
	}

	client := &http.Client{Timeout: 5 * time.Second}

	proxyRequest := func(w http.ResponseWriter, r *http.Request, target string) {
		req, _ := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
		if pid := r.Header.Get("x-preview-id"); pid != "" {
			req.Header.Set("x-preview-id", pid)
		}
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "gateway", "version": version})
	})

	http.HandleFunc("/api/payments/transactions", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, paymentsURL+"/api/payments/transactions")
	})

	http.HandleFunc("/api/payments", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, paymentsURL+"/api/payments")
	})

	http.HandleFunc("/api/accounts", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, accountsURL+"/api/accounts/balance")
	})

	// ── Module Federation: Module Registry ─────────────────────
	// Returns a manifest of available frontend modules and their URLs.
	// When x-preview-id is set, checks if a preview module pod exists
	// and returns the preview URL instead of baseline.
	http.HandleFunc("/api/module-registry", func(w http.ResponseWriter, r *http.Request) {
		previewID := r.Header.Get("x-preview-id")

		type moduleInfo struct {
			URL     string `json:"url"`
			Version string `json:"version"`
		}
		type registryResponse struct {
			Modules map[string]moduleInfo `json:"modules"`
		}

		paymentsModule := moduleInfo{
			URL:     "/modules/payments-module/remoteEntry.js",
			Version: "baseline",
		}

		// If preview header is set, check if preview module pod exists
		if previewID != "" {
			previewSvc := fmt.Sprintf("http://preview-%s-payments-module:8080", previewID)
			healthURL := previewSvc + "/health"
			resp, err := client.Get(healthURL)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var health map[string]string
					json.NewDecoder(resp.Body).Decode(&health)
					previewVersion := health["version"]
					if previewVersion == "" {
						previewVersion = "preview-" + previewID
					}
					paymentsModule = moduleInfo{
						URL:     fmt.Sprintf("/modules/preview-%s-payments-module/remoteEntry.js", previewID),
						Version: previewVersion,
					}
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(registryResponse{
			Modules: map[string]moduleInfo{
				"payments": paymentsModule,
			},
		})
	})

	// ── Module Federation: Module Proxy ─────────────────────────
	// Proxies /modules/{service-name}/* → {service-name}:8080/*
	// This lets the shell load module JS bundles through the gateway.
	http.HandleFunc("/modules/", func(w http.ResponseWriter, r *http.Request) {
		// Parse: /modules/{service-name}/remoteEntry.js
		path := strings.TrimPrefix(r.URL.Path, "/modules/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 2 {
			http.Error(w, "invalid module path", http.StatusBadRequest)
			return
		}
		serviceName := parts[0]
		remainder := parts[1]

		target := fmt.Sprintf("http://%s:8080/%s", serviceName, remainder)
		proxyRequest(w, r, target)
	})

	log.Printf("gateway %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
