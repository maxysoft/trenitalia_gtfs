// Pacchetto monitor implementa la logica di controllo dei ritardi ferroviari.
package monitor

import (
	"fmt"
	"log"
	"strconv"
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
	// Recuperato is true when the train has recovered (delay dropped below threshold).
	Recuperato bool
	// UltimaPosizioneNota is the last known station from andamentoTreno.
	UltimaPosizioneNota string
	// RitardoAttuale is the current overall delay in minutes from andamentoTreno.
	RitardoAttuale int
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

// trenoKey contiene i dati necessari per chiamare andamentoTreno per un servizio specifico.
type trenoKey struct {
	codOrigine string
	numero     string
	dataMs     int64
}

// statoTreno tracks the current delay state of a single train service.
type statoTreno struct {
	inRitardo    bool
	aggiornatoAl time.Time
}

// Monitor checks train delays against the configured threshold.
type Monitor struct {
	cfg           *config.Config
	client        *trenitalia.Client
	stati         map[string]*statoTreno
	seenToday     map[string]*trenoKey
	origineVTCode string
	destVTCode    string
	extraVTCodes  []string
	oggi          string
	mu            sync.Mutex
}

// NuovoMonitor creates a new Monitor with the given configuration.
func NuovoMonitor(cfg *config.Config) *Monitor {
	return &Monitor{
		cfg:       cfg,
		client:    trenitalia.NuovoClient(),
		stati:     make(map[string]*statoTreno),
		seenToday: make(map[string]*trenoKey),
	}
}

// Controlla esegue un ciclo di verifica ritardi.
// Scopre i treni in transito tramite l'endpoint viaggiatreno.it /partenze,
// poi controlla il ritardo in tempo reale per ogni treno noto tramite /andamentoTreno.
// Restituisce le notifiche per i treni il cui stato ha cambiato soglia.
func (m *Monitor) Controlla() ([]NotificaRitardo, error) {
	if err := m.inizializzaCodiciFermate(); err != nil {
		return nil, err
	}

	// Reset seenToday al cambio di giornata.
	oggi := time.Now().Format("2006-01-02")
	m.mu.Lock()
	if m.oggi != oggi {
		m.seenToday = make(map[string]*trenoKey)
		m.oggi = oggi
	}
	m.mu.Unlock()

	// Scopri i treni dalle stazioni terminali configurate più quelle extra.
	codiceDaInterrogare := append([]string{m.origineVTCode, m.destVTCode}, m.extraVTCodes...)
	for _, codice := range codiceDaInterrogare {
		partenze, err := m.client.Partenze(codice, time.Now())
		if err != nil {
			log.Printf("warning: partenze per %s: %v", codice, err)
			continue
		}
		if m.cfg.DebugAbilitato {
			log.Printf("[DEBUG] partenze %s: %d treni rilevati", codice, len(partenze))
		}
		for _, p := range partenze {
			m.aggiungiSeenToday(p)
		}
	}

	soglia := time.Duration(m.cfg.RitardoSogliaMinuti) * time.Minute
	tipi := tipiTrenoSet(m.cfg.TipiTrenoAccettati)

	m.mu.Lock()
	keys := make([]*trenoKey, 0, len(m.seenToday))
	for _, k := range m.seenToday {
		keys = append(keys, k)
	}
	m.mu.Unlock()

	var notifiche []NotificaRitardo
	for _, key := range keys {
		andamento, err := m.client.AndamentoTreno(key.codOrigine, key.numero, key.dataMs)
		if err != nil {
			log.Printf("warning: andamento treno %s: %v", key.numero, err)
			continue
		}

		// Salta treni non ancora partiti o già arrivati senza ritardo.
		if andamento.NonPartito {
			continue
		}
		if andamento.Arrivato && andamento.Ritardo <= 0 {
			continue
		}

		// Filtra per tipo treno: controlla sia il codice breve sia la descrizione.
		if !matchTipoTreno(andamento.Categoria, andamento.CategoriaDescrizione, tipi) {
			if m.cfg.DebugAbilitato {
				log.Printf("[DEBUG] skip treno=%s categoria=%q desc=%q non in filtro",
					key.numero, andamento.Categoria, andamento.CategoriaDescrizione)
			}
			continue
		}

		if m.cfg.DebugAbilitato {
			log.Printf("[DEBUG] treno=%s cat=%q ritardo=%d dest=%q",
				key.numero, andamento.Categoria, andamento.Ritardo, andamento.Destinazione)
		}

		fermateInRitardo := m.estraiFermateInRitardoVT(andamento, soglia)
		chiave := fmt.Sprintf("%s_%s_%d", m.cfg.LineaTreno, key.numero, key.dataMs)
		changed, nowDelayed := m.aggiornaStato(chiave, len(fermateInRitardo) > 0)
		if !changed {
			continue
		}

		notifiche = append(notifiche, NotificaRitardo{
			LineaTreno:           m.cfg.LineaTreno,
			NumeroTreno:          key.numero,
			DataServizio:         andamento.DataPartenzaTrenoAsDate,
			StazioneOrigine:      andamento.Origine,
			StazioneDestinazione: andamento.Destinazione,
			FermateInRitardo:     fermateInRitardo,
			Recuperato:           !nowDelayed,
			UltimaPosizioneNota:  andamento.StazioneUltimoRilevamento,
			RitardoAttuale:       andamento.Ritardo,
		})

		time.Sleep(pausaTraRichieste)
	}

	m.pulisciStati()
	return notifiche, nil
}

// inizializzaCodiciFermate risolve i nomi delle stazioni in codici viaggiatreno
// tramite l'API lefrecce.it e li memorizza nella cache. Viene chiamato una sola volta.
func (m *Monitor) inizializzaCodiciFermate() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.origineVTCode != "" {
		return nil
	}

	origine, err := m.client.RicercaLocalita(m.cfg.StazioneOrigine)
	if err != nil {
		return fmt.Errorf("stazione origine non trovata '%s': %w", m.cfg.StazioneOrigine, err)
	}
	destinazione, err := m.client.RicercaLocalita(m.cfg.StazioneDestinazione)
	if err != nil {
		return fmt.Errorf("stazione destinazione non trovata '%s': %w", m.cfg.StazioneDestinazione, err)
	}

	m.origineVTCode = trenitalia.ConvertiCodiceVT(origine.ID)
	m.destVTCode = trenitalia.ConvertiCodiceVT(destinazione.ID)

	for _, nomeStazione := range m.cfg.StazioniExtraPartenze {
		loc, err := m.client.RicercaLocalita(nomeStazione)
		if err != nil {
			log.Printf("warning: stazione extra non trovata '%s': %v", nomeStazione, err)
			continue
		}
		m.extraVTCodes = append(m.extraVTCodes, trenitalia.ConvertiCodiceVT(loc.ID))
	}

	if m.cfg.DebugAbilitato {
		log.Printf("[DEBUG] stazioni VT: origine=%q (%s) destinazione=%q (%s)",
			origine.Name, m.origineVTCode, destinazione.Name, m.destVTCode)
	}
	return nil
}

