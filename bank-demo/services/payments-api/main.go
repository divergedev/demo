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

// BaselineTxn is what the baseline API returns — simple payment records.
type BaselineTxn struct {
	ID     string  `json:"id"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
	Status string  `json:"status"`
}

// PreviewTxn adds fraud detection fields — only in the preview version.
type PreviewTxn struct {
	ID         string  `json:"id"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	Amount     float64 `json:"amount"`
	FraudScore float64 `json:"fraud_score"`
	FraudFlag  bool    `json:"fraud_flag"`
	Fee        float64 `json:"fee"`
	Status     string  `json:"status"`
}

var baselineTxns = []BaselineTxn{
	{ID: "tx_001", From: "savings", To: "checking", Amount: 250.00, Status: "completed"},
	{ID: "tx_002", From: "employer", To: "checking", Amount: 1200.00, Status: "completed"},
	{ID: "tx_003", From: "checking", To: "amazon", Amount: 89.99, Status: "completed"},
	{ID: "tx_004", From: "checking", To: "wire", Amount: 3500.00, Status: "completed"},
	{ID: "tx_005", From: "acct_unknown", To: "acct_offshore_001", Amount: 15000.00, Status: "completed"},
	{ID: "tx_006", From: "checking", To: "coffee_shop", Amount: 42.50, Status: "completed"},
	{ID: "tx_007", From: "checking", To: "crypto_exchange", Amount: 8750.00, Status: "completed"},
	{ID: "tx_008", From: "checking", To: "atm", Amount: 2100.00, Status: "completed"},
}

var previewTxns = []PreviewTxn{
	{ID: "tx_001", From: "savings", To: "checking", Amount: 250.00, FraudScore: 0.05, FraudFlag: false, Fee: 3.75, Status: "completed"},
	{ID: "tx_002", From: "employer", To: "checking", Amount: 1200.00, FraudScore: 0.02, FraudFlag: false, Fee: 18.00, Status: "completed"},
	{ID: "tx_003", From: "checking", To: "amazon", Amount: 89.99, FraudScore: 0.12, FraudFlag: false, Fee: 1.35, Status: "completed"},
	{ID: "tx_004", From: "checking", To: "wire", Amount: 3500.00, FraudScore: 0.45, FraudFlag: false, Fee: 52.50, Status: "completed"},
	{ID: "tx_005", From: "acct_unknown", To: "acct_offshore_001", Amount: 15000.00, FraudScore: 0.92, FraudFlag: true, Fee: 225.00, Status: "pending"},
	{ID: "tx_006", From: "checking", To: "coffee_shop", Amount: 42.50, FraudScore: 0.01, FraudFlag: false, Fee: 0.64, Status: "completed"},
	{ID: "tx_007", From: "checking", To: "crypto_exchange", Amount: 8750.00, FraudScore: 0.87, FraudFlag: true, Fee: 131.25, Status: "pending"},
	{ID: "tx_008", From: "checking", To: "atm", Amount: 2100.00, FraudScore: 0.78, FraudFlag: true, Fee: 31.50, Status: "pending"},
}

func isPreview(version string) bool {
	return version != "baseline"
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
		if !isPreview(version) {
			// Baseline: no fraud detection
			json.NewEncoder(w).Encode(map[string]interface{}{
				"service":         "payments-api",
				"version":         version,
				"fraud_detection": false,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service":         "payments-api",
			"version":         version,
			"fraud_detection": true,
			"flagged_count":   3,
			"alerts": []map[string]interface{}{
				{
					"transaction_id": "tx_005",
					"fraud_score":    0.92,
					"reason":         "Unusual amount pattern",
					"amount":         15000.00,
					"from":           "acct_unknown",
					"to":             "acct_offshore_001",
				},
			},
		})
	})

	mux.HandleFunc("/api/payments", func(w http.ResponseWriter, r *http.Request) {
		client := &http.Client{
			Timeout:   5 * time.Second,
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
		if !isPreview(version) {
			// Baseline: simple payment list, no fraud fields
			json.NewEncoder(w).Encode(map[string]interface{}{
				"service":       "payments-api",
				"version":       version,
				"balance":       balance,
				"flagged_count": 0,
				"total_fees":    0,
				"payments":      baselineTxns,
			})
			return
		}

		// Preview: full fraud detection data
		var flaggedCount int
		var totalFees float64
		for _, t := range previewTxns {
			if t.FraudFlag {
				flaggedCount++
			}
			totalFees += t.Fee
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"service":       "payments-api",
			"version":       version,
			"preview":       divergehttp.FromContext(r.Context()),
			"balance":       balance,
			"flagged_count": flaggedCount,
			"total_fees":    totalFees,
			"payments":      previewTxns,
		})
	})

	mux.HandleFunc("/api/payments/transactions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if db == nil {
			var txns interface{}
			if isPreview(version) {
				txns = previewTxns
			} else {
				txns = baselineTxns
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"service":      "payments-api",
				"version":      version,
				"source":       "static",
				"transactions": txns,
			})
			return
		}

		type txn struct {
			ID         int      `json:"id"`
			From       string   `json:"from"`
			To         string   `json:"to"`
			Amt        float64  `json:"amount"`
			Fee        *float64 `json:"fee,omitempty"`
			FraudScore *float64 `json:"fraud_score,omitempty"`
			FraudFlag  *bool    `json:"fraud_flag,omitempty"`
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

		hasFraud := false
		err = db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_name='transactions'
				AND column_name='fraud_score'
				AND table_schema = current_schema()
			)`).Scan(&hasFraud)
		if err != nil {
			log.Printf("Error checking fraud_score column: %v", err)
		}

		var rows *sql.Rows
		query := "SELECT id, from_account, to_account, amount"
		if hasFee {
			query += ", fee"
		}
		if hasFraud {
			query += ", fraud_score, fraud_flag"
		}
		query += " FROM transactions ORDER BY id LIMIT 10"

		rows, err = db.QueryContext(ctx, query)
		if err != nil {
			log.Printf("Error querying transactions: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to query transactions"})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var t txn
			scanArgs := []interface{}{&t.ID, &t.From, &t.To, &t.Amt}
			if hasFee {
				scanArgs = append(scanArgs, &t.Fee)
			}
			if hasFraud {
				scanArgs = append(scanArgs, &t.FraudScore, &t.FraudFlag)
			}

			if err := rows.Scan(scanArgs...); err != nil {
				log.Printf("Error scanning row: %v", err)
				continue
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
			"has_fraud":    hasFraud,
			"transactions": transactions,
		})
	})

	handler := divergehttp.PropagateEnvironment(mux)

	log.Printf("payments-api %s listening on :%s", version, port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
