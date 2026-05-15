// Pacchetto telegram implementa l'invio di messaggi tramite Bot API di Telegram.
package telegram

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/maxysoft/trenitalia_gtfs/bot/internal/monitor"
	"github.com/maxysoft/trenitalia_gtfs/bot/internal/storage"
)

const endpointSendMessage = "https://api.telegram.org/bot%s/sendMessage"

// Client invia messaggi a una chat Telegram tramite HTTP.
type Client struct {
	token  string
	chatID int64
	http   *http.Client
}

// NuovoClient crea un nuovo client Telegram.
func NuovoClient(token string, chatID int64) *Client {
	return &Client{
		token:  token,
		chatID: chatID,
		http:   &http.Client{Timeout: 15 * time.Second},
	}
}

// InviaMessaggio invia un testo formattato in MarkdownV2 alla chat configurata.
func (c *Client) InviaMessaggio(testo string) error {
	return c.inviaMessaggio(c.chatID, testo)
}

func (c *Client) InviaMessaggioConChatID(chatID int64, testo string) error {
	return c.inviaMessaggio(chatID, testo)
}

func (c *Client) inviaMessaggio(chatID int64, testo string) error {
	endpoint := fmt.Sprintf(endpointSendMessage, c.token)

	params := url.Values{}
	params.Set("chat_id", fmt.Sprintf("%d", chatID))
	params.Set("text", testo)
	params.Set("parse_mode", "MarkdownV2")
	params.Set("disable_web_page_preview", "true")

	resp, err := c.http.PostForm(endpoint, params)
	if err != nil {
		return fmt.Errorf("invio messaggio a Telegram: %w", err)
	}
	defer resp.Body.Close()

	var risposta struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&risposta); err != nil {
		return fmt.Errorf("decodifica risposta Telegram: %w", err)
	}
	if !risposta.OK {
		return fmt.Errorf("errore API Telegram: %s", risposta.Description)
	}
	return nil
}

// InviaNotificaRitardo formatta e invia una notifica di ritardo.
func (c *Client) InviaNotificaRitardo(n monitor.NotificaRitardo) error {
	return c.InviaMessaggio(FormattaNotificaRitardo(n))
}

// InviaMessaggioAvvio invia il messaggio di avvio del bot.
func (c *Client) InviaMessaggioAvvio(linea string, soglia int, intervallo time.Duration) error {
	intervalloMin := int(intervallo.Minutes())
	testo := fmt.Sprintf(
		"🚂 *Bot Monitoraggio Trenitalia* avviato\\!\n\n"+
			"📋 *Linea monitorata:* `%s`\n"+
			"⏱ *Soglia ritardo:* %s minuti\n"+
			"🔄 *Intervallo controllo:* ogni %s minuti\n\n"+
			"_Il bot è ora operativo e controlla i treni in tempo reale\\._",
		escapeMD(linea),
		escapeMD(fmt.Sprintf("%d", soglia)),
		escapeMD(fmt.Sprintf("%d", intervalloMin)),
	)
	return c.InviaMessaggio(testo)
}

func (c *Client) InviaReportMensile(chatID int64, stats storage.StatisticheMensili) error {
	return c.InviaMessaggioConChatID(chatID, FormattaReportMensile(stats))
}

