// Consola web de GameStats. Corre en el navegador y usa solo la API pública de
// la ingestión, igual que la aplicación de escritorio (simulator/cmd/gui).
'use strict';

const FINAL = ['ANALYZED', 'FAILED'];
const state = { key: '', tab: 'matches', contracts: null, contractsError: '', matches: [], selected: null, sort: { col: 'updatedAt', desc: true } };

// ---------- utilidades ----------

const $ = (sel) => document.querySelector(sel);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function el(tag, props, ...kids) {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v == null || v === false) continue;
    if (k === 'class') n.className = v;
    else if (k === 'text') n.textContent = v;
    else if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
    else n.setAttribute(k, v === true ? '' : v);
  }
  for (const c of kids.flat()) if (c != null && c !== false) n.append(c instanceof Node ? c : String(c));
  return n;
}

function store(key, value) {
  try {
    if (value === undefined) return localStorage.getItem(key) || '';
    if (value) localStorage.setItem(key, value); else localStorage.removeItem(key);
  } catch { /* almacenamiento bloqueado: la clave dura solo esta sesión */ }
  return '';
}

// "killEfficiency" -> "Kill efficiency"
function humanize(key) {
  const s = String(key).replace(/[_-]/g, ' ').replace(/([a-z0-9])([A-Z])/g, (_, a, b) => a + ' ' + b.toLowerCase());
  return s.charAt(0).toUpperCase() + s.slice(1);
}

// null significa "no calculable": se muestra "—", nunca un 0 inventado.
function value(v) {
  if (v === null || v === undefined) return '—';
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}

function patternText(p) {
  const keys = Object.keys(p).filter((k) => k !== 'type').sort();
  return value(p.type) + ' — ' + keys.map((k) => humanize(k).toLowerCase() + ': ' + value(p[k])).join(' · ');
}

function statusClass(s) {
  if (s === 'ANALYZED' || s === 'COMPLETE') return 'ok';
  if (s === 'FAILED') return 'bad';
  if (s === 'INCOMPLETE' || s === 'ANALYZING' || s === 'FINISHING') return 'warn';
  return 'accent';
}

const pill = (text, cls) => el('span', { class: 'pill pill-' + cls, text });

function deepEqual(a, b) {
  if (a === b) return true;
  if (typeof a !== typeof b || a === null || b === null || typeof a !== 'object') return false;
  if (Array.isArray(a) !== Array.isArray(b)) return false;
  const ka = Object.keys(a), kb = Object.keys(b);
  if (ka.length !== kb.length) return false;
  return ka.every((k) => Object.prototype.hasOwnProperty.call(b, k) && deepEqual(a[k], b[k]));
}

function timeOf(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  return isNaN(d) ? '' : d.toLocaleTimeString();
}

// ---------- API ----------

async function call(method, path, body) {
  const headers = {};
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET' && state.key) headers['X-API-Key'] = state.key;
  const res = await fetch(path, { method, headers, cache: 'no-store', body: body === undefined ? undefined : typeof body === 'string' ? body : JSON.stringify(body) });
  const text = await res.text();
  let data = {};
  try { data = text ? JSON.parse(text) : {}; } catch { data = { raw: text }; }
  return { status: res.status, data };
}

const getJSON = (path) => call('GET', path);
const postEvent = (body) => call('POST', '/v1/events', body);
const matchPath = (g, m) => '/v1/games/' + encodeURIComponent(g) + '/matches/' + encodeURIComponent(m);

// "409 REJECTED LATE_EVENT": la misma clave que usa la aplicación de escritorio.
const ackKey = (r) => [r.status, r.data.status, r.data.code].filter(Boolean).join(' ');

const EXPLAIN = {
  '202 ACCEPTED': 'Evento aceptado',
  '200 DUPLICATE': 'Duplicado idéntico (idempotente, no se cuenta dos veces)',
  '409 DUPLICATE_ID_CONFLICT': 'Mismo eventId con contenido distinto',
  '409 LATE_EVENT': 'Llegó después de enviar la partida a analizar',
  '409 MATCH_CLOSED': 'La partida ya estaba cerrada',
  '409 MATCH_ALREADY_STARTED': 'Segundo MATCH_STARTED',
  '409 MATCH_ALREADY_FINISHED': 'Segundo MATCH_FINISHED',
  '413 MATCH_TOO_LARGE': 'La partida superó el límite de eventos',
  '413 EVENT_TOO_LARGE': 'Evento mayor a 64 KB',
  '503 BACKPRESSURE': 'Cola de la partida llena: reintentar',
  '401 UNAUTHORIZED': 'Falta la clave de API o es incorrecta (arriba a la derecha)',
};

function explain(key) {
  const k = key.replace(' REJECTED', '');
  if (EXPLAIN[k]) return EXPLAIN[k];
  if (key.startsWith('400 ')) return 'Evento inválido: rechazado con campo y código';
  return '';
}

function ackClass(key) {
  if (key.startsWith('2')) return key.startsWith('200 DUPLICATE') ? 'warn' : 'ok';
  if (key.startsWith('409') || key.startsWith('503')) return 'warn';
  return 'bad';
}

