package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
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

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "accounts-api", "version": version})
	})

	http.HandleFunc("/api/accounts/balance", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "accounts-api",
			"version": version,
			"preview": r.Header.Get("x-preview-id"),
			"balance": 10542.87,
		})
	})

	log.Printf("accounts-api %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
