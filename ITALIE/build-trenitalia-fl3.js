#!/usr/bin/env node
'use strict';

/**
 * Build GTFS + realtime snapshot for Trenitalia FL3 (Rome suburban line)
 *
 * Data sources (lefrecce public BFF API):
 * - GET  /website/locations/search
 * - POST /website/ticket/solutions
 * - GET  /website/stops?cartId=...&solutionId=...
 */

const fs = require('fs');
const path = require('path');
const https = require('https');
const http = require('http');

function getArg(flag, fallback) {
  const i = process.argv.indexOf(flag);
  return i === -1 || i >= process.argv.length - 1 ? fallback : process.argv[i + 1];
}
function hasFlag(flag) {
  return process.argv.includes(flag);
}

const DAYS = Math.max(1, parseInt(getArg('--days', '7'), 10) || 7);
const OUT_DIR = getArg('--out', './gtfs/trenitalia_fl3');
const RT_FILE = getArg('--realtime-out', path.join(OUT_DIR, 'fl3_realtime.json'));
const DRY_RUN = hasFlag('--dry-run');
const VERBOSE = hasFlag('--verbose');

const API_BASE = 'https://www.lefrecce.it/Channels.Website.BFF.WEB';
const API_LOCATIONS = `${API_BASE}/website/locations/search?name=`;
const API_SOLUTIONS = `${API_BASE}/website/ticket/solutions`;
const API_STOPS = `${API_BASE}/website/stops`;

const REQ_TIMEOUT_MS = 20000;
const MAX_RETRIES = 3;
const DELAY_MS = 350;
const VALID_FL3_TRAIN_TYPES = new Set(['R', 'REGIONALE', 'RV', 'RE']);

const FL3_STATION_HINTS = [
  { canonical: 'ROMA OSTIENSE', query: 'Roma Ostiense', lat: 41.8757, lon: 12.4765 },
  { canonical: 'ROMA TRASTEVERE', query: 'Roma Trastevere', lat: 41.8726, lon: 12.4664 },
  { canonical: 'ROMA SAN PIETRO', query: 'Roma San Pietro', lat: 41.8987, lon: 12.4530 },
  { canonical: 'VALLE AURELIA', query: 'Valle Aurelia', lat: 41.9033, lon: 12.4312 },
  { canonical: 'ROMA BALDUINA', query: 'Roma Balduina', lat: 41.9181, lon: 12.4389 },
  { canonical: 'ROMA MONTE MARIO', query: 'Roma Monte Mario', lat: 41.9343, lon: 12.4522 },
  { canonical: 'ROMA SAN FILIPPO NERI', query: 'Roma San Filippo Neri', lat: 41.9528, lon: 12.4429 },
  { canonical: 'ROMA OTTAVIA', query: 'Roma Ottavia', lat: 41.9640, lon: 12.4308 },
  { canonical: 'IPOGEO DEGLI OTTAVI', query: 'Ipogeo degli Ottavi', lat: 41.9742, lon: 12.4230 },
  { canonical: 'LA GIUSTINIANA', query: 'La Giustiniana', lat: 42.0010, lon: 12.4577 },
  { canonical: 'OLGIATA', query: 'Olgiata', lat: 42.0413, lon: 12.3653 },
  { canonical: 'CESANO DI ROMA', query: 'Cesano di Roma', lat: 42.0698, lon: 12.3335 },
  { canonical: 'ANGUILLARA', query: 'Anguillara', lat: 42.0799, lon: 12.2704 },
  { canonical: 'BRACCIANO', query: 'Bracciano', lat: 42.1033, lon: 12.1757 },
  { canonical: 'MANZIANA - CANALE MONTERANO', query: 'Manziana Canale Monterano', lat: 42.1317, lon: 12.1087 },
  { canonical: 'ORIOLO', query: 'Oriolo', lat: 42.1571, lon: 12.1392 },
  { canonical: 'CAPRANICA - SUTRI', query: 'Capranica Sutri', lat: 42.2642, lon: 12.1707 },
  { canonical: 'VETRALLA', query: 'Vetralla', lat: 42.3156, lon: 12.0746 },
  { canonical: 'TRE CROCI', query: 'Tre Croci', lat: 42.3554, lon: 12.0335 },
  { canonical: 'VITERBO PORTA FIORENTINA', query: 'Viterbo Porta Fiorentina', lat: 42.4274, lon: 12.1004 },
  { canonical: 'VITERBO PORTA ROMANA', query: 'Viterbo Porta Romana', lat: 42.4205, lon: 12.1022 },
];

const FL3_KEYWORDS = new Set(
  FL3_STATION_HINTS.map(s => normalizeName(s.canonical))
);