// ---------- cabecera: conexión y clave ----------

async function checkHealth() {
  const conn = $('#conn');
  const t0 = performance.now();
  try {
    const r = await getJSON('/healthz');
    const ms = Math.round(performance.now() - t0);
    if (r.status === 200) { conn.className = 'pill pill-ok'; conn.textContent = '● ingestión en línea · ' + ms + ' ms'; return; }
    throw new Error('HTTP ' + r.status);
  } catch {
    conn.className = 'pill pill-bad';
    conn.textContent = '● sin conexión';
  }
}

function renderKeyState() {
  const s = $('#key-state');
  if (state.key) { s.textContent = 'clave guardada'; s.className = 'hint t-ok'; }
  else { s.textContent = 'sin clave: solo lectura'; s.className = 'hint t-warn'; }
}

function needKeyNotice() {
  return state.key ? null : el('div', { class: 'notice notice-warn' },
    'No hay clave de API guardada. Si el servidor la exige (como en Render), los envíos responderán 401. Pégala arriba a la derecha.');
}

// ---------- pestañas ----------

function showTab(name) {
  state.tab = name;
  document.querySelectorAll('.tabs button').forEach((b) => b.classList.toggle('active', b.dataset.tab === name));
  document.querySelectorAll('.tab').forEach((s) => s.classList.toggle('active', s.id === 'tab-' + name));
  if (name === 'matches') refreshMatches();
  if (name === 'rejections') refreshRejections();
}

// ---------- Partidas ----------

const TILES = [
  ['matches', 'Partidas', 'accent'],
  ['accepted', 'Aceptados', 'ok'],
  ['rejected', 'Rechazados', 'bad'],
  ['duplicates', 'Duplicados', 'warn'],
  ['late', 'Tardíos', 'warn'],
];

const COLUMNS = [
  ['gameId', 'Juego'], ['matchId', 'Partida'], ['status', 'Estado'],
  ['completeness', 'Completitud'], ['eventCount', 'Eventos'], ['updatedAt', 'Actualizada'],
];

async function refreshMatches() {
  try {
    const [m, s] = await Promise.all([getJSON('/v1/matches'), getJSON('/v1/stats')]);
    state.matches = Array.isArray(m.data) ? m.data : [];
    const ev = (s.data && s.data.events) || {};
    const values = { matches: state.matches.length, ...ev };
    $('#tiles').replaceChildren(...TILES.map(([k, label, c]) => el('div', { class: 'tile c-' + c }, el('b', { text: value(values[k] ?? '—') }), el('span', { text: label }))));
    fillOptions($('#m-game'), 'Todos los juegos', state.matches.map((x) => x.gameId));
    fillOptions($('#m-state'), 'Todos los estados', state.matches.map((x) => x.status));
    renderMatchTable();
    const sel = state.selected && state.matches.find((x) => x.gameId === state.selected.gameId && x.matchId === state.selected.matchId);
    if (sel && state.selected.status && sel.status !== state.selected.status) openMatch(sel.gameId, sel.matchId);
  } catch { /* el indicador de conexión ya avisa */ }
}

function fillOptions(select, all, values) {
  const current = select.value;
  const uniq = [...new Set(values.filter(Boolean))].sort();
  select.replaceChildren(el('option', { value: '', text: all }), ...uniq.map((v) => el('option', { value: v, text: v })));
  select.value = uniq.includes(current) ? current : '';
}

function renderMatchTable() {
  const q = $('#m-search').value.trim().toLowerCase();
  const game = $('#m-game').value, st = $('#m-state').value;
  const { col, desc } = state.sort;
  const rows = state.matches
    .filter((m) => (!q || m.matchId.toLowerCase().includes(q) || m.gameId.includes(q)) && (!game || m.gameId === game) && (!st || m.status === st))
    .sort((a, b) => {
      const x = a[col] ?? '', y = b[col] ?? '';
      const r = typeof x === 'number' && typeof y === 'number' ? x - y : String(x).localeCompare(String(y));
      return desc ? -r : r;
    });
  const head = el('tr', null, COLUMNS.map(([k, label]) => el('th', {
    class: 'sortable', title: 'Ordenar', onclick: () => { state.sort = { col: k, desc: state.sort.col === k ? !state.sort.desc : false }; renderMatchTable(); },
  }, label + (col === k ? (desc ? ' ▼' : ' ▲') : ''))));
  const body = rows.map((m) => {
    const isSel = state.selected && state.selected.gameId === m.gameId && state.selected.matchId === m.matchId;
    return el('tr', { class: isSel ? 'selected' : null, onclick: () => openMatch(m.gameId, m.matchId) },
      el('td', { text: m.gameId }), el('td', { text: m.matchId }),
      el('td', null, el('span', { class: 't-' + statusClass(m.status), text: m.status })),
      el('td', null, m.completeness ? el('span', { class: 't-' + statusClass(m.completeness), text: m.completeness }) : '—'),
      el('td', { class: 'num', text: m.eventCount }), el('td', { text: timeOf(m.updatedAt) }));
  });
  $('#m-table').replaceChildren(el('thead', null, head), el('tbody', null, body));
  $('#m-count').textContent = state.matches.length
    ? 'Mostrando ' + rows.length + ' de ' + state.matches.length + ' partidas. Se actualiza cada 4 s.'
    : 'Aún no hay partidas. Ve a «Generar datos» para enviar algunas.';
}

