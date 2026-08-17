package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
)

//go:embed dist/*
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

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"service": "payments-module",
			"version": version,
		})
	})

	// Get subdirectory for static files
	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		log.Fatalf("failed to open dist filesystem: %v", err)
	}

	// Serve static files with CORS headers
	fileServer := http.FileServer(http.FS(fsys))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(w, r)
	})

	log.Printf("payments-module %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