function normalizeName(s) {
  return String(s || '')
    .toUpperCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .replace(/[^A-Z0-9]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
}

function stationKey(s) {
  return normalizeName(s).replace(/\s+/g, '_');
}

function sleep(ms) {
  return new Promise(r => setTimeout(r, ms));
}

function fetchJSON(urlOrOptions, body = null, retries = MAX_RETRIES) {
  return new Promise((resolve, reject) => {
    const attempt = (n) => {
      _fetch(urlOrOptions, body)
        .then(resolve)
        .catch(async (err) => {
          if (n >= retries) return reject(err);
          await sleep(DELAY_MS * Math.pow(2, n + 1));
          attempt(n + 1);
        });
    };
    attempt(0);
  });
}

function _fetch(urlOrOptions, body = null) {
  return new Promise((resolve, reject) => {
    const isString = typeof urlOrOptions === 'string';
    const parsed = isString ? new URL(urlOrOptions) : null;
    const protocol = isString ? parsed.protocol : (urlOrOptions.protocol || 'https:');
    const lib = protocol === 'http:' ? http : https;
    const bodyStr = body ? JSON.stringify(body) : null;

    const options = isString
      ? {
          hostname: parsed.hostname,
          path: parsed.pathname + parsed.search,
          method: bodyStr ? 'POST' : 'GET',
          headers: {
            'Accept': 'application/json',
            'Content-Type': 'application/json',
            'Origin': 'https://www.lefrecce.it',
            'Referer': 'https://www.lefrecce.it/',
            'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0 Safari/537.36',
            ...(bodyStr ? { 'Content-Length': Buffer.byteLength(bodyStr) } : {}),
          },
        }
      : urlOrOptions;

    const req = lib.request(options, (res) => {
      let raw = '';
      res.on('data', c => { raw += c; });
      res.on('end', () => {
        if (res.statusCode >= 400) {
          return reject(new Error(`HTTP ${res.statusCode}: ${raw.slice(0, 200)}`));
        }
        try {
          resolve(JSON.parse(raw));
        } catch (e) {
          reject(new Error('JSON parse error: ' + raw.slice(0, 120)));
        }
      });
    });

    req.setTimeout(REQ_TIMEOUT_MS, () => req.destroy(new Error('timeout')));
    req.on('error', reject);
    if (bodyStr) req.write(bodyStr);
    req.end();
  });
}

function toISOItaly(date, hour = 6) {
  const d = new Date(date);
  d.setHours(hour, 0, 0, 0);
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: 'Europe/Rome',
    timeZoneName: 'shortOffset',
  }).formatToParts(d);
  const tz = parts.find(p => p.type === 'timeZoneName')?.value || '';
  const match = tz.match(/GMT([+-])(\d{1,2})(?::(\d{2}))?/i);
  if (!match) {
    throw new Error(
      `Unable to parse Europe/Rome offset from timeZoneName='${tz || 'empty'}' ` +
      `for date='${d.toISOString()}' parts='${JSON.stringify(parts)}'`
    );
  }
  const sign = match[1];
  const hh = String(parseInt(match[2], 10)).padStart(2, '0');
  const mm = String(match[3] ? parseInt(match[3], 10) : 0).padStart(2, '0');
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}T${String(hour).padStart(2, '0')}:00:00.000${sign}${hh}:${mm}`;
}

function toGTFSTime(iso) {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`;
}

function dateKey(iso) {
  return iso.slice(0, 10);
}

function csvEscape(v) {
  const s = String(v == null ? '' : v);
  return (s.includes(',') || s.includes('"') || s.includes('\n')) ? `"${s.replace(/"/g, '""')}"` : s;
}

function writeCSV(filePath, rows) {
  if (!rows.length) {
    fs.writeFileSync(filePath, '', 'utf8');
    return;
  }
  const headers = Object.keys(rows[0]);
  const lines = [headers.join(',')];
  for (const row of rows) {
    lines.push(headers.map(h => csvEscape(row[h])).join(','));
  }
  fs.writeFileSync(filePath, lines.join('\n') + '\n', 'utf8');
}

async function resolveStationIdByName(query, canonical) {
  const url = `${API_LOCATIONS}${encodeURIComponent(query)}&limit=10`;
  const data = await fetchJSON(url);
  if (!Array.isArray(data) || !data.length) {
    throw new Error(`No station match for ${query}`);
  }

  const canonicalNorm = normalizeName(canonical);
  const exact = data.find(x => normalizeName(x.name) === canonicalNorm);
  const containsCanonical = data.find(x => normalizeName(x.name).includes(canonicalNorm));
  const canonicalContains = data.find(x => canonicalNorm.includes(normalizeName(x.name)));
  const best = exact || containsCanonical || canonicalContains || data[0];
  return { id: best.id, name: best.name || canonical };
}

