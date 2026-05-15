// Pacchetto monitor implementa la logica di controllo dei ritardi ferroviari.
package monitor

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/maxysoft/trenitalia_gtfs/bot/internal/config"
	"github.com/maxysoft/trenitalia_gtfs/bot/internal/trenitalia"
)

// NotificaRitardo contiene tutti i dati di un treno in ritardo da notificare.
type NotificaRitardo struct {
	LineaTreno           string
	NumeroTreno          string
	DataServizio         string
	StazioneOrigine      string
	StazioneDestinazione string
	FermateInRitardo     []FermataRitardo
}

// FermataRitardo contiene i dati di ritardo per una singola stazione.
type FermataRitardo struct {
	NomeStazione  string
	OraOraria     time.Time
	OraReale      time.Time
	RitardoMinuti int
	Binario       string
	Stato         string
}

// pausaTraRichieste è l'attesa tra chiamate API consecutive per evitare
// rate-limiting lato server lefrecce.it.
const pausaTraRichieste = 350 * time.Millisecond
type Monitor struct {
	cfg        *config.Config
	client     *trenitalia.Client
	notificati map[string]time.Time
	mu         sync.Mutex
}

// NuovoMonitor crea un nuovo Monitor con la configurazione fornita.
func NuovoMonitor(cfg *config.Config) *Monitor {
	return &Monitor{
		cfg:        cfg,
		client:     trenitalia.NuovoClient(),
		notificati: make(map[string]time.Time),
	}
}

// Controlla esegue un singolo ciclo di controllo:
//  1. Risolve gli ID delle stazioni configurate
//  2. Recupera le soluzioni (treni) per la giornata odierna
//  3. Per ogni treno, recupera le fermate con dati real-time
//  4. Restituisce le notifiche per i treni in ritardo oltre la soglia
func (m *Monitor) Controlla() ([]NotificaRitardo, error) {
	origine, err := m.client.RicercaLocalita(m.cfg.StazioneOrigine)
	if err != nil {
		return nil, fmt.Errorf("stazione origine non trovata '%s': %w", m.cfg.StazioneOrigine, err)
	}

	destinazione, err := m.client.RicercaLocalita(m.cfg.StazioneDestinazione)
	if err != nil {
		return nil, fmt.Errorf("stazione destinazione non trovata '%s': %w", m.cfg.StazioneDestinazione, err)
	}

	oggi := time.Now()
	risposta, err := m.client.CercaSoluzioni(origine.ID, destinazione.ID, oggi)
	if err != nil {
		return nil, fmt.Errorf("ricerca soluzioni: %w", err)
	}

	soglia := time.Duration(m.cfg.RitardoSogliaMinuti) * time.Minute
	tipiAccettati := tipiTrenoSet(m.cfg.TipiTrenoAccettati)

	var notifiche []NotificaRitardo

	for _, wrapper := range risposta.Solutions {
		sol := wrapper.Solution
		if sol.ID == "" {
			continue
		}

		for _, nodo := range sol.Nodes {
			acronimo := estraiAcronimo(nodo)
			if len(tipiAccettati) > 0 && !tipiAccettati[acronimo] {
				continue
			}

			numeroTreno := estraiNumeroTreno(nodo)
			if numeroTreno == "" || nodo.DepartureTime == "" {
				continue
			}

			dataServizio := nodo.DepartureTime[:10]
			chiave := fmt.Sprintf("%s_%s_%s", m.cfg.LineaTreno, numeroTreno, dataServizio)

			m.mu.Lock()
			_, giaNotificato := m.notificati[chiave]
			m.mu.Unlock()
			if giaNotificato {
				continue
			}

			if risposta.CartID == "" {
				continue
			}

			fermate, err := m.client.GetFermate(risposta.CartID, sol.ID)
			if err != nil {
				log.Printf("⚠ impossibile ottenere fermate per treno %s: %v", chiave, err)
				continue
			}

			fermateInRitardo := m.estraiFermateInRitardo(fermate, numeroTreno, soglia)
			if len(fermateInRitardo) == 0 {
				continue
			}

			notifiche = append(notifiche, NotificaRitardo{
				LineaTreno:           m.cfg.LineaTreno,
				NumeroTreno:          numeroTreno,
				DataServizio:         dataServizio,
				StazioneOrigine:      nodo.Origin,
				StazioneDestinazione: nodo.Destination,
				FermateInRitardo:     fermateInRitardo,
			})

			m.mu.Lock()
			m.notificati[chiave] = time.Now()
			m.mu.Unlock()
		}

		// Piccola pausa tra richieste API per non sovraccaricare il server
		time.Sleep(pausaTraRichieste)
	}

	// Rimuove notifiche più vecchie di 24 ore per liberare memoria
	m.pulisciNotifiche()

	return notifiche, nil
}

