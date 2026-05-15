// Pacchetto trenitalia fornisce un client HTTP per le API pubbliche lefrecce.it.
package trenitalia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiBase      = "https://www.lefrecce.it/Channels.Website.BFF.WEB"
	apiLocalita  = apiBase + "/website/locations/search"
	apiSoluzioni = apiBase + "/website/ticket/solutions"
	apiFermate   = apiBase + "/website/stops"

	timeoutRichiesta = 20 * time.Second
)

// userAgent simula un browser per evitare blocchi lato API.
const userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0 Safari/537.36"

// Client è il client HTTP per le API di Trenitalia/lefrecce.it.
type Client struct {
	http *http.Client
}

// NuovoClient crea un nuovo client con timeout configurato.
func NuovoClient() *Client {
	return &Client{
		http: &http.Client{Timeout: timeoutRichiesta},
	}
}

// RicercaLocalita cerca una stazione per nome e restituisce la prima corrispondenza.
func (c *Client) RicercaLocalita(nome string) (*Localita, error) {
	u := fmt.Sprintf("%s?name=%s&limit=10", apiLocalita, url.QueryEscape(nome))
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creazione richiesta: %w", err)
	}
	c.impostaHeader(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("richiesta API localita: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		corpo, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d dalla ricerca localita: %s", resp.StatusCode, tronca(string(corpo), 200))
	}

	var risultati []Localita
	if err := json.NewDecoder(resp.Body).Decode(&risultati); err != nil {
		return nil, fmt.Errorf("decodifica risposta localita: %w", err)
	}

	if len(risultati) == 0 {
		return nil, fmt.Errorf("nessuna stazione trovata per: %s", nome)
	}

	nomeNorm := normalizza(nome)
	for _, r := range risultati {
		if normalizza(r.Name) == nomeNorm {
			return &r, nil
		}
	}
	// Nessuna corrispondenza esatta: restituisce il primo risultato
	return &risultati[0], nil
}

// CercaSoluzioni esegue una POST per trovare i treni tra due stazioni in una data.
func (c *Client) CercaSoluzioni(origineID, destinazioneID string, giorno time.Time) (*RispostaPercorsi, error) {
	richiesta := RichiestaPercorsi{
		DepartureLocationId: origineID,
		ArrivalLocationId:   destinazioneID,
		DepartureTime:       formatDataItalia(giorno),
		Adults:              1,
		Children:            0,
		Criteria: CriteriaPercorso{
			RegionalOnly: true,
			NoChanges:    true,
			Order:        "DEPARTURE_DATE",
			Limit:        50,
			Offset:       0,
		},
		AdvancedSearchRequest: RicercaAvanzata{},
	}

	corpo, err := json.Marshal(richiesta)
	if err != nil {
		return nil, fmt.Errorf("serializzazione richiesta soluzioni: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiSoluzioni, bytes.NewReader(corpo))
	if err != nil {
		return nil, fmt.Errorf("creazione richiesta soluzioni: %w", err)
	}
	c.impostaHeader(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("richiesta API soluzioni: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d dalla ricerca soluzioni: %s", resp.StatusCode, tronca(string(b), 200))
	}

	var risultato RispostaPercorsi
	if err := json.NewDecoder(resp.Body).Decode(&risultato); err != nil {
		return nil, fmt.Errorf("decodifica risposta soluzioni: %w", err)
	}

	return &risultato, nil
}

// GetFermate recupera le fermate dettagliate (con orari reali) per una soluzione.
func (c *Client) GetFermate(cartID, solutionID string) ([]DettaglioFermate, error) {
	u := fmt.Sprintf("%s?cartId=%s&solutionId=%s",
		apiFermate, url.QueryEscape(cartID), url.QueryEscape(solutionID))

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("creazione richiesta fermate: %w", err)
	}
	c.impostaHeader(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("richiesta API fermate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d dalle fermate: %s", resp.StatusCode, tronca(string(b), 200))
	}

	var risultati []DettaglioFermate
	if err := json.NewDecoder(resp.Body).Decode(&risultati); err != nil {
		return nil, fmt.Errorf("decodifica risposta fermate: %w", err)
	}

	return risultati, nil
}

// impostaHeader aggiunge gli header necessari per simulare una richiesta browser.
func (c *Client) impostaHeader(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://www.lefrecce.it")
	req.Header.Set("Referer", "https://www.lefrecce.it/")
	req.Header.Set("User-Agent", userAgent)
}

// normalizza converte in maiuscolo e rimuove spazi iniziali/finali.
func normalizza(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// formatDataItalia formatta una data nel fuso orario italiano per le API.
func formatDataItalia(t time.Time) string {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		// Fallback UTC+1 se il database dei fusi orari non è disponibile
		loc = time.FixedZone("CET", 3600)
	}
	t = time.Date(t.Year(), t.Month(), t.Day(), 6, 0, 0, 0, loc)
	_, offset := t.Zone()
	segno := "+"
	if offset < 0 {
		segno = "-"
		offset = -offset
	}
	h := offset / 3600
	m := (offset % 3600) / 60
	return fmt.Sprintf("%04d-%02d-%02dT06:00:00.000%s%02d:%02d",
		t.Year(), int(t.Month()), t.Day(), segno, h, m)
}

// tronca limita una stringa a maxLen caratteri.
func tronca(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