async function openMatch(gameId, matchId) {
  if (state.tab !== 'matches') showTab('matches');
  state.selected = { gameId, matchId, status: '' };
  renderMatchTable();
  const box = $('#detail');
  box.replaceChildren(el('p', { class: 'muted', text: 'Cargando ' + matchId + '…' }));
  const r = await getJSON(matchPath(gameId, matchId) + '/results');
  if (!state.selected || state.selected.matchId !== matchId) return;
  state.selected.status = r.data.status || '';
  box.replaceChildren(renderDetail(r.data, r.status));
}

function renderDetail(rec, status) {
  if (status === 404) return el('p', { class: 'muted', text: 'La partida ya no existe (el servicio pudo reiniciarse).' });
  const res = rec.result;
  const summary = [rec.gameId, rec.eventCount + ' eventos', rec.attempts + ' intento(s)', rec.closedReason ? 'cierre: ' + rec.closedReason : ''].filter(Boolean).join(' · ');
  const out = el('div', { class: 'detail' },
    el('h2', { text: rec.matchId }),
    el('div', { class: 'detail-head' }, pill(rec.status, statusClass(rec.status)), rec.completeness ? pill(rec.completeness, statusClass(rec.completeness)) : null, el('span', { class: 't-muted', text: summary })));
  if (!res) {
    const msg = rec.status === 'FAILED' ? 'El análisis falló: ' + (rec.lastError || 'sin detalle')
      : 'La partida aún no tiene resultado: Go la está recibiendo o Scala la está analizando.';
    out.append(el('div', { class: 'notice ' + (rec.status === 'FAILED' ? 'notice-bad' : 'notice-warn'), text: msg }));
    return out;
  }

  const ms = res.matchStats || {};
  const metrics = [['Duración (s)', ms.durationSeconds], ['Participantes', ms.participants], ['Eventos', ms.totalEvents],
    ...Object.keys(ms.metrics || {}).sort().map((k) => [humanize(k), ms.metrics[k]])];
  out.append(el('section', null, el('h3', { text: 'Estadísticas de la partida' }),
    el('div', { class: 'tiles' }, metrics.map(([k, v]) => el('div', { class: 'tile' }, el('b', { text: value(v) }), el('span', { text: k }))))));

  const players = res.playerStats || [];
  const cols = [];
  players.forEach((p) => Object.keys(p.metrics || {}).sort().forEach((k) => { if (!cols.includes(k)) cols.push(k); }));
  const headRow = el('tr', null, el('th', { text: 'Jugador' }), cols.map((c) => el('th', { text: humanize(c) })));
  const bodyRows = players.map((p) => el('tr', null, el('td', { text: p.playerId }), cols.map((c) => el('td', { class: 'num', text: value(p.metrics[c]) }))));
  out.append(el('section', null, el('h3', { text: 'Estadísticas por jugador (' + players.length + ')' }),
    el('div', { class: 'table-wrap' }, el('table', { class: 'table' }, el('thead', null, headRow), el('tbody', null, bodyRows))),
    el('p', { class: 'hint', text: '«—» = no calculable con los datos recibidos (el sistema no inventa valores).' })));

  const patterns = res.patterns || [];
  out.append(el('section', null, el('h3', { text: 'Patrones secuenciales (' + patterns.length + ')' }),
    patterns.length ? el('ul', { class: 'list' }, patterns.map((p) => el('li', { text: patternText(p) }))) : el('p', { class: 'muted', text: 'No se detectó ningún patrón en esta partida.' })));

  const warnings = res.warnings || [];
  const inp = res.input || {};
  out.append(el('section', null, el('h3', { text: 'Calidad de los datos (' + warnings.length + ' avisos)' }),
    el('p', { text: 'Eventos recibidos ' + value(inp.eventsReceived) + ' · usados ' + value(inp.eventsUsed) + ' · ignorados ' + value(inp.eventsIgnored) }),
    warnings.length ? el('ul', { class: 'list' }, warnings.map((w) => el('li', null, el('span', { class: 't-warn', text: w.code }), ' — ' + (w.message || '') + (w.eventId ? ' (' + w.eventId + ')' : ''))))
      : el('p', { class: 't-ok', text: 'Sin avisos: todos los eventos eran coherentes.' })));

  const pre = el('pre', { hidden: true, text: JSON.stringify(res, null, 2) });
  out.append(el('section', null, el('h3', null, el('button', { class: 'btn btn-small', onclick: () => { pre.hidden = !pre.hidden; } }, 'Ver JSON del resultado')), pre));
  return out;
}

// ---------- envío de partidas (Generar datos y Verificación) ----------

function newSuffix() { return '-w' + Date.now().toString(36); }

