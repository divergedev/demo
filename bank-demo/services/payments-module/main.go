package main

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

//go:embed remoteEntry.js
var moduleJS embed.FS

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
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "payments-module",
			"version": version,
		})
	})

	http.HandleFunc("/remoteEntry.js", func(w http.ResponseWriter, r *http.Request) {
		data, err := moduleJS.ReadFile("remoteEntry.js")
		if err != nil {
			http.Error(w, "module not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(data)
	})

	log.Printf("payments-module %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
