/* ==========================================================================
   Argus dashboard client
   ========================================================================== */

const el = {
  table: document.getElementById('monitorTable'),
  incidents: document.getElementById('incidentList'),
  incidentsOverview: document.getElementById('incidentListOverview'),
  form: document.getElementById('monitorForm'),
  refreshBtn: document.getElementById('refreshBtn'),
  refreshDot: document.getElementById('refreshDot'),
  apiKey: document.getElementById('apiKey'),
  saveKeyBtn: document.getElementById('saveKeyBtn'),
  toggleKeyVisibility: document.getElementById('toggleKeyVisibility'),
  channelForm: document.getElementById('channelForm'),
  maintenanceForm: document.getElementById('maintenanceForm'),
  statusPageForm: document.getElementById('statusPageForm'),
  statusPages: document.getElementById('statusPages'),
  pingTable: document.getElementById('pingTable'),
  toastStack: document.getElementById('toastStack'),
  themeToggle: document.getElementById('themeToggle'),
  monitorType: document.getElementById('monitorType'),
  keywordField: document.getElementById('keywordField'),
  monitorSearch: document.getElementById('monitorSearch'),
  monitorFilter: document.getElementById('monitorFilter'),
  refreshCountdown: document.getElementById('refreshCountdown'),
  confirmModal: document.getElementById('confirmModal'),
  confirmTitle: document.getElementById('confirmTitle'),
  confirmBody: document.getElementById('confirmBody'),
  confirmCancelBtn: document.getElementById('confirmCancelBtn'),
  confirmOkBtn: document.getElementById('confirmOkBtn'),
  statTotal: document.getElementById('statTotal'),
  statUp: document.getElementById('statUp'),
  statDown: document.getElementById('statDown'),
  statIncidents: document.getElementById('statIncidents'),
  statusBanner: document.getElementById('statusBanner'),
  statusBannerLamp: document.getElementById('statusBannerLamp'),
  statusBannerText: document.getElementById('statusBannerText'),
};

let latestWebsites = [];
const AUTO_REFRESH_SECONDS = 30;
let countdown = AUTO_REFRESH_SECONDS;

/* ---------------------------- Toasts ---------------------------- */

const ICONS = {
  success: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20 6 9 17l-5-5"/></svg>',
  error: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M15 9l-6 6M9 9l6 6"/></svg>',
  info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/></svg>',
};

function showToast(message, type = 'success', timeout = 4200) {
  const toast = document.createElement('div');
  toast.className = `toast ${type}`;
  toast.setAttribute('data-entering', 'true');
  const time = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  toast.innerHTML = `${ICONS[type] || ICONS.info}<span>${escapeHtml(message)}</span><span class="toast-time">${time}</span><button class="toast-close" aria-label="Dismiss">&times;</button>`;
  el.toastStack.appendChild(toast);
  requestAnimationFrame(() => toast.removeAttribute('data-entering'));

  const remove = () => {
    toast.classList.add('leaving');
    setTimeout(() => toast.remove(), 200);
  };
  toast.querySelector('.toast-close').addEventListener('click', remove);
  if (timeout) setTimeout(remove, timeout);
}

