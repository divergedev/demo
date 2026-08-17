package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	divergehttp "github.com/divergedev/diverge/pkg/sdk/http"
	_ "github.com/jackc/pgx/v5/stdlib"
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
	accountsURL := os.Getenv("ACCOUNTS_API_URL")
	if accountsURL == "" {
		accountsURL = "http://accounts-api:8080"
	}

	headerKey := os.Getenv("DIVERGE_HEADER_KEY")
	if headerKey == "" {
		os.Setenv("DIVERGE_HEADER_KEY", "x-preview-id")
	}

	var db *sql.DB
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		var err error
		db, err = sql.Open("pgx", dbURL)
		if err != nil {
			log.Printf("Warning: could not connect to database: %v", err)
		} else {
			db.SetMaxOpenConns(5)
			db.SetConnMaxLifetime(5 * time.Minute)
			if err := db.Ping(); err != nil {
				log.Printf("Warning: database ping failed: %v", err)
				db = nil
			} else {
				log.Printf("Connected to database")
			}
		}
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "payments-api", "version": version})
	})

	mux.HandleFunc("/api/payments/fraud-alerts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service": "payments-api",
			"version": "baseline",
			"fraud_detection": false,
			"message": "Fraud detection not available in baseline",
		})
	})

	mux.HandleFunc("/api/payments", func(w http.ResponseWriter, r *http.Request) {
		client := &http.Client{
			Timeout: 5 * time.Second,
			Transport: divergehttp.RoundTripper(http.DefaultTransport),
		}
		req, _ := http.NewRequestWithContext(r.Context(), "GET", accountsURL+"/api/accounts/balance", nil)

		var balance string
		resp, err := client.Do(req)
		if err != nil {
			balance = "unavailable"
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			balance = string(body)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service":  "payments-api",
			"version":  version,
			"preview":  divergehttp.FromContext(r.Context()),
			"balance":  balance,
			"flagged_count": 0,
			"payments": []map[string]interface{}{
				{"id": "pay-001", "amount": 150.00, "status": "completed"},
				{"id": "pay-002", "amount": 75.50, "status": "pending"},
			},
		})
	})

	mux.HandleFunc("/api/payments/transactions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if db == nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"service": "payments-api",
				"version": version,
				"source":  "static",
				"transactions": []map[string]interface{}{
					{"id": 1, "from": "ACC-001", "to": "ACC-002", "amount": 150.00},
					{"id": 2, "from": "ACC-003", "to": "ACC-001", "amount": 75.50},
					{"id": 3, "from": "ACC-002", "to": "ACC-004", "amount": 200.00},
					{"id": 4, "from": "ACC-005", "to": "ACC-001", "amount": 50.00},
					{"id": 5, "from": "ACC-001", "to": "ACC-003", "amount": 300.00},
				},
			})
			return
		}

		type txn struct {
			ID   int      `json:"id"`
			From string   `json:"from"`
			To   string   `json:"to"`
			Amt  float64  `json:"amount"`
			Fee  *float64 `json:"fee"`
		}

		transactions := make([]txn, 0)

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		hasFee := false
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name='transactions'
				AND column_name='fee'
				AND table_schema = current_schema()
			)`).Scan(&hasFee)
		if err != nil {
			log.Printf("Error checking fee column: %v", err)
		}

		var rows *sql.Rows
		if hasFee {
			rows, err = db.QueryContext(ctx, "SELECT id, from_account, to_account, amount, fee FROM transactions ORDER BY id LIMIT 10")
		} else {
			rows, err = db.QueryContext(ctx, "SELECT id, from_account, to_account, amount FROM transactions ORDER BY id LIMIT 10")
		}
		if err != nil {
			log.Printf("Error querying transactions: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to query transactions"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var t txn
			if hasFee {
				if err := rows.Scan(&t.ID, &t.From, &t.To, &t.Amt, &t.Fee); err != nil {
					log.Printf("Error scanning row: %v", err)
					continue
				}
			} else {
				if err := rows.Scan(&t.ID, &t.From, &t.To, &t.Amt); err != nil {
					log.Printf("Error scanning row: %v", err)
					continue
				}
			}
			transactions = append(transactions, t)
		}
		if err := rows.Err(); err != nil {
			log.Printf("Error iterating rows: %v", err)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"service":      "payments-api",
			"version":      version,
			"source":       "database",
			"has_fee":      hasFee,
			"transactions": transactions,
		})
	})

	handler := divergehttp.PropagateEnvironment(mux)

	log.Printf("payments-api %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
