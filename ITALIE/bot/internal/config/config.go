// Pacchetto config legge la configurazione del bot tramite variabili d'ambiente.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contiene tutti i parametri di configurazione del bot.
type Config struct {
	// Token del bot Telegram fornito da @BotFather
	TelegramToken string
	// ID numerico della chat/gruppo Telegram a cui inviare le notifiche
	TelegramChatID int64
	// Codice linea ferroviaria da monitorare (es. FL3, FL1, FL7)
	LineaTreno string
	// Nome stazione di origine (es. Roma Ostiense)
	StazioneOrigine string
	// Nome stazione di destinazione (es. Viterbo Porta Fiorentina)
	StazioneDestinazione string
	// Soglia minima di ritardo in minuti per inviare una notifica
	RitardoSogliaMinuti int
	// Intervallo tra un controllo e il successivo
	IntervalloPolling time.Duration
	// Tipi di treno accettati (es. R,RV,RE per regionali)
	TipiTrenoAccettati []string
	// Percorso del database SQLite dei ritardi
	SQLiteDBPath string
	// Abilita invio report mensile automatico
	ReportMensileAbilitato bool
	// Chat ID Telegram per report mensili (fallback: TelegramChatID)
	ReportTelegramChatID int64
}

// LeggiConfig legge la configurazione dalle variabili d'ambiente e restituisce
// un oggetto Config o un errore se i campi obbligatori mancano.
func LeggiConfig() (*Config, error) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("variabile TELEGRAM_BOT_TOKEN non impostata")
	}

	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" {
		return nil, fmt.Errorf("variabile TELEGRAM_CHAT_ID non impostata")
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("TELEGRAM_CHAT_ID non valido: %w", err)
	}

	lineaTreno := valorePredefinito("LINEA_TRENO", "FL3")
	stazioneOrigine := valorePredefinito("STAZIONE_ORIGINE", "Roma Ostiense")
	stazioneDestinazione := valorePredefinito("STAZIONE_DESTINAZIONE", "Viterbo Porta Fiorentina")

	ritardoSoglia := interi("RITARDO_SOGLIA_MINUTI", 10)
	if ritardoSoglia <= 0 {
		ritardoSoglia = 10
	}

	intervalloSec := interi("INTERVALLO_POLLING_SECONDI", 300)
	if intervalloSec <= 0 {
		intervalloSec = 300
	}

	tipiTreno := []string{"R", "REGIONALE", "RV", "RE"}
	if v := os.Getenv("TIPI_TRENO"); v != "" {
		tipiTreno = splitVirgola(v)
	}

	reportChatID := chatID
	if reportChatIDStr := os.Getenv("REPORT_TELEGRAM_CHAT_ID"); reportChatIDStr != "" {
		parsedReportChatID, parseErr := strconv.ParseInt(reportChatIDStr, 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("REPORT_TELEGRAM_CHAT_ID non valido: %w", parseErr)
		}
		reportChatID = parsedReportChatID
	}

	return &Config{
		TelegramToken:        token,
		TelegramChatID:       chatID,
		LineaTreno:           lineaTreno,
		StazioneOrigine:      stazioneOrigine,
		StazioneDestinazione: stazioneDestinazione,
		RitardoSogliaMinuti:  ritardoSoglia,
		IntervalloPolling:    time.Duration(intervalloSec) * time.Second,
		TipiTrenoAccettati:   tipiTreno,
		SQLiteDBPath:         valorePredefinito("SQLITE_DB_PATH", "/data/ritardi.sqlite"),
		ReportMensileAbilitato: booleano(
			"INVIO_REPORT_MENSILE",
			true,
		),
		ReportTelegramChatID: reportChatID,
	}, nil
}

func valorePredefinito(chiave, predefinito string) string {
	if v := os.Getenv(chiave); v != "" {
		return v
	}
	return predefinito
}

func interi(chiave string, predefinito int) int {
	s := os.Getenv(chiave)
	if s == "" {
		return predefinito
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return predefinito
	}
	return v
}

func splitVirgola(s string) []string {
	var risultato []string
	for _, parte := range splitStr(s, ',') {
		parte = strings.TrimSpace(parte)
		if parte != "" {
			risultato = append(risultato, parte)
		}
	}
	return risultato
}

func booleano(chiave string, predefinito bool) bool {
	valore := strings.TrimSpace(strings.ToLower(os.Getenv(chiave)))
	if valore == "" {
		return predefinito
	}
	switch valore {
	case "1", "true", "vero", "yes", "y", "on":
		return true
	case "0", "false", "falso", "no", "n", "off":
		return false
	default:
		return predefinito
	}
}

func splitStr(s string, sep rune) []string {
	var parts []string
	current := []rune{}
	for _, r := range s {
		if r == sep {
			parts = append(parts, string(current))
			current = current[:0]
		} else {
			current = append(current, r)
		}
	}
	parts = append(parts, string(current))
	return parts
}
