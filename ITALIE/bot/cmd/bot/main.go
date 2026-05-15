// Punto di ingresso del bot Telegram per il monitoraggio ritardi Trenitalia.
package main

import (
	"log"
	"os"
	"os/signal"
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

	tg := telegram.NuovoClient(cfg.TelegramToken, cfg.TelegramChatID)
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

	// Prima esecuzione immediata senza attendere il ticker
	eseguiControllo(mon, tg, store)
	verificaEInviaReportMensile(cfg, tg, store)

	for {
		select {
		case <-ticker.C:
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
		log.Printf("❌ Errore durante il controllo: %v", err)
		return
	}

	if len(notifiche) == 0 {
		log.Println("✅ Nessun ritardo significativo rilevato")
		return
	}

	log.Printf("⚠ Trovati %d treni in ritardo — invio notifiche...", len(notifiche))

	for _, n := range notifiche {
		if err := store.SalvaNotifica(n); err != nil {
			log.Printf("⚠ Errore salvataggio ritardi treno %s su SQLite: %v", n.NumeroTreno, err)
		}
		if err := tg.InviaNotificaRitardo(n); err != nil {
			log.Printf("❌ Errore invio notifica treno %s: %v", n.NumeroTreno, err)
		} else {
			log.Printf("📨 Notifica inviata per treno %s (%d fermate in ritardo)",
				n.NumeroTreno, len(n.FermateInRitardo))
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
