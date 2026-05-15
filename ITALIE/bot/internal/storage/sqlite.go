package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/maxysoft/trenitalia_gtfs/bot/internal/monitor"
)

type StatisticheMensili struct {
	MeseRiferimento       string
	Linea                 string
	NumeroRitardi         int
	RitardoMedioMinuti    float64
	RitardoMassimoMinuti  int
	FasciaPeggiore        string
	RitardiFasciaPeggiore int
}

type SQLiteStore struct {
	db *sql.DB
}

func NuovoSQLiteStore(dbPath string) (*SQLiteStore, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("percorso database non impostato")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creazione directory database: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("apertura database sqlite: %w", err)
	}

	if _, err = db.Exec(`PRAGMA journal_mode = WAL; PRAGMA synchronous = NORMAL;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("inizializzazione PRAGMA sqlite: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migra(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) Chiudi() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) migra() error {
	schema := `
CREATE TABLE IF NOT EXISTS delay_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  line_code TEXT NOT NULL,
  train_number TEXT NOT NULL,
  service_date TEXT NOT NULL,
  station_name TEXT NOT NULL,
  scheduled_time TEXT,
  actual_time TEXT,
  delay_minutes INTEGER NOT NULL,
  platform TEXT,
  status TEXT,
  observed_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_delay_records_line_date ON delay_records(line_code, service_date);
CREATE INDEX IF NOT EXISTS idx_delay_records_observed_at ON delay_records(observed_at);

CREATE TABLE IF NOT EXISTS monthly_reports (
  month_key TEXT NOT NULL,
  line_code TEXT NOT NULL,
  sent_at TEXT NOT NULL,
  PRIMARY KEY (month_key, line_code)
);`

	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrazione schema sqlite: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SalvaNotifica(notifica monitor.NotificaRitardo) error {
	if len(notifica.FermateInRitardo) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("apertura transazione sqlite: %w", err)
	}

	stmt, err := tx.Prepare(`
		INSERT INTO delay_records (
			line_code, train_number, service_date, station_name,
			scheduled_time, actual_time, delay_minutes, platform, status, observed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare inserimento ritardi: %w", err)
	}
	defer stmt.Close()

	observedAt := time.Now().UTC().Format(time.RFC3339)
	for _, f := range notifica.FermateInRitardo {
		scheduled := orarioUTC(f.OraOraria)
		actual := orarioUTC(f.OraReale)
		if _, err := stmt.Exec(
			notifica.LineaTreno,
			notifica.NumeroTreno,
			notifica.DataServizio,
			f.NomeStazione,
			scheduled,
			actual,
			f.RitardoMinuti,
			f.Binario,
			f.Stato,
			observedAt,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("inserimento ritardo sqlite: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit inserimento ritardi sqlite: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ReportMensileDaInviare(linea string, now time.Time) (StatisticheMensili, bool, error) {
	stats, monthKey, err := s.statisticheMesePrecedente(linea, now)
	if err != nil {
		return StatisticheMensili{}, false, err
	}
	if stats.NumeroRitardi == 0 {
		return StatisticheMensili{}, false, nil
	}

	inviato, err := s.ReportMensileGiaInviato(monthKey, linea)
	if err != nil {
		return StatisticheMensili{}, false, err
	}
	if inviato {
		return StatisticheMensili{}, false, nil
	}

	return stats, true, nil
}

func (s *SQLiteStore) ReportMensileGiaInviato(monthKey, linea string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(1) FROM monthly_reports WHERE month_key = ? AND line_code = ?`,
		monthKey,
		linea,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("query report mensile già inviato: %w", err)
	}
	return count > 0, nil
}

func (s *SQLiteStore) RegistraInvioReportMensile(monthKey, linea string) error {
	_, err := s.db.Exec(
		`INSERT INTO monthly_reports (month_key, line_code, sent_at) VALUES (?, ?, ?)`,
		monthKey,
		linea,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("registrazione invio report mensile: %w", err)
	}
	return nil
}

func (s *SQLiteStore) statisticheMesePrecedente(linea string, now time.Time) (StatisticheMensili, string, error) {
	ref := now.In(time.UTC)
	inizioMeseCorrente := time.Date(ref.Year(), ref.Month(), 1, 0, 0, 0, 0, time.UTC)
	inizioMesePrecedente := inizioMeseCorrente.AddDate(0, -1, 0)
	monthKey := inizioMesePrecedente.Format("2006-01")

	var (
		count int
		avg   sql.NullFloat64
		max   sql.NullInt64
	)
	err := s.db.QueryRow(`
		SELECT COUNT(1), AVG(delay_minutes), MAX(delay_minutes)
		FROM delay_records
		WHERE line_code = ?
		  AND observed_at >= ?
		  AND observed_at < ?
	`,
		linea,
		inizioMesePrecedente.Format(time.RFC3339),
		inizioMeseCorrente.Format(time.RFC3339),
	).Scan(&count, &avg, &max)
	if err != nil {
		return StatisticheMensili{}, "", fmt.Errorf("calcolo statistiche mensili: %w", err)
	}
	if count == 0 {
		return StatisticheMensili{}, monthKey, nil
	}

	rows, err := s.db.Query(`
		SELECT COALESCE(strftime('%H', scheduled_time), strftime('%H', observed_at)) AS hour,
		       COUNT(1) AS total
		FROM delay_records
		WHERE line_code = ?
		  AND observed_at >= ?
		  AND observed_at < ?
		GROUP BY hour
	`,
		linea,
		inizioMesePrecedente.Format(time.RFC3339),
		inizioMeseCorrente.Format(time.RFC3339),
	)
	if err != nil {
		return StatisticheMensili{}, "", fmt.Errorf("query fasce orarie mensili: %w", err)
	}
	defer rows.Close()

	contatori := map[string]int{
		"Notte (00-05)":      0,
		"Mattina (06-11)":    0,
		"Pomeriggio (12-17)": 0,
		"Sera (18-23)":       0,
	}
	for rows.Next() {
		var (
			hourText string
			total    int
		)
		if err := rows.Scan(&hourText, &total); err != nil {
			return StatisticheMensili{}, "", fmt.Errorf("scan fasce orarie mensili: %w", err)
		}
		fascia := fasciaOraria(hourText)
		contatori[fascia] += total
	}
	if err := rows.Err(); err != nil {
		return StatisticheMensili{}, "", fmt.Errorf("iterazione fasce orarie mensili: %w", err)
	}

	fasciaPeggiore := "N/D"
	ritardiFasciaPeggiore := 0
	for fascia, totale := range contatori {
		if totale > ritardiFasciaPeggiore {
			fasciaPeggiore = fascia
			ritardiFasciaPeggiore = totale
		}
	}

	return StatisticheMensili{
		MeseRiferimento:       monthKey,
		Linea:                 linea,
		NumeroRitardi:         count,
		RitardoMedioMinuti:    avg.Float64,
		RitardoMassimoMinuti:  int(max.Int64),
		FasciaPeggiore:        fasciaPeggiore,
		RitardiFasciaPeggiore: ritardiFasciaPeggiore,
	}, monthKey, nil
}

func fasciaOraria(hourText string) string {
	hourText = strings.TrimSpace(hourText)
	if len(hourText) == 1 {
		hourText = "0" + hourText
	}
	hour, err := time.Parse("15", hourText)
	if err != nil {
		return "N/D"
	}
	h := hour.Hour()
	switch {
	case h >= 0 && h <= 5:
		return "Notte (00-05)"
	case h >= 6 && h <= 11:
		return "Mattina (06-11)"
	case h >= 12 && h <= 17:
		return "Pomeriggio (12-17)"
	default:
		return "Sera (18-23)"
	}
}

func orarioUTC(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
