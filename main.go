package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed web/index.html
var indexHTML []byte

type Record struct {
	Date   string  `json:"date"`
	Client string  `json:"client"`
	Model  string  `json:"model"`
	Input  int64   `json:"input"`
	Output int64   `json:"output"`
	CR     int64   `json:"cr"`
	CW     int64   `json:"cw"`
	Total  int64   `json:"total"`
	Cost   float64 `json:"cost"`
}

type ReportPayload struct {
	Machine string   `json:"machine"`
	User    string   `json:"user"`
	Records []Record `json:"records"`
}

type DailyItem struct {
	Date     string  `json:"date"`
	Client   string  `json:"client"`
	Model    string  `json:"model"`
	Input    int64   `json:"input"`
	Output   int64   `json:"output"`
	CR       int64   `json:"cr"`
	CW       int64   `json:"cw"`
	Total    int64   `json:"total"`
	Cost     float64 `json:"cost"`
	Machines string  `json:"machines"`
}

type MachineStat struct {
	Machine  string  `json:"machine"`
	LastSeen string  `json:"last_seen"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

type DailyResponse struct {
	Date     string        `json:"date"`
	Items    []DailyItem   `json:"items"`
	Machines []MachineStat `json:"machines"`
	Totals   struct {
		Input  int64   `json:"input"`
		Output int64   `json:"output"`
		CR     int64   `json:"cr"`
		CW     int64   `json:"cw"`
		Total  int64   `json:"total"`
		Cost   float64 `json:"cost"`
	} `json:"totals"`
}

var (
	db        *sql.DB
	authToken string
)

func initDB(dbPath string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	// Pragmas for performance and concurrency on NAS
	pragmas := []string{
		"PRAGMA journal_mode = WAL;",
		"PRAGMA busy_timeout = 5000;",
		"PRAGMA synchronous = NORMAL;",
	}
	for _, p := range pragmas {
		if _, err := d.Exec(p); err != nil {
			log.Printf("[warn] pragma %s failed: %v", p, err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS usage_records (
		date TEXT NOT NULL,
		machine TEXT NOT NULL,
		client TEXT NOT NULL,
		model TEXT NOT NULL,
		input INTEGER NOT NULL,
		output INTEGER NOT NULL,
		cache_read INTEGER NOT NULL,
		cache_write INTEGER NOT NULL,
		total INTEGER NOT NULL,
		cost REAL NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (date, machine, client, model)
	);
	CREATE INDEX IF NOT EXISTS idx_usage_date ON usage_records(date);
	CREATE INDEX IF NOT EXISTS idx_usage_machine ON usage_records(machine);
	`
	if _, err := d.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return d, nil
}

func checkAuth(r *http.Request) bool {
	if authToken == "" {
		return true
	}
	token := r.Header.Get("X-Auth-Token")
	if token == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	return token == authToken
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !checkAuth(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var payload ReportPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid json: %v"}`, err), http.StatusBadRequest)
		return
	}

	machine := strings.TrimSpace(payload.Machine)
	if machine == "" {
		machine = "unknown"
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, `{"error":"failed to begin transaction"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO usage_records (date, machine, client, model, input, output, cache_read, cache_write, total, cost, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(date, machine, client, model) DO UPDATE SET
			input = excluded.input,
			output = excluded.output,
			cache_read = excluded.cache_read,
			cache_write = excluded.cache_write,
			total = excluded.total,
			cost = excluded.cost,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to prepare statement: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	for _, rec := range payload.Records {
		if rec.Date == "" || rec.Client == "" || rec.Model == "" {
			continue
		}
		_, err := stmt.Exec(
			rec.Date, machine, rec.Client, rec.Model,
			rec.Input, rec.Output, rec.CR, rec.CW, rec.Total, rec.Cost,
		)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to insert: %v"}`, err), http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"failed to commit"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"count":   len(payload.Records),
		"machine": machine,
	})
}

func handleDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	resp := DailyResponse{
		Date:     date,
		Items:    make([]DailyItem, 0),
		Machines: make([]MachineStat, 0),
	}

	// 1. Query model-level records for this date (supports ?by_machine=1 to view breakdown per machine)
	byMachine := r.URL.Query().Get("by_machine") == "1" || r.URL.Query().Get("by_machine") == "true"
	var query string
	if byMachine {
		query = `
			SELECT date, client, model,
			       input, output, cache_read, cache_write, total, cost,
			       machine AS machines
			FROM usage_records
			WHERE date = ?
			ORDER BY cost DESC, total DESC
		`
	} else {
		query = `
			SELECT date, client, model,
			       SUM(input) AS input,
			       SUM(output) AS output,
			       SUM(cache_read) AS cr,
			       SUM(cache_write) AS cw,
			       SUM(total) AS total,
			       SUM(cost) AS cost,
			       GROUP_CONCAT(DISTINCT machine) AS machines
			FROM usage_records
			WHERE date = ?
			GROUP BY date, client, model
			ORDER BY cost DESC, total DESC
		`
	}

	rows, err := db.Query(query, date)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"query failed: %v"}`, err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var item DailyItem
		if err := rows.Scan(
			&item.Date, &item.Client, &item.Model,
			&item.Input, &item.Output, &item.CR, &item.CW,
			&item.Total, &item.Cost, &item.Machines,
		); err != nil {
			continue
		}
		resp.Items = append(resp.Items, item)
		resp.Totals.Input += item.Input
		resp.Totals.Output += item.Output
		resp.Totals.CR += item.CR
		resp.Totals.CW += item.CW
		resp.Totals.Total += item.Total
		resp.Totals.Cost += item.Cost
	}

	// 2. Query per-machine breakdown for this date
	mRows, err := db.Query(`
		SELECT machine, MAX(updated_at) AS last_seen, SUM(total) AS tokens, SUM(cost) AS cost
		FROM usage_records
		WHERE date = ?
		GROUP BY machine
		ORDER BY cost DESC
	`, date)
	if err == nil {
		defer mRows.Close()
		for mRows.Next() {
			var m MachineStat
			if err := mRows.Scan(&m.Machine, &m.LastSeen, &m.Tokens, &m.Cost); err == nil {
				resp.Machines = append(resp.Machines, m)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func handleMachines(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT machine, MAX(updated_at) AS last_seen, SUM(total) AS tokens, SUM(cost) AS cost
		FROM usage_records
		GROUP BY machine
		ORDER BY last_seen DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	list := make([]MachineStat, 0)
	for rows.Next() {
		var m MachineStat
		if err := rows.Scan(&m.Machine, &m.LastSeen, &m.Tokens, &m.Cost); err == nil {
			list = append(list, m)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Auth-Token, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3888"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/usage.db"
	}
	authToken = os.Getenv("AUTH_TOKEN")

	var err error
	db, err = initDB(dbPath)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/api/report", handleReport)
	mux.HandleFunc("/api/daily", handleDaily)
	mux.HandleFunc("/api/machines", handleMachines)

	server := &http.Server{
		Addr:         "0.0.0.0:" + port,
		Handler:      withCORS(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[tsusage-hub] server listening on http://0.0.0.0:%s (db: %s)", port, dbPath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[tsusage-hub] shutting down...")
}