function fixtureMatch(f, suffix) {
  return {
    name: f.name, gameId: f.gameId, matchId: f.matchId + suffix, expected: f.expected || null,
    events: f.events.map((e) => ({ ...e, matchId: e.matchId + suffix, eventId: e.eventId + suffix })),
    after: [],
  };
}

// Escenario demo: dos fixtures intercalados con eventos inválidos, un duplicado,
// un conflicto de eventId, un evento tardío y una partida sin MATCH_FINISHED.
function demoMatches(suffix) {
  const fx = (n) => state.contracts.fixtures.find((f) => f.name === n);
  const arena = fixtureMatch(fx('F01-arena-normal'), suffix);
  const race = fixtureMatch(fx('F17-racing-comeback'), suffix);
  const i = arena.events.findIndex((e) => e.type === 'ATTACK' && e.data && e.data.hit);
  const a = arena.events[i];
  const specials = [
    { ...a, eventId: 'bad-ts' + suffix, timestamp: '2026-10-01 18:00' },
    { ...a, eventId: 'bad-type' + suffix, type: 'DANCE' },
    (({ playerId, ...rest }) => ({ ...rest, eventId: 'no-player' + suffix }))(a),
    { ...a },
    { ...a, data: { ...a.data, damage: a.data.damage + 7 } },
  ];
  arena.events.splice(i + 1, 0, ...specials);
  const joined = arena.events.find((e) => e.type === 'PLAYER_JOINED');
  arena.after = [{ ...joined, eventId: joined.eventId + '-late' }];

  const f10 = fx('F10-arena-zero-denominator');
  const open = fixtureMatch({ ...f10, matchId: 'demo-open', expected: null }, suffix);
  open.events = open.events.filter((e) => e.type !== 'MATCH_FINISHED').map((e) => ({ ...e, matchId: 'demo-open' + suffix, eventId: 'open-' + e.eventId }));
  open.open = true;
  return [arena, race, open];
}

// Envía las partidas en paralelo (cada una en su orden), espera su análisis y
// compara con el resultado calculado a mano cuando existe.
async function runMatches(matches, suffix, ui) {
  const counts = {};
  const total = matches.reduce((n, m) => n + m.events.length + m.after.length, 0);
  let sent = 0;
  const ack = (r) => { const k = ackKey(r); counts[k] = (counts[k] || 0) + 1; sent++; ui.progress(sent, total, counts); return r; };

  ui.step('Enviando ' + total + ' eventos de ' + matches.length + ' partidas en paralelo…');
  await Promise.all(matches.map(async (m, idx) => {
    await sleep(idx * 40);
    for (const e of m.events) { ack(await postEvent(e)); await sleep(25); }
  }));
  if (counts['401 REJECTED UNAUTHORIZED']) throw new Error('El servidor rechazó los envíos por la clave de API (401). Pega la clave correcta arriba a la derecha.');

  const hasOpen = matches.some((m) => m.open);
  ui.step('Esperando el análisis de Scala… Si el servicio estaba dormido puede tardar alrededor de un minuto.' + (hasOpen ? ' La partida sin final se cierra sola por inactividad.' : ''));
  const records = {};
  const deadline = Date.now() + 180000;
  await Promise.all(matches.map(async (m) => {
    while (Date.now() < deadline) {
      const r = await getJSON(matchPath(m.gameId, m.matchId));
      if (r.status === 200 && FINAL.includes(r.data.status)) break;
      await sleep(1000);
    }
    const r = await getJSON(matchPath(m.gameId, m.matchId) + '/results');
    records[m.matchId] = r.data;
    for (const e of m.after) ack(await postEvent(e));
  }));

  return matches.map((m) => {
    const rec = records[m.matchId] || {};
    const v = { name: m.name, gameId: m.gameId, matchId: m.matchId, status: rec.status || 'SIN RESPUESTA', completeness: rec.completeness || '', eventCount: rec.eventCount, result: rec.result, check: null };
    if (m.expected) {
      const c = { group: 'fixtures', name: m.name, pass: false, expected: 'idéntico a ' + m.name + '.result.json', actual: '' };
      if (!rec.result) c.actual = 'la partida no tiene resultado (' + v.status + ')';
      else {
        const got = JSON.parse(JSON.stringify(rec.result).split(suffix + '"').join('"'));
        c.pass = deepEqual(got, m.expected);
        c.actual = c.pass ? 'resultado idéntico al calculado a mano' : 'el resultado difiere del calculado a mano';
      }
      v.check = c;
    }
    return v;
  });
}

function progressUI(box) {
  const bar = el('div');
  const stepEl = el('p', { class: 'hint' });
  const countsBox = el('div');
  box.append(stepEl, el('div', { class: 'progress' }, bar), countsBox);
  return {
    step: (t) => { stepEl.textContent = t; },
    progress: (sent, total, counts) => {
      bar.style.width = Math.round((100 * sent) / Math.max(total, 1)) + '%';
      const keys = Object.keys(counts).sort();
      countsBox.replaceChildren(el('h3', { text: 'Respuestas del sistema (' + sent + ' de ' + total + ')' }),
        el('div', { class: 'table-wrap' }, el('table', { class: 'table' },
          el('thead', null, el('tr', null, el('th', { text: 'Respuesta' }), el('th', { class: 'num', text: 'Cantidad' }), el('th', { text: 'Significado' }))),
          el('tbody', null, keys.map((k) => el('tr', null, el('td', null, el('span', { class: 't-' + ackClass(k), text: k })), el('td', { class: 'num', text: counts[k] }), el('td', { class: 'wrap', text: explain(k) })))))));
    },
  };
}

