package main

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"
)

//go:embed all:dist/*
var distFS embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "baseline"
	}
	gatewayURL := os.Getenv("GATEWAY_URL")
	if gatewayURL == "" {
		gatewayURL = "http://gateway:8080"
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
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "web-app", "version": version})
	})

	http.HandleFunc("/api/payments/transactions", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, gatewayURL+"/api/payments/transactions")
	})

	http.HandleFunc("/api/payments", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, gatewayURL+"/api/payments")
	})

	http.HandleFunc("/api/accounts", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, gatewayURL+"/api/accounts")
	})

	// Module Federation: proxy module registry and module assets through gateway
	http.HandleFunc("/api/module-registry", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, gatewayURL+"/api/module-registry")
	})

	http.HandleFunc("/modules/", func(w http.ResponseWriter, r *http.Request) {
		proxyRequest(w, r, gatewayURL+r.URL.Path)
	})

	// Serve the React frontend from dist folder
	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatalf("failed to open dist filesystem: %v", err)
	}
	
	fileServer := http.FileServer(http.FS(fsys))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Basic SPA routing fallback to index.html for non-file requests could be added,
		// but simple FileServer is OK for demo
		fileServer.ServeHTTP(w, r)
	})

	log.Printf("web-app %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