function escapeHtml(str) {
  return String(str).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

/* ---------------------------- Theme ---------------------------- */

function initTheme() {
  const saved = localStorage.getItem('argus_theme');
  const theme = saved || 'dark';
  applyTheme(theme);
}

function applyTheme(theme) {
  if (theme === 'light') {
    document.documentElement.setAttribute('data-theme', 'light');
  } else {
    document.documentElement.removeAttribute('data-theme');
  }
  localStorage.setItem('argus_theme', theme);
}

el.themeToggle.addEventListener('click', () => {
  const isLight = document.documentElement.getAttribute('data-theme') === 'light';
  applyTheme(isLight ? 'dark' : 'light');
});

/* ---------------------------- Tabs ---------------------------- */

const tabButtons = Array.from(document.querySelectorAll('.tab-btn'));

function activateTab(btn, { focus = false } = {}) {
  tabButtons.forEach((b) => {
    const isActive = b === btn;
    b.classList.toggle('active', isActive);
    b.setAttribute('aria-selected', String(isActive));
    b.tabIndex = isActive ? 0 : -1;
  });
  document.querySelectorAll('.tab-panel').forEach((p) => p.classList.remove('active'));
  document.getElementById(`panel-${btn.dataset.tab}`).classList.add('active');
  if (focus) btn.focus();
}

tabButtons.forEach((btn, index) => {
  btn.addEventListener('click', () => activateTab(btn));
  btn.addEventListener('keydown', (e) => {
    const keyToIndex = {
      ArrowRight: (index + 1) % tabButtons.length,
      ArrowLeft: (index - 1 + tabButtons.length) % tabButtons.length,
      Home: 0,
      End: tabButtons.length - 1,
    };
    if (keyToIndex[e.key] === undefined) return;
    e.preventDefault();
    activateTab(tabButtons[keyToIndex[e.key]], { focus: true });
  });
});

/* ---------------------------- Global keyboard shortcuts ---------------------------- */

document.addEventListener('keydown', (e) => {
  if (e.key !== '/') return;
  const tag = (document.activeElement && document.activeElement.tagName) || '';
  if (['INPUT', 'SELECT', 'TEXTAREA'].includes(tag)) return;
  e.preventDefault();
  activateTab(document.getElementById('tab-monitors'));
  el.monitorSearch.focus();
});

/* ---------------------------- API key visibility ---------------------------- */

el.toggleKeyVisibility.addEventListener('click', () => {
  el.apiKey.type = el.apiKey.type === 'password' ? 'text' : 'password';
});

/* ---------------------------- Monitor type field toggle ---------------------------- */

function syncKeywordFieldVisibility() {
  el.keywordField.style.display = el.monitorType.value === 'keyword' ? '' : 'none';
}
el.monitorType.addEventListener('change', syncKeywordFieldVisibility);
syncKeywordFieldVisibility();

/* ---------------------------- API helper ---------------------------- */

async function api(path, options = {}) {
  const key = localStorage.getItem('argus_api_key') || '';
  const res = await fetch(`/api${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', 'X-API-Key': key, ...(options.headers || {}) },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText);
    throw new Error(text || `Request failed (${res.status})`);
  }
  if (res.status === 204) return null;
  return res.json();
}

function toUTC(localDateTimeValue) {
  if (!localDateTimeValue) return null;
  return new Date(localDateTimeValue).toISOString();
}

function setButtonLoading(button, loading, labelWhenLoading) {
  if (!button) return;
  if (loading) {
    button.dataset.originalHtml = button.innerHTML;
    button.disabled = true;
    button.innerHTML = `<span class="btn-spinner"></span>${labelWhenLoading || 'Working...'}`;
  } else {
    button.disabled = false;
    if (button.dataset.originalHtml) button.innerHTML = button.dataset.originalHtml;
  }
}

/* ---------------------------- Rendering ---------------------------- */

function relativeTime(dateStr) {
  const date = new Date(dateStr);
  const diffMs = Date.now() - date.getTime();
  const diffSec = Math.round(diffMs / 1000);
  if (diffSec < 60) return `${diffSec}s ago`;
  const diffMin = Math.round(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.round(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.round(diffHr / 24);
  return `${diffDay}d ago`;
}

function emptyStateRow(colspan, iconSvg, title, subtitle) {
  return `<tr><td colspan="${colspan}"><div class="empty-state">${iconSvg}<strong>${title}</strong><span>${subtitle}</span></div></td></tr>`;
}

const emptyIcons = {
  monitor: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><rect x="3" y="4" width="18" height="14" rx="2"/><path d="M3 9h18M8 4v5"/></svg>',
  incident: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/><path d="M12 9v4M12 17h.01"/></svg>',
  page: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M8 12h8M8 8h8M8 16h5"/></svg>',
  ping: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M3 3v18h18"/><path d="m19 9-5 5-4-4-3 3"/></svg>',
};

function applyMonitorFilters(websites) {
  const search = (el.monitorSearch.value || '').toLowerCase().trim();
  const status = el.monitorFilter.value;
  return websites.filter((w) => {
    const matchesSearch = !search || w.url.toLowerCase().includes(search) || w.monitorType.toLowerCase().includes(search);
    const matchesStatus = status === 'all' || w.status === status;
    return matchesSearch && matchesStatus;
  });
}

/* ---------------------------- Monitor table sorting ---------------------------- */

const sortState = { key: null, dir: 1 };
const sortHeaders = Array.from(document.querySelectorAll('th.sortable'));

function sortWebsites(websites) {
  if (!sortState.key) return websites;
  const { key, dir } = sortState;
  return [...websites].sort((a, b) => {
    const av = a[key];
    const bv = b[key];
    if (typeof av === 'number' && typeof bv === 'number') return (av - bv) * dir;
    return String(av ?? '').localeCompare(String(bv ?? '')) * dir;
  });
}

sortHeaders.forEach((th) => {
  th.addEventListener('click', () => {
    const key = th.dataset.sort;
    if (sortState.key === key) {
      sortState.dir *= -1;
    } else {
      sortState.key = key;
      sortState.dir = 1;
    }
    sortHeaders.forEach((h) => h.classList.remove('sort-active', 'sort-desc'));
    th.classList.add('sort-active');
    if (sortState.dir === -1) th.classList.add('sort-desc');
    renderMonitors();
  });
});

function renderMonitors() {
  const filtered = sortWebsites(applyMonitorFilters(latestWebsites));
  if (!latestWebsites.length) {
    el.table.innerHTML = emptyStateRow(8, emptyIcons.monitor, 'No monitors yet', 'Add your first monitor using the form in the Overview tab.');
    return;
  }
  if (!filtered.length) {
    el.table.innerHTML = emptyStateRow(8, emptyIcons.monitor, 'No matches', 'Try a different search term or status filter.');
    return;
  }
  el.table.innerHTML = filtered
    .map(
      (w) => `
    <tr>
      <td class="mono">#${w.id}</td>
      <td class="url-cell" title="${escapeHtml(w.url)}">${escapeHtml(w.url)}</td>
      <td><span class="type-tag">${escapeHtml(w.monitorType)}</span></td>
      <td class="mono">${w.checkInterval ?? '-'}s</td>
      <td><span class="badge status-${w.status}"><span class="lamp"></span>${w.status}</span></td>
      <td class="mono">${w.lastStatusCode ?? '–'}</td>
      <td class="mono">${w.lastCheckedAt ? relativeTime(w.lastCheckedAt) : 'never'}</td>
      <td>
        <div class="row-actions">
          ${w.monitorType === 'heartbeat' ? `<button class="secondary sm" onclick="sendHeartbeat(${w.id})">Heartbeat</button>` : ''}
          <button class="danger sm" onclick="confirmDeleteMonitor(${w.id}, '${escapeHtml(w.url).replace(/'/g, "\\'")}')">Delete</button>
        </div>
      </td>
    </tr>
  `
    )
    .join('');
}

function incidentItemHtml(i) {
  return `
    <li class="list-item">
      <div class="list-item-main">
        <span class="list-item-title">Incident #${i.id} &middot; Website #${i.websiteId}</span>
        <span class="list-item-meta">Started ${relativeTime(i.startedAt)} (${new Date(i.startedAt).toLocaleString()})</span>
      </div>
      <span class="badge status-${(i.state || '').toLowerCase()}"><span class="lamp"></span>${i.state}</span>
    </li>`;
}

function renderIncidents(incidentResult) {
  const targets = [el.incidents, el.incidentsOverview];
  if (incidentResult.__error) {
    targets.forEach((t) => (t.innerHTML = `<li class="list-item"><span class="list-item-meta">Incident feed unavailable: ${escapeHtml(incidentResult.__error)}</span></li>`));
    el.statIncidents.textContent = '–';
    return;
  }
  if (!incidentResult.length) {
    targets.forEach((t) => (t.innerHTML = `<div class="empty-state">${emptyIcons.incident}<strong>No incidents</strong><span>All monitors are healthy.</span></div>`));
    el.statIncidents.textContent = '0';
    return;
  }
  const openCount = incidentResult.filter((i) => (i.state || '').toLowerCase() === 'open').length;
  el.statIncidents.textContent = String(openCount);
  el.incidents.innerHTML = incidentResult.map(incidentItemHtml).join('');
  el.incidentsOverview.innerHTML = incidentResult.slice(0, 5).map(incidentItemHtml).join('');
}

function renderStatusPages(statusPagesResult) {
  if (statusPagesResult.__error) {
    el.statusPages.innerHTML = `<li class="list-item"><span class="list-item-meta">Failed to load status pages: ${escapeHtml(statusPagesResult.__error)}</span></li>`;
    return;
  }
  if (!statusPagesResult.length) {
    el.statusPages.innerHTML = `<div class="empty-state">${emptyIcons.page}<strong>No status pages</strong><span>Create one from the Overview tab to share uptime publicly.</span></div>`;
    return;
  }
  el.statusPages.innerHTML = statusPagesResult
    .map((p) => {
      const path = `/api/public/status/${p.slug}`;
      return `
      <li class="list-item">
        <div class="list-item-main">
          <span class="list-item-title">${escapeHtml(p.title)}</span>
          <span class="list-item-meta">/${p.slug}</span>
        </div>
        <span class="link-chip" onclick="copyStatusLink('${escapeHtml(path)}')" title="Copy link">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          Copy link
        </span>
      </li>`;
    })
    .join('');
}

function copyStatusLink(path) {
  const fullUrl = `${window.location.origin}${path}`;
  navigator.clipboard
    ?.writeText(fullUrl)
    .then(() => showToast('Status page link copied to clipboard.', 'success'))
    .catch(() => showToast(fullUrl, 'info'));
}
window.copyStatusLink = copyStatusLink;

function renderChecks(checksResult) {
  if (checksResult.__error) {
    el.pingTable.innerHTML = emptyStateRow(6, emptyIcons.ping, 'Failed to load ping history', escapeHtml(checksResult.__error));
    return;
  }
  if (!checksResult.length) {
    el.pingTable.innerHTML = emptyStateRow(6, emptyIcons.ping, 'No ping history yet', 'Checks will appear here once monitors start running.');
    return;
  }
  el.pingTable.innerHTML = checksResult
    .map(
      (c) => `
    <tr>
      <td class="mono">#${c.websiteId}</td>
      <td><span class="badge status-${c.status}"><span class="lamp"></span>${c.status}</span></td>
      <td class="mono">${c.statusCode ?? '–'}</td>
      <td class="mono">${c.latencyMs} ms</td>
      <td class="mono">${new Date(c.checkedAt).toLocaleString()}</td>
      <td>${c.failureReason ? escapeHtml(c.failureReason) : '–'}</td>
    </tr>
  `
    )
    .join('');
}

function renderStats(websites) {
  if (!Array.isArray(websites)) {
    el.statTotal.textContent = '–';
    el.statUp.textContent = '–';
    el.statDown.textContent = '–';
    setStatusBanner('unknown', 0, 0, 0);
    return;
  }
  const total = websites.length;
  const up = websites.filter((w) => w.status === 'up').length;
  const down = websites.filter((w) => w.status === 'down').length;
  el.statTotal.textContent = String(total);
  el.statUp.textContent = String(up);
  el.statDown.textContent = String(down);
  setStatusBanner(down > 0 ? 'down' : total === 0 ? 'empty' : 'up', total, up, down);
}

function setStatusBanner(state, total, up, down) {
  el.refreshDot.classList.toggle('is-down', state === 'down');
  const messages = {
    unknown: ['var(--text-faint)', 'Scanning your infrastructure&hellip;'],
    empty: ['var(--text-faint)', 'No monitors yet — add one to start watching.'],
    up: ['var(--ok)', `All ${total} monitor${total === 1 ? '' : 's'} operational.`],
    down: ['var(--down)', `${down} of ${total} monitor${total === 1 ? '' : 's'} <strong>down</strong> right now.`],
  };
  const [color, text] = messages[state] || messages.unknown;
  el.statusBannerLamp.style.background = color;
  el.statusBannerText.innerHTML = text;
}

function showTableSkeleton(tbody, cols, rows = 4) {
  tbody.innerHTML = Array.from({ length: rows })
    .map(
      () => `<tr class="skeleton-row">${Array.from({ length: cols })
        .map(() => `<td><div class="skeleton"></div></td>`)
        .join('')}</tr>`
    )
    .join('');
}

/* ---------------------------- Data refresh ---------------------------- */

async function refresh({ silent = false } = {}) {
  if (!silent) {
    showTableSkeleton(el.table, 8);
    showTableSkeleton(el.pingTable, 6);
  }

  const [websites, incidents, statusPages, checks] = await Promise.all([
    api('/websites?limit=100&offset=0').catch((e) => ({ __error: e.message })),
    api('/incidents?limit=20&offset=0').catch((e) => ({ __error: e.message })),
    api('/status-pages?limit=50&offset=0').catch((e) => ({ __error: e.message })),
    api('/checks?limit=100').catch((e) => ({ __error: e.message })),
  ]);

  if (websites.__error) {
    el.table.innerHTML = emptyStateRow(8, emptyIcons.monitor, 'Failed to load monitors', escapeHtml(websites.__error));
    renderStats(null);
    latestWebsites = [];
  } else {
    latestWebsites = websites;
    renderMonitors();
    renderStats(websites);
  }
  renderIncidents(incidents);
  renderStatusPages(statusPages);
  renderChecks(checks);
  countdown = AUTO_REFRESH_SECONDS;
}

/* ---------------------------- Delete confirmation modal ---------------------------- */

let pendingDeleteId = null;
let modalTriggerEl = null;
const modalDialog = el.confirmModal.querySelector('.modal');

function confirmDeleteMonitor(id, url) {
  pendingDeleteId = id;
  modalTriggerEl = document.activeElement;
  el.confirmTitle.textContent = `Delete monitor #${id}?`;
  el.confirmBody.textContent = `This will permanently remove monitoring for "${url}". This action cannot be undone.`;
  el.confirmModal.classList.remove('hidden');
  el.confirmCancelBtn.focus();
}
window.confirmDeleteMonitor = confirmDeleteMonitor;

function closeConfirmModal() {
  pendingDeleteId = null;
  el.confirmModal.classList.add('hidden');
  if (modalTriggerEl && typeof modalTriggerEl.focus === 'function') modalTriggerEl.focus();
  modalTriggerEl = null;
}

el.confirmCancelBtn.addEventListener('click', closeConfirmModal);
el.confirmModal.addEventListener('click', (e) => {
  if (e.target === el.confirmModal) closeConfirmModal();
});
document.addEventListener('keydown', (e) => {
  if (el.confirmModal.classList.contains('hidden')) return;
  if (e.key === 'Escape') {
    closeConfirmModal();
    return;
  }
  if (e.key === 'Tab') {
    const focusable = modalDialog.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }
});

el.confirmOkBtn.addEventListener('click', async () => {
  if (pendingDeleteId == null) return;
  const id = pendingDeleteId;
  setButtonLoading(el.confirmOkBtn, true, 'Deleting...');
  try {
    await api(`/websites/${id}`, { method: 'DELETE' });
    showToast(`Monitor #${id} deleted.`, 'success');
    closeConfirmModal();
    refresh();
  } catch (e) {
    showToast(`Delete failed: ${e.message}`, 'error');
  } finally {
    setButtonLoading(el.confirmOkBtn, false);
  }
});

async function sendHeartbeat(id) {
  try {
    await api(`/websites/${id}/heartbeat`, { method: 'POST' });
    showToast(`Heartbeat accepted for #${id}.`, 'success');
    refresh();
  } catch (e) {
    showToast(`Heartbeat failed: ${e.message}`, 'error');
  }
}
window.sendHeartbeat = sendHeartbeat;

/* ---------------------------- Search / filter listeners ---------------------------- */

el.monitorSearch.addEventListener('input', renderMonitors);
el.monitorFilter.addEventListener('change', renderMonitors);

/* ---------------------------- Forms ---------------------------- */

el.form.addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = {
    url: document.getElementById('url').value.trim(),
    checkInterval: Number(document.getElementById('interval').value),
    monitorType: document.getElementById('monitorType').value,
  };
  const kw = document.getElementById('keyword').value.trim();
  if (kw) payload.expectedKeyword = kw;

  const btn = document.getElementById('monitorSubmitBtn');
  setButtonLoading(btn, true, 'Adding...');
  try {
    await api('/websites', { method: 'POST', body: JSON.stringify(payload) });
    el.form.reset();
    syncKeywordFieldVisibility();
    showToast('Monitor created successfully.', 'success');
    refresh();
  } catch (err) {
    showToast(`Create monitor failed: ${err.message}`, 'error');
  } finally {
    setButtonLoading(btn, false);
  }
});