// ---------- Generar datos ----------

function setupGenerate() {
  const kind = $('#g-kind');
  const opts = [el('option', { value: 'all', text: 'Todos los casos de prueba (8 partidas)' }), el('option', { value: 'demo', text: 'Escenario demo (casos especiales)' })];
  state.contracts.fixtures.forEach((f) => opts.push(el('option', { value: 'fx:' + f.name, text: 'Caso ' + f.name })));
  kind.replaceChildren(...opts);
  const desc = () => {
    const v = kind.value;
    if (v === 'all') return 'Envía las 8 partidas de prueba de los dos juegos a la vez y compara cada resultado con el calculado a mano.';
    if (v === 'demo') return 'Una partida de arena con 3 eventos inválidos, un duplicado, un conflicto de eventId y un evento tardío; una carrera con remontada; y una partida que nunca recibe MATCH_FINISHED y se cierra sola por inactividad.';
    const f = state.contracts.fixtures.find((x) => 'fx:' + x.name === v);
    return f ? f.description + ' (' + f.events.length + ' eventos)' : '';
  };
  kind.addEventListener('change', () => { $('#g-desc').textContent = desc(); });
  $('#g-desc').textContent = desc();
  $('#g-run').addEventListener('click', runGenerate);
}

async function runGenerate() {
  const btn = $('#g-run');
  const out = $('#g-out');
  btn.disabled = true;
  out.replaceChildren();
  const notice = needKeyNotice();
  if (notice) out.append(notice);
  const suffix = newSuffix();
  const v = $('#g-kind').value;
  let matches;
  if (v === 'all') matches = state.contracts.fixtures.map((f) => fixtureMatch(f, suffix));
  else if (v === 'demo') matches = demoMatches(suffix);
  else matches = state.contracts.fixtures.filter((f) => 'fx:' + f.name === v).map((f) => fixtureMatch(f, suffix));
  const ui = progressUI(out);
  try {
    const views = await runMatches(matches, suffix, ui);
    ui.step('Terminado. Haz clic en una partida para ver sus estadísticas.');
    out.append(el('h3', { text: 'Partidas enviadas' }), el('div', { class: 'table-wrap' }, el('table', { class: 'table clickable' },
      el('thead', null, el('tr', null, ['Partida', 'Estado', 'Completitud', 'Patrones', 'Avisos', '¿Coincide con lo calculado a mano?'].map((h) => el('th', { text: h })))),
      el('tbody', null, views.map((x) => el('tr', { onclick: () => openMatch(x.gameId, x.matchId) },
        el('td', { text: x.matchId }),
        el('td', null, el('span', { class: 't-' + statusClass(x.status), text: x.status })),
        el('td', null, x.completeness ? el('span', { class: 't-' + statusClass(x.completeness), text: x.completeness }) : '—'),
        el('td', { class: 'wrap', text: ((x.result && x.result.patterns) || []).map((p) => p.type + ' · ' + p.playerId).join(', ') || '—' }),
        el('td', { class: 'num', text: ((x.result && x.result.warnings) || []).length }),
        el('td', null, x.check ? el('span', { class: x.check.pass ? 't-ok' : 't-bad', text: x.check.pass ? '✓ sí' : '✗ no' }) : el('span', { class: 't-muted', text: 'sin resultado esperado' }))))))));
  } catch (err) {
    out.append(el('div', { class: 'notice notice-bad', text: err.message }));
  } finally {
    btn.disabled = false;
  }
}

// ---------- Verificación ----------

const GROUPS = {
  fixtures: 'Casos de prueba contra resultados calculados a mano',
  'inválidos': 'Eventos inválidos (código y campo del primer error)',
  comportamiento: 'Comportamiento del sistema',
};
const GROUP_SHORT = { fixtures: 'Casos de prueba', 'inválidos': 'Eventos inválidos', comportamiento: 'Comportamiento' };
let lastChecks = null;

function label(r) {
  return r.status + ' ' + (r.data.code || r.data.status || '');
}

