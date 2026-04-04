const table = document.getElementById('monitorTable');
const incidents = document.getElementById('incidentList');
const form = document.getElementById('monitorForm');
const refreshBtn = document.getElementById('refreshBtn');

async function api(path, options={}) {
  const key = localStorage.getItem('argus_api_key') || '';
  const res = await fetch(`/api${path}`, { ...options, headers: { 'Content-Type':'application/json', 'X-API-Key': key, ...(options.headers||{}) } });
  if (!res.ok) throw new Error(await res.text());
  if (res.status === 204) return null;
  return res.json();
}

async function refresh() {
  const websiteResult = await api('/websites?limit=50&offset=0').catch((e) => ({ __error: e.message }));
  const incidentResult = await api('/incidents?limit=20&offset=0').catch((e) => ({ __error: e.message }));

  if (websiteResult.__error) {
    table.innerHTML = `<tr><td colspan="5">Failed to load monitors: ${websiteResult.__error}</td></tr>`;
  } else {
    table.innerHTML = websiteResult.map(w => `<tr><td>${w.id}</td><td>${w.url}</td><td>${w.monitorType}</td><td class="status-${w.status}">${w.status}</td><td>${w.lastStatusCode ?? '-'}</td></tr>`).join('');
  }

  if (incidentResult.__error) {
    incidents.innerHTML = `<li>Incident feed unavailable: ${incidentResult.__error}</li>`;
  } else {
    incidents.innerHTML = incidentResult.map(i => `<li>#${i.id} website:${i.websiteId} • ${i.state}</li>`).join('');
  }
}

form.addEventListener('submit', async (e) => {
  e.preventDefault();
  const payload = {
    url: document.getElementById('url').value,
    checkInterval: Number(document.getElementById('interval').value),
    monitorType: document.getElementById('monitorType').value,
  };
  const kw = document.getElementById('keyword').value.trim();
  if (kw) payload.expectedKeyword = kw;
  await api('/websites', { method:'POST', body: JSON.stringify(payload) });
  form.reset();
  refresh();
});

refreshBtn.addEventListener('click', refresh);
refresh();