el.channelForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = {
    name: document.getElementById('channelName').value.trim(),
    channelType: document.getElementById('channelType').value,
    target: document.getElementById('channelTarget').value.trim(),
  };
  const btn = document.getElementById('channelSubmitBtn');
  setButtonLoading(btn, true, 'Creating...');
  try {
    await api('/alert-channels', { method: 'POST', body: JSON.stringify(payload) });
    el.channelForm.reset();
    showToast('Alert channel created.', 'success');
  } catch (err) {
    showToast(`Create channel failed: ${err.message}`, 'error');
  } finally {
    setButtonLoading(btn, false);
  }
});

el.statusPageForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = {
    slug: document.getElementById('statusSlug').value.trim(),
    title: document.getElementById('statusTitle').value.trim(),
  };
  const btn = document.getElementById('statusPageSubmitBtn');
  setButtonLoading(btn, true, 'Creating...');
  try {
    await api('/status-pages', { method: 'POST', body: JSON.stringify(payload) });
    el.statusPageForm.reset();
    showToast('Status page created.', 'success');
    refresh();
  } catch (err) {
    showToast(`Create status page failed: ${err.message}`, 'error');
  } finally {
    setButtonLoading(btn, false);
  }
});

el.maintenanceForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const websiteIdRaw = document.getElementById('maintenanceWebsiteId').value;
  const payload = {
    websiteId: websiteIdRaw ? Number(websiteIdRaw) : null,
    startsAt: toUTC(document.getElementById('maintenanceStart').value),
    endsAt: toUTC(document.getElementById('maintenanceEnd').value),
    reason: document.getElementById('maintenanceReason').value.trim() || null,
  };
  const btn = document.getElementById('maintenanceSubmitBtn');
  setButtonLoading(btn, true, 'Creating...');
  try {
    await api('/maintenance-windows', { method: 'POST', body: JSON.stringify(payload) });
    el.maintenanceForm.reset();
    showToast('Maintenance window created.', 'success');
  } catch (err) {
    showToast(`Create maintenance failed: ${err.message}`, 'error');
  } finally {
    setButtonLoading(btn, false);
  }
});

el.saveKeyBtn.addEventListener('click', () => {
  localStorage.setItem('argus_api_key', el.apiKey.value.trim());
  showToast('API key saved in your browser.', 'success');
  refresh();
});

/* ---------------------------- Auto refresh countdown ---------------------------- */

el.refreshCountdown.textContent = `auto-refresh ${AUTO_REFRESH_SECONDS}s`;
setInterval(() => {
  countdown -= 1;
  if (countdown <= 0) {
    refresh({ silent: true });
    return;
  }
  el.refreshCountdown.textContent = `refreshing in ${countdown}s`;
}, 1000);

/* ---------------------------- Init ---------------------------- */

initTheme();
el.apiKey.value = localStorage.getItem('argus_api_key') || '';
el.refreshBtn.addEventListener('click', () => refresh());
refresh();

/* ==========================================================================
   Project-based API monitoring
   ========================================================================== */

const projectApp = document.getElementById('projectApp');
const PROJECT_TOKEN_KEY = 'argus_project_token';
const projectState = {
  project: null,
  route: null,
  routes: [],
  routeTotal: 0,
  incidents: [],
  metrics: [],
  metricRange: '24h',
  page: 0,
  pageSize: 50,
  filters: { search: '', method: '', status: '', enabled: '', deprecated: '', sortBy: 'path', sortDir: 'asc' },
  importJob: null,
};

async function apiProjects(path, options = {}) {
  const token = localStorage.getItem(PROJECT_TOKEN_KEY) || '';
  const headers = { Authorization: `Bearer ${token}`, ...(options.headers || {}) };
  if (!(options.body instanceof FormData)) headers['Content-Type'] = 'application/json';
  const response = await fetch(`/api${path}`, { ...options, headers });
  if (response.status === 401) {
    localStorage.removeItem(PROJECT_TOKEN_KEY);
    renderProjectAuth();
    throw new Error('Your project session expired. Please sign in again.');
  }
  if (!response.ok) {
    const payload = await response.json().catch(() => ({}));
    throw new Error(payload.error || `Request failed (${response.status})`);
  }
  if (response.status === 204) return null;
  return response.json();
}