async function behaviors(suffix, add) {
  const match = 'verif' + suffix;
  const mk = (id, type, player, data) => {
    const e = { eventId: id + suffix, timestamp: '2026-10-01T15:00:00Z', gameId: 'arena-combat', gameVersion: '1.0', matchId: match, type, data };
    if (player) e.playerId = player;
    return e;
  };
  const check = (name, want, got) => add({ group: 'comportamiento', name, expected: want, actual: got, pass: got === want });
  const post = async (name, want, body) => check(name, want, label(await postEvent(body)));
  const get = async (name, want, path) => check(name, want, label(await getJSON(path)));
  const game = '/v1/games/arena-combat/matches/';
  const start = mk('v1', 'MATCH_STARTED', '', { map: 'dust' });
  await post('Evento válido aceptado', '202 ACCEPTED', start);
  await post('Mismo evento repetido es idempotente', '200 DUPLICATE', start);
  await post('Mismo eventId con otro contenido es conflicto', '409 DUPLICATE_ID_CONFLICT', mk('v1', 'MATCH_STARTED', '', { map: 'otro' }));
  await post('Segundo MATCH_STARTED se rechaza', '409 MATCH_ALREADY_STARTED', mk('v2', 'MATCH_STARTED', '', { map: 'dust' }));
  await post('Jugador se une', '202 ACCEPTED', mk('v3', 'PLAYER_JOINED', 'p1', {}));
  await get('Resultado de una partida abierta aún no existe', '202 COLLECTING', game + match + '/results');
  await post('Fin de partida aceptado', '202 ACCEPTED', mk('v4', 'MATCH_FINISHED', '', { reason: 'COMPLETED' }));
  await post('Segundo MATCH_FINISHED se rechaza', '409 MATCH_ALREADY_FINISHED', mk('v5', 'MATCH_FINISHED', '', { reason: 'COMPLETED' }));
  let status = '';
  for (const deadline = Date.now() + 120000; Date.now() < deadline; await sleep(500)) {
    const r = await getJSON(game + match);
    status = r.data.status || '';
    if (FINAL.includes(status)) break;
  }
  check('La partida se analiza tras el período de gracia', 'ANALYZED', status);
  await post('Evento posterior al análisis se rechaza', '409 LATE_EVENT', mk('v6', 'PLAYER_JOINED', 'p2', {}));
  await get('Partida inexistente responde 404', '404 MATCH_NOT_FOUND', game + 'no-existe' + suffix);
  await get('Servicio de ingestión saludable', '200 ok', '/healthz');
}

async function runVerify() {
  const btn = $('#v-run');
  const out = $('#v-out');
  btn.disabled = true;
  $('#v-report').disabled = true;
  out.replaceChildren();
  const notice = needKeyNotice();
  if (notice) out.append(notice);
  const suffix = newSuffix();
  const checks = [];
  const summary = el('div');
  const list = el('div');
  const ui = progressUI(el('div'));
  const stepEl = el('p', { class: 'hint' });
  out.append(stepEl, summary, list);
  const render = () => {
    const ok = checks.filter((c) => c.pass).length;
    const chips = Object.keys(GROUPS).map((g) => {
      const cs = checks.filter((c) => c.group === g);
      if (!cs.length) return null;
      const p = cs.filter((c) => c.pass).length;
      return pill(GROUP_SHORT[g] + ' ' + p + '/' + cs.length, p === cs.length ? 'ok' : 'bad');
    });
    summary.replaceChildren(el('p', { class: ok === checks.length ? 't-ok' : 't-bad', text: (ok === checks.length ? '✓ ' : '✗ ') + ok + ' de ' + checks.length + ' comprobaciones pasaron' }), el('div', { class: 'chips' }, chips));
    let group = '';
    list.replaceChildren(...checks.flatMap((c) => {
      const items = [];
      if (c.group !== group) { group = c.group; items.push(el('h3', { text: GROUPS[group] })); }
      const sub = c.group === 'fixtures' ? c.actual : 'esperado: ' + c.expected + '  ·  obtenido: ' + (c.actual || '—');
      items.push(el('div', { class: 'check' }, pill(c.pass ? 'PASS' : 'FAIL', c.pass ? 'ok' : 'bad'), el('div', null, el('div', { class: 'name', text: c.name }), el('div', { class: 'sub', text: sub }))));
      return items;
    }));
  };
  const add = (c) => { checks.push(c); render(); };
  try {
    stepEl.textContent = 'Grupo 1 de 3: enviando los 8 casos de prueba y esperando su análisis… (si Scala estaba dormido, puede tardar alrededor de un minuto)';
    const views = await runMatches(state.contracts.fixtures.map((f) => fixtureMatch(f, suffix)), suffix, { step: () => {}, progress: ui.progress });
    views.forEach((v) => add(v.check || { group: 'fixtures', name: v.name, pass: false, actual: 'sin resultado esperado' }));

    stepEl.textContent = 'Grupo 2 de 3: enviando los eventos inválidos del contrato…';
    for (const ex of state.contracts.invalid) {
      const r = await postEvent(ex.body);
      const e0 = (r.data.errors && r.data.errors[0]) || {};
      const code = e0.code || r.data.code || '', field = e0.field || '';
      const wantField = ex.field || '';
      add({ group: 'inválidos', name: ex.name, expected: ('400 ' + ex.code + ' ' + wantField).trim(), actual: (r.status + ' ' + code + ' ' + field).trim(), pass: r.status === 400 && code === ex.code && field === wantField });
    }

    stepEl.textContent = 'Grupo 3 de 3: ciclo de vida, duplicados y eventos tardíos…';
    await behaviors(suffix, add);
    stepEl.textContent = 'Terminado (' + new Date().toLocaleString() + ').';
    lastChecks = checks;
    $('#v-report').disabled = false;
  } catch (err) {
    out.prepend(el('div', { class: 'notice notice-bad', text: err.message }));
  } finally {
    btn.disabled = false;
  }
}

