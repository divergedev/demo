package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	divergehttp "github.com/divergedev/diverge/pkg/sdk/http"
)

type Account struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
	Type    string  `json:"type"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	version := os.Getenv("APP_VERSION")
	if version == "" {
		version = "baseline"
	}

	headerKey := os.Getenv("DIVERGE_HEADER_KEY")
	if headerKey == "" {
		os.Setenv("DIVERGE_HEADER_KEY", "x-preview-id")
	}

	accounts := []Account{
		{ID: "ACC-001", Name: "Primary Checking", Balance: 10542.87, Type: "checking"},
		{ID: "ACC-002", Name: "Savings", Balance: 25000.00, Type: "savings"},
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "accounts-api", "version": version})
	})

	mux.HandleFunc("/api/accounts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "accounts-api",
			"version": version,
			"preview": divergehttp.FromContext(r.Context()),
			"accounts": accounts,
		})
	})

	mux.HandleFunc("/api/accounts/balance", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "accounts-api",
			"version": version,
			"preview": divergehttp.FromContext(r.Context()),
			"balance": accounts[0].Balance,
		})
	})

	handler := divergehttp.PropagateEnvironment(mux)

	log.Printf("accounts-api %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