function projectStatusBadge(status) {
  const value = status || 'unknown';
  return `<span class="badge route-${escapeHtml(value)}"><span class="lamp"></span>${escapeHtml(value)}</span>`;
}

function projectMetric(value, label, tone = '') {
  return `<div class="project-metric ${tone}"><strong>${value ?? '–'}</strong><span>${label}</span></div>`;
}

function projectError(message, retry = '') {
  return `<div class="project-error"><strong>Unable to load this view</strong><span>${escapeHtml(message)}</span>${retry ? `<button class="secondary sm" onclick="${retry}">Try again</button>` : ''}</div>`;
}

function projectSkeleton(cards = 3) {
  return `<div class="project-card-grid">${Array.from({ length: cards }, () => '<div class="project-card"><div class="skeleton wide"></div><div class="skeleton"></div><div class="skeleton"></div></div>').join('')}</div>`;
}

function projectNavigate(path) {
  window.location.hash = path;
}
window.projectNavigate = projectNavigate;

function activateProjectsTab() {
  const button = document.getElementById('tab-projects');
  if (button) activateTab(button);
}

function renderProjectAuth(mode = 'login') {
  activateProjectsTab();
  projectApp.innerHTML = `
    <section class="project-auth card">
      <div class="project-eyebrow">PROJECT MONITORING</div>
      <h2>${mode === 'register' ? 'Create your workspace account' : 'Sign in to your projects'}</h2>
      <p>Project sessions are separate from the legacy API key above.</p>
      <form id="projectAuthForm" class="stack">
        ${mode === 'register' ? '<div class="field"><label for="projectAuthName">Name</label><input id="projectAuthName" autocomplete="name" required /></div>' : ''}
        <div class="field"><label for="projectAuthEmail">Email</label><input id="projectAuthEmail" type="email" autocomplete="email" required /></div>
        <div class="field"><label for="projectAuthPassword">Password</label><input id="projectAuthPassword" type="password" minlength="8" autocomplete="${mode === 'register' ? 'new-password' : 'current-password'}" required /></div>
        <button id="projectAuthSubmit" type="submit">${mode === 'register' ? 'Create account' : 'Sign in'}</button>
      </form>
      <button class="ghost project-auth-switch" type="button" id="projectAuthSwitch">${mode === 'register' ? 'Already have an account? Sign in' : 'Need an account? Register'}</button>
    </section>`;
  document.getElementById('projectAuthSwitch').addEventListener('click', () => renderProjectAuth(mode === 'register' ? 'login' : 'register'));
  document.getElementById('projectAuthForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const button = document.getElementById('projectAuthSubmit');
    setButtonLoading(button, true, mode === 'register' ? 'Creating...' : 'Signing in...');
    try {
      const payload = {
        email: document.getElementById('projectAuthEmail').value.trim(),
        password: document.getElementById('projectAuthPassword').value,
        name: document.getElementById('projectAuthName')?.value.trim() || '',
      };
      const result = await fetch(`/api/auth/${mode}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const body = await result.json().catch(() => ({}));
      if (!result.ok) throw new Error(body.error || 'Authentication failed');
      localStorage.setItem(PROJECT_TOKEN_KEY, body.token);
      showToast(mode === 'register' ? 'Account created.' : 'Signed in.', 'success');
      projectNavigate('#/projects');
      await renderProjectRoute();
    } catch (error) {
      showToast(error.message, 'error');
    } finally {
      setButtonLoading(button, false);
    }
  });
}

async function renderProjectsList() {
  activateProjectsTab();
  projectApp.innerHTML = `
    <div class="project-view-header">
      <div><div class="project-eyebrow">API PORTFOLIO</div><h2>Projects</h2><p>Health, incidents, and performance across every monitored route.</p></div>
      <div class="project-header-actions"><button class="secondary" id="projectLogout">Sign out</button><button id="newProjectBtn">New project</button></div>
    </div>
    <section class="card project-toolbar">
      <div class="search-input"><input id="projectSearch" placeholder="Search projects..." aria-label="Search projects" /></div>
      <select id="projectStatusFilter" aria-label="Filter project status"><option value="">All statuses</option><option value="active">Active</option><option value="archived">Archived</option></select>
    </section>
    <div id="projectList">${projectSkeleton(3)}</div>`;
  document.getElementById('projectLogout').addEventListener('click', async () => {
    try { await apiProjects('/auth/logout', { method: 'POST' }); } catch (_) {}
    localStorage.removeItem(PROJECT_TOKEN_KEY);
    renderProjectAuth();
  });
  document.getElementById('newProjectBtn').addEventListener('click', () => renderProjectForm());
  let debounce;
  const reload = () => {
    clearTimeout(debounce);
    debounce = setTimeout(loadProjects, 180);
  };
  document.getElementById('projectSearch').addEventListener('input', reload);
  document.getElementById('projectStatusFilter').addEventListener('change', loadProjects);
  await loadProjects();
}

async function loadProjects() {
  const target = document.getElementById('projectList');
  if (!target) return;
  target.innerHTML = projectSkeleton(3);
  const search = document.getElementById('projectSearch')?.value.trim() || '';
  const status = document.getElementById('projectStatusFilter')?.value || '';
  try {
    const data = await apiProjects(`/projects?limit=100&search=${encodeURIComponent(search)}&status=${encodeURIComponent(status)}`);
    if (!data.items.length) {
      target.innerHTML = `<div class="empty-state project-empty">${emptyIcons.monitor}<strong>No projects found</strong><span>Create a project or adjust the current filters.</span><button onclick="renderProjectForm()">Create project</button></div>`;
      return;
    }
    target.innerHTML = `<div class="project-card-grid">${data.items.map((project) => `
      <article class="project-card" tabindex="0" onclick="projectNavigate('#/projects/${project.id}')" onkeydown="if(event.key==='Enter')projectNavigate('#/projects/${project.id}')">
        <div class="project-card-head"><div><span class="project-id">PROJECT / ${project.id}</span><h3>${escapeHtml(project.name)}</h3></div>${projectStatusBadge(project.status)}</div>
        <p>${escapeHtml(project.description || 'No description')}</p>
        <div class="project-health-strip"><span class="ok" style="--amount:${project.routesHealthy || 0}"></span><span class="warn" style="--amount:${project.routesDegraded || 0}"></span><span class="down" style="--amount:${project.routesFailing || 0}"></span></div>
        <div class="project-metrics-compact">
          <span><strong>${project.routesTotal || 0}</strong> routes</span>
          <span><strong>${Number(project.uptime24hPct || 0).toFixed(2)}%</strong> uptime</span>
          <span><strong>${project.avgLatency24hMs || 0}ms</strong> latency</span>
          <span><strong>${project.openIncidents || 0}</strong> incidents</span>
        </div>
        <div class="project-card-foot"><span>${project.lastCheckAt ? `checked ${relativeTime(project.lastCheckAt)}` : 'no checks yet'}</span><span class="project-card-actions">${project.viewerRole === 'owner' ? `<button class="ghost sm" onclick="event.stopPropagation();toggleProjectArchive(${project.id},'${project.status}')">${project.status === 'archived' ? 'Unarchive' : 'Archive'}</button><button class="danger sm" onclick="event.stopPropagation();deleteProject(${project.id})">Delete</button>` : ''}<span class="project-open">Open →</span></span></div>
      </article>`).join('')}</div>`;
  } catch (error) {
    target.innerHTML = projectError(error.message, 'loadProjects()');
  }
}

async function toggleProjectArchive(id, currentStatus) {
  try {
    await apiProjects(`/projects/${id}/${currentStatus === 'archived' ? 'unarchive' : 'archive'}`, { method: 'POST' });
    showToast(currentStatus === 'archived' ? 'Project restored.' : 'Project archived.', 'success');
    await loadProjects();
  } catch (error) { showToast(error.message, 'error'); }
}
window.toggleProjectArchive = toggleProjectArchive;
async function deleteProject(id) {
  if (!window.confirm('Delete this project and all of its route history?')) return;
  try {
    await apiProjects(`/projects/${id}`, { method: 'DELETE' });
    showToast('Project deleted.', 'success');
    await loadProjects();
  } catch (error) { showToast(error.message, 'error'); }
}
window.deleteProject = deleteProject;

function renderProjectForm(project = null) {
  projectApp.innerHTML = `
    <div class="project-view-header"><div><button class="ghost sm" onclick="projectNavigate('#/projects')">← Projects</button><h2>${project ? 'Edit project' : 'New project'}</h2></div></div>
    <section class="card project-form-card">
      <form id="projectForm" class="stack">
        <div class="field"><label>Name</label><input id="projectName" value="${escapeHtml(project?.name || '')}" required /></div>
        <div class="field"><label>Description</label><textarea id="projectDescription" rows="3">${escapeHtml(project?.description || '')}</textarea></div>
        <div class="field-row"><div class="field"><label>Default interval (seconds)</label><input id="projectInterval" type="number" min="10" value="${project?.defaultIntervalSeconds || 300}" /></div><div class="field"><label>Timeout (ms)</label><input id="projectTimeout" type="number" min="200" value="${project?.defaultTimeoutMs || 5000}" /></div></div>
        <div class="field-row"><div class="field"><label>Retries</label><input id="projectRetries" type="number" min="0" max="5" value="${project?.defaultRetries ?? 1}" /></div><div class="field"><label>Failures to incident</label><input id="projectFailureThreshold" type="number" min="1" value="${project?.failureThreshold || 3}" /></div><div class="field"><label>Successes to recover</label><input id="projectRecoveryThreshold" type="number" min="1" value="${project?.recoverySuccessThreshold || 1}" /></div></div>
        <div class="modal-actions"><button class="secondary" type="button" onclick="history.back()">Cancel</button><button id="saveProjectBtn" type="submit">Save project</button></div>
      </form>
    </section>`;
  document.getElementById('projectForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const button = document.getElementById('saveProjectBtn');
    setButtonLoading(button, true, 'Saving...');
    const payload = {
      name: document.getElementById('projectName').value.trim(),
      description: document.getElementById('projectDescription').value.trim(),
      defaultIntervalSeconds: Number(document.getElementById('projectInterval').value),
      defaultTimeoutMs: Number(document.getElementById('projectTimeout').value),
      defaultRetries: Number(document.getElementById('projectRetries').value),
      failureThreshold: Number(document.getElementById('projectFailureThreshold').value),
      recoverySuccessThreshold: Number(document.getElementById('projectRecoveryThreshold').value),
    };
    try {
      const saved = await apiProjects(project ? `/projects/${project.id}` : '/projects', { method: project ? 'PUT' : 'POST', body: JSON.stringify(payload) });
      showToast(project ? 'Project updated.' : 'Project created.', 'success');
      projectNavigate(`#/projects/${saved.id}`);
    } catch (error) {
      showToast(error.message, 'error');
    } finally {
      setButtonLoading(button, false);
    }
  });
}
window.renderProjectForm = renderProjectForm;