// FormattaNotificaRitardo crea il messaggio Markdown per un treno in ritardo.
func FormattaNotificaRitardo(n monitor.NotificaRitardo) string {
	var sb strings.Builder

	// Intestazione con linea e numero treno
	sb.WriteString(fmt.Sprintf(
		"🚨 *RITARDO %s* — Treno `%s`\n\n",
		escapeMD(n.LineaTreno),
		escapeMD(n.NumeroTreno),
	))

	// Percorso e data
	sb.WriteString(fmt.Sprintf(
		"📍 *Percorso:* %s → %s\n",
		escapeMD(n.StazioneOrigine),
		escapeMD(n.StazioneDestinazione),
	))
	sb.WriteString(fmt.Sprintf("📅 *Data servizio:* %s\n\n", escapeMD(n.DataServizio)))

	// Elenco fermate in ritardo (massimo 5 per non superare il limite Telegram)
	totale := len(n.FermateInRitardo)
	sb.WriteString(fmt.Sprintf("⚠️ *Fermate con ritardo* \\(%d\\):\n", totale))

	loc := fusoOrarioItalia()
	limite := totale
	if limite > 5 {
		limite = 5
	}

	for i := 0; i < limite; i++ {
		f := n.FermateInRitardo[i]

		sb.WriteString(fmt.Sprintf("\n🔸 *%s*\n", escapeMD(f.NomeStazione)))

		ritardoStr := fmt.Sprintf("%d min", f.RitardoMinuti)
		sb.WriteString(fmt.Sprintf("   ⏱ Ritardo: *%s*\n", escapeMD(ritardoStr)))

		if !f.OraOraria.IsZero() {
			sb.WriteString(fmt.Sprintf(
				"   🕐 Previsto: `%s`\n",
				f.OraOraria.In(loc).Format("15:04"),
			))
		}
		if !f.OraReale.IsZero() {
			sb.WriteString(fmt.Sprintf(
				"   ⌚ Stimato: `%s`\n",
				f.OraReale.In(loc).Format("15:04"),
			))
		}
		if f.Binario != "" {
			sb.WriteString(fmt.Sprintf("   🛤 Binario: *%s*\n", escapeMD(f.Binario)))
		}
		if f.Stato != "" {
			sb.WriteString(fmt.Sprintf("   📊 Stato: _%s_\n", escapeMD(f.Stato)))
		}
	}

	if totale > 5 {
		sb.WriteString(fmt.Sprintf(
			"\n_\\.\\.\\. e altre %d fermate in ritardo_",
			totale-5,
		))
	}

	sb.WriteString("\n\n_Dati forniti da [lefrecce\\.it](https://www\\.lefrecce\\.it)_")

	return sb.String()
}

// escapeMD esegue l'escape di tutti i caratteri speciali di MarkdownV2.
func escapeMD(s string) string {
	// Caratteri che vanno preceduti da backslash in MarkdownV2
	caratteriSpeciali := []string{
		"_", "*", "[", "]", "(", ")", "~", "`", ">",
		"#", "+", "-", "=", "|", "{", "}", ".", "!",
	}
	for _, c := range caratteriSpeciali {
		s = strings.ReplaceAll(s, c, "\\"+c)
	}
	return s
}

func FormattaReportMensile(stats storage.StatisticheMensili) string {
	var sb strings.Builder
	sb.WriteString("📊 *Report mensile ritardi Trenitalia*\n\n")
	sb.WriteString(fmt.Sprintf("🚆 *Linea:* `%s`\n", escapeMD(stats.Linea)))
	sb.WriteString(fmt.Sprintf("🗓 *Mese:* `%s`\n\n", escapeMD(stats.MeseRiferimento)))
	sb.WriteString(fmt.Sprintf("• Ritardi registrati: *%d*\n", stats.NumeroRitardi))
	sb.WriteString(fmt.Sprintf("• Ritardo medio: *%.1f min*\n", stats.RitardoMedioMinuti))
	sb.WriteString(fmt.Sprintf("• Ritardo massimo: *%d min*\n", stats.RitardoMassimoMinuti))
	sb.WriteString(fmt.Sprintf(
		"• Fascia più critica: *%s* \\(%d ritardi\\)\n",
		escapeMD(stats.FasciaPeggiore),
		stats.RitardiFasciaPeggiore,
	))
	sb.WriteString("\n_Dati aggregati dal monitor interno_")
	return sb.String()
}

// fusoOrarioItalia restituisce il fuso orario Europe/Rome.
// Richiede tzdata installato nel sistema (incluso nell'immagine Docker).
func fusoOrarioItalia() *time.Location {
	loc, err := time.LoadLocation("Europe/Rome")
	if err != nil {
		// Fallback CEST (UTC+2) — copre la maggior parte dell'anno in Italia
		return time.FixedZone("CEST", 7200)
	}
	return loc
}
