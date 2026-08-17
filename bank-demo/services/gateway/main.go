package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
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

	// serviceNames maps base URLs to their k8s service name for preview lookup
	serviceNames := map[string]string{
		paymentsURL:       "payments-api",
		paymentsModuleURL: "payments-module",
	}

	// previewServiceCache caches DNS lookups for preview services (30s TTL)
	type cacheEntry struct {
		svcName string
		exists  bool
		expires time.Time
	}
	var cacheMu sync.RWMutex
	previewCache := make(map[string]cacheEntry)

	// lookupPreviewService checks if a preview service exists for a given
	// base service and preview ID by querying the Kubernetes API via DNS.
	// The controller names services: {envName}-{serviceName}
	// where envName = pg-{group}-{service}-{hash8}
	lookupPreviewService := func(baseSvc, previewID string) (string, bool) {
		cacheKey := baseSvc + "/" + previewID

		cacheMu.RLock()
		if entry, ok := previewCache[cacheKey]; ok && time.Now().Before(entry.expires) {
			cacheMu.RUnlock()
			return entry.svcName, entry.exists
		}
		cacheMu.RUnlock()

		// Try to find the preview service by probing the health endpoint
		// List candidate service names from env CRs (convention-based)
		// The controller creates services like: pg-{group}-{baseSvc}-{hash}-{baseSvc}
		// We discover by trying a DNS lookup for common patterns
		namespace := "demo-bank"

		// Try DNS resolution of known naming patterns
		candidates := []string{}

		// Query k8s services API via DNS SRV or direct HTTP
		// For simplicity, try common group names from PreviewGroups
		groups := []string{"fraud-detection"}
		for _, group := range groups {
			envName := childEnvironmentName(group, baseSvc)
			candidate := envName + "-" + baseSvc
			candidates = append(candidates, candidate)
		}

		for _, candidate := range candidates {
			// DNS probe: try to resolve {candidate}.{namespace}.svc.cluster.local
			host := fmt.Sprintf("%s.%s.svc.cluster.local", candidate, namespace)
			_, err := net.LookupHost(host)
			if err == nil {
				// Verify it's actually responding on the health endpoint
				healthURL := fmt.Sprintf("http://%s:8080/health", candidate)
				healthReq, _ := http.NewRequest("GET", healthURL, nil)
				healthResp, err := client.Do(healthReq)
				if err == nil {
					healthResp.Body.Close()
					if healthResp.StatusCode == http.StatusOK {
						cacheMu.Lock()
						previewCache[cacheKey] = cacheEntry{svcName: candidate, exists: true, expires: time.Now().Add(30 * time.Second)}
						cacheMu.Unlock()
						return candidate, true
					}
				}
			}
		}

		cacheMu.Lock()
		previewCache[cacheKey] = cacheEntry{exists: false, expires: time.Now().Add(30 * time.Second)}
		cacheMu.Unlock()
		return "", false
	}

	resolveTarget := func(r *http.Request, baseTarget string) string {
		previewID := r.Header.Get("x-preview-id")
		if previewID == "" {
			previewID = divergehttp.FromContext(r.Context())
		}
		if previewID == "" {
			return baseTarget
		}
		// Find the base service name for this target URL
		for baseURL, baseSvc := range serviceNames {
			if strings.HasPrefix(baseTarget, baseURL) {
				if previewSvc, ok := lookupPreviewService(baseSvc, previewID); ok {
					remainder := strings.TrimPrefix(baseTarget, baseURL)
					return fmt.Sprintf("http://%s:8080%s", previewSvc, remainder)
				}
			}
		}
		return baseTarget
	}

	proxyRequest := func(w http.ResponseWriter, r *http.Request, target string) {
		actual := resolveTarget(r, target)
		req, _ := http.NewRequestWithContext(r.Context(), r.Method, actual, r.Body)
		resp, err := client.Do(req)
		if err != nil {
			// Fallback to baseline if preview service is unavailable
			if actual != target {
				req2, _ := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
				resp, err = client.Do(req2)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
			} else {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
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

		// Determine actual service URLs based on preview routing
		actualPaymentsURL := paymentsURL
		actualModuleURL := paymentsModuleURL
		previewID := r.Header.Get("x-preview-id")
		if previewID != "" {
			if svc, ok := lookupPreviewService("payments-api", previewID); ok {
				actualPaymentsURL = fmt.Sprintf("http://%s:8080", svc)
			}
			if svc, ok := lookupPreviewService("payments-module", previewID); ok {
				actualModuleURL = fmt.Sprintf("http://%s:8080", svc)
			}
		}

		services := map[string]string{
			"gateway":         "http://localhost:" + port + "/health",
			"payments-api":    actualPaymentsURL + "/health",
			"accounts-api":    accountsURL + "/health",
			"web-app":         webAppURL + "/health",
			"payments-module": actualModuleURL + "/health",
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
		previewID := r.Header.Get("x-preview-id")
		if previewID == "" {
			previewID = divergehttp.FromContext(r.Context())
		}

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

// childEnvironmentName generates a DNS-safe name matching the controller's naming.
// Format: pg-{group}-{service}-{hash8}
func childEnvironmentName(groupName, serviceName string) string {
	raw := fmt.Sprintf("pg-%s-%s", groupName, serviceName)
	raw = strings.ToLower(raw)
	raw = strings.NewReplacer(".", "-", "_", "-").Replace(raw)
	raw = regexp.MustCompile(`[^a-z0-9-]`).ReplaceAllString(raw, "")
	raw = regexp.MustCompile(`-{2,}`).ReplaceAllString(raw, "-")
	raw = strings.Trim(raw, "-")

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(groupName+"/"+serviceName)))[:8]

	if len(raw) <= 63-9 {
		return raw + "-" + hash
	}
	return raw[:63-9] + "-" + hash
}