async function renderProjectDashboard(projectId) {
  activateProjectsTab();
  projectApp.innerHTML = projectSkeleton(4);
  try {
    const [project, routeData, incidents, metrics] = await Promise.all([
      apiProjects(`/projects/${projectId}`),
      apiProjects(`/projects/${projectId}/routes?limit=${projectState.pageSize}&offset=0&sortBy=path&sortDir=asc`),
      apiProjects(`/projects/${projectId}/incidents?limit=20`),
      apiProjects(`/projects/${projectId}/metrics/timeseries?range=${projectState.metricRange}`),
    ]);
    projectState.project = project;
    projectState.routes = routeData.items;
    projectState.routeTotal = routeData.total;
    projectState.incidents = incidents;
    projectState.metrics = metrics.items;
    projectState.page = 0;
    renderProjectDashboardShell();
  } catch (error) {
    projectApp.innerHTML = projectError(error.message, `renderProjectDashboard(${projectId})`);
  }
}

function renderProjectDashboardShell() {
  const project = projectState.project;
  const canEdit = project.viewerRole === 'owner' || project.viewerRole === 'editor';
  projectApp.innerHTML = `
    <div class="project-view-header">
      <div><button class="ghost sm" onclick="projectNavigate('#/projects')">← Projects</button><div class="project-eyebrow">PROJECT / ${project.id}</div><h2>${escapeHtml(project.name)}</h2><p>${escapeHtml(project.description || 'API route monitoring workspace')}</p></div>
      <div class="project-header-actions">${canEdit ? `<button class="secondary" onclick="renderProjectForm(projectState.project)">Edit</button><button class="secondary" onclick="projectNavigate('#/projects/${project.id}/import')">Import spec</button><button onclick="renderRouteForm()">Add route</button>` : ''}</div>
    </div>
    <section class="project-metric-grid">
      ${projectMetric(project.routesTotal || 0, 'Total routes')}
      ${projectMetric(project.routesHealthy || 0, 'Healthy', 'metric-ok')}
      ${projectMetric(project.routesDegraded || 0, 'Degraded', 'metric-warn')}
      ${projectMetric(project.routesFailing || 0, 'Failing', 'metric-down')}
      ${projectMetric(project.routesDisabled || 0, 'Disabled')}
      ${projectMetric(`${Number(project.uptime24hPct || 0).toFixed(2)}%`, '24h uptime')}
      ${projectMetric(`${project.avgLatency24hMs || 0} ms`, '24h latency')}
      ${projectMetric(project.openIncidents || 0, 'Open incidents', project.openIncidents ? 'metric-down' : '')}
    </section>
    <div class="project-dashboard-grid">
      <section class="card project-chart-card"><div class="card-header"><div><h2>Uptime signal</h2><span class="card-subtitle">Grouped route checks; uptime and normalized latency</span></div><select id="projectMetricRange" style="width:auto"><option value="1h">1 hour</option><option value="24h">24 hours</option><option value="7d">7 days</option></select></div><canvas id="projectSignalChart" height="110"></canvas><div class="chart-legend"><span class="uptime">Uptime</span><span class="latency">Latency signal</span></div></section>
      <section class="card project-incident-card"><div class="card-header"><h2>Recent incidents</h2></div><div id="projectIncidentList"></div></section>
    </div>
    <section class="card project-routes-card">
      <div class="card-header"><div><h2>API routes</h2><span class="card-subtitle">${projectState.routeTotal.toLocaleString()} operations</span></div></div>
      <div class="route-filters">
        <input id="routeSearch" placeholder="Search path, summary, operation ID..." value="${escapeHtml(projectState.filters.search)}" />
        <select id="routeMethod"><option value="">All methods</option>${['GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS'].map((m) => `<option ${projectState.filters.method === m ? 'selected' : ''}>${m}</option>`).join('')}</select>
        <select id="routeStatus"><option value="">All health states</option>${['healthy','degraded','failing','disabled','unknown'].map((s) => `<option value="${s}" ${projectState.filters.status === s ? 'selected' : ''}>${s}</option>`).join('')}</select>
        <select id="routeEnabled"><option value="">Enabled + disabled</option><option value="true">Enabled</option><option value="false">Disabled</option></select>
        <select id="routeSort"><option value="path">Path</option><option value="status">Health</option><option value="latency">Latency</option><option value="uptime">Uptime</option><option value="updated">Updated</option></select>
      </div>
      <div class="bulk-bar hidden" id="routeBulkBar"><span id="routeBulkCount">0 selected</span>${canEdit ? '<button class="secondary sm" onclick="bulkSetRoutes(false)">Disable</button><button class="secondary sm" onclick="bulkSetRoutes(true)">Enable</button><button class="danger sm" onclick="bulkDeleteRoutes()">Delete</button>' : ''}</div>
      <div class="table-wrap"><table><thead><tr><th><input type="checkbox" id="routeSelectAll" aria-label="Select all visible routes" /></th><th>Method</th><th>Path</th><th>Health</th><th>Uptime</th><th>Latency</th><th>Last check</th><th>Actions</th></tr></thead><tbody id="projectRouteTable"></tbody></table></div>
      <div class="route-pagination"><span id="routePageSummary"></span><div><button class="secondary sm" id="routePrev">Previous</button><button class="secondary sm" id="routeNext">Next</button></div></div>
    </section>`;
  renderRouteRows();
  renderProjectIncidents();
  drawProjectSignal();
  bindRouteControls();
  document.getElementById('projectMetricRange').value = projectState.metricRange;
  document.getElementById('projectMetricRange').addEventListener('change', loadProjectMetrics);
}

