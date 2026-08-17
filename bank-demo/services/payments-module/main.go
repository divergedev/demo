package main

import (
	"embed"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
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

	// Also get the assets subdirectory to serve at root
	assetsFS, err := fs.Sub(distFS, "dist/assets")
	if err != nil {
		log.Fatalf("failed to open assets filesystem: %v", err)
	}

	// Find the hashed remoteEntry file at startup
	var remoteEntryName string
	fs.WalkDir(assetsFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), "remoteEntry-") && strings.HasSuffix(d.Name(), ".js") {
			remoteEntryName = d.Name()
		}
		return nil
	})
	log.Printf("Resolved remoteEntry: %s", remoteEntryName)

	// Serve static files with CORS headers
	// Files live in dist/assets/ but the remoteEntry imports them with "./" relative paths.
	// When loaded via /modules/payments-module/remoteEntry.js, the browser resolves
	// "./foo.js" to /modules/payments-module/foo.js, which the gateway proxies to
	// http://payments-module:8080/foo.js. So we need to serve assets at root level too.
	distFileServer := http.FileServer(http.FS(fsys))
	assetsFileServer := http.FileServer(http.FS(assetsFS))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Cache-Control", "no-cache")

		path := strings.TrimPrefix(r.URL.Path, "/")

		// Alias /remoteEntry.js to the actual hashed file
		if path == "remoteEntry.js" && remoteEntryName != "" {
			path = remoteEntryName
		}

		// Try assets directory first (for relative imports from remoteEntry)
		if path != "" {
			if f, err := assetsFS.Open(path); err == nil {
				f.Close()
				r.URL.Path = "/" + path
				assetsFileServer.ServeHTTP(w, r)
				return
			}
		}

		// Fall back to full dist directory
		distFileServer.ServeHTTP(w, r)
	})

	log.Printf("payments-module %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