// aggiungiSeenToday aggiunge un treno al set dei treni noti per oggi, se non già presente.
// I treni la cui origine/destinazione non corrispondono alle keyword configurate vengono ignorati.
func (m *Monitor) aggiungiSeenToday(p trenitalia.PartenzaVT) {
	if !m.matchStazioneLinea(p.Origine, p.Destinazione) {
		if m.cfg.DebugAbilitato {
			log.Printf("[DEBUG] skip partenza %d orig=%q dest=%q non in StazioniLinea",
				p.NumeroTreno, p.Origine, p.Destinazione)
		}
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%d_%d", p.NumeroTreno, p.DataPartenzaTreno)
	if _, exists := m.seenToday[key]; !exists {
		m.seenToday[key] = &trenoKey{
			codOrigine: p.CodOrigine,
			numero:     strconv.Itoa(p.NumeroTreno),
			dataMs:     p.DataPartenzaTreno,
		}
	}
}

// matchStazioneLinea verifica se l'origine o la destinazione del treno corrisponde
// a una delle keyword configurate in StazioniLinea (confronto case-insensitive, sottostringa).
// Restituisce true se la lista è vuota (nessun filtro) o se almeno una keyword corrisponde.
// Restituisce true anche se origine e destinazione sono entrambe vuote (dati non disponibili).
func (m *Monitor) matchStazioneLinea(origine, destinazione string) bool {
	if len(m.cfg.StazioniLinea) == 0 {
		return true
	}
	// Allow through if no data is available; andamentoTreno will provide better info.
	if origine == "" && destinazione == "" {
		return true
	}
	combined := strings.ToUpper(origine) + " " + strings.ToUpper(destinazione)
	for _, s := range m.cfg.StazioniLinea {
		keyword := strings.ToUpper(strings.TrimSpace(s))
		if keyword != "" && strings.Contains(combined, keyword) {
			return true
		}
	}
	return false
}

// estraiFermateInRitardoVT estrae le fermate con ritardo oltre la soglia
// dalla risposta andamentoTreno.
func (m *Monitor) estraiFermateInRitardoVT(andamento *trenitalia.AndamentoVT, soglia time.Duration) []FermataRitardo {
	var risultato []FermataRitardo
	loc := fusoOrarioItalia()

	for _, f := range andamento.Fermate {
		ritardoMinuti := f.RitardoPartenza
		if f.RitardoArrivo > ritardoMinuti {
			ritardoMinuti = f.RitardoArrivo
		}

		const sogliaDebugMin = 2
		if m.cfg.DebugAbilitato && ritardoMinuti > sogliaDebugMin {
			log.Printf("[DEBUG] treno=%d fermata=%q ritardo=%dm",
				andamento.NumeroTreno, f.Stazione, ritardoMinuti)
		}

		if time.Duration(ritardoMinuti)*time.Minute < soglia {
			continue
		}

		var oraOraria, oraReale time.Time
		scheduled := primoNonZeroInt64(f.PartenzaTeorica, f.ArrivoTeoricoMs)
		actual := primoNonZeroInt64(f.PartenzaReale, f.ArrivoReale)
		if scheduled > 0 {
			oraOraria = time.UnixMilli(scheduled).In(loc)
		}
		if actual > 0 {
			oraReale = time.UnixMilli(actual).In(loc)
		}

		risultato = append(risultato, FermataRitardo{
			NomeStazione:  f.Stazione,
			OraOraria:     oraOraria,
			OraReale:      oraReale,
			RitardoMinuti: ritardoMinuti,
			Binario:       f.BinarioEffettivoPDescr,
		})
	}
	return risultato
}

// matchTipoTreno controlla se la categoria del treno è inclusa nel filtro configurato.
// Verifica sia il codice breve (es. "REG") sia la descrizione (es. "Regionale").
func matchTipoTreno(categoria, categoriaDescrizione string, tipi map[string]bool) bool {
	if len(tipi) == 0 {
		return true
	}
	if tipi[strings.ToUpper(categoria)] {
		return true
	}
	return tipi[strings.ToUpper(categoriaDescrizione)]
}

// fusoOrarioItalia restituisce il fuso orario italiano (Europe/Rome).
func fusoOrarioItalia() *time.Location {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		loc = time.FixedZone("CEST", 7200)
	}
	return loc
}

// primoNonZeroInt64 restituisce il primo valore int64 non zero.
func primoNonZeroInt64(vals ...int64) int64 {
	for _, v := range vals {
		if v != 0 {
			return v
		}
	}
	return 0
}

// aggiornaStato updates the delay state for a train.
// Returns (stateChanged, newDelayedValue).
func (m *Monitor) aggiornaStato(chiave string, inRitardo bool) (bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stato, exists := m.stati[chiave]
	if !exists {
		stato = &statoTreno{}
		m.stati[chiave] = stato
	}

	changed := stato.inRitardo != inRitardo
	stato.inRitardo = inRitardo
	stato.aggiornatoAl = time.Now()
	return changed, inRitardo
}

// pulisciStati removes state entries older than 24 hours.
func (m *Monitor) pulisciStati() {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().Add(-24 * time.Hour)
	for chiave, stato := range m.stati {
		if stato.aggiornatoAl.Before(cutoff) {
			delete(m.stati, chiave)
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
