# ════════════════════════════════════════════════════════════════════════
# Esempi: scarica il file ZIP GTFS dal tuo backend
# URL stabile (non cambia mai anche se il ZIP viene aggiornato):
#
#   https://github.com/<UTENTE>/<REPO>/releases/download/latest-gtfs/trenitalia_gtfs.zip
#
# Sostituisci <UTENTE>/<REPO> con il tuo repository GitHub.
# ════════════════════════════════════════════════════════════════════════

# ── Python (requests) ────────────────────────────────────────────────────────
import requests, zipfile, io, os

GTFS_URL = "https://github.com/<UTENTE>/<REPO>/releases/download/latest-gtfs/trenitalia_gtfs.zip"

def scarica_gtfs(cartella_dest="./dati_gtfs"):
    r = requests.get(GTFS_URL, timeout=60, stream=True)
    r.raise_for_status()
    with zipfile.ZipFile(io.BytesIO(r.content)) as z:
        z.extractall(cartella_dest)
    print(f"GTFS estratto in {cartella_dest}/")
    for f in os.listdir(cartella_dest + "/trenitalia_it_api"):
        print(" ", f)

scarica_gtfs()

# ── Node.js (senza dipendenze esterne) ───────────────────────────────────────
# const https = require('https');
# const fs    = require('fs');
# const path  = require('path');
# const { execSync } = require('child_process');
#
# const GTFS_URL = 'https://github.com/<UTENTE>/<REPO>/releases/download/latest-gtfs/trenitalia_gtfs.zip';
# const ZIP_PATH = '/tmp/trenitalia_gtfs.zip';
# const OUT_DIR  = './dati_gtfs';
#
# function scaricaFile(url, dest) {
#   return new Promise((resolve, reject) => {
#     const file = fs.createWriteStream(dest);
#     https.get(url, res => {
#       // Segui i redirect (GitHub reindirizza verso lo storage CDN)
#       if (res.statusCode === 301 || res.statusCode === 302) {
#         file.close();
#         return scaricaFile(res.headers.location, dest).then(resolve).catch(reject);
#       }
#       res.pipe(file);
#       file.on('finish', () => { file.close(); resolve(); });
#     }).on('error', reject);
#   });
# }
#
# async function scaricaGTFS() {
#   console.log('Download ZIP GTFS in corso...');
#   await scaricaFile(GTFS_URL, ZIP_PATH);
#   fs.mkdirSync(OUT_DIR, { recursive: true });
#   execSync(`unzip -o ${ZIP_PATH} -d ${OUT_DIR}`);
#   console.log('GTFS pronto in', OUT_DIR);
# }
#
# scaricaGTFS().catch(console.error);

# ── curl (shell / entrypoint Docker) ─────────────────────────────────────────
# curl -L \
#   "https://github.com/<UTENTE>/<REPO>/releases/download/latest-gtfs/trenitalia_gtfs.zip" \
#   -o /tmp/gtfs.zip \
#   && unzip -o /tmp/gtfs.zip -d /dati/gtfs \
#   && rm /tmp/gtfs.zip
