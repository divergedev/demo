package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	divergehttp "github.com/divergedev/diverge/pkg/sdk/http"
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
	webAppURL := os.Getenv("WEB_APP_URL")
	if webAppURL == "" {
		webAppURL = "http://web-app:8080"
	}

	headerKey := os.Getenv("DIVERGE_HEADER_KEY")
	if headerKey == "" {
		os.Setenv("DIVERGE_HEADER_KEY", "x-preview-id")
	}

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: divergehttp.RoundTripper(http.DefaultTransport),
	}

	proxyRequest := func(w http.ResponseWriter, r *http.Request, target string) {
		req, _ := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "gateway", "version": version})
	})

	mux.HandleFunc("/topology", func(w http.ResponseWriter, r *http.Request) {
		var wg sync.WaitGroup
		results := make(map[string]string)
		var mu sync.Mutex

		services := map[string]string{
			"gateway":      "http://localhost:" + port + "/health",
			"payments-api": paymentsURL + "/health",
			"accounts-api": accountsURL + "/health",
		}

		for name, url := range services {
			wg.Add(1)
			go func(n, u string) {
				defer wg.Done()
				req, _ := http.NewRequestWithContext(r.Context(), "GET", u, nil)
				resp, err := client.Do(req)
				if err == nil {
					defer resp.Body.Close()
					var h map[string]string
					if json.NewDecoder(resp.Body).Decode(&h) == nil {
						mu.Lock()
						results[n] = h["version"]
						mu.Unlock()
						return
					}
				}
				mu.Lock()
				results[n] = "unavailable"
				mu.Unlock()
			}(name, url)
		}
		wg.Wait()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	})

	mux.HandleFunc("/api/payments/transactions", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, paymentsURL+"/api/payments/transactions")
	})

	mux.HandleFunc("/api/payments/fraud-alerts", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, paymentsURL+"/api/payments/fraud-alerts")
	})

	mux.HandleFunc("/api/payments", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, paymentsURL+"/api/payments")
	})

	mux.HandleFunc("/api/accounts", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, accountsURL+"/api/accounts")
	})

	mux.HandleFunc("/api/accounts/balance", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, accountsURL+"/api/accounts/balance")
	})

	mux.HandleFunc("/api/module-registry", func(w http.ResponseWriter, r *http.Request) {
		previewID := divergehttp.FromContext(r.Context())

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

		if previewID != "" {
			previewSvc := fmt.Sprintf("http://preview-%s-payments-module:8080", previewID)
			healthURL := previewSvc + "/health"
			req, _ := http.NewRequestWithContext(r.Context(), "GET", healthURL, nil)
			resp, err := client.Do(req)
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

	mux.HandleFunc("/modules/", func(w http.ResponseWriter, r *http.Request) {
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

	// Catch-all: proxy to the web-app React shell
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		target := webAppURL + r.URL.Path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		proxyRequest(w, r, target)
	})

	handler := divergehttp.PropagateEnvironment(mux)

	log.Printf("gateway %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