function looksLikeFL3Stop(name) {
  return FL3_KEYWORDS.has(normalizeName(name));
}

async function fetchSolutions(originId, destinationId, day) {
  const body = {
    departureLocationId: originId,
    arrivalLocationId: destinationId,
    departureTime: toISOItaly(day, 6),
    adults: 1,
    children: 0,
    criteria: {
      frecceOnly: false,
      regionalOnly: true,
      intercityOnly: false,
      tourismOnly: false,
      noChanges: true,
      order: 'DEPARTURE_DATE',
      limit: 50,
      offset: 0,
    },
    advancedSearchRequest: {
      bestFare: false,
      bikeFilter: false,
    },
  };
  return fetchJSON(API_SOLUTIONS, body);
}

async function fetchStops(cartId, solutionId) {
  const url = `${API_STOPS}?cartId=${encodeURIComponent(cartId)}&solutionId=${encodeURIComponent(solutionId)}`;
  return fetchJSON(url);
}

(async function main() {
  const fromMain = FL3_STATION_HINTS.find(s => s.canonical === 'ROMA OSTIENSE');
  const toMain = FL3_STATION_HINTS.find(s => s.canonical === 'VITERBO PORTA FIORENTINA');

  if (!fromMain || !toMain) throw new Error('FL3 terminal stations are not configured.');

  console.log(`\n🇮🇹 FL3 builder (Trenitalia) — days=${DAYS}`);
  console.log(`   Output: ${OUT_DIR}`);
  console.log(`   Realtime: ${RT_FILE}`);

  const fromResolved = await resolveStationIdByName(fromMain.query, fromMain.canonical);
  const toResolved = await resolveStationIdByName(toMain.query, toMain.canonical);

  const days = [];
  const start = new Date();
  start.setHours(0, 0, 0, 0);
  for (let i = 0; i < DAYS; i++) {
    const d = new Date(start);
    d.setDate(start.getDate() + i);
    days.push(d);
  }

  const stopsMap = new Map();
  const trips = [];
  const stopTimes = [];
  const calendarDates = [];
  const realtime = [];
  const seenTrips = new Set();
  const seenServices = new Set();

  const route = {
    route_id: 'TI_FL3',
    agency_id: 'TI',
    route_short_name: 'FL3',
    route_long_name: 'Roma Ostiense - Viterbo Porta Fiorentina',
    route_type: '2',
  };

  const stationMetadata = new Map(
    FL3_STATION_HINTS.map(s => [stationKey(s.canonical), { lat: s.lat, lon: s.lon }])
  );

  for (const day of days) {
    const dayLabel = day.toISOString().slice(0, 10);
    console.log(`\n📅 ${dayLabel}`);

    const directions = [
      { origin: fromResolved, destination: toResolved },
      { origin: toResolved, destination: fromResolved },
    ];

    for (const dir of directions) {
      await sleep(DELAY_MS);
      const res = await fetchSolutions(dir.origin.id, dir.destination.id, day);
      const cartId = res?.cartId;
      const solutions = Array.isArray(res?.solutions) ? res.solutions : [];

      console.log(`   ${dir.origin.name} -> ${dir.destination.name}: ${solutions.length} solution(s)`);

      for (const item of solutions) {
        const sol = item?.solution;
        if (!sol || !sol.id) continue;

        const nodes = Array.isArray(sol.nodes) ? sol.nodes : [];
        if (!nodes.length) continue;

        for (const node of nodes) {
          const acronym =
            normalizeName(node?.train?.acronym || '') ||
            normalizeName(node?.trainInfo?.acronym || '') ||
            normalizeName(node?.trainAcronym || '');
          if (!VALID_FL3_TRAIN_TYPES.has(acronym)) continue;
          if (VERBOSE && !node?.train?.acronym && (node?.trainInfo?.acronym || node?.trainAcronym)) {
            console.warn(`   ℹ acronym fallback used for solution ${sol.id}`);
          }

          const tripNo = String(node?.train?.name || node?.train?.description || '').trim();
          const depIso = node?.departureTime;
          const arrIso = node?.arrivalTime;
          if (!tripNo || !depIso || !arrIso) continue;

          const serviceDate = dateKey(depIso);
          const serviceId = serviceDate.replace(/-/g, '');
          const tripId = `TI_FL3_${tripNo}_${serviceId}`;
          if (seenTrips.has(tripId)) continue;

          let detailStops = [];
          if (cartId) {
            try {
              await sleep(DELAY_MS);
              const detail = await fetchStops(cartId, sol.id);
              const detailsArr = Array.isArray(detail) ? detail : [];
              const match = detailsArr.find(
                x => String(x?.summary?.trainInfo?.name || x?.summary?.trainInfo?.description || '').trim() === tripNo
              );
              if (!match) continue;
              detailStops = Array.isArray(match?.stops) ? match.stops : [];

              realtime.push({
                trip_id: tripId,
                train_number: tripNo,
                service_date: serviceDate,
                origin_name: node.origin,
                destination_name: node.destination,
                summary: match?.summary || {},
                stops: detailStops.map(s => ({
                  stop_name: s?.location?.name || '',
                  departure_time: s?.departureTime || null,
                  arrival_time: s?.arrivalTime || null,
                  train_number: s?.trainNumber || null,
                  platform: s?.platform || s?.binario || null,
                  real_departure_time: s?.realDepartureTime || s?.actualDepartureTime || null,
                  real_arrival_time: s?.realArrivalTime || s?.actualArrivalTime || null,
                  status: s?.status || null,
                  delay_seconds: s?.delaySeconds || null,
                })),
              });
            } catch (e) {
              if (VERBOSE) console.warn(`   ⚠ stops fetch failed for ${tripId}: ${e.message}`);
            }
          }

          const fl3Stops = detailStops
            .map(s => ({
              stop_name: s?.location?.name || '',
              dep: s?.departureTime || s?.arrivalTime,
              arr: s?.arrivalTime || s?.departureTime,
            }))
            .filter(s => s.stop_name && s.dep && looksLikeFL3Stop(s.stop_name));

          if (fl3Stops.length < 2) continue;

          seenTrips.add(tripId);
          trips.push({
            route_id: route.route_id,
            service_id: serviceId,
            trip_id: tripId,
            trip_headsign: node.destination,
            direction_id: normalizeName(node.origin).includes('ROMA') ? '0' : '1',
          });

          if (!seenServices.has(serviceId)) {
            seenServices.add(serviceId);
            calendarDates.push({ service_id: serviceId, date: serviceId, exception_type: '1' });
          }

          fl3Stops.forEach((st, idx) => {
            const key = stationKey(st.stop_name);
            const stopId = `TI_FL3:${key}`;
            const meta = stationMetadata.get(key) || { lat: '', lon: '' };

            if (!stopsMap.has(stopId)) {
              stopsMap.set(stopId, {
                stop_id: stopId,
                stop_name: st.stop_name,
                stop_lat: meta.lat,
                stop_lon: meta.lon,
              });
            }

            stopTimes.push({
              trip_id: tripId,
              arrival_time: toGTFSTime(st.arr),
              departure_time: toGTFSTime(st.dep),
              stop_id: stopId,
              stop_sequence: idx + 1,
              pickup_type: '0',
              drop_off_type: '0',
            });
          });
        }
      }
    }
  }

  console.log(`\n✅ FL3 trips: ${trips.length}, stops: ${stopsMap.size}, realtime snapshots: ${realtime.length}`);

  if (DRY_RUN) return;

  fs.mkdirSync(OUT_DIR, { recursive: true });

  writeCSV(path.join(OUT_DIR, 'agency.txt'), [{
    agency_id: 'TI',
    agency_name: 'Trenitalia',
    agency_url: 'https://www.trenitalia.com',
    agency_timezone: 'Europe/Rome',
    agency_lang: 'it',
  }]);

  writeCSV(path.join(OUT_DIR, 'routes.txt'), [route]);
  writeCSV(path.join(OUT_DIR, 'trips.txt'), trips);
  writeCSV(path.join(OUT_DIR, 'stop_times.txt'), stopTimes);
  writeCSV(path.join(OUT_DIR, 'stops.txt'), [...stopsMap.values()]);
  writeCSV(path.join(OUT_DIR, 'calendar_dates.txt'), calendarDates);

  const today = new Date().toISOString().slice(0, 10).replace(/-/g, '');
  const end = new Date();
  end.setDate(end.getDate() + DAYS);
  const endS = end.toISOString().slice(0, 10).replace(/-/g, '');

  writeCSV(path.join(OUT_DIR, 'feed_info.txt'), [{
    feed_publisher_name: 'TrainNomad (Trenitalia FL3)',
    feed_publisher_url: 'https://trainnomad.eu',
    feed_lang: 'it',
    feed_start_date: today,
    feed_end_date: endS,
    feed_version: today,
  }]);

  fs.writeFileSync(RT_FILE, JSON.stringify({
    generated_at: new Date().toISOString(),
    source: 'https://www.lefrecce.it/Channels.Website.BFF.WEB',
    line: 'FL3',
    trips: realtime,
  }, null, 2));

  console.log(`📁 GTFS written to ${OUT_DIR}`);
  console.log(`📡 Realtime snapshot written to ${RT_FILE}`);
})().catch(err => {
  console.error(`❌ ${err.message}`);
  process.exit(1);
});