function renderProjectIncidents() {
  const target = document.getElementById('projectIncidentList');
  if (!projectState.incidents.length) {
    target.innerHTML = `<div class="empty-state compact">${emptyIcons.incident}<strong>No route incidents</strong><span>Threshold breaches will appear here.</span></div>`;
    return;
  }
  target.innerHTML = projectState.incidents.slice(0, 8).map((incident) => `<div class="route-incident"><div><strong>Route #${incident.routeId}</strong><span>${escapeHtml(incident.lastFailureReason || 'Check failure')}</span></div>${projectStatusBadge(incident.state)}<time>${relativeTime(incident.startedAt)}</time></div>`).join('');
}

function drawProjectSignal() {
  const canvas = document.getElementById('projectSignalChart');
  if (!canvas) return;
  const rect = canvas.getBoundingClientRect();
  canvas.width = Math.max(600, rect.width * devicePixelRatio);
  canvas.height = 110 * devicePixelRatio;
  const ctx = canvas.getContext('2d');
  ctx.scale(devicePixelRatio, devicePixelRatio);
  const width = canvas.width / devicePixelRatio;
  const height = canvas.height / devicePixelRatio;
  const points = projectState.metrics;
  ctx.strokeStyle = getComputedStyle(document.documentElement).getPropertyValue('--line');
  for (let y = 15; y < height; y += 24) { ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(width, y); ctx.stroke(); }
  const styles = getComputedStyle(document.documentElement);
  if (!points.length) {
    ctx.fillStyle = styles.getPropertyValue('--text-faint');
    ctx.font = '11px IBM Plex Mono';
    ctx.fillText('No checks in this range yet', 12, height / 2);
    return;
  }
  const maxLatency = Math.max(1, ...points.map((point) => point.avgLatencyMs));
  const lines = [
    { values: points.map((point) => point.uptimePct), color: styles.getPropertyValue('--signal') },
    { values: points.map((point) => 100 - (point.avgLatencyMs / maxLatency) * 100), color: styles.getPropertyValue('--warn') },
  ];
  lines.forEach(({ values, color }) => {
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.beginPath();
    values.forEach((value, index) => {
      const x = values.length === 1 ? width / 2 : (index / (values.length - 1)) * width;
      const y = height - 12 - (value / 100) * 72;
      if (index === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
    });
    ctx.stroke();
  });
}

async function loadProjectMetrics() {
  projectState.metricRange = document.getElementById('projectMetricRange').value;
  try {
    const data = await apiProjects(`/projects/${projectState.project.id}/metrics/timeseries?range=${projectState.metricRange}`);
    projectState.metrics = data.items;
    drawProjectSignal();
  } catch (error) { showToast(error.message, 'error'); }
}

function renderRouteRows() {
  const tbody = document.getElementById('projectRouteTable');
  if (!projectState.routes.length) {
    tbody.innerHTML = emptyStateRow(8, emptyIcons.monitor, 'No routes match', 'Add routes manually or import an OpenAPI specification.');
  } else {
    tbody.innerHTML = projectState.routes.map((route) => `<tr>
      <td><input class="route-select" type="checkbox" value="${route.id}" aria-label="Select ${escapeHtml(route.method + ' ' + route.path)}" /></td>
      <td><span class="method method-${route.method.toLowerCase()}">${route.method}</span></td>
      <td><button class="route-link" onclick="projectNavigate('#/projects/${route.projectId}/routes/${route.id}')"><strong>${escapeHtml(route.path)}</strong><span>${escapeHtml(route.summary || route.operationId || '')}</span></button></td>
      <td>${projectStatusBadge(route.status)}</td><td class="mono">${Number(route.uptime24hPct || 0).toFixed(2)}%</td><td class="mono">${route.lastLatencyMs || 0} ms</td>
      <td class="mono">${route.lastCheckedAt ? relativeTime(route.lastCheckedAt) : 'never'}</td>
      <td><div class="row-actions"><button class="secondary sm" onclick="projectNavigate('#/projects/${route.projectId}/routes/${route.id}')">Details</button>${projectState.project.viewerRole !== 'viewer' ? `<button class="secondary sm" onclick="setRouteEnabled(${route.id}, ${!route.enabled})">${route.enabled ? 'Disable' : 'Enable'}</button>` : ''}</div></td>
    </tr>`).join('');
  }
  const start = projectState.page * projectState.pageSize;
  document.getElementById('routePageSummary').textContent = projectState.routeTotal ? `${start + 1}–${Math.min(start + projectState.pageSize, projectState.routeTotal)} of ${projectState.routeTotal}` : '0 routes';
  document.getElementById('routePrev').disabled = projectState.page === 0;
  document.getElementById('routeNext').disabled = start + projectState.pageSize >= projectState.routeTotal;
  document.querySelectorAll('.route-select').forEach((box) => box.addEventListener('change', updateBulkBar));
}

function bindRouteControls() {
  let timer;
  const refreshFilters = () => { clearTimeout(timer); timer = setTimeout(() => loadProjectRoutes(0), 180); };
  ['routeSearch', 'routeMethod', 'routeStatus', 'routeEnabled', 'routeSort'].forEach((id) => document.getElementById(id).addEventListener(id === 'routeSearch' ? 'input' : 'change', refreshFilters));
  document.getElementById('routeSelectAll').addEventListener('change', (event) => {
    document.querySelectorAll('.route-select').forEach((box) => { box.checked = event.target.checked; });
    updateBulkBar();
  });
  document.getElementById('routePrev').addEventListener('click', () => loadProjectRoutes(projectState.page - 1));
  document.getElementById('routeNext').addEventListener('click', () => loadProjectRoutes(projectState.page + 1));
}

async function loadProjectRoutes(page = 0) {
  projectState.filters = {
    search: document.getElementById('routeSearch').value.trim(),
    method: document.getElementById('routeMethod').value,
    status: document.getElementById('routeStatus').value,
    enabled: document.getElementById('routeEnabled').value,
    deprecated: '',
    sortBy: document.getElementById('routeSort').value,
    sortDir: 'asc',
  };
  const query = new URLSearchParams({ limit: projectState.pageSize, offset: Math.max(0, page) * projectState.pageSize, ...projectState.filters });
  try {
    const result = await apiProjects(`/projects/${projectState.project.id}/routes?${query}`);
    projectState.routes = result.items;
    projectState.routeTotal = result.total;
    projectState.page = Math.max(0, page);
    renderRouteRows();
  } catch (error) {
    showToast(error.message, 'error');
  }
}

function selectedRouteIds() {
  return Array.from(document.querySelectorAll('.route-select:checked')).map((box) => Number(box.value));
}
function updateBulkBar() {
  const ids = selectedRouteIds();
  document.getElementById('routeBulkBar').classList.toggle('hidden', !ids.length);
  document.getElementById('routeBulkCount').textContent = `${ids.length} selected`;
}
async function setRouteEnabled(id, enabled) {
  try {
    await apiProjects(`/projects/${projectState.project.id}/routes/${id}/${enabled ? 'enable' : 'disable'}`, { method: 'POST' });
    showToast(`Route ${enabled ? 'enabled' : 'disabled'}.`, 'success');
    if (document.getElementById('projectRouteTable')) await loadProjectRoutes(projectState.page);
    else await renderRouteDetail(projectState.project.id, id);
  } catch (error) { showToast(error.message, 'error'); }
}
window.setRouteEnabled = setRouteEnabled;
async function bulkSetRoutes(enabled) {
  const ids = selectedRouteIds();
  try {
    await Promise.all(ids.map((id) => apiProjects(`/projects/${projectState.project.id}/routes/${id}/${enabled ? 'enable' : 'disable'}`, { method: 'POST' })));
    showToast(`${ids.length} routes ${enabled ? 'enabled' : 'disabled'}.`, 'success');
    await loadProjectRoutes(projectState.page);
  } catch (error) { showToast(error.message, 'error'); }
}
window.bulkSetRoutes = bulkSetRoutes;
async function bulkDeleteRoutes() {
  const ids = selectedRouteIds();
  if (!ids.length || !window.confirm(`Delete ${ids.length} selected routes?`)) return;
  try {
    await apiProjects(`/projects/${projectState.project.id}/routes/bulk-delete`, { method: 'POST', body: JSON.stringify({ ids }) });
    showToast(`${ids.length} routes deleted.`, 'success');
    await loadProjectRoutes(projectState.page);
  } catch (error) { showToast(error.message, 'error'); }
}
window.bulkDeleteRoutes = bulkDeleteRoutes;

function renderRouteForm(route = null) {
  const project = projectState.project;
  projectApp.innerHTML = `<div class="project-view-header"><div><button class="ghost sm" onclick="history.back()">← Project</button><h2>${route ? 'Edit route' : 'Add route'}</h2></div></div>
    <section class="card project-form-card"><form id="routeForm" class="stack">
      <div class="field-row"><div class="field"><label>Method</label><select id="routeFormMethod" ${route ? 'disabled' : ''}>${['GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS'].map((m) => `<option ${route?.method === m ? 'selected' : ''}>${m}</option>`).join('')}</select></div><div class="field"><label>Path</label><input id="routeFormPath" value="${escapeHtml(route?.path || '/')}" ${route ? 'disabled' : ''} required /></div></div>
      <div class="field"><label>Base URL</label><input id="routeFormBase" type="url" value="${escapeHtml(route?.baseUrl || '')}" placeholder="https://api.example.com" required /></div>
      <div class="field"><label>Name / summary</label><input id="routeFormName" value="${escapeHtml(route?.name || route?.summary || '')}" /></div>
      <div class="field-row"><div class="field"><label>Interval (seconds)</label><input id="routeFormInterval" type="number" min="10" value="${route?.monitorIntervalSeconds || project.defaultIntervalSeconds}" /></div><div class="field"><label>Timeout (ms)</label><input id="routeFormTimeout" type="number" min="200" value="${route?.timeoutMs || project.defaultTimeoutMs}" /></div><div class="field"><label>Retries</label><input id="routeFormRetries" type="number" min="0" max="5" value="${route?.retries ?? project.defaultRetries}" /></div></div>
      <div class="field-row"><div class="field"><label>Expected status</label><input id="routeFormStatus" value="${escapeHtml(route?.expectedStatusRange || '200-399')}" /></div><div class="field"><label>Failure threshold</label><input id="routeFormFailures" type="number" min="1" value="${route?.failureThreshold || project.failureThreshold}" /></div><div class="field"><label>Recovery successes</label><input id="routeFormRecovery" type="number" min="1" value="${route?.recoverySuccesses || project.recoverySuccessThreshold}" /></div></div>
      <div class="field"><label>Headers (JSON; secrets are redacted on read)</label><textarea id="routeFormHeaders" rows="3" placeholder='{"Authorization":"Bearer ..."}'></textarea></div>
      <div class="modal-actions"><button class="secondary" type="button" onclick="history.back()">Cancel</button><button id="routeSaveBtn" type="submit">Save route</button></div>
    </form></section>`;
  document.getElementById('routeForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const button = document.getElementById('routeSaveBtn');
    setButtonLoading(button, true, 'Saving...');
    const payload = {
      method: document.getElementById('routeFormMethod').value, path: document.getElementById('routeFormPath').value.trim(),
      baseUrl: document.getElementById('routeFormBase').value.trim(), name: document.getElementById('routeFormName').value.trim(),
      monitorIntervalSeconds: Number(document.getElementById('routeFormInterval').value), timeoutMs: Number(document.getElementById('routeFormTimeout').value),
      retries: Number(document.getElementById('routeFormRetries').value), expectedStatusRange: document.getElementById('routeFormStatus').value.trim(),
      failureThreshold: Number(document.getElementById('routeFormFailures').value), recoverySuccesses: Number(document.getElementById('routeFormRecovery').value),
      headers: document.getElementById('routeFormHeaders').value.trim(), enabled: route?.enabled ?? true,
    };
    try {
      const saved = await apiProjects(`/projects/${project.id}/routes${route ? `/${route.id}` : ''}`, { method: route ? 'PUT' : 'POST', body: JSON.stringify(payload) });
      showToast('Route saved.', 'success');
      projectNavigate(`#/projects/${project.id}/routes/${saved.id}`);
    } catch (error) { showToast(error.message, 'error'); } finally { setButtonLoading(button, false); }
  });
}
window.renderRouteForm = renderRouteForm;