function downloadReport() {
  if (!lastChecks) return;
  const ok = lastChecks.filter((c) => c.pass).length;
  const cell = (s) => (s && s.length > 60 ? '(JSON)' : '`' + String(s || '').replace(/\|/g, '\\|') + '`');
  let md = '# Informe de verificación de GameStats\n\nFecha: ' + new Date().toLocaleString() + '\n\nServidor: ' + location.origin + '\n\nResultado: **' + ok + '/' + lastChecks.length + '** comprobaciones pasaron.\n';
  let group = '';
  for (const c of lastChecks) {
    if (c.group !== group) { group = c.group; md += '\n## ' + GROUPS[group] + '\n\n| Estado | Comprobación | Esperado | Obtenido |\n|---|---|---|---|\n'; }
    md += '| ' + (c.pass ? 'PASS' : 'FAIL') + ' | ' + c.name + ' | ' + cell(c.expected) + ' | ' + cell(c.actual) + ' |\n';
  }
  const a = el('a', { href: URL.createObjectURL(new Blob([md], { type: 'text/markdown' })), download: 'verificacion-gamestats.md' });
  document.body.append(a);
  a.click();
  a.remove();
}

// ---------- Enviar evento ----------

function setupSend() {
  const sel = $('#s-template');
  const group = (title, list, kind) => el('optgroup', { label: title }, list.map((x) => el('option', { value: kind + ':' + x.name, text: x.name })));
  sel.append(group('Válidos', state.contracts.valid, 'valid'), group('Inválidos', state.contracts.invalid, 'invalid'));
  sel.addEventListener('change', () => {
    const [kind, name] = sel.value.split(/:(.*)/s);
    const ex = (state.contracts[kind] || []).find((x) => x.name === name);
    if (!ex) return;
    $('#s-body').value = ex.body.trim();
    $('#s-hint').textContent = kind === 'invalid' ? 'Error esperado: 400 ' + ex.code + (ex.field ? ' en ' + ex.field : '') :'Evento válido: la primera vez debe responder 202 ACCEPTED; si lo repites, 200 DUPLICATE.';
    validateBody();
  });
}

function validateBody() {
  const t = $('#s-body').value.trim();
  if (!t) return;
  try { JSON.parse(t); $('#s-format').disabled = false; }
  catch { $('#s-hint').textContent = 'Aviso: el texto no es JSON válido. Se puede enviar igual para ver cómo lo rechaza el sistema.'; $('#s-format').disabled = true; }
}

function setupSendButtons() {
  $('#s-body').addEventListener('input', validateBody);
  $('#s-format').addEventListener('click', () => {
    try { $('#s-body').value = JSON.stringify(JSON.parse($('#s-body').value), null, 2); } catch { /* el aviso ya se muestra */ }
  });
  $('#s-fresh').addEventListener('click', () => {
    try {
      const e = JSON.parse($('#s-body').value);
      const s = newSuffix();
      if (typeof e.eventId === 'string') e.eventId = e.eventId.replace(/-w[0-9a-z]+$/, '') + s;
      if (typeof e.matchId === 'string') e.matchId = e.matchId.replace(/-w[0-9a-z]+$/, '') + s;
      $('#s-body').value = JSON.stringify(e, null, 2);
    } catch { $('#s-hint').textContent = 'Para cambiar los IDs el texto tiene que ser JSON válido.'; }
  });
  $('#s-send').addEventListener('click', async () => {
    const out = $('#s-out');
    try {
      const r = await postEvent($('#s-body').value);
      const k = ackKey(r);
      out.replaceChildren(el('div', { class: 'detail-head' }, pill(k, ackClass(k)), el('span', { class: 't-muted', text: explain(k) })), el('pre', { text: JSON.stringify(r.data, null, 2) }));
    } catch (err) {
      out.replaceChildren(el('div', { class: 'notice notice-bad', text: 'No se pudo enviar: ' + err.message }));
    }
  });
}

// ---------- Rechazos ----------

async function refreshRejections() {
  try {
    const r = await getJSON('/v1/rejections?limit=100');
    const all = Array.isArray(r.data) ? r.data : [];
    fillOptions($('#r-code'), 'Todos los códigos', all.map((x) => x.code));
    const code = $('#r-code').value;
    const rows = all.filter((x) => !code || x.code === code);
    $('#r-count').textContent = rows.length + ' de ' + all.length + ' rechazos recientes (inválidos, tardíos, conflictos…). Se actualiza cada 3 s.';
    $('#r-table').replaceChildren(
      el('thead', null, el('tr', null, ['Hora', 'Código', 'Campo', 'Juego', 'Partida', 'Evento', 'Mensaje'].map((h) => el('th', { text: h })))),
      el('tbody', null, rows.map((x) => el('tr', null,
        el('td', { text: timeOf(x.at) }),
        el('td', null, el('span', { class: 't-' + (x.code === 'LATE_EVENT' || x.code === 'MATCH_CLOSED' || String(x.code).startsWith('MATCH_ALREADY') || x.code === 'DUPLICATE_ID_CONFLICT' ? 'warn' : 'bad'), text: x.code })),
        el('td', { text: x.field || '—' }), el('td', { text: x.gameId || '—' }), el('td', { text: x.matchId || '—' }), el('td', { text: x.eventId || '—' }),
        el('td', { class: 'wrap', text: x.message || '' })))));
  } catch { /* el indicador de conexión ya avisa */ }
}

