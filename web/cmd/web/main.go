package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

type record struct {
	ID            int64  `json:"id"`
	LineCode      string `json:"line_code"`
	TrainNumber   string `json:"train_number"`
	ServiceDate   string `json:"service_date"`
	StationName   string `json:"station_name"`
	ScheduledTime string `json:"scheduled_time"`
	ActualTime    string `json:"actual_time"`
	DelayMinutes  int    `json:"delay_minutes"`
	Platform      string `json:"platform"`
	Status        string `json:"status"`
	ObservedAt    string `json:"observed_at"`
}

type responseList struct {
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	Total      int      `json:"total"`
	Records    []record `json:"records"`
	HasNext    bool     `json:"has_next"`
	HasPrev    bool     `json:"has_prev"`
	TotalPages int      `json:"total_pages"`
}

//go:embed static/index.html
var indexHTML string

//go:embed static/app.css
var appCSS string

//go:embed static/app.js
var appJS string

func main() {
	dbPath := getenv("SQLITE_DB_PATH", "/data/ritardi.sqlite")
	addr := getenv("WEB_BIND_ADDR", "0.0.0.0")
	port := getenv("WEB_PORT", "8080")
	pageSizeDefault := atoiDefault(getenv("WEB_PAGE_SIZE", "25"), 25)
	if pageSizeDefault <= 0 {
		pageSizeDefault = 25
	}

	db, err := openReadOnlySQLite(dbPath)
	if err != nil {
		log.Fatalf("errore apertura sqlite in sola lettura: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	})
	mux.HandleFunc("/app.css", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write([]byte(appCSS))
	})
	mux.HandleFunc("/app.js", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write([]byte(appJS))
	})
	mux.HandleFunc("/api/healthz", func(w http.ResponseWriter, _ *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/delays", func(w http.ResponseWriter, r *http.Request) {
		handleDelays(w, r, db, pageSizeDefault)
	})

	serverAddr := fmt.Sprintf("%s:%s", addr, port)
	log.Printf("web viewer avviato su http://%s (sqlite: %s)", serverAddr, dbPath)
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatalf("errore avvio server web: %v", err)
	}
}

func handleDelays(w http.ResponseWriter, r *http.Request, db *sql.DB, pageSizeDefault int) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	line := strings.TrimSpace(r.URL.Query().Get("line"))

	page := atoiDefault(r.URL.Query().Get("page"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := atoiDefault(r.URL.Query().Get("page_size"), pageSizeDefault)
	if pageSize < 1 {
		pageSize = pageSizeDefault
	}
	if pageSize > 100 {
		pageSize = 100
	}

	whereSQL, whereArgs := buildWhere(q, line)

	var total int
	err := db.QueryRow(
		"SELECT COUNT(1) FROM delay_records "+whereSQL,
		whereArgs...,
	).Scan(&total)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "errore conteggio record"})
		return
	}

	offset := (page - 1) * pageSize
	query := `
SELECT id, line_code, train_number, service_date, station_name,
       COALESCE(scheduled_time, ''), COALESCE(actual_time, ''),
       delay_minutes, COALESCE(platform, ''), COALESCE(status, ''),
       observed_at
FROM delay_records ` + whereSQL + `
ORDER BY observed_at DESC, id DESC
LIMIT ? OFFSET ?`

	args := append(whereArgs, pageSize, offset)
	rows, err := db.Query(query, args...)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "errore query record"})
		return
	}
	defer rows.Close()

	var records []record
	for rows.Next() {
		var rec record
		if err := rows.Scan(
			&rec.ID,
			&rec.LineCode,
			&rec.TrainNumber,
			&rec.ServiceDate,
			&rec.StationName,
			&rec.ScheduledTime,
			&rec.ActualTime,
			&rec.DelayMinutes,
			&rec.Platform,
			&rec.Status,
			&rec.ObservedAt,
		); err != nil {
			respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "errore lettura record"})
			return
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "errore iterazione record"})
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	respondJSON(w, http.StatusOK, responseList{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		Records:    records,
		HasPrev:    page > 1,
		HasNext:    page < totalPages,
		TotalPages: totalPages,
	})
}

func buildWhere(q, line string) (string, []any) {
	var clauses []string
	var args []any

	if line != "" {
		clauses = append(clauses, "line_code = ?")
		args = append(args, line)
	}
	if q != "" {
		p := "%" + escapeLike(q) + "%"
		clauses = append(clauses, `(train_number LIKE ? ESCAPE '\' OR station_name LIKE ? ESCAPE '\' OR service_date LIKE ? ESCAPE '\' OR status LIKE ? ESCAPE '\')`)
		args = append(args, p, p, p, p)
	}

	if len(clauses) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func escapeLike(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

func openReadOnlySQLite(path string) (*sql.DB, error) {
	dsn := (&url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: "mode=ro",
	}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA query_only = ON;`); err != nil {
		db.Close()
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func atoiDefault(value string, fallback int) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return n
}
