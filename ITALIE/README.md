# Trenitalia GTFS Auto-Builder

Génère un feed GTFS depuis l'API lefrecce.it, mis à jour automatiquement chaque jour via GitHub Actions.

## Télécharger le dernier GTFS

```bash
curl -L https://github.com/<USER>/<REPO>/releases/download/latest-gtfs/trenitalia_gtfs.zip -o gtfs.zip
```

**Fichiers inclus** dans `gtfs/trenitalia_it_api/` :

| Fichier | Contenu |
|---|---|
| `agency.txt` | Trenitalia |
| `routes.txt` | FR · FA · FB · IC · ICN · EC · EN |
| `trips.txt` | Un voyage par train × jour |
| `stop_times.txt` | Horaires HH:MM:SS par arrêt |
| `stops.txt` | Gares + coordonnées GPS |
| `calendar.txt` | Service par date exacte |
| `feed_info.txt` | Métadonnées du feed |

## Utilisation locale

```bash
node build-trenitalia-gtfs.js --days 30 --out ./gtfs/trenitalia_it_api
```

Options :
- `--days N` : fenêtre de jours à crawler (défaut 30)
- `--out ./path` : dossier de sortie
- `--dry-run` : affiche les stats sans écrire de fichiers
- `--verbose` : logs détaillés

## FL3 Rome (GTFS statique + snapshot real-time)

Le script `build-trenitalia-fl3.js` génère un feed ciblé FL3 (Roma Ostiense ↔ Viterbo Porta Fiorentina) et un snapshot de données temps réel exploitables.

```bash
node build-trenitalia-fl3.js \
  --days 7 \
  --out ./gtfs/trenitalia_fl3 \
  --realtime-out ./gtfs/trenitalia_fl3/fl3_realtime.json
```

Options :
- `--days N` : nombre de jours de service à crawler (défaut 7)
- `--out ./path` : dossier de sortie GTFS FL3
- `--realtime-out ./path/file.json` : sortie real-time JSON
- `--dry-run` : exécute les appels API sans écrire de fichiers
- `--verbose` : logs détaillés (notamment erreurs de récupération des détails)

Sorties :
- `agency.txt`, `routes.txt`, `trips.txt`, `stop_times.txt`, `stops.txt`, `calendar_dates.txt`, `feed_info.txt`
- `fl3_realtime.json` : snapshot real-time par train/arrêt (si exposé par l’API)

## Architecture

```
GitHub Actions (cron 03:00 UTC)
  └─ node build-trenitalia-gtfs.js
       └─ API lefrecce.it (POST /website/ticket/solutions)
            └─ gtfs/trenitalia_it_api/*.txt
                 └─ trenitalia_gtfs.zip → GitHub Release "latest-gtfs"
                      └─ Ton backend télécharge via URL stable
```
