// Pacchetto trenitalia definisce i modelli dati per l'API lefrecce.it.
package trenitalia

import "encoding/json"

// Localita rappresenta una stazione restituita dall'API di ricerca.
type Localita struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// UnmarshalJSON handles both string and numeric IDs from the API.
// The lefrecce.it API returns numeric IDs, so we normalize to string.
func (l *Localita) UnmarshalJSON(data []byte) error {
	type Alias struct {
		ID   json.RawMessage `json:"id"`
		Name string          `json:"name"`
	}
	var a Alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	l.Name = a.Name
	if len(a.ID) == 0 {
		return nil
	}
	// Try string first, then number
	var s string
	if err := json.Unmarshal(a.ID, &s); err == nil {
		l.ID = s
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(a.ID, &n); err != nil {
		return err
	}
	l.ID = n.String()
	return nil
}

// RichiestaPercorsi è il corpo della POST /website/ticket/solutions.
type RichiestaPercorsi struct {
	DepartureLocationId   string           `json:"departureLocationId"`
	ArrivalLocationId     string           `json:"arrivalLocationId"`
	DepartureTime         string           `json:"departureTime"`
	Adults                int              `json:"adults"`
	Children              int              `json:"children"`
	Criteria              CriteriaPercorso `json:"criteria"`
	AdvancedSearchRequest RicercaAvanzata  `json:"advancedSearchRequest"`
}

// CriteriaPercorso contiene i filtri per la ricerca delle soluzioni.
type CriteriaPercorso struct {
	FrecceOnly    bool   `json:"frecceOnly"`
	RegionalOnly  bool   `json:"regionalOnly"`
	IntercityOnly bool   `json:"intercityOnly"`
	TourismOnly   bool   `json:"tourismOnly"`
	NoChanges     bool   `json:"noChanges"`
	Order         string `json:"order"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
}

// RicercaAvanzata contiene filtri aggiuntivi per la ricerca.
type RicercaAvanzata struct {
	BestFare   bool `json:"bestFare"`
	BikeFilter bool `json:"bikeFilter"`
}

// RispostaPercorsi è la risposta JSON da /website/ticket/solutions.
type RispostaPercorsi struct {
	CartID    string             `json:"cartId"`
	Solutions []WrapperSoluzione `json:"solutions"`
}

// WrapperSoluzione avvolge un singolo oggetto soluzione.
type WrapperSoluzione struct {
	Solution Soluzione `json:"solution"`
}

// Soluzione rappresenta un singolo percorso con uno o più treni.
type Soluzione struct {
	ID    string `json:"id"`
	Nodes []Nodo `json:"nodes"`
}

// Nodo rappresenta un segmento del percorso (singolo treno).
type Nodo struct {
	Train         *InfoTreno `json:"train"`
	TrainInfo     *InfoTreno `json:"trainInfo"`
	TrainAcronym  string     `json:"trainAcronym"`
	DepartureTime string     `json:"departureTime"`
	ArrivalTime   string     `json:"arrivalTime"`
	Origin        string     `json:"origin"`
	Destination   string     `json:"destination"`
}

// InfoTreno contiene i dati identificativi di un treno.
type InfoTreno struct {
	Acronym     string `json:"acronym"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// DettaglioFermate è la risposta di /website/stops per una soluzione.
type DettaglioFermate struct {
	Summary RiepilogoSoluzione `json:"summary"`
	Stops   []Fermata          `json:"stops"`
}

// RiepilogoSoluzione contiene le informazioni riassuntive del treno.
type RiepilogoSoluzione struct {
	TrainInfo *InfoTreno `json:"trainInfo"`
}

// Fermata rappresenta una singola stazione nel percorso del treno,
// con orari previsti, orari reali, stato e ritardo.
type Fermata struct {
	Location            *LocalitaFermata `json:"location"`
	DepartureTime       string           `json:"departureTime"`
	ArrivalTime         string           `json:"arrivalTime"`
	RealDepartureTime   string           `json:"realDepartureTime"`
	ActualDepartureTime string           `json:"actualDepartureTime"`
	RealArrivalTime     string           `json:"realArrivalTime"`
	ActualArrivalTime   string           `json:"actualArrivalTime"`
	TrainNumber         string           `json:"trainNumber"`
	Platform            string           `json:"platform"`
	Binario             string           `json:"binario"`
	Status              string           `json:"status"`
	DelaySeconds        int              `json:"delaySeconds"`
}

// LocalitaFermata è il sotto-oggetto con il nome della stazione.
type LocalitaFermata struct {
	Name string `json:"name"`
}

// PartenzaVT è una singola partenza restituita dall'endpoint viaggiatreno.it /partenze.
type PartenzaVT struct {
	NumeroTreno          int    `json:"numeroTreno"`
	Categoria            string `json:"categoria"`
	CategoriaDescrizione string `json:"categoriaDescrizione"`
	CodOrigine           string `json:"codOrigine"`
	Origine              string `json:"origine"`
	Destinazione         string `json:"destinazione"`
	DataPartenzaTreno    int64  `json:"dataPartenzaTreno"`
	Ritardo              int    `json:"ritardo"`
	Circolante           bool   `json:"circolante"`
	NonPartito           bool   `json:"nonPartito"`
	Arrivato             bool   `json:"arrivato"`
}

// AndamentoVT è lo stato in tempo reale di un treno dall'endpoint viaggiatreno.it /andamentoTreno.
type AndamentoVT struct {
	NumeroTreno               int         `json:"numeroTreno"`
	Categoria                 string      `json:"categoria"`
	CategoriaDescrizione      string      `json:"categoriaDescrizione"`
	CodOrigine                string      `json:"codOrigine"`
	Origine                   string      `json:"origine"`
	Destinazione              string      `json:"destinazione"`
	Ritardo                   int         `json:"ritardo"`
	Circolante                bool        `json:"circolante"`
	Arrivato                  bool        `json:"arrivato"`
	NonPartito                bool        `json:"nonPartito"`
	StazioneUltimoRilevamento string      `json:"stazioneUltimoRilevamento"`
	DataPartenzaTreno         int64       `json:"dataPartenzaTreno"`
	DataPartenzaTrenoAsDate   string      `json:"dataPartenzaTrenoAsDate"`
	Fermate                   []FermataVT `json:"fermate"`
}

// FermataVT è una fermata all'interno della risposta andamentoTreno.
type FermataVT struct {
	Stazione               string `json:"stazione"`
	PartenzaTeorica        int64  `json:"partenza_teorica"`
	ArrivoTeoricoMs        int64  `json:"arrivo_teorico"`
	PartenzaReale          int64  `json:"partenzaReale"`
	ArrivoReale            int64  `json:"arrivoReale"`
	RitardoPartenza        int    `json:"ritardoPartenza"`
	RitardoArrivo          int    `json:"ritardoArrivo"`
	BinarioEffettivoPDescr string `json:"binarioEffettivoPartenzaDescrizione"`
}