// estraiFermateInRitardo filtra le fermate del treno con ritardo oltre la soglia.
func (m *Monitor) estraiFermateInRitardo(
	fermate []trenitalia.DettaglioFermate,
	numeroTreno string,
	soglia time.Duration,
) []FermataRitardo {
	var risultato []FermataRitardo

	for _, dettaglio := range fermate {
		trainName := estraiNomeTreno(dettaglio.Summary.TrainInfo)
		if trainName != numeroTreno {
			continue
		}

		for _, f := range dettaglio.Stops {
			nomeFermata := ""
			if f.Location != nil {
				nomeFermata = f.Location.Name
			}

			ritardoSecondi := f.DelaySeconds

			// Se delaySeconds non è disponibile, calcola dalla differenza degli orari
			if ritardoSecondi == 0 {
				oraOraria := parsaOrario(f.DepartureTime)
				oraReale := parsaOrario(primoNonVuoto(f.RealDepartureTime, f.ActualDepartureTime))
				if !oraOraria.IsZero() && !oraReale.IsZero() {
					diff := oraReale.Sub(oraOraria)
					if diff > 0 {
						ritardoSecondi = int(diff.Seconds())
					}
				}
			}

			if time.Duration(ritardoSecondi)*time.Second < soglia {
				continue
			}

			oraOraria := parsaOrario(f.DepartureTime)
			oraReale := parsaOrario(primoNonVuoto(f.RealDepartureTime, f.ActualDepartureTime))
			binario := primoNonVuoto(f.Platform, f.Binario)

			risultato = append(risultato, FermataRitardo{
				NomeStazione:  nomeFermata,
				OraOraria:     oraOraria,
				OraReale:      oraReale,
				RitardoMinuti: ritardoSecondi / 60,
				Binario:       binario,
				Stato:         f.Status,
			})
		}
	}

	return risultato
}

// pulisciNotifiche rimuove le voci più vecchie di 24 ore dalla mappa.
func (m *Monitor) pulisciNotifiche() {
	m.mu.Lock()
	defer m.mu.Unlock()
	limite := time.Now().Add(-24 * time.Hour)
	for chiave, quando := range m.notificati {
		if quando.Before(limite) {
			delete(m.notificati, chiave)
		}
	}
}

// estraiAcronimo restituisce il tipo del treno (es. R, RV, RE) in maiuscolo.
func estraiAcronimo(nodo trenitalia.Nodo) string {
	if nodo.Train != nil && nodo.Train.Acronym != "" {
		return strings.ToUpper(nodo.Train.Acronym)
	}
	if nodo.TrainInfo != nil && nodo.TrainInfo.Acronym != "" {
		return strings.ToUpper(nodo.TrainInfo.Acronym)
	}
	return strings.ToUpper(nodo.TrainAcronym)
}

// estraiNumeroTreno restituisce il numero identificativo del treno.
func estraiNumeroTreno(nodo trenitalia.Nodo) string {
	if nodo.Train != nil {
		if nodo.Train.Name != "" {
			return strings.TrimSpace(nodo.Train.Name)
		}
		if nodo.Train.Description != "" {
			return strings.TrimSpace(nodo.Train.Description)
		}
	}
	if nodo.TrainInfo != nil {
		if nodo.TrainInfo.Name != "" {
			return strings.TrimSpace(nodo.TrainInfo.Name)
		}
		if nodo.TrainInfo.Description != "" {
			return strings.TrimSpace(nodo.TrainInfo.Description)
		}
	}
	return ""
}

// estraiNomeTreno restituisce il nome/numero dal campo TrainInfo del riepilogo.
func estraiNomeTreno(info *trenitalia.InfoTreno) string {
	if info == nil {
		return ""
	}
	if info.Name != "" {
		return strings.TrimSpace(info.Name)
	}
	return strings.TrimSpace(info.Description)
}

// tipiTrenoSet costruisce un set (mappa) dai tipi di treno configurati.
func tipiTrenoSet(tipi []string) map[string]bool {
	m := make(map[string]bool, len(tipi))
	for _, t := range tipi {
		m[strings.ToUpper(strings.TrimSpace(t))] = true
	}
	return m
}

// parsaOrario analizza una stringa ISO 8601 e restituisce un time.Time.
func parsaOrario(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	formati := []string{
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02T15:04:05Z07:00",
		time.RFC3339,
	}
	for _, f := range formati {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// primoNonVuoto restituisce il primo valore non vuoto tra quelli forniti.
func primoNonVuoto(valori ...string) string {
	for _, v := range valori {
		if v != "" {
			return v
		}
	}
	return ""
}