async function renderRouteDetail(projectId, routeId) {
  activateProjectsTab();
  projectApp.innerHTML = projectSkeleton(2);
  try {
    const [project, route, checks, incidents] = await Promise.all([
      apiProjects(`/projects/${projectId}`), apiProjects(`/projects/${projectId}/routes/${routeId}`),
      apiProjects(`/projects/${projectId}/routes/${routeId}/checks?limit=50`),
      apiProjects(`/projects/${projectId}/incidents?routeId=${routeId}&limit=20`),
    ]);
    projectState.project = project;
    projectState.route = route;
    const canEdit = project.viewerRole !== 'viewer';
    projectApp.innerHTML = `<div class="project-view-header"><div><button class="ghost sm" onclick="projectNavigate('#/projects/${projectId}')">← ${escapeHtml(project.name)}</button><div class="route-title-line"><span class="method method-${route.method.toLowerCase()}">${route.method}</span><h2>${escapeHtml(route.path)}</h2>${projectStatusBadge(route.status)}</div><p>${escapeHtml(route.summary || route.description || 'Monitored API operation')}</p></div><div class="project-header-actions">${canEdit ? `<button class="secondary" onclick="setRouteEnabled(${route.id}, ${!route.enabled})">${route.enabled ? 'Disable' : 'Enable'}</button><button onclick="renderRouteForm(projectState.route)">Edit route</button>` : ''}</div></div>
      <section class="project-metric-grid route-detail-metrics">${projectMetric(`${Number(route.uptime24hPct || 0).toFixed(2)}%`, '24h uptime')}${projectMetric(`${route.avgLatency24hMs || 0} ms`, 'Average latency')}${projectMetric(route.checks24h || 0, 'Checks')}${projectMetric(route.failures24h || 0, 'Failures', route.failures24h ? 'metric-down' : '')}${projectMetric(route.lastStatusCode || '–', 'Last status')}${projectMetric(route.consecutiveFailures || 0, 'Failure streak')}</section>
      <div class="project-dashboard-grid"><section class="card route-config"><div class="card-header"><h2>Configuration</h2></div>${routeConfigRows(route)}</section><section class="card"><div class="card-header"><h2>Incidents</h2></div>${incidents.length ? incidents.map((i) => `<div class="route-incident"><div><strong>${escapeHtml(i.lastFailureReason || 'Failure')}</strong><span>${new Date(i.startedAt).toLocaleString()}</span></div>${projectStatusBadge(i.state)}</div>`).join('') : '<div class="empty-state compact"><strong>No incidents</strong><span>This route has not crossed its failure threshold.</span></div>'}</section></div>
      <section class="card"><div class="card-header"><h2>Check & status-code history</h2></div><div class="table-wrap"><table><thead><tr><th>Result</th><th>Status code</th><th>Latency</th><th>Attempt</th><th>Checked</th><th>Error</th></tr></thead><tbody>${checks.length ? checks.map((check) => `<tr><td>${projectStatusBadge(check.status === 'up' ? 'healthy' : 'failing')}</td><td class="mono">${check.statusCode || '–'}</td><td class="mono">${check.latencyMs} ms</td><td class="mono">${check.attempt}</td><td class="mono">${new Date(check.checkedAt).toLocaleString()}</td><td>${escapeHtml(check.failureReason || '–')}</td></tr>`).join('') : emptyStateRow(6, emptyIcons.ping, 'No checks yet', 'The background worker will record checks here.')}</tbody></table></div></section>`;
  } catch (error) { projectApp.innerHTML = projectError(error.message, `renderRouteDetail(${projectId},${routeId})`); }
}

function routeConfigRows(route) {
  const rows = [
    ['Target', `${route.baseUrl}${route.path}`], ['Interval', `${route.monitorIntervalSeconds}s`], ['Timeout', `${route.timeoutMs}ms`],
    ['Retries', route.retries], ['Expected status', route.expectedStatusRange], ['Failure / recovery', `${route.failureThreshold} / ${route.recoverySuccesses}`],
    ['Tags', (route.tags || []).join(', ') || '–'], ['Deprecated', route.deprecated ? 'Yes' : 'No'], ['Source', route.source],
    ['Parameters', route.parameters || '–'], ['Request body', route.requestBody || '–'], ['Responses', route.responses || '–'], ['Security', route.security || '–'],
  ];
  return `<dl class="config-list">${rows.map(([label, value]) => `<div><dt>${label}</dt><dd>${escapeHtml(value)}</dd></div>`).join('')}</dl>`;
}

