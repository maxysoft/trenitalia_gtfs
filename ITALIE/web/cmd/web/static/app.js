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

function esc(v) {
  return String(v ?? '').replace(/[&<>"']/g, c => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[c]));
}

function buildUrl() {
  const p = new URLSearchParams();
  p.set('page', String(state.page));
  p.set('page_size', String(state.pageSize));
  if (state.q) p.set('q', state.q);
  if (state.line) p.set('line', state.line);
  return `/api/delays?${p.toString()}`;
}

async function loadData() {
  els.rows.innerHTML = `<tr><td colspan="9">Caricamento...</td></tr>`;
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
    els.rows.innerHTML = `<tr><td colspan="9">Errore caricamento dati: ${esc(err.message)}</td></tr>`;
    els.summary.textContent = 'Errore';
    els.pageinfo.textContent = 'N/D';
    els.prev.disabled = true;
    els.next.disabled = true;
  }
}

function renderRows(records) {
  if (!records.length) {
    els.rows.innerHTML = `<tr><td colspan="9">Nessun dato disponibile per i filtri selezionati.</td></tr>`;
    return;
  }
  els.rows.innerHTML = records.map(r => `
    <tr>
      <td>${esc(r.line_code)}</td>
      <td>${esc(r.train_number)}</td>
      <td>${esc(r.service_date)}</td>
      <td>${esc(r.station_name)}</td>
      <td>${esc(r.delay_minutes)}</td>
      <td>${esc(r.scheduled_time)}</td>
      <td>${esc(r.actual_time)}</td>
      <td>${esc(r.status)}</td>
      <td>${esc(r.observed_at)}</td>
    </tr>
  `).join('');
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
