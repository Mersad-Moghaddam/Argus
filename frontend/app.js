/* ==========================================================================
   Argus dashboard client
   ========================================================================== */

const el = {
  table: document.getElementById('monitorTable'),
  incidents: document.getElementById('incidentList'),
  incidentsOverview: document.getElementById('incidentListOverview'),
  form: document.getElementById('monitorForm'),
  refreshBtn: document.getElementById('refreshBtn'),
  refreshIndicator: document.getElementById('refreshIndicator'),
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
  // Legacy API credentials are intentionally memory-only. Browser storage is
  // readable by injected scripts and must never hold a management secret.
  const key = el.apiKey.value.trim();
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
	if (el.refreshIndicator) {
		el.refreshIndicator.classList.add('is-refreshing');
		el.refreshIndicator.setAttribute('aria-busy', 'true');
	}
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
	if (el.refreshIndicator) {
		el.refreshIndicator.classList.remove('is-refreshing');
		el.refreshIndicator.removeAttribute('aria-busy');
	}
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
  showToast('API key is available for this tab only.', 'success');
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
el.refreshBtn.addEventListener('click', () => refresh());
refresh();
