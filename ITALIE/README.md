# Trenitalia GTFS — Builder Automatico

Genera un feed GTFS dall'API lefrecce.it, aggiornato automaticamente ogni giorno tramite GitHub Actions.

## Scaricare l'ultimo GTFS

```bash
curl -L https://github.com/<UTENTE>/<REPO>/releases/download/latest-gtfs/trenitalia_gtfs.zip -o gtfs.zip
```

**File inclusi** in `gtfs/trenitalia_it_api/`:

| File | Contenuto |
|---|---|
| `agency.txt` | Trenitalia |
| `routes.txt` | FR · FA · FB · IC · ICN · EC · EN |
| `trips.txt` | Un viaggio per treno × giorno |
| `stop_times.txt` | Orari HH:MM:SS per fermata |
| `stops.txt` | Stazioni + coordinate GPS |
| `calendar.txt` | Servizio per data esatta |
| `feed_info.txt` | Metadati del feed |

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

Il bot Telegram monitora i ritardi superiori a 10 minuti e invia notifiche formattate.

### Avvio rapido con Docker Compose

```bash
# 1. Copia il file di configurazione
cp .env.esempio .env

# 2. Modifica .env con il tuo token Telegram e chat ID
nano .env

# 3. Avvia il bot
docker compose up -d

# Visualizza i log
docker compose logs -f
```

### Configurazione tramite variabili d'ambiente

| Variabile | Descrizione | Default |
|---|---|---|
| `TELEGRAM_BOT_TOKEN` | Token bot da @BotFather | **obbligatorio** |
| `TELEGRAM_CHAT_ID` | ID numerico chat/gruppo | **obbligatorio** |
| `LINEA_TRENO` | Codice linea (es. FL3, FL1) | `FL3` |
| `STAZIONE_ORIGINE` | Nome stazione di partenza | `Roma Ostiense` |
| `STAZIONE_DESTINAZIONE` | Nome stazione di arrivo | `Viterbo Porta Fiorentina` |
| `RITARDO_SOGLIA_MINUTI` | Soglia minima in minuti | `10` |
| `INTERVALLO_POLLING_SECONDI` | Frequenza di controllo | `300` |
| `TIPI_TRENO` | Tipi treno da monitorare | `R,RV,RE,REGIONALE` |

### Esempio notifica Telegram

```
🚨 RITARDO FL3 — Treno 12345

📍 Percorso: Roma Ostiense → Viterbo Porta Fiorentina
📅 Data servizio: 2025-05-15

⚠️ Fermate con ritardo (3):

🔸 Roma Trastevere
   ⏱ Ritardo: 15 min
   🕐 Previsto: 09:12
   ⌚ Stimato: 09:27
   🛤 Binario: 2

🔸 Valle Aurelia
   ⏱ Ritardo: 14 min
   🕐 Previsto: 09:18
   ⌚ Stimato: 09:32

Dati forniti da lefrecce.it
```

### Compilazione manuale (senza Docker)

```bash
cd bot
go build -o bot-trenitalia ./cmd/bot
./bot-trenitalia
```

## Architettura

```
GitHub Actions (cron 03:00 UTC)
  └─ node build-trenitalia-gtfs.js
       └─ API lefrecce.it (POST /website/ticket/solutions)
            └─ gtfs/trenitalia_it_api/*.txt
                 └─ trenitalia_gtfs.zip → GitHub Release "latest-gtfs"
                      └─ Il tuo backend scarica tramite URL stabile

Bot Telegram (Docker Compose, polling ogni 5 min)
  └─ API lefrecce.it (GET /website/stops con dati real-time)
       └─ Ritardo > soglia → notifica Telegram (MarkdownV2)
```
