const state = {
  page: 1,
  pageSize: 25,
  q: '',
  line: ''
};

const els = {
  search: document.getElementById('search'),
  line: document.getElementById('line'),
  apply: document.getElementById('apply'),
  reset: document.getElementById('reset'),
  prev: document.getElementById('prev'),
  next: document.getElementById('next'),
  rows: document.getElementById('rows'),
  summary: document.getElementById('summary'),
  pageinfo: document.getElementById('pageinfo')
};

function buildUrl() {
  const p = new URLSearchParams();
  p.set('page', String(state.page));
  p.set('page_size', String(state.pageSize));
  if (state.q) p.set('q', state.q);
  if (state.line) p.set('line', state.line);
  return `/api/delays?${p.toString()}`;
}

async function loadData() {
  els.rows.textContent = '';
  appendMessageRow('Caricamento...');
  try {
    const res = await fetch(buildUrl(), { headers: { 'Accept': 'application/json' } });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = await res.json();

    renderRows(data.records || []);
    els.summary.textContent = `Record totali: ${data.total}`;
    els.pageinfo.textContent = `Pagina ${data.page} / ${data.total_pages}`;
    els.prev.disabled = !data.has_prev;
    els.next.disabled = !data.has_next;
  } catch (err) {
    els.rows.textContent = '';
    appendMessageRow(`Errore caricamento dati: ${String(err.message || 'sconosciuto')}`);
    els.summary.textContent = 'Errore';
    els.pageinfo.textContent = 'N/D';
    els.prev.disabled = true;
    els.next.disabled = true;
  }
}

function renderRows(records) {
  els.rows.textContent = '';
  if (!records.length) {
    appendMessageRow('Nessun dato disponibile per i filtri selezionati.');
    return;
  }
  records.forEach((r) => {
    const tr = document.createElement('tr');
    appendCell(tr, r.line_code);
    appendCell(tr, r.train_number);
    appendCell(tr, r.service_date);
    appendCell(tr, r.station_name);
    appendCell(tr, r.delay_minutes);
    appendCell(tr, r.scheduled_time);
    appendCell(tr, r.actual_time);
    appendCell(tr, r.status);
    appendCell(tr, r.observed_at);
    els.rows.appendChild(tr);
  });
}

function appendCell(row, value) {
  const td = document.createElement('td');
  td.textContent = value ?? '';
  row.appendChild(td);
}

function appendMessageRow(message) {
  const tr = document.createElement('tr');
  const td = document.createElement('td');
  td.colSpan = 9;
  td.textContent = message;
  tr.appendChild(td);
  els.rows.appendChild(tr);
}

els.apply.addEventListener('click', () => {
  state.q = els.search.value.trim();
  state.line = els.line.value.trim();
  state.page = 1;
  loadData();
});

els.reset.addEventListener('click', () => {
  els.search.value = '';
  els.line.value = '';
  state.q = '';
  state.line = '';
  state.page = 1;
  loadData();
});

els.prev.addEventListener('click', () => {
  if (state.page > 1) {
    state.page -= 1;
    loadData();
  }
});

els.next.addEventListener('click', () => {
  state.page += 1;
  loadData();
});

loadData();
