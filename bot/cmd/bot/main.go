// Punto di ingresso del bot Telegram per il monitoraggio ritardi Trenitalia.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/maxysoft/trenitalia_gtfs/bot/internal/config"
	"github.com/maxysoft/trenitalia_gtfs/bot/internal/monitor"
	"github.com/maxysoft/trenitalia_gtfs/bot/internal/storage"
	"github.com/maxysoft/trenitalia_gtfs/bot/internal/telegram"
)

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println("🚂 Bot Monitoraggio Trenitalia — avvio in corso...")

	cfg, err := config.LeggiConfig()
	if err != nil {
		log.Fatalf("❌ Errore configurazione: %v", err)
	}

	log.Printf("✅ Configurazione caricata")
	log.Printf("   Linea:    %s", cfg.LineaTreno)
	log.Printf("   Percorso: %s → %s", cfg.StazioneOrigine, cfg.StazioneDestinazione)
	log.Printf("   Soglia:   %d minuti", cfg.RitardoSogliaMinuti)
	log.Printf("   Polling:  ogni %v", cfg.IntervalloPolling)
	log.Printf("   SQLite:   %s", cfg.SQLiteDBPath)
	log.Printf("   Report:   abilitato=%t chat=%d", cfg.ReportMensileAbilitato, cfg.ReportTelegramChatID)
	log.Printf("   Admin:    chat=%d", cfg.AdminChatID)
	log.Printf("   Pausa:    %s \u2013 %s (Europe/Rome)", cfg.PausaNotturnaInizio, cfg.PausaNotturnaFine)
	if cfg.DebugAbilitato {
		log.Printf("   ⚠ DEBUG:  attivo — log ritardi >2 min abilitato")
	}

	tg := telegram.NuovoClient(cfg.TelegramToken, cfg.TelegramChatID, cfg.AdminChatID)
	mon := monitor.NuovoMonitor(cfg)
	store, err := storage.NuovoSQLiteStore(cfg.SQLiteDBPath)
	if err != nil {
		log.Fatalf("❌ Errore inizializzazione database SQLite: %v", err)
	}
	defer store.Chiudi()

	// Messaggio di avvio su Telegram
	if err := tg.InviaMessaggioAvvio(cfg.LineaTreno, cfg.RitardoSogliaMinuti, cfg.IntervalloPolling); err != nil {
		log.Printf("⚠ Impossibile inviare messaggio di avvio: %v", err)
	}

	// Canale per segnali di terminazione (Ctrl+C, docker stop, ecc.)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(cfg.IntervalloPolling)
	defer ticker.Stop()

	// Prima esecuzione immediata senza attendere il ticker (solo fuori dalla pausa notturna).
	if !eOraNotturnaPausa(cfg) {
		eseguiControllo(mon, tg, store)
		verificaEInviaReportMensile(cfg, tg, store)
	} else {
		log.Printf("🌙 Pausa notturna attiva (%s\u2013%s) — avvio controllo rinviato",
			cfg.PausaNotturnaInizio, cfg.PausaNotturnaFine)
	}

	for {
		select {
		case <-ticker.C:
			if eOraNotturnaPausa(cfg) {
				log.Printf("🌙 Pausa notturna (%s\u2013%s) — polling sospeso",
					cfg.PausaNotturnaInizio, cfg.PausaNotturnaFine)
				continue
			}
			eseguiControllo(mon, tg, store)
			verificaEInviaReportMensile(cfg, tg, store)
		case <-stop:
			log.Println("🛑 Segnale di arresto ricevuto — bot in chiusura...")
			return
		}
	}
}

// eseguiControllo esegue un ciclo di controllo e invia le notifiche necessarie.
func eseguiControllo(mon *monitor.Monitor, tg *telegram.Client, store *storage.SQLiteStore) {
	log.Println("🔍 Controllo ritardi in corso...")

	notifiche, err := mon.Controlla()
	if err != nil {
		log.Printf("error during check: %v", err)
		_ = tg.InviaErroreAdmin(err)
		return
	}

	if len(notifiche) == 0 {
		log.Println("✅ Nessun ritardo significativo rilevato")
		return
	}

	log.Printf("%d notifications to send (delays/recoveries)", len(notifiche))

	for _, n := range notifiche {
		if !n.Recuperato {
			if err := store.SalvaNotifica(n); err != nil {
				log.Printf("error saving train %s to SQLite: %v", n.NumeroTreno, err)
			}
		}
		if err := tg.InviaNotificaRitardo(n); err != nil {
			log.Printf("error sending notification for train %s: %v", n.NumeroTreno, err)
		} else {
			if n.Recuperato {
				log.Printf("recovery notification sent for train %s", n.NumeroTreno)
			} else {
				log.Printf("delay notification sent for train %s (%d delayed stops)", n.NumeroTreno, len(n.FermateInRitardo))
			}
		}
		// Pausa tra messaggi per evitare il rate-limit di Telegram
		time.Sleep(500 * time.Millisecond)
	}
}

func verificaEInviaReportMensile(cfg *config.Config, tg *telegram.Client, store *storage.SQLiteStore) {
	if !cfg.ReportMensileAbilitato {
		return
	}
	ora := time.Now()
	if ora.Day() != 1 {
		return
	}

	stats, daInviare, err := store.ReportMensileDaInviare(cfg.LineaTreno, ora)
	if err != nil {
		log.Printf("⚠ Errore calcolo report mensile: %v", err)
		return
	}
	if !daInviare {
		return
	}

	if err := tg.InviaReportMensile(cfg.ReportTelegramChatID, stats); err != nil {
		log.Printf("⚠ Errore invio report mensile: %v", err)
		return
	}

	if err := store.RegistraInvioReportMensile(stats.MeseRiferimento, stats.Linea); err != nil {
		log.Printf("⚠ Report inviato ma non registrato su SQLite: %v", err)
		return
	}

	log.Printf("📈 Report mensile inviato per linea %s (%s)", stats.Linea, stats.MeseRiferimento)
}

// eOraNotturnaPausa restituisce true se l'orario attuale (Europe/Rome) cade
// nella finestra di pausa notturna configurata.
func eOraNotturnaPausa(cfg *config.Config) bool {
	if cfg.PausaNotturnaInizio == "" || cfg.PausaNotturnaFine == "" {
		return false
	}
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		loc = time.FixedZone("CET", 3600)
	}
	ora := time.Now().In(loc)
	inizio, err1 := parseHHMM(cfg.PausaNotturnaInizio)
	fine, err2 := parseHHMM(cfg.PausaNotturnaFine)
	if err1 != nil || err2 != nil {
		log.Printf("⚠ Orario pausa notturna non valido (%q, %q): %v %v",
			cfg.PausaNotturnaInizio, cfg.PausaNotturnaFine, err1, err2)
		return false
	}
	attuale := ora.Hour()*60 + ora.Minute()
	if inizio <= fine {
		return attuale >= inizio && attuale < fine
	}
	// Pausa che supera la mezzanotte (es. 22:00 – 04:30)
	return attuale >= inizio || attuale < fine
}

// parseHHMM converte una stringa "HH:MM" in minuti dall'inizio della giornata.
func parseHHMM(s string) (int, error) {
	parti := strings.SplitN(s, ":", 2)
	if len(parti) != 2 {
		return 0, fmt.Errorf("formato orario non valido: %q (atteso HH:MM)", s)
	}
	h, err := strconv.Atoi(parti[0])
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("ora non valida in %q", s)
	}
	m, err := strconv.Atoi(parti[1])
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("minuti non validi in %q", s)
	}
	return h*60 + m, nil
}
