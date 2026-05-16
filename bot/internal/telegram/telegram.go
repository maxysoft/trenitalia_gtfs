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
	token       string
	delayChatID int64
	adminChatID int64
	http        *http.Client
}

// NuovoClient creates a new Telegram client.
// delayChatID receives delay/recovery notifications; adminChatID receives startup and error messages.
func NuovoClient(token string, delayChatID, adminChatID int64) *Client {
	return &Client{
		token:       token,
		delayChatID: delayChatID,
		adminChatID: adminChatID,
		http:        &http.Client{Timeout: 15 * time.Second},
	}
}

// InviaMessaggio invia un testo formattato in MarkdownV2 alla chat configurata.
func (c *Client) InviaMessaggio(testo string) error {
	return c.inviaMessaggio(c.delayChatID, testo)
}

func (c *Client) InviaMessaggioConChatID(chatID int64, testo string) error {
	return c.inviaMessaggio(chatID, testo)
}

// InviaMessaggioAdmin sends a plain text notification to the admin chat.
func (c *Client) InviaMessaggioAdmin(testo string) error {
	return c.inviaMessaggio(c.adminChatID, escapeMD(testo))
}

// InviaErroreAdmin sends a formatted error notification to the admin chat.
func (c *Client) InviaErroreAdmin(err error) error {
	testo := fmt.Sprintf("*Errore monitor:*\n\n`%s`", escapeMD(err.Error()))
	return c.inviaMessaggio(c.adminChatID, testo)
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

// InviaMessaggioAvvio sends the bot startup message to the admin chat.
func (c *Client) InviaMessaggioAvvio(linea string, soglia int, intervallo time.Duration) error {
	intervalloMin := int(intervallo.Minutes())
	testo := fmt.Sprintf(
		"*Bot Monitoraggio Trenitalia* avviato\\!\n\n"+
			"*Linea monitorata:* `%s`\n"+
			"*Soglia ritardo:* %s minuti\n"+
			"*Intervallo controllo:* ogni %s minuti\n\n"+
			"_Il bot e' ora operativo e controlla entrambe le direzioni\\._",
		escapeMD(linea),
		escapeMD(fmt.Sprintf("%d", soglia)),
		escapeMD(fmt.Sprintf("%d", intervalloMin)),
	)
	return c.inviaMessaggio(c.adminChatID, testo)
}

func (c *Client) InviaReportMensile(chatID int64, stats storage.StatisticheMensili) error {
	return c.InviaMessaggioConChatID(chatID, FormattaReportMensile(stats))
}

// FormattaNotificaRitardo creates the MarkdownV2 message for a delay or recovery notification.
// For delays, it shows the train, route, and the last delayed stop (furthest along the route).
// For recoveries, it shows the current position and residual delay.
func FormattaNotificaRitardo(n monitor.NotificaRitardo) string {
	loc := fusoOrarioItalia()
	var sb strings.Builder

	percorso := escapeMD(n.StazioneDestinazione)
	if n.StazioneOrigine != "" {
		percorso = escapeMD(n.StazioneOrigine) + " → " + escapeMD(n.StazioneDestinazione)
	}

	if n.Recuperato {
		sb.WriteString(fmt.Sprintf(
			"✅ *%s* — Treno `%s` — Recuperato\n%s\n",
			escapeMD(n.LineaTreno),
			escapeMD(n.NumeroTreno),
			percorso,
		))
		if n.UltimaPosizioneNota != "" {
			sb.WriteString(fmt.Sprintf("\n📍 Ultima posizione: *%s*\n", escapeMD(n.UltimaPosizioneNota)))
		}
		if n.RitardoAttuale > 0 {
			sb.WriteString(fmt.Sprintf("⏱ Ritardo residuo: *%d min*\n", n.RitardoAttuale))
		} else {
			sb.WriteString("_In orario_\n")
		}
		sb.WriteString("\n_Dati forniti da [lefrecce\\.it](https://www\\.lefrecce\\.it/)_")
		return sb.String()
	}

	// Delay notification header.
	sb.WriteString(fmt.Sprintf(
		"🚨 *%s* — Treno `%s`\n%s\n",
		escapeMD(n.LineaTreno),
		escapeMD(n.NumeroTreno),
		percorso,
	))

	// Show the last delayed stop (furthest along the route with delay above threshold).
	if len(n.FermateInRitardo) > 0 {
		f := n.FermateInRitardo[len(n.FermateInRitardo)-1]
		sb.WriteString(fmt.Sprintf("\n🔸 *%s*\n", escapeMD(f.NomeStazione)))
		sb.WriteString(fmt.Sprintf("   ⏱ Ritardo: *%d min*\n", f.RitardoMinuti))
		if !f.OraOraria.IsZero() {
			sb.WriteString(fmt.Sprintf("   🕐 Previsto: `%s`\n", f.OraOraria.In(loc).Format("15:04")))
		}
		if !f.OraReale.IsZero() {
			sb.WriteString(fmt.Sprintf("   ⌚ Stimato: `%s`\n", f.OraReale.In(loc).Format("15:04")))
		}
		if f.Binario != "" {
			sb.WriteString(fmt.Sprintf("   🛤 Binario: *%s*\n", escapeMD(f.Binario)))
		}
	}

	sb.WriteString("\n_Dati forniti da [lefrecce\\.it](https://www\\.lefrecce\\.it/)_")
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
