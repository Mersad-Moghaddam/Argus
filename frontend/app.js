const el = {
  table: document.getElementById('monitorTable'),
  incidents: document.getElementById('incidentList'),
  form: document.getElementById('monitorForm'),
  refreshBtn: document.getElementById('refreshBtn'),
  message: document.getElementById('message'),
  apiKey: document.getElementById('apiKey'),
  saveKeyBtn: document.getElementById('saveKeyBtn'),
  channelForm: document.getElementById('channelForm'),
  maintenanceForm: document.getElementById('maintenanceForm'),
  statusPageForm: document.getElementById('statusPageForm'),
  statusPages: document.getElementById('statusPages')
};

function showMessage(text, type = 'ok') {
  el.message.className = type;
  el.message.textContent = text;
  if (text) setTimeout(() => { if (el.message.textContent === text) el.message.textContent = ''; }, 3500);
}

async function api(path, options = {}) {
  const key = localStorage.getItem('argus_api_key') || '';
  const res = await fetch(`/api${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', 'X-API-Key': key, ...(options.headers || {}) }
  });
  if (!res.ok) throw new Error(await res.text());
  if (res.status === 204) return null;
  return res.json();
}

function toUTC(localDateTimeValue) {
  if (!localDateTimeValue) return null;
  return new Date(localDateTimeValue).toISOString();
}

function renderMonitors(websites) {
  if (!Array.isArray(websites) || websites.length === 0) {
    el.table.innerHTML = '<tr><td colspan="6">No monitors yet.</td></tr>';
    return;
  }
  el.table.innerHTML = websites.map(w => `
    <tr>
      <td>${w.id}</td>
      <td>${w.url}</td>
      <td>${w.monitorType}</td>
      <td class="status-${w.status}">${w.status}</td>
      <td>${w.lastStatusCode ?? '-'}</td>
      <td>
        <button onclick="deleteMonitor(${w.id})">Delete</button>
        ${w.monitorType === 'heartbeat' ? `<button onclick="sendHeartbeat(${w.id})">Heartbeat</button>` : ''}
      </td>
    </tr>
  `).join('');
}

function renderIncidents(incidentResult) {
  if (incidentResult.__error) {
    el.incidents.innerHTML = `<li>Incident feed unavailable: ${incidentResult.__error}</li>`;
    return;
  }
  if (!incidentResult.length) {
    el.incidents.innerHTML = '<li>No incidents.</li>';
    return;
  }
  el.incidents.innerHTML = incidentResult.map(i => `<li>#${i.id} website:${i.websiteId} • ${i.state} • started ${new Date(i.startedAt).toLocaleString()}</li>`).join('');
}

function renderStatusPages(statusPagesResult) {
  if (statusPagesResult.__error) {
    el.statusPages.innerHTML = `<li>Failed to load status pages: ${statusPagesResult.__error}</li>`;
    return;
  }
  if (!statusPagesResult.length) {
    el.statusPages.innerHTML = '<li>No status pages created.</li>';
    return;
  }
  el.statusPages.innerHTML = statusPagesResult.map(p => `<li><strong>${p.title}</strong> — /api/public/status/${p.slug}</li>`).join('');
}

async function refresh() {
  const websites = await api('/websites?limit=100&offset=0').catch((e) => ({ __error: e.message }));
  const incidents = await api('/incidents?limit=20&offset=0').catch((e) => ({ __error: e.message }));
  const statusPages = await api('/status-pages?limit=50&offset=0').catch((e) => ({ __error: e.message }));

  if (websites.__error) {
    el.table.innerHTML = `<tr><td colspan="6">Failed to load monitors: ${websites.__error}</td></tr>`;
  } else {
    renderMonitors(websites);
  }
  renderIncidents(incidents);
  renderStatusPages(statusPages);
}

async function deleteMonitor(id) {
  try {
    await api(`/websites/${id}`, { method: 'DELETE' });
    showMessage(`Monitor ${id} deleted.`);
    refresh();
  } catch (e) {
    showMessage(`Delete failed: ${e.message}`, 'error');
  }
}

async function sendHeartbeat(id) {
  try {
    await api(`/websites/${id}/heartbeat`, { method: 'POST' });
    showMessage(`Heartbeat accepted for #${id}.`);
    refresh();
  } catch (e) {
    showMessage(`Heartbeat failed: ${e.message}`, 'error');
  }
}
window.deleteMonitor = deleteMonitor;
window.sendHeartbeat = sendHeartbeat;

el.form.addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = {
    url: document.getElementById('url').value.trim(),
    checkInterval: Number(document.getElementById('interval').value),
    monitorType: document.getElementById('monitorType').value,
  };
  const kw = document.getElementById('keyword').value.trim();
  if (kw) payload.expectedKeyword = kw;

  try {
    await api('/websites', { method: 'POST', body: JSON.stringify(payload) });
    el.form.reset();
    showMessage('Monitor created.');
    refresh();
  } catch (err) {
    showMessage(`Create monitor failed: ${err.message}`, 'error');
  }
});

el.channelForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = {
    name: document.getElementById('channelName').value.trim(),
    channelType: document.getElementById('channelType').value,
    target: document.getElementById('channelTarget').value.trim(),
  };
  try {
    await api('/alert-channels', { method: 'POST', body: JSON.stringify(payload) });
    el.channelForm.reset();
    showMessage('Alert channel created.');
  } catch (err) {
    showMessage(`Create channel failed: ${err.message}`, 'error');
  }
});

el.statusPageForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = {
    slug: document.getElementById('statusSlug').value.trim(),
    title: document.getElementById('statusTitle').value.trim(),
  };
  try {
    await api('/status-pages', { method: 'POST', body: JSON.stringify(payload) });
    el.statusPageForm.reset();
    showMessage('Status page created.');
    refresh();
  } catch (err) {
    showMessage(`Create status page failed: ${err.message}`, 'error');
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
  try {
    await api('/maintenance-windows', { method: 'POST', body: JSON.stringify(payload) });
    el.maintenanceForm.reset();
    showMessage('Maintenance window created.');
  } catch (err) {
    showMessage(`Create maintenance failed: ${err.message}`, 'error');
  }
});

el.saveKeyBtn.addEventListener('click', () => {
  localStorage.setItem('argus_api_key', el.apiKey.value.trim());
  showMessage('API key saved in browser localStorage.');
  refresh();
});

el.apiKey.value = localStorage.getItem('argus_api_key') || '';
el.refreshBtn.addEventListener('click', refresh);
refresh();