async function renderImportWizard(projectId) {
  activateProjectsTab();
  try {
    projectState.project = await apiProjects(`/projects/${projectId}`);
  } catch (error) { projectApp.innerHTML = projectError(error.message); return; }
  projectState.importJob = null;
  projectApp.innerHTML = `<div class="project-view-header"><div><button class="ghost sm" onclick="projectNavigate('#/projects/${projectId}')">← ${escapeHtml(projectState.project.name)}</button><div class="project-eyebrow">IMPORT / STEP 1 OF 3</div><h2>Import OpenAPI specification</h2><p>Upload JSON/YAML or paste the full OpenAPI 3.x / Swagger 2.0 document.</p></div></div>
    <section class="card import-card"><form id="importForm" class="stack"><div class="import-drop"><input id="importFile" type="file" accept=".json,.yaml,.yml,application/json,application/yaml" /><strong>Choose a specification file</strong><span>Maximum 10 MB. The server never fetches remote references.</span></div><div class="import-or">OR PASTE</div><div class="field"><textarea id="importSpec" rows="14" placeholder="openapi: 3.0.3&#10;info: ..."></textarea></div><div class="field"><label>Base URL override (optional)</label><input id="importBaseURL" type="url" placeholder="https://api.example.com" /></div><div class="modal-actions"><button class="secondary" type="button" onclick="history.back()">Cancel</button><button id="importValidateBtn" type="submit">Validate & preview</button></div></form></section>`;
  document.getElementById('importForm').addEventListener('submit', validateProjectImport);
}

async function validateProjectImport(event) {
  event.preventDefault();
  const button = document.getElementById('importValidateBtn');
  setButtonLoading(button, true, 'Validating...');
  try {
    const file = document.getElementById('importFile').files[0];
    const pasted = document.getElementById('importSpec').value;
    const baseUrlOverride = document.getElementById('importBaseURL').value.trim();
    let options;
    if (file) {
      const form = new FormData();
      form.append('file', file);
      if (baseUrlOverride) form.append('baseUrlOverride', baseUrlOverride);
      options = { method: 'POST', body: form };
    } else {
      if (!pasted.trim()) throw new Error('Choose a file or paste a specification.');
      options = { method: 'POST', body: JSON.stringify({ spec: pasted, baseUrlOverride }) };
    }
    projectState.importJob = await apiProjects(`/projects/${projectState.project.id}/imports/validate`, options);
    renderImportPreview();
  } catch (error) { showToast(error.message, 'error'); } finally { setButtonLoading(button, false); }
}

function renderImportPreview() {
  const job = projectState.importJob;
  const counts = job.items.reduce((acc, item) => { acc[item.action] = (acc[item.action] || 0) + 1; return acc; }, {});
  projectApp.innerHTML = `<div class="project-view-header"><div><button class="ghost sm" onclick="renderImportWizard(${job.projectId})">← Source</button><div class="project-eyebrow">IMPORT / STEP 2 OF 3</div><h2>Review ${job.totalParsed.toLocaleString()} operations</h2><p>Select exactly which changes to apply. Removed routes remain untouched unless selected.</p></div><button id="commitImportBtn">Commit selected</button></div>
    <section class="project-metric-grid import-metrics">${projectMetric(counts.create || 0, 'New')}${projectMetric(counts.update || 0, 'Changed', 'metric-warn')}${projectMetric(counts.skip || 0, 'Unchanged')}${projectMetric(counts.remove || 0, 'Removed', 'metric-down')}</section>
    <section class="card"><div class="route-filters"><button class="secondary sm" onclick="selectImportItems('all')">Select defaults</button><button class="secondary sm" onclick="selectImportItems('none')">Select none</button><select id="importActionFilter"><option value="">All changes</option><option value="create">New</option><option value="update">Changed</option><option value="skip">Unchanged / duplicate</option><option value="remove">Removed</option></select><span id="importSelectedCount"></span></div><div class="table-wrap import-preview-table"><table><thead><tr><th></th><th>Action</th><th>Method</th><th>Path</th><th>Conflict / warning</th></tr></thead><tbody id="importItems"></tbody></table></div></section>`;
  const renderItems = () => {
    const filter = document.getElementById('importActionFilter').value;
    const items = job.items.map((item, index) => ({ item, index })).filter(({ item }) => !filter || item.action === filter);
    document.getElementById('importItems').innerHTML = items.map(({ item, index }) => `<tr><td><input class="import-select" data-index="${index}" type="checkbox" ${item.selected ? 'checked' : ''} ${item.action === 'skip' ? 'disabled' : ''} /></td><td><span class="import-action action-${item.action}">${item.action}</span></td><td><span class="method method-${item.method.toLowerCase()}">${escapeHtml(item.method)}</span></td><td class="mono">${escapeHtml(item.path)}</td><td>${escapeHtml(item.validationWarning || (item.conflict === 'none' ? '–' : item.conflict.replaceAll('_', ' ')))}</td></tr>`).join('');
    document.querySelectorAll('.import-select').forEach((box) => box.addEventListener('change', () => { job.items[Number(box.dataset.index)].selected = box.checked; updateImportSelectedCount(); }));
    updateImportSelectedCount();
  };
  document.getElementById('importActionFilter').addEventListener('change', renderItems);
  document.getElementById('commitImportBtn').addEventListener('click', commitProjectImport);
  projectState.renderImportItems = renderItems;
  renderItems();
}

function updateImportSelectedCount() {
  const count = projectState.importJob.items.filter((item) => item.selected).length;
  document.getElementById('importSelectedCount').textContent = `${count} selected`;
  document.getElementById('commitImportBtn').disabled = count === 0;
}
function selectImportItems(mode) {
  projectState.importJob.items.forEach((item) => { item.selected = mode === 'all' ? item.action === 'create' || item.action === 'update' : false; });
  projectState.renderImportItems();
}
window.selectImportItems = selectImportItems;
async function commitProjectImport() {
  const button = document.getElementById('commitImportBtn');
  setButtonLoading(button, true, 'Importing...');
  const job = projectState.importJob;
  try {
    const result = await apiProjects(`/projects/${job.projectId}/imports/${job.id}/commit`, { method: 'POST', body: JSON.stringify({ selections: job.items.map((item) => ({ key: item.key, selected: item.selected, action: item.action })) }) });
    projectApp.innerHTML = `<div class="import-result card"><div class="project-eyebrow">IMPORT / COMPLETE</div><div class="result-check">✓</div><h2>Import completed</h2><p>Every selected operation was processed. Monitoring settings on existing routes were preserved.</p><section class="project-metric-grid">${projectMetric(result.createdRoutes, 'Created', 'metric-ok')}${projectMetric(result.updatedRoutes, 'Updated')}${projectMetric(result.skippedRoutes, 'Skipped')}${projectMetric(result.removedRoutes, 'Disabled', 'metric-warn')}</section><button onclick="projectNavigate('#/projects/${job.projectId}')">Open project dashboard</button></div>`;
    showToast('Specification import completed.', 'success');
  } catch (error) { showToast(error.message, 'error'); setButtonLoading(button, false); }
}

async function renderProjectRoute() {
  const hash = window.location.hash || '';
  if (!hash.startsWith('#/projects')) return;
  if (!localStorage.getItem(PROJECT_TOKEN_KEY)) { renderProjectAuth(); return; }
  const parts = hash.replace(/^#\//, '').split('/').filter(Boolean);
  const projectId = Number(parts[1]);
  if (!projectId) { await renderProjectsList(); return; }
  if (parts[2] === 'routes' && Number(parts[3])) { await renderRouteDetail(projectId, Number(parts[3])); return; }
  if (parts[2] === 'import') { await renderImportWizard(projectId); return; }
  await renderProjectDashboard(projectId);
}

window.renderProjectRoute = renderProjectRoute;
window.addEventListener('hashchange', renderProjectRoute);
document.getElementById('tab-projects').addEventListener('click', () => {
  if (!window.location.hash.startsWith('#/projects')) projectNavigate('#/projects');
  else renderProjectRoute();
});
if (window.location.hash.startsWith('#/projects')) renderProjectRoute();