// ---------- Guía ----------

function renderGuide() {
  const step = (title, text) => el('li', null, el('b', { text: title }), ' ', text);
  $('#guide').replaceChildren(
    el('h2', { text: 'Recorrido sugerido (5 minutos)' }),
    el('p', { class: 'muted', text: 'Esta página es una consola web: corre en tu navegador y usa la misma API pública que la aplicación de escritorio. Go recibe y agrupa los eventos; Scala calcula las estadísticas.' }),
    el('ol', null,
      step('Clave de API.', 'Para enviar datos, pega la clave arriba a la derecha y pulsa Guardar. Sin clave, todo lo de lectura funciona igual.'),
      step('Generar datos.', 'Elige «Escenario demo» y pulsa Ejecutar. Verás cada respuesta del sistema explicada: eventos aceptados, inválidos, un duplicado, un conflicto y un evento tardío.'),
      step('Partidas.', 'Cada partida pasa por COLLECTING → ANALYZING → ANALYZED. La que nunca recibió MATCH_FINISHED se cierra sola por inactividad y queda INCOMPLETE.'),
      step('Selecciona una partida.', 'Aparecen sus estadísticas por partida y por jugador, los patrones detectados (racha o remontada) y los avisos de datos incoherentes. Lo no calculable se muestra como «—».'),
      step('Verificación.', 'Pulsa «Ejecutar suite completa»: deben pasar las 37 comprobaciones. Con «Descargar informe» obtienes la evidencia en Markdown.'),
      step('Enviar evento.', 'Elige una plantilla inválida (por ejemplo bad-timestamp.json), pulsa Enviar y mira cómo el sistema indica el campo y el motivo del error.')),
    el('h3', { text: 'Si es la primera visita del día' }),
    el('p', { text: 'En el plan gratuito de Render los servicios se duermen tras 15 minutos sin uso. Si esta página cargó, la ingestión ya está despierta; el análisis en Scala puede tardar alrededor de un minuto más la primera vez. Los resultados viven en memoria: si el servicio se durmió, genera datos de nuevo.' }),
    el('h3', { text: 'Juegos soportados' }),
    el('ul', null,
      el('li', null, el('b', { text: 'arena-combat' }), ': combate por equipos. Precisión, daño, eliminaciones por minuto y el patrón KILL_STREAK.'),
      el('li', null, el('b', { text: 'street-racing' }), ': carreras por vueltas. Mejor vuelta, posiciones ganadas y el patrón COMEBACK.')),
    el('h3', { text: 'Más información' }),
    el('p', null, 'Código y documentación: ', el('a', { href: 'https://github.com/kayy1y/ProyectoLenguajes', target: '_blank', rel: 'noopener', text: 'github.com/kayy1y/ProyectoLenguajes' }), '.'));
}

// ---------- arranque ----------

async function loadContracts() {
  try {
    const r = await getJSON('/ui/contracts.json');
    if (r.status !== 200) throw new Error(r.data.message || 'HTTP ' + r.status);
    state.contracts = r.data;
  } catch (err) {
    state.contractsError = err.message;
  }
}

function contractsMissing(sel) {
  $(sel).replaceChildren(el('div', { class: 'notice notice-bad', text: 'Los casos de prueba no están disponibles en este servidor (' + state.contractsError + '). El resto de la consola funciona.' }));
}

async function init() {
  document.querySelectorAll('.tabs button').forEach((b) => b.addEventListener('click', () => showTab(b.dataset.tab)));
  state.key = store('gamestats-key');
  $('#key-input').value = state.key;
  renderKeyState();
  $('#key-form').addEventListener('submit', (e) => {
    e.preventDefault();
    state.key = $('#key-input').value.trim();
    store('gamestats-key', state.key);
    renderKeyState();
  });
  ['#m-search', '#m-game', '#m-state'].forEach((s) => $(s).addEventListener('input', renderMatchTable));
  $('#m-refresh').addEventListener('click', refreshMatches);
  $('#r-code').addEventListener('change', refreshRejections);
  $('#v-run').addEventListener('click', runVerify);
  $('#v-report').addEventListener('click', downloadReport);
  setupSendButtons();
  renderGuide();

  checkHealth();
  refreshMatches();
  setInterval(checkHealth, 10000);
  setInterval(() => { if (state.tab === 'matches' && !document.hidden) refreshMatches(); }, 4000);
  setInterval(() => { if (state.tab === 'rejections' && !document.hidden) refreshRejections(); }, 3000);

  await loadContracts();
  if (state.contracts) { setupGenerate(); setupSend(); }
  else { contractsMissing('#g-out'); contractsMissing('#v-out'); $('#g-run').disabled = true; $('#v-run').disabled = true; }
}

init();
