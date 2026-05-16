# Trenitalia GTFS + Bot Ritardi FL3

Questo repository contiene:

1. **Builder GTFS automatico** — genera feed GTFS da lefrecce.it, aggiornato ogni giorno via GitHub Actions
2. **Bot Telegram** — monitora i ritardi in tempo reale sulla linea FL3 (e non solo) via viaggiatreno.it
3. **Dashboard web** — interfaccia read-only per consultare i ritardi storici salvati su SQLite

## Scaricare l'ultimo GTFS

```bash
curl -L https://github.com/<UTENTE>/<REPO>/releases/download/latest-gtfs/trenitalia_gtfs.zip -o gtfs.zip
curl -L https://github.com/<UTENTE>/<REPO>/releases/download/latest-gtfs/trenitalia_fl3_gtfs.zip -o fl3_gtfs.zip
```

**File inclusi** in `gtfs/trenitalia_it_api/`:

| File | Contenuto |
| --- | --- |
| `agency.txt` | Trenitalia |
| `routes.txt` | FR · FA · FB · IC · ICN · EC · EN |
| `trips.txt` | Un viaggio per treno × giorno |
| `stop_times.txt` | Orari HH:MM:SS per fermata |
| `stops.txt` | Stazioni + coordinate GPS |
| `calendar.txt` | Servizio per data esatta |
| `feed_info.txt` | Metadati del feed |

Per FL3 (`gtfs/trenitalia_fl3/`), la release include:

- GTFS statico della linea FL3
- `fl3_realtime.json` con snapshot real-time

## Utilizzo locale

```bash
node build-trenitalia-gtfs.js --days 30 --out ./gtfs/trenitalia_it_api
```

Opzioni:

- `--days N` : finestra di giorni da elaborare (default 30)
- `--out ./percorso` : cartella di output
- `--dry-run` : mostra le statistiche senza scrivere file
- `--verbose` : log dettagliati

## FL3 Roma (GTFS statico + snapshot real-time)

Lo script `build-trenitalia-fl3.js` genera un feed mirato FL3 (Roma Ostiense ↔ Viterbo Porta Fiorentina) e uno snapshot di dati in tempo reale utilizzabili.

```bash
node build-trenitalia-fl3.js \
  --days 7 \
  --out ./gtfs/trenitalia_fl3 \
  --realtime-out ./gtfs/trenitalia_fl3/fl3_realtime.json
```

Opzioni:

- `--days N` : numero di giorni di servizio da elaborare (default 7)
- `--out ./percorso` : cartella di output GTFS FL3
- `--realtime-out ./percorso/file.json` : output real-time JSON
- `--dry-run` : esegue le chiamate API senza scrivere file
- `--verbose` : log dettagliati (inclusi errori di recupero dettagli)

Output:

- `agency.txt`, `routes.txt`, `trips.txt`, `stop_times.txt`, `stops.txt`, `calendar_dates.txt`, `feed_info.txt`
- `fl3_realtime.json` : snapshot real-time per treno/fermata (se esposto dall'API)

## Bot Telegram — Notifiche Ritardi in Tempo Reale

Il bot monitora i ritardi superiori alla soglia configurata (default 10 min) e invia notifiche compatte su Telegram.  
Utilizza l'API pubblica **viaggiatreno.it** per i dati in tempo reale (partenze + andamento treno).

### Come funziona

1. Ogni 5 minuti (configurabile), interroga `/partenze` per le stazioni di origine, destinazione ed eventuali stazioni extra
2. Raccoglie tutti i treni della linea (filtro per keyword su origine/destinazione)
3. Chiama `/andamentoTreno` per ogni treno noto → rileva ritardi fermata per fermata
4. Invia una notifica Telegram quando un treno supera la soglia, e un'altra quando recupera
5. Sospende automaticamente il polling nella finestra notturna (default 00:00–04:30)

### Avvio rapido con Docker Compose

```bash
# 1. Copia il file di configurazione
cp .env.esempio .env

# 2. Modifica .env con il tuo token Telegram e chat ID
nano .env

# 3. Avvia il bot
docker compose up -d

# Visualizza i log
docker compose logs -f bot-trenitalia
```

### Configurazione tramite variabili d'ambiente

| Variabile | Descrizione | Default |
| --- | --- | --- |
| `TELEGRAM_BOT_TOKEN` | Token bot da @BotFather | **obbligatorio** |
| `TELEGRAM_CHAT_ID` | ID numerico chat/gruppo | **obbligatorio** |
| `LINEA_TRENO` | Codice linea (es. FL3, FL1) | `FL3` |
| `STAZIONE_ORIGINE` | Nome stazione di partenza | `Roma Ostiense` |
| `STAZIONE_DESTINAZIONE` | Nome stazione di arrivo | `Viterbo Porta Fiorentina` |
| `RITARDO_SOGLIA_MINUTI` | Soglia minima in minuti | `10` |
| `INTERVALLO_POLLING_SECONDI` | Frequenza di controllo | `300` |
| `TIPI_TRENO` | Tipi treno da monitorare | `R,RV,RE,REGIONALE,REG` |
| `STAZIONI_LINEA` | Keyword per filtrare i treni per linea | `VITERBO,CESANO,...` |
| `STAZIONI_EXTRA_PARTENZE` | Stazioni aggiuntive da cui interrogare `/partenze` | vuoto |
| `ORA_INIZIO_PAUSA` | Inizio pausa notturna (HH:MM, Europe/Rome) | `00:00` |
| `ORA_FINE_PAUSA` | Fine pausa notturna (HH:MM, Europe/Rome) | `04:30` |
| `SQLITE_DB_PATH` | Percorso DB SQLite ritardi | `/data/ritardi.sqlite` |
| `INVIO_REPORT_MENSILE` | Abilita report mensile automatico | `true` |
| `REPORT_TELEGRAM_CHAT_ID` | Chat ID report (fallback: chat principale) | `TELEGRAM_CHAT_ID` |
| `ADMIN_CHAT_ID` | Chat ID per notifiche di avvio ed errori | `TELEGRAM_CHAT_ID` |
| `DEBUG_MODE` | Log ritardi >2 min e dettagli filtri | `false` |

#### `STAZIONI_LINEA` — filtro per linea

Keyword separate da virgola confrontate (sottostringa, case-insensitive) con l'origine e la destinazione di ogni treno.  
Solo i treni che corrispondono almeno a una keyword vengono monitorati.  
Lasciare vuoto per monitorare tutti i treni regionali indipendentemente dalla linea.

Esempio per FL3:

```text
STAZIONI_LINEA=VITERBO,CESANO,BRACCIANO,ANGUILLARA,TIBURTINA,MONTEROTONDO,OSTIENSE,STORTA,OLGIATA,PRIMA PORTA
```

#### `STAZIONI_EXTRA_PARTENZE` — stazioni aggiuntive

Alcuni treni partono da stazioni con nome diverso da quello configurato in `STAZIONE_ORIGINE`/`STAZIONE_DESTINAZIONE`.  
Esempio: sulla FL3, alcuni treni originano da **Viterbo Porta Romana** invece che da **Viterbo Porta Fiorentina**.

```text
STAZIONI_EXTRA_PARTENZE=Viterbo Porta Romana
```

#### Pausa notturna

Il polling si sospende automaticamente tra `ORA_INIZIO_PAUSA` e `ORA_FINE_PAUSA` (fuso Europe/Rome).  
Default: 00:00–04:30 (nessun treno FL3 in servizio).  
Per disabilitare la pausa, lasciare entrambe le variabili vuote.

### Report mensile automatico (inizio mese)

Il bot salva i ritardi in SQLite e, il giorno 1 di ogni mese, invia un riepilogo del mese precedente:

- ritardo medio
- ritardo massimo
- fascia oraria con più ritardi (notte/mattina/pomeriggio/sera)

### Esempio notifica Telegram

**Ritardo:**

```text
🚨 FL3 — Treno 12821
VITERBO PORTA ROMANA → ROMA OSTIENSE

🔸 APPIANO
   ⏱ Ritardo: 18 min
   🕐 Previsto: 07:42
   ⌚ Stimato: 08:00
   🛤 Binario: 1

Dati forniti da lefrecce.it
```

**Recupero:**

```text
✅ FL3 — Treno 12821 — Recuperato
VITERBO PORTA ROMANA → ROMA OSTIENSE

📍 Ultima posizione: ROMA OSTIENSE
⏱ Ritardo residuo: 3 min

Dati forniti da lefrecce.it
```

### Compilazione manuale (senza Docker)

```bash
cd bot
go build -o bot-trenitalia ./cmd/bot
./bot-trenitalia
```

## Dashboard Web (read-only su SQLite)

È disponibile un secondo servizio web con interfaccia moderna (senza librerie/font esterni) per consultare i ritardi:

- ricerca testuale
- filtro linea
- paginazione server-side

### Avvio con Docker Compose

```bash
docker compose up -d --build
```

Poi apri:

```text
http://localhost:8080
```

## GitHub Actions

### CI (`ci.yml`)

Eseguito su ogni push a `main` e pull request:

1. **Test** — `go vet` + `go test -race` per i moduli `bot` e `web`
2. **Build** — costruisce le immagini Docker e le salva come artifact
3. **Publish** — pubblica su GHCR (`ghcr.io/<owner>/<repo>/bot:latest` e `web:latest`) solo su push a `main`

### GTFS Builder (`build-gtfs.yml`)

Eseguito ogni giorno alle 03:00 UTC (avviabile anche manualmente):

1. Genera il feed GTFS nazionale (30 giorni)
2. Genera il feed GTFS FL3 + snapshot real-time (7 giorni)
3. Comprime i feed in ZIP
4. Crea/aggiorna la GitHub Release `latest-gtfs` con i file ZIP

## Architettura

```text
GitHub Actions (cron 03:00 UTC)
  ├─ node build-trenitalia-gtfs.js
  │    └─ gtfs/trenitalia_it_api/*.txt → trenitalia_gtfs.zip
  └─ node build-trenitalia-fl3.js
       └─ gtfs/trenitalia_fl3/* + fl3_realtime.json → trenitalia_fl3_gtfs.zip
            └─ GitHub Release "latest-gtfs"

Bot Telegram (Docker Compose, polling ogni 5 min)
  ├─ viaggiatreno.it /partenze     → scoperta treni in transito
  ├─ viaggiatreno.it /andamentoTreno → ritardo per fermata
  ├─ Pausa notturna 00:00–04:30   → polling sospeso automaticamente
  ├─ Ritardo > soglia → notifica Telegram (MarkdownV2)
  └─ Persistenza SQLite + report mensile automatico

Web dashboard (Docker Compose)
  └─ Lettura SQLite in sola lettura
       └─ Ricerca + paginazione via API /api/delays
```
