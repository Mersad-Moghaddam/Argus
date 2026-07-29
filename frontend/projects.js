/* ==========================================================================
   Argus — API Projects client
   Project-based API route monitoring. Loaded after app.js and reuses its
   helpers (showToast, escapeHtml, relativeTime, setButtonLoading,
   showTableSkeleton, activateTab) plus the shared design tokens in styles.css.

   All monitoring happens in the backend worker. The only timer here is a
   display refresh for whichever view is currently open.
   ========================================================================== */

(() => {
  'use strict';

  const VIEW_REFRESH_SECONDS = 20;
  const ROUTE_PAGE_SIZE = 25;
  const PREVIEW_PAGE_SIZE = 100;

  const pel = {
    tab: document.getElementById('tab-projects'),
    panel: document.getElementById('panel-projects'),
    globalAuthPanel: document.getElementById('globalAuthPanel'),
    globalAccountPanel: document.getElementById('globalAccountPanel'),
    guestActions: document.getElementById('accountGuestActions'),
    signedInActions: document.getElementById('accountSignedInActions'),
    globalUserLabel: document.getElementById('globalUserLabel'),
    globalSignOut: document.getElementById('globalSignOut'),
    accountEmail: document.getElementById('accountEmail'),
    accountPasswordForm: document.getElementById('accountPasswordForm'),
    accountCurrentPassword: document.getElementById('accountCurrentPassword'),
    accountNewPassword: document.getElementById('accountNewPassword'),
    accountPasswordError: document.getElementById('accountPasswordError'),
    accountPasswordSubmit: document.getElementById('accountPasswordSubmit'),
    accountSessions: document.getElementById('accountSessions'),
    accountRevokeOthers: document.getElementById('accountRevokeOthers'),
    authGate: document.getElementById('projAuthGate'),
    authForm: document.getElementById('projAuthForm'),
    authTitle: document.getElementById('projAuthTitle'),
    authIntro: document.getElementById('projAuthIntro'),
    authNameField: document.getElementById('projAuthNameField'),
    authName: document.getElementById('projAuthName'),
    authEmail: document.getElementById('projAuthEmail'),
    authPassword: document.getElementById('projAuthPassword'),
    authError: document.getElementById('projAuthError'),
    authSubmit: document.getElementById('projAuthSubmit'),
    authSwitch: document.getElementById('projAuthSwitch'),
    authSwitchPrompt: document.getElementById('projAuthSwitchPrompt'),
    forgotPassword: document.getElementById('projForgotPassword'),
    recoveryGate: document.getElementById('projRecoveryGate'),
    recoveryRequestForm: document.getElementById('projRecoveryRequestForm'),
    recoveryEmail: document.getElementById('projRecoveryEmail'),
    recoveryRequestError: document.getElementById('projRecoveryRequestError'),
    recoveryRequestSubmit: document.getElementById('projRecoveryRequestSubmit'),
    recoveryCompleteForm: document.getElementById('projRecoveryCompleteForm'),
    recoveryToken: document.getElementById('projRecoveryToken'),
    recoveryNewPassword: document.getElementById('projRecoveryNewPassword'),
    recoveryCompleteError: document.getElementById('projRecoveryCompleteError'),
    recoveryCompleteSubmit: document.getElementById('projRecoveryCompleteSubmit'),
    shell: document.getElementById('projShell'),
    crumbs: document.getElementById('projCrumbs'),
    userLabel: document.getElementById('projUserLabel'),
    signOut: document.getElementById('projSignOut'),
    viewList: document.getElementById('projViewList'),
    viewProject: document.getElementById('projViewProject'),
    viewRoute: document.getElementById('projViewRoute'),
    viewImport: document.getElementById('projViewImport'),

    projectModal: document.getElementById('projProjectModal'),
    projectForm: document.getElementById('projProjectForm'),
    projectModalTitle: document.getElementById('projProjectModalTitle'),
    projectFormError: document.getElementById('projProjectFormError'),
    projectCancel: document.getElementById('projProjectCancel'),
    projectBack: document.getElementById('projProjectBack'),
    projectSubmit: document.getElementById('projProjectSubmit'),
    projectSteps: document.getElementById('projProjectSteps'),
projectAdvanced: document.getElementById('projProjectAdvanced'),
projectReview: document.getElementById('projOnboardingReview'),
projectVerification: document.getElementById('projOnboardingVerification'),
projectSLO: document.getElementById('projOnboardingSLO'),
projectNotification: document.getElementById('projOnboardingNotification'),
projectNext: document.getElementById('projOnboardingNext'),

    environmentModal: document.getElementById('projEnvironmentModal'),
    environmentForm: document.getElementById('projEnvironmentForm'),
    environmentName: document.getElementById('projEnvironmentName'),
    environmentBaseURL: document.getElementById('projEnvironmentBaseUrl'),
    environmentFormError: document.getElementById('projEnvironmentFormError'),
    environmentCancel: document.getElementById('projEnvironmentCancel'),
    environmentSubmit: document.getElementById('projEnvironmentSubmit'),

    telemetryCredentialModal: document.getElementById('projTelemetryCredentialModal'),
    telemetryCredentialForm: document.getElementById('projTelemetryCredentialForm'),
    telemetryCredentialName: document.getElementById('projTelemetryCredentialName'),
    telemetryCredentialEnvironment: document.getElementById('projTelemetryCredentialEnvironment'),
    telemetryCredentialExpiry: document.getElementById('projTelemetryCredentialExpiry'),
    telemetryCredentialFormError: document.getElementById('projTelemetryCredentialFormError'),
    telemetryCredentialCancel: document.getElementById('projTelemetryCredentialCancel'),
    telemetryCredentialSubmit: document.getElementById('projTelemetryCredentialSubmit'),
    telemetrySecretModal: document.getElementById('projTelemetrySecretModal'),
    telemetrySecretValue: document.getElementById('projTelemetrySecretValue'),
    telemetrySecretCopy: document.getElementById('projTelemetrySecretCopy'),
    telemetrySecretClose: document.getElementById('projTelemetrySecretClose'),

    heartbeatModal: document.getElementById('projHeartbeatModal'),
    heartbeatForm: document.getElementById('projHeartbeatForm'),
    heartbeatName: document.getElementById('projHeartbeatName'),
    heartbeatEnvironment: document.getElementById('projHeartbeatEnvironment'),
    heartbeatInterval: document.getElementById('projHeartbeatInterval'),
    heartbeatGrace: document.getElementById('projHeartbeatGrace'),
    heartbeatFormError: document.getElementById('projHeartbeatFormError'),
    heartbeatCancel: document.getElementById('projHeartbeatCancel'),
    heartbeatSubmit: document.getElementById('projHeartbeatSubmit'),
    heartbeatSecretModal: document.getElementById('projHeartbeatSecretModal'),
    heartbeatSecretValue: document.getElementById('projHeartbeatSecretValue'),
    heartbeatSecretCopy: document.getElementById('projHeartbeatSecretCopy'),
    heartbeatSecretClose: document.getElementById('projHeartbeatSecretClose'),

    agentModal: document.getElementById('projAgentModal'),
    agentForm: document.getElementById('projAgentForm'),
    agentName: document.getElementById('projAgentName'),
    agentEnvironment: document.getElementById('projAgentEnvironment'),
    agentInterval: document.getElementById('projAgentInterval'),
    agentFormError: document.getElementById('projAgentFormError'),
    agentCancel: document.getElementById('projAgentCancel'),
    agentSubmit: document.getElementById('projAgentSubmit'),
    agentSecretModal: document.getElementById('projAgentSecretModal'),
    agentSecretValue: document.getElementById('projAgentSecretValue'),
    agentSecretCopy: document.getElementById('projAgentSecretCopy'),
    agentSecretClose: document.getElementById('projAgentSecretClose'),
    agentAssignmentModal: document.getElementById('projAgentAssignmentModal'),
    agentAssignmentForm: document.getElementById('projAgentAssignmentForm'),
    agentAssignmentName: document.getElementById('projAgentAssignmentName'),
    agentAssignmentEnvironment: document.getElementById('projAgentAssignmentEnvironment'),
    agentAssignmentRoute: document.getElementById('projAgentAssignmentRoute'),
    agentAssignmentMethod: document.getElementById('projAgentAssignmentMethod'),
    agentAssignmentTarget: document.getElementById('projAgentAssignmentTarget'),
    agentAssignmentInterval: document.getElementById('projAgentAssignmentInterval'),
    agentAssignmentTimeout: document.getElementById('projAgentAssignmentTimeout'),
    agentAssignmentFormError: document.getElementById('projAgentAssignmentFormError'),
    agentAssignmentCancel: document.getElementById('projAgentAssignmentCancel'),
    agentAssignmentSubmit: document.getElementById('projAgentAssignmentSubmit'),

    sloModal: document.getElementById('projSLOModal'),
    sloForm: document.getElementById('projSLOForm'),
    sloName: document.getElementById('projSLOName'),
    sloKind: document.getElementById('projSLOKind'),
    sloTarget: document.getElementById('projSLOTarget'),
    sloLatencyField: document.getElementById('projSLOLatencyField'),
    sloLatencyThreshold: document.getElementById('projSLOLatencyThreshold'),
    sloWindowDays: document.getElementById('projSLOWindowDays'),
    sloMinEvents: document.getElementById('projSLOMinEvents'),
    sloFormError: document.getElementById('projSLOFormError'),
    sloCancel: document.getElementById('projSLOCancel'),
    sloSubmit: document.getElementById('projSLOSubmit'),

    telemetryMappingModal: document.getElementById('projTelemetryMappingModal'),
    telemetryMappingForm: document.getElementById('projTelemetryMappingForm'),
    telemetryMappingEnvironment: document.getElementById('projTelemetryMappingEnvironment'),
    telemetryMappingService: document.getElementById('projTelemetryMappingService'),
    telemetryMappingDeployment: document.getElementById('projTelemetryMappingDeployment'),
    telemetryMappingRoute: document.getElementById('projTelemetryMappingRoute'),
    telemetryMappingFormError: document.getElementById('projTelemetryMappingFormError'),
    telemetryMappingCancel: document.getElementById('projTelemetryMappingCancel'),
    telemetryMappingSubmit: document.getElementById('projTelemetryMappingSubmit'),

    routeModal: document.getElementById('projRouteModal'),
    routeForm: document.getElementById('projRouteForm'),
    routeModalTitle: document.getElementById('projRouteModalTitle'),
    routeFormError: document.getElementById('projRouteFormError'),
    routeCancel: document.getElementById('projRouteCancel'),
    routeSubmit: document.getElementById('projRouteSubmit'),

    bulkModal: document.getElementById('projBulkModal'),
    bulkForm: document.getElementById('projBulkForm'),
    bulkFormError: document.getElementById('projBulkFormError'),
    bulkResult: document.getElementById('projBulkResult'),
    bulkCancel: document.getElementById('projBulkCancel'),
    bulkSubmit: document.getElementById('projBulkSubmit'),

    confirmModal: document.getElementById('projConfirmModal'),
    confirmTitle: document.getElementById('projConfirmTitle'),
    confirmBody: document.getElementById('projConfirmBody'),
    confirmCancel: document.getElementById('projConfirmCancel'),
    confirmOk: document.getElementById('projConfirmOk'),
  };

  /* ------------------------------------------------------------------ state */

  const state = {
    route: { name: 'list' },
    authMode: 'login',
    authReturnTo: null,
    sessionUser: null,
    sessionResolved: false,
    // Every local session transition advances this version. Responses from a
    // request started against an older session must never overwrite the newer
    // state (for example, a delayed /identity/profile 401 after a successful
    // sign-in).
    sessionVersion: 0,
    account: { sessions: null, loading: false },
    onboarding: { step: 1, source: 'telemetry', createdProject: null },
    // Projects list view.
    list: { search: '', status: '', offset: 0, limit: 24, loading: false, data: null, total: 0 },
    // Project detail view.
    project: {
      id: null,
      project: null,
      range: '24h',
      series: null,
      incidents: [],
	  projectIncidents: [],
      environments: [],
      telemetryIngress: [],
      telemetryMappings: [],
      slos: [],
	  heartbeats: [],
agents: [],
agentAssignments: [],
      routes: [],
      routesTotal: 0,
      filters: { search: '', method: '', status: '', tag: '', enabled: '', deprecated: '' },
      sortBy: 'path',
      sortDir: 'asc',
      offset: 0,
      limit: ROUTE_PAGE_SIZE,
      selected: new Set(),
      loading: false,
    },
    // Route detail view.
    routeDetail: { projectId: null, routeId: null, route: null, checks: [], incidents: [], series: null, range: '24h', loading: false },
    // Import wizard.
    importer: { projectId: null, step: 1, job: null, selections: new Map(), conflictFilter: 'all', previewPage: 0, busy: false, result: null },
  };

  let refreshTimer = null;
  let modalReturnFocus = null;
  let confirmAction = null;
  let sessionRestorePromise = null;
  const ONBOARDING_DRAFT_KEY = 'argus_project_onboarding_draft_v1';
  const projectModals = [pel.projectModal, pel.environmentModal, pel.telemetryCredentialModal, pel.telemetrySecretModal, pel.heartbeatModal, pel.heartbeatSecretModal, pel.agentModal, pel.agentSecretModal, pel.agentAssignmentModal, pel.sloModal, pel.telemetryMappingModal, pel.routeModal, pel.bulkModal, pel.confirmModal];

  /* ------------------------------------------------------- auth + api client */

  function getUser() {
    return state.sessionUser || null;
  }

  function setSession(user) {
    state.sessionUser = user || null;
    state.sessionResolved = true;
    state.sessionVersion += 1;
    syncAccountChrome();
  }

  function clearSession() {
    state.sessionUser = null;
    state.sessionResolved = true;
    state.sessionVersion += 1;
    syncAccountChrome();
  }

  function beginSessionTransition() {
    // Invalidate any restore/request response that started before this
    // register/login completed, without briefly rendering an old account.
    state.sessionUser = null;
    state.sessionResolved = false;
    state.sessionVersion += 1;
    syncAccountChrome();
    return state.sessionVersion;
  }

  function syncAccountChrome() {
    const user = getUser();
    pel.guestActions.classList.toggle('hidden', Boolean(user));
    pel.signedInActions.classList.toggle('hidden', !user);
    pel.globalUserLabel.textContent = user ? (user.email || user.name || 'Signed in') : '';
  }

  function csrfToken() {
    const prefix = 'argus_csrf=';
    const cookie = document.cookie.split(';').map((part) => part.trim()).find((part) => part.startsWith(prefix));
    return cookie ? cookie.slice(prefix.length) : '';
  }

  /** Raised when the server rejects our bearer token; the caller stops quietly. */
  class SessionExpired extends Error {
    constructor() {
      super('Your session has expired. Please sign in again.');
      this.name = 'SessionExpired';
    }
  }

  /** Cookie-authenticated project API client. The session identifier is
   * HttpOnly and never appears in JavaScript storage or request headers. */
  async function apiProjects(path, options = {}) {
    const sessionVersion = state.sessionVersion;
    const headers = { ...(options.headers || {}) };
    if (options.body && !(options.body instanceof FormData) && !headers['Content-Type']) {
      headers['Content-Type'] = 'application/json';
    }
    if (!['GET', 'HEAD', 'OPTIONS'].includes((options.method || 'GET').toUpperCase())) {
      headers['X-CSRF-Token'] = csrfToken();
    }

    let res;
    try {
	  res = await fetch(path, { ...options, headers, credentials: 'same-origin' });
    } catch (networkErr) {
      throw new Error(`Network error: ${networkErr.message}`);
    }

    if (res.status === 401) {
      // An earlier request can fail after the user has signed in again. Do not
      // let that stale response sign out the new session.
      if (sessionVersion === state.sessionVersion) {
        clearSession();
        navigate(authHash('login', window.location.hash));
      }
      throw new SessionExpired();
    }
    if (res.status === 204) return null;

    const raw = await res.text();
    let payload = null;
    if (raw) {
      try {
        payload = JSON.parse(raw);
      } catch {
        payload = null;
      }
    }
    if (!res.ok) {
      const message = (payload && payload.error) || raw || `Request failed (${res.status})`;
      const err = new Error(message);
      err.status = res.status;
      throw err;
    }
    return payload;
  }

  /** reportError shows a failure without spamming toasts for expired sessions. */
  function reportError(prefix, err) {
    if (err instanceof SessionExpired) return;
    showToast(`${prefix}: ${err.message}`, 'error');
  }

  /* ---------------------------------------------------------------- helpers */

  function qs(params) {
    const search = new URLSearchParams();
    Object.entries(params).forEach(([k, v]) => {
      if (v !== '' && v !== null && v !== undefined) search.set(k, String(v));
    });
    const str = search.toString();
    return str ? `?${str}` : '';
  }

  function num(value, fallback = '–') {
    return value === null || value === undefined ? fallback : String(value);
  }

  function pct(value) {
    if (value === null || value === undefined) return '–';
    return `${Number(value).toFixed(2)}%`;
  }

  function ms(value) {
    if (!value) return '0 ms';
    return `${Math.round(value)} ms`;
  }

  function healthBadge(status) {
    const label = status || 'unknown';
    return `<span class="badge route-${escapeHtml(label)}"><span class="lamp"></span>${escapeHtml(label)}</span>`;
  }

  function methodChip(method) {
    return `<span class="method-chip m-${escapeHtml(String(method).toLowerCase())}">${escapeHtml(method)}</span>`;
  }

  const CONFLICT_LABELS = {
    none: 'new',
    changed: 'changed',
    duplicate_in_spec: 'duplicate',
    removed_from_spec: 'removed',
  };

  function conflictBadge(item) {
    const key = item.conflict || 'none';
    const label = key === 'none' && item.action === 'skip' ? 'unchanged' : CONFLICT_LABELS[key] || key;
    return `<span class="conflict-badge c-${escapeHtml(label)}">${escapeHtml(label)}</span>`;
  }

  const ICON = {
    project:
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/></svg>',
    route:
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M4 6h10a4 4 0 0 1 0 8H8a4 4 0 0 0 0 8h12"/><circle cx="4" cy="6" r="1.6"/></svg>',
    incident:
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0Z"/><path d="M12 9v4M12 17h.01"/></svg>',
    spec:
      '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><path d="M14 3v5h5"/><path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z"/><path d="M8 13h8M8 17h5"/></svg>',
  };

  function emptyPanel(icon, title, subtitle, actionHtml = '') {
    return `<div class="empty-state">${icon}<strong>${escapeHtml(title)}</strong><span>${escapeHtml(subtitle)}</span>${actionHtml}</div>`;
  }

  function errorPanel(title, message, retryAction) {
    const retry = retryAction ? `<button class="secondary sm" data-action="${escapeHtml(retryAction)}">Try again</button>` : '';
    return `<div class="empty-state is-error">${ICON.incident}<strong>${escapeHtml(title)}</strong><span>${escapeHtml(message)}</span>${retry}</div>`;
  }

  function skeletonCards(count = 4) {
    return `<div class="proj-grid">${Array.from({ length: count })
      .map(() => '<div class="card proj-card is-skeleton"><div class="skeleton" style="height:1.1rem;width:55%"></div><div class="skeleton" style="height:.7rem;width:35%"></div><div class="skeleton" style="height:3.2rem"></div></div>')
      .join('')}</div>`;
  }

  /* ----------------------------------------------------------------- router */

  function parseHash() {
    const hash = window.location.hash.replace(/^#/, '');
    const [path, query = ''] = hash.split('?', 2);
    const parts = path.split('/').filter(Boolean);
    if (parts.length === 1 && parts[0] === 'account') return { name: 'account' };
    if (parts.length === 1 && parts[0] === 'recover') return { name: 'recovery' };
    if (parts.length === 1 && (parts[0] === 'login' || parts[0] === 'register')) {
      return {
        name: 'auth',
        mode: parts[0] === 'register' ? 'register' : 'login',
        returnTo: validatedReturnTo(new URLSearchParams(query).get('returnTo')),
      };
    }
    if (parts[0] !== 'projects') return null;
    if (parts.length === 1) return { name: 'list' };
    const projectId = Number(parts[1]);
    if (!Number.isInteger(projectId) || projectId <= 0) return { name: 'list' };
    if (parts.length === 2) return { name: 'project', projectId };
    if (parts[2] === 'import') return { name: 'import', projectId };
    if (parts[2] === 'routes' && parts[3]) {
      const routeId = Number(parts[3]);
      if (Number.isInteger(routeId) && routeId > 0) return { name: 'route', projectId, routeId };
    }
    return { name: 'project', projectId };
  }

  function validatedReturnTo(value) {
    if (typeof value !== 'string' || !value.startsWith('#/projects')) return null;
    const parsed = value.slice(1).split('?', 1)[0].split('/').filter(Boolean);
    if (parsed[0] !== 'projects') return null;
    if (parsed.length > 4 || (parsed[1] && (!/^\d+$/.test(parsed[1]) || Number(parsed[1]) <= 0))) return null;
    if (parsed[2] && parsed[2] !== 'routes' && parsed[2] !== 'import') return null;
    if (parsed[2] === 'routes' && (!parsed[3] || !/^\d+$/.test(parsed[3]) || Number(parsed[3]) <= 0)) return null;
    return value;
  }

  function authHash(mode, returnTo = null) {
    const safeReturnTo = validatedReturnTo(returnTo);
    return `#/${mode}${safeReturnTo ? `?returnTo=${encodeURIComponent(safeReturnTo)}` : ''}`;
  }

  function navigate(hash) {
    if (window.location.hash === hash) {
      handleRoute();
      return;
    }
    window.location.hash = hash;
  }

  function showView(name) {
    const views = {
      list: pel.viewList,
      project: pel.viewProject,
      route: pel.viewRoute,
      import: pel.viewImport,
    };
    Object.entries(views).forEach(([key, node]) => node.classList.toggle('hidden', key !== name));
  }

  function renderCrumbs() {
    const crumbs = [{ label: 'Projects', hash: '#/projects' }];
    const { name } = state.route;
    if (name === 'project' || name === 'route' || name === 'import') {
      const project = state.project.id === state.route.projectId ? state.project.project : null;
      const label = project ? project.name : `Project #${state.route.projectId}`;
      crumbs.push({ label, hash: `#/projects/${state.route.projectId}` });
    }
    if (name === 'route') {
      const route = state.routeDetail.route;
      crumbs.push({ label: route ? `${route.method} ${route.path}` : 'Route', hash: null });
    }
    if (name === 'import') crumbs.push({ label: 'Import specification', hash: null });

    pel.crumbs.innerHTML = crumbs
      .map((crumb, i) => {
        const isLast = i === crumbs.length - 1;
        const body = isLast || !crumb.hash
          ? `<span class="proj-crumb is-current">${escapeHtml(crumb.label)}</span>`
          : `<a class="proj-crumb" href="${escapeHtml(crumb.hash)}">${escapeHtml(crumb.label)}</a>`;
        return i === 0 ? body : `<span class="proj-crumb-sep" aria-hidden="true">/</span>${body}`;
      })
      .join('');
  }

  function handleRoute() {
    const parsed = parseHash();
    if (!parsed) return; // Not our hash; leave the other tabs alone.

    if (parsed.name === 'auth') {
      renderAuthGate(parsed.mode, parsed.returnTo);
      if (getUser()) navigate(parsed.returnTo || '#/projects');
      else if (!state.sessionResolved) restoreSession();
      return;
    }

    if (parsed.name === 'recovery') {
      if (getUser()) {
        navigate('#/account');
        return;
      }
      renderRecovery();
      if (!state.sessionResolved) restoreSession();
      return;
    }

    if (parsed.name === 'account') {
      if (!getUser()) {
        if (!state.sessionResolved) restoreSession();
        navigate(authHash('login'));
        return;
      }
      renderAccount();
      return;
    }

    document.body.classList.remove('identity-route');
    pel.globalAuthPanel.classList.add('hidden');
    pel.globalAccountPanel.classList.add('hidden');

    if (pel.tab && !pel.tab.classList.contains('active') && typeof activateTab === 'function') {
      activateTab(pel.tab);
    }
    state.route = parsed;
    renderCrumbs();

    if (!getUser()) {
      if (!state.sessionResolved) restoreSession();
      navigate(authHash('login', window.location.hash));
      return;
    }
    pel.shell.classList.remove('hidden');
    pel.userLabel.textContent = (getUser() && (getUser().email || getUser().name)) || '';

    showView(parsed.name);
    stopAutoRefresh();

    if (parsed.name === 'list') {
      state.list.offset = 0;
      loadProjectsList();
      startAutoRefresh(loadProjectsList);
    } else if (parsed.name === 'project') {
      if (state.project.id !== parsed.projectId) resetProjectState(parsed.projectId);
      loadProjectDetail();
      startAutoRefresh(() => loadProjectDetail({ silent: true }));
    } else if (parsed.name === 'route') {
      state.routeDetail = { ...state.routeDetail, projectId: parsed.projectId, routeId: parsed.routeId, route: null };
      loadRouteDetail();
      startAutoRefresh(() => loadRouteDetail({ silent: true }));
    } else if (parsed.name === 'import') {
      if (state.importer.projectId !== parsed.projectId) {
        state.importer = { projectId: parsed.projectId, step: 1, job: null, selections: new Map(), conflictFilter: 'all', previewPage: 0, busy: false, result: null };
      }
      renderImportWizard();
    }
  }

  function resetProjectState(projectId) {
    state.project = {
      ...state.project,
      id: projectId,
      project: null,
      series: null,
      incidents: [],
      environments: [],
      telemetryIngress: [],
      telemetryMappings: [],
      slos: [],
      heartbeats: [],
agents: [],
agentAssignments: [],
      routes: [],
      routesTotal: 0,
      filters: { search: '', method: '', status: '', tag: '', enabled: '', deprecated: '' },
      sortBy: 'path',
      sortDir: 'asc',
      offset: 0,
      selected: new Set(),
    };
  }

  /* --------------------------------------------------------- auto refresh */

  function startAutoRefresh(fn) {
    stopAutoRefresh();
    refreshTimer = setInterval(() => {
      // Only poll while this tab is visible and in the foreground; the backend
      // worker performs the actual checks regardless.
      if (document.hidden) return;
      if (!pel.panel.classList.contains('active')) return;
      fn();
    }, VIEW_REFRESH_SECONDS * 1000);
  }

  function stopAutoRefresh() {
    if (refreshTimer) clearInterval(refreshTimer);
    refreshTimer = null;
  }

  /* ------------------------------------------------------------- auth gate */

  function renderAuthGate(mode = 'login', returnTo = null) {
    stopAutoRefresh();
    pel.shell.classList.add('hidden');
    state.authReturnTo = validatedReturnTo(returnTo);
    document.body.classList.add('identity-route');
    pel.globalAuthPanel.classList.remove('hidden');
    pel.globalAccountPanel.classList.add('hidden');
    pel.authGate.classList.remove('hidden');
    pel.recoveryGate.classList.add('hidden');
    setAuthMode(mode);
  }

  function renderRecovery() {
    stopAutoRefresh();
    document.body.classList.add('identity-route');
    pel.globalAuthPanel.classList.remove('hidden');
    pel.globalAccountPanel.classList.add('hidden');
    pel.authGate.classList.add('hidden');
    pel.recoveryGate.classList.remove('hidden');
    pel.shell.classList.add('hidden');
    pel.recoveryCompleteForm.classList.add('hidden');
    hideFormError(pel.recoveryRequestError);
    hideFormError(pel.recoveryCompleteError);
  }

  function setAuthMode(mode) {
    state.authMode = mode;
    const registering = mode === 'register';
    pel.authTitle.textContent = registering ? 'Create your Argus account' : 'Sign in to Argus';
    pel.authIntro.textContent = registering
      ? 'Your account owns the private monitoring projects you create.'
      : 'Sign in to create and manage private monitoring projects.';
    pel.authNameField.hidden = !registering;
    pel.authSubmit.textContent = registering ? 'Create account' : 'Sign in';
    pel.authPassword.autocomplete = registering ? 'new-password' : 'current-password';
    pel.authSwitchPrompt.textContent = registering ? 'Already have an account?' : 'No account yet?';
    pel.authSwitch.textContent = registering ? 'Sign in instead' : 'Create one';
    hideFormError(pel.authError);
  }

  function renderAccount() {
    stopAutoRefresh();
    document.body.classList.add('identity-route');
    pel.globalAuthPanel.classList.add('hidden');
    pel.globalAccountPanel.classList.remove('hidden');
    pel.shell.classList.add('hidden');
    pel.accountEmail.textContent = (getUser() && (getUser().email || getUser().name)) || '';
    hideFormError(pel.accountPasswordError);
    loadAccountSessions();
  }

  function sessionTime(value) {
    if (!value) return 'Not used yet';
    if (typeof relativeTime === 'function') return relativeTime(value);
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? 'Unknown time' : date.toLocaleString();
  }

  function renderAccountSessions() {
    const sessions = state.account.sessions;
    if (!sessions) {
      pel.accountSessions.innerHTML = '<div class="empty-state"><span>Loading active sessions…</span></div>';
      return;
    }
    if (!sessions.length) {
      pel.accountSessions.innerHTML = '<div class="empty-state"><span>No active sessions found.</span></div>';
      return;
    }
    pel.accountSessions.innerHTML = `<div class="account-session-list">${sessions.map((session) => `
      <div class="account-session">
        <div>
          <strong>${escapeHtml(session.current ? 'This session' : (session.name || 'Session'))}</strong>
          <span>Last active ${escapeHtml(sessionTime(session.lastUsedAt || session.createdAt))}</span>
        </div>
        ${session.current ? '<span class="badge status-up">Current</span>' : '<span class="badge status-pending">Active</span>'}
      </div>`).join('')}</div>`;
  }

  async function loadAccountSessions() {
    if (state.account.loading) return;
    state.account.loading = true;
    renderAccountSessions();
    try {
	  const result = await apiProjects('/identity/sessions');
      state.account.sessions = result.sessions || [];
      renderAccountSessions();
    } catch (err) {
      if (!(err instanceof SessionExpired)) {
        pel.accountSessions.innerHTML = `<div class="empty-state is-error"><strong>Could not load sessions</strong><span>${escapeHtml(err.message)}</span></div>`;
      }
    } finally {
      state.account.loading = false;
    }
  }

  function showFormError(node, message) {
    node.textContent = message;
    node.classList.remove('hidden');
  }

  function hideFormError(node) {
    node.textContent = '';
    node.classList.add('hidden');
  }

  pel.authSwitch.addEventListener('click', () => {
    navigate(authHash(state.authMode === 'login' ? 'register' : 'login', state.authReturnTo));
  });
  pel.forgotPassword.addEventListener('click', () => navigate('#/recover'));

  pel.authForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.authError);
    const registering = state.authMode === 'register';
    let pendingSessionVersion = null;
    const body = {
      email: pel.authEmail.value.trim(),
      password: pel.authPassword.value,
    };
    if (registering) body.name = pel.authName.value.trim();

    setButtonLoading(pel.authSubmit, true, registering ? 'Creating...' : 'Signing in...');
    try {
      const res = await fetch(`/identity/${registering ? 'register' : 'login'}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
        credentials: 'same-origin',
      });
      const raw = await res.text();
      let payload = null;
      try {
        payload = raw ? JSON.parse(raw) : null;
      } catch {
        payload = null;
      }
      if (!res.ok) {
        showFormError(pel.authError, (payload && payload.error) || `Request failed (${res.status})`);
        return;
      }
      const sessionVersion = beginSessionTransition();
      pendingSessionVersion = sessionVersion;
      // Registration/login returning 2xx is not enough for the browser UI:
      // confirm that the HttpOnly session cookie was actually stored before
      // leaving this form. This makes an HTTP deployment with Secure cookies
      // fail clearly instead of showing “Signed in” then bouncing back here.
      const profile = await fetch('/identity/profile', { credentials: 'same-origin' });
      if (sessionVersion !== state.sessionVersion) return;
      if (!profile.ok) {
        clearSession();
        const guidance = window.location.protocol === 'https:'
          ? 'Your browser could not verify the new session. Please try again.'
          : 'Your browser could not start a session over HTTP. Use HTTPS or set AUTH_COOKIE_SECURE=false only for local HTTP development.';
        showFormError(pel.authError, guidance);
        return;
      }
      const profilePayload = await profile.json();
      if (sessionVersion !== state.sessionVersion || !profilePayload || !profilePayload.user) return;
      setSession(profilePayload.user);
      pendingSessionVersion = null;
      pel.authPassword.value = '';
      showToast(registering ? 'Account created. Welcome to API Projects.' : 'Signed in.', 'success');
      navigate(state.authReturnTo || '#/projects');
    } catch (err) {
      if (pendingSessionVersion === state.sessionVersion) clearSession();
      showFormError(pel.authError, `Network error: ${err.message}`);
    } finally {
      setButtonLoading(pel.authSubmit, false);
    }
  });

  pel.recoveryRequestForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.recoveryRequestError);
    setButtonLoading(pel.recoveryRequestSubmit, true, 'Sending...');
    try {
      const response = await fetch('/identity/recovery/request', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: pel.recoveryEmail.value.trim() }),
      });
      if (!response.ok) throw new Error(`Request failed (${response.status})`);
      pel.recoveryCompleteForm.classList.remove('hidden');
      pel.recoveryToken.focus();
      showToast('If recovery is available for that account, instructions have been sent.', 'info');
    } catch (err) {
      showFormError(pel.recoveryRequestError, `Network error: ${err.message}`);
    } finally {
      setButtonLoading(pel.recoveryRequestSubmit, false);
    }
  });

  pel.recoveryCompleteForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.recoveryCompleteError);
    setButtonLoading(pel.recoveryCompleteSubmit, true, 'Updating...');
    try {
      const response = await fetch('/identity/recovery/complete', {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: pel.recoveryToken.value.trim(), newPassword: pel.recoveryNewPassword.value }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => null);
        throw new Error((payload && payload.error) || `Request failed (${response.status})`);
      }
      pel.recoveryToken.value = '';
      pel.recoveryNewPassword.value = '';
      showToast('Password updated. Sign in with your new password.', 'success');
      navigate('#/login');
    } catch (err) {
      showFormError(pel.recoveryCompleteError, err.message);
    } finally {
      setButtonLoading(pel.recoveryCompleteSubmit, false);
    }
  });

  pel.accountPasswordForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.accountPasswordError);
    setButtonLoading(pel.accountPasswordSubmit, true, 'Changing...');
    try {
	  await apiProjects('/identity/password', {
        method: 'POST',
        body: JSON.stringify({
          currentPassword: pel.accountCurrentPassword.value,
          newPassword: pel.accountNewPassword.value,
        }),
      });
      pel.accountCurrentPassword.value = '';
      pel.accountNewPassword.value = '';
      showToast('Password changed. Other sessions were signed out.', 'success');
      state.account.sessions = null;
      loadAccountSessions();
    } catch (err) {
      if (!(err instanceof SessionExpired)) showFormError(pel.accountPasswordError, err.message);
    } finally {
      setButtonLoading(pel.accountPasswordSubmit, false);
    }
  });

  pel.accountRevokeOthers.addEventListener('click', async () => {
    if (!window.confirm('Revoke every other active session?')) return;
    setButtonLoading(pel.accountRevokeOthers, true, 'Revoking...');
    try {
	  await apiProjects('/identity/sessions/revoke-others', { method: 'POST' });
      state.account.sessions = null;
      await loadAccountSessions();
      showToast('Other active sessions were revoked.', 'success');
    } catch (err) {
      reportError('Could not revoke other sessions', err);
    } finally {
      setButtonLoading(pel.accountRevokeOthers, false);
    }
  });

  async function signOut() {
    try {
	  await apiProjects('/identity/logout', { method: 'POST' });
    } catch {
      // A failed logout still clears the local session.
    }
    clearSession();
    showToast('Signed out of API Projects.', 'info');
    navigate(authHash('login'));
  }

  pel.signOut.addEventListener('click', signOut);
  pel.globalSignOut.addEventListener('click', signOut);

  async function restoreSession() {
    if (state.sessionResolved) return;
    // Several routing paths can ask for session restoration during startup.
    // Share one request so their responses cannot race one another.
    if (sessionRestorePromise) return sessionRestorePromise;

    const startedAtVersion = state.sessionVersion;
    sessionRestorePromise = (async () => {
      let restored = false;
      try {
	    const res = await fetch('/identity/profile', { credentials: 'same-origin' });
        if (startedAtVersion !== state.sessionVersion) return;
        if (res.ok) {
          const payload = await res.json();
          if (startedAtVersion !== state.sessionVersion) return;
          setSession(payload.user);
        } else {
          clearSession();
        }
        restored = true;
      } catch {
        if (startedAtVersion !== state.sessionVersion) return;
        clearSession();
        restored = true;
      } finally {
        sessionRestorePromise = null;
      }
      // A successful login/register may have completed while the old profile
      // request was in flight. In that case it owns routing; do not redirect.
      if (restored) handleRoute();
    })();
    return sessionRestorePromise;
  }

  /* ------------------------------------------------------ projects list view */

  async function loadProjectsList() {
    if (state.list.loading) return;
    state.list.loading = true;
    if (!state.list.data) pel.viewList.innerHTML = listShellHtml(skeletonCards());
    try {
      const res = await apiProjects(
		`/project/catalog${qs({ search: state.list.search, status: state.list.status, limit: state.list.limit, offset: state.list.offset })}`
      );
      state.list.data = res.items || [];
      state.list.total = res.total || 0;
      renderProjectsList();
    } catch (err) {
      if (!(err instanceof SessionExpired)) {
        pel.viewList.innerHTML = listShellHtml(errorPanel('Could not load projects', err.message, 'reload-projects'));
      }
    } finally {
      state.list.loading = false;
    }
  }

  function listShellHtml(body) {
    const { search, status } = state.list;
    return `
      <section class="card">
        <div class="card-header">
          <h2>API projects</h2>
          <div class="card-toolbar">
            <div class="search-input">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>
              <input id="projListSearch" placeholder="Search projects..." aria-label="Search projects" value="${escapeHtml(search)}" />
            </div>
            <select id="projListStatus" style="width:auto" aria-label="Filter by status">
              <option value="">All statuses</option>
              <option value="active"${status === 'active' ? ' selected' : ''}>Active</option>
              <option value="archived"${status === 'archived' ? ' selected' : ''}>Archived</option>
            </select>
            <button data-action="new-project" type="button">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.2"><path d="M12 5v14M5 12h14"/></svg>
              New project
            </button>
          </div>
        </div>
        ${body}
      </section>`;
  }

  function healthChips(p) {
    const chips = [
      ['healthy', p.routesHealthy],
      ['degraded', p.routesDegraded],
      ['failing', p.routesFailing],
      ['unknown', p.routesUnknown],
      ['disabled', p.routesDisabled],
    ].filter(([, count]) => count > 0);
    if (!chips.length) return '<span class="proj-chip is-muted">no routes</span>';
    return chips.map(([name, count]) => `<span class="proj-chip route-${name}">${count} ${name}</span>`).join('');
  }

  function projectCardHtml(p) {
    const canOwn = p.viewerRole === 'owner';
    const canEdit = canOwn || p.viewerRole === 'editor';
    const archived = p.status === 'archived';
    return `
      <article class="card proj-card${archived ? ' is-archived' : ''}">
        <header class="proj-card-head">
          <div>
            <a class="proj-card-title" href="#/projects/${p.id}">${escapeHtml(p.name)}</a>
            <span class="proj-card-slug mono">${escapeHtml(p.slug)}</span>
          </div>
          <span class="badge proj-status-${escapeHtml(p.status)}">${escapeHtml(p.status)}</span>
        </header>
        ${p.description ? `<p class="proj-card-desc">${escapeHtml(p.description)}</p>` : ''}
        <div class="proj-card-metrics">
          <div><span class="proj-metric-value">${num(p.routesTotal)}</span><span class="proj-metric-label">routes</span></div>
          <div><span class="proj-metric-value">${pct(p.uptime24hPct)}</span><span class="proj-metric-label">uptime 24h</span></div>
          <div><span class="proj-metric-value">${ms(p.avgLatency24hMs)}</span><span class="proj-metric-label">avg latency</span></div>
          <div><span class="proj-metric-value${p.openIncidents > 0 ? ' is-bad' : ''}">${num(p.openIncidents)}</span><span class="proj-metric-label">open incidents</span></div>
          <div><span class="proj-metric-value">${num(p.failures24h, '0')}</span><span class="proj-metric-label">failures 24h</span></div>
        </div>
        <div class="proj-card-chips">${healthChips(p)}</div>
        <footer class="proj-card-foot">
          <span class="list-item-meta">${p.lastCheckAt ? `last check ${escapeHtml(relativeTime(p.lastCheckAt))}` : 'never checked'}</span>
          <div class="row-actions">
            <a class="btn-link" href="#/projects/${p.id}">Open</a>
            ${canEdit ? `<button class="secondary sm" type="button" data-action="edit-project" data-id="${p.id}">Edit</button>` : ''}
            ${canOwn ? `<button class="secondary sm" type="button" data-action="${archived ? 'unarchive' : 'archive'}-project" data-id="${p.id}">${archived ? 'Unarchive' : 'Archive'}</button>` : ''}
            ${canOwn ? `<button class="danger sm" type="button" data-action="delete-project" data-id="${p.id}">Delete</button>` : ''}
          </div>
        </footer>
      </article>`;
  }

  function renderProjectsList() {
    const items = state.list.data || [];
    let body;
    if (!items.length) {
      const filtering = state.list.search || state.list.status;
      body = filtering
        ? emptyPanel(ICON.project, 'No matching projects', 'Try a different search term or status filter.')
        : emptyPanel(
            ICON.project,
            'No projects yet',
            'Create a project, then add routes manually or import an OpenAPI specification.',
            '<button data-action="new-project" type="button" class="secondary sm">Create your first project</button>'
          );
    } else {
      body = `<div class="proj-grid">${items.map(projectCardHtml).join('')}</div>${paginationHtml(state.list.offset, state.list.limit, state.list.total, 'projects-page')}`;
    }
    pel.viewList.innerHTML = listShellHtml(body);

    const search = document.getElementById('projListSearch');
    if (search) {
      search.addEventListener('input', debounce(() => {
        state.list.search = search.value.trim();
        state.list.offset = 0;
        loadProjectsList();
      }, 250));
    }
    const status = document.getElementById('projListStatus');
    if (status) {
      status.addEventListener('change', () => {
        state.list.status = status.value;
        state.list.offset = 0;
        loadProjectsList();
      });
    }
  }

  function paginationHtml(offset, limit, total, action) {
    if (total <= limit) return '';
    const page = Math.floor(offset / limit) + 1;
    const pages = Math.max(1, Math.ceil(total / limit));
    return `
      <nav class="proj-pager" aria-label="Pagination">
        <button class="secondary sm" type="button" data-action="${action}" data-offset="${Math.max(0, offset - limit)}" ${offset === 0 ? 'disabled' : ''}>Previous</button>
        <span class="proj-pager-label">Page ${page} of ${pages} &middot; ${total} total</span>
        <button class="secondary sm" type="button" data-action="${action}" data-offset="${offset + limit}" ${page >= pages ? 'disabled' : ''}>Next</button>
      </nav>`;
  }

  function debounce(fn, wait) {
    let timer = null;
    return (...args) => {
      clearTimeout(timer);
      timer = setTimeout(() => fn(...args), wait);
    };
  }

  /* ----------------------------------------------------- project detail view */

  async function loadProjectDetail({ silent = false } = {}) {
    const id = state.project.id;
    if (!id || state.project.loading) return;
    state.project.loading = true;
    if (!silent && !state.project.project) {
      pel.viewProject.innerHTML = `<section class="card"><div class="card-header"><h2>Loading project…</h2></div>${skeletonCards(3)}</section>`;
    }
    try {
      const p = state.project;
	  const [project, series, incidents, projectIncidents, routes, environments, telemetryIngress, telemetryMappings, slos, heartbeats, agents, agentAssignments] = await Promise.all([
		apiProjects(`/project/catalog/${id}`),
		apiProjects(`/route/metrics/${id}${qs({ range: p.range })}`),
		apiProjects(`/route/incidents/${id}${qs({ limit: 15 })}`),
		apiProjects(`/incident/catalog/${id}${qs({ limit: 15 })}`),
		apiProjects(`/route/catalog/${id}${routeQuery(p)}`),
		apiProjects(`/environment/catalog/${id}`),
		apiProjects(`/telemetry/ingress/${id}${qs({ limit: 20 })}`),
		apiProjects(`/telemetry/mappings/${id}`),
		apiProjects(`/slo/catalog/${id}`),
		apiProjects(`/heartbeat/catalog/${id}`),
apiProjects(`/agent/catalog/${id}`),
apiProjects(`/agent/assignments/${id}`),
      ]);
      state.project.project = project;
      state.project.series = series;
      state.project.incidents = incidents || [];
	  state.project.projectIncidents = projectIncidents.items || [];
      state.project.routes = routes.items || [];
      state.project.routesTotal = routes.total || 0;
      state.project.environments = environments.items || [];
	  state.project.telemetryIngress = telemetryIngress.items || [];
	  state.project.telemetryMappings = telemetryMappings.items || [];
	  state.project.slos = slos.items || [];
	  state.project.heartbeats = heartbeats.items || [];
      state.project.agents = agents.items || [];
      state.project.agentAssignments = agentAssignments.items || [];
      renderCrumbs();
      renderProjectDetail();
    } catch (err) {
      if (!(err instanceof SessionExpired)) {
        if (err.status === 404) {
          pel.viewProject.innerHTML = errorPanel('Project not found', 'It may have been deleted, or you no longer have access.', 'back-to-projects');
        } else if (!silent) {
          pel.viewProject.innerHTML = errorPanel('Could not load this project', err.message, 'reload-project');
        } else {
          reportError('Refresh failed', err);
        }
      }
    } finally {
      state.project.loading = false;
    }
  }

  /** routeQuery builds the server-side search/filter/sort/paginate query. */
  function routeQuery(p) {
    return qs({
      search: p.filters.search,
      method: p.filters.method,
      status: p.filters.status,
      tag: p.filters.tag,
      enabled: p.filters.enabled,
      deprecated: p.filters.deprecated,
      sortBy: p.sortBy,
      sortDir: p.sortDir,
      limit: p.limit,
      offset: p.offset,
    });
  }

  const RANGES = ['1h', '6h', '24h', '7d', '30d'];

  function rangePickerHtml(current, action) {
    return `<div class="range-picker" role="group" aria-label="Time range">${RANGES.map(
      (r) => `<button type="button" class="range-btn${r === current ? ' is-active' : ''}" data-action="${action}" data-range="${r}">${r}</button>`
    ).join('')}</div>`;
  }

  function metricTile(value, label, tone = '') {
    return `<div class="metric-tile${tone ? ` tone-${tone}` : ''}"><span class="metric-value">${value}</span><span class="metric-label">${escapeHtml(label)}</span></div>`;
  }

  function renderProjectDetail() {
    const p = state.project.project;
    if (!p) return;
    const canEdit = p.viewerRole === 'owner' || p.viewerRole === 'editor';
    const f = state.project.filters;

    pel.viewProject.innerHTML = `
      <section class="card">
        <div class="card-header">
          <h2>${escapeHtml(p.name)}</h2>
          <div class="card-toolbar">
            <span class="badge proj-status-${escapeHtml(p.status)}">${escapeHtml(p.status)}</span>
            <span class="proj-role-chip">your role: ${escapeHtml(p.viewerRole || 'viewer')}</span>
            ${canEdit ? '<button class="secondary sm" type="button" data-action="edit-current-project">Settings</button>' : ''}
            ${canEdit ? '<button class="secondary sm" type="button" data-action="add-route">Add route</button>' : ''}
            ${canEdit ? '<button class="secondary sm" type="button" data-action="bulk-add">Bulk add</button>' : ''}
            ${canEdit ? `<button type="button" data-action="open-import">${ICON.spec}Import spec</button>` : ''}
          </div>
        </div>
        ${p.description ? `<p class="card-subtitle">${escapeHtml(p.description)}</p>` : ''}
        <div class="metric-row">
          ${metricTile(num(p.routesTotal), 'routes')}
          ${metricTile(num(p.routesHealthy), 'healthy', 'ok')}
          ${metricTile(num(p.routesDegraded), 'degraded', 'warn')}
          ${metricTile(num(p.routesFailing), 'failing', 'down')}
          ${metricTile(num(p.routesUnknown), 'unknown')}
          ${metricTile(num(p.routesDisabled), 'disabled')}
          ${metricTile(pct(p.uptime24hPct), 'uptime 24h')}
          ${metricTile(ms(p.avgLatency24hMs), 'avg latency 24h')}
          ${metricTile(num(p.openIncidents), 'open incidents', p.openIncidents > 0 ? 'down' : '')}
          ${metricTile(p.lastCheckAt ? escapeHtml(relativeTime(p.lastCheckAt)) : 'never', 'last check')}
        </div>
        <p class="hint">Defaults for new routes: every ${num(p.defaultIntervalSeconds)}s, ${num(p.defaultTimeoutMs)}ms timeout,
          ${num(p.defaultRetries)} retries, incident after ${num(p.failureThreshold)} consecutive failures,
          resolved after ${num(p.recoverySuccessThreshold)} successes.</p>
      </section>

      <section class="card">
        <div class="card-header"><h2>Environments</h2>${canEdit ? '<button class="secondary sm" type="button" data-action="create-environment">Add environment</button>' : ''}</div>
        ${state.project.environments.length ? `<div class="list">${state.project.environments.map((env) => `<div class="list-item"><div class="list-item-main"><span class="list-item-title">${escapeHtml(env.name)}</span><span class="list-item-meta">${escapeHtml(env.canonicalBaseUrl || 'Base URL not configured')}</span></div>${env.isDefault ? '<span class="badge status-up">default</span>' : ''}${canEdit ? `<button class="secondary sm" type="button" data-action="edit-environment" data-id="${Number(env.id)}">Edit</button>` : ''}</div>`).join('')}</div>` : '<div class="empty-state"><span>No environments configured.</span></div>'}
      </section>

      <section class="card">
        <div class="card-header"><h2>Telemetry signals</h2>${canEdit ? '<button class="secondary sm" type="button" data-action="create-telemetry-credential">Create OTLP credential</button>' : ''}</div>
        <p class="hint">Recent accepted OTLP resource groups. Only safe service and deployment labels are shown; raw telemetry is not displayed here.</p>
        ${telemetryIngressHtml(state.project.telemetryIngress)}
      </section>

      <section class="card">
        <div class="card-header"><h2>Telemetry route mappings</h2>${canEdit ? '<button class="secondary sm" type="button" data-action="create-telemetry-mapping">Create mapping</button>' : ''}</div>
        <p class="hint">A mapping joins one trusted telemetry identity to one project route. It does not infer tenant ownership from incoming telemetry.</p>
        ${telemetryMappingsHtml(state.project.telemetryMappings, canEdit)}
      </section>

      <section class="card">
        <div class="card-header"><h2>Service-level objectives</h2>${canEdit ? '<button class="secondary sm" type="button" data-action="create-slo">Create SLO</button>' : ''}</div>
        <p class="hint">Objectives are calculated from project-scoped telemetry. No-data, stale, maintenance, and configuration errors are never presented as healthy.</p>
        ${sloDefinitionsHtml(state.project.slos)}
      </section>

      <section class="card">
        <div class="card-header"><h2>Heartbeats</h2>${canEdit ? '<button class="secondary sm" type="button" data-action="create-heartbeat">Create heartbeat</button>' : ''}</div>
        <p class="hint">Use a project-bound one-time token for jobs and scheduled workloads. Late and missing runs remain visible; duplicate retries do not refresh liveness.</p>
        ${heartbeatMonitorsHtml(state.project.heartbeats, canEdit)}
      </section>

      <section class="card">
        <div class="card-header"><h2>Private agents</h2>${canEdit ? '<button class="secondary sm" type="button" data-action="create-agent">Enroll agent</button><button class="secondary sm" type="button" data-action="create-agent-assignment">Create assignment</button>' : ''}</div>
        <p class="hint">Agents report outbound liveness only. Argus never connects into your private network. The enrollment token is shown once and can be revoked at any time.</p>
        ${privateAgentsHtml(state.project.agents, canEdit)}
        <h3 class="section-subheading">Scoped assignments</h3>
        <p class="hint">Only signed GET/HEAD assignments for the agent’s environment are executed locally. Targets are never dialed by the Argus control plane.</p>
        ${privateAgentAssignmentsHtml(state.project.agentAssignments, canEdit)}
      </section>

      <section class="card">
        <div class="card-header"><h2>Operational incidents</h2></div>
        <p class="hint">Source-aware incidents cover agents, heartbeats, telemetry, and SLOs. Acknowledgement records ownership but does not suppress automatic recovery.</p>
        ${projectIncidentsHtml(state.project.projectIncidents, canEdit)}
      </section>

      <section class="card">
        <div class="card-header">
          <h2>Uptime &amp; response time</h2>
          <div class="card-toolbar">${rangePickerHtml(state.project.range, 'project-range')}</div>
        </div>
        <div class="chart-grid">
          <figure class="chart-box">
            <figcaption>Uptime %</figcaption>
            <canvas id="projUptimeChart" height="150" role="img" aria-label="Uptime over the selected range"></canvas>
          </figure>
          <figure class="chart-box">
            <figcaption>Response time (ms)</figcaption>
            <canvas id="projLatencyChart" height="150" role="img" aria-label="Average response time over the selected range"></canvas>
          </figure>
        </div>
      </section>

      <section class="card">
        <div class="card-header"><h2>Incidents</h2></div>
        ${incidentsListHtml(state.project.incidents, state.project.id, canEdit)}
      </section>

      <section class="card">
        <div class="card-header">
          <h2>Routes</h2>
          <div class="card-toolbar">
            <div class="search-input">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="7"/><path d="m21 21-4.3-4.3"/></svg>
              <input id="projRouteSearch" placeholder="Search path, name or summary..." aria-label="Search routes" value="${escapeHtml(f.search)}" />
            </div>
            <select id="projFilterMethod" style="width:auto" aria-label="Filter by method">
              <option value="">All methods</option>
              ${['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS', 'TRACE']
                .map((m) => `<option value="${m}"${f.method === m ? ' selected' : ''}>${m}</option>`)
                .join('')}
            </select>
            <select id="projFilterStatus" style="width:auto" aria-label="Filter by health">
              <option value="">All health</option>
              ${['healthy', 'degraded', 'failing', 'unknown', 'disabled']
                .map((s) => `<option value="${s}"${f.status === s ? ' selected' : ''}>${s}</option>`)
                .join('')}
            </select>
            <select id="projFilterEnabled" style="width:auto" aria-label="Filter by enabled">
              <option value="">Enabled &amp; disabled</option>
              <option value="true"${f.enabled === 'true' ? ' selected' : ''}>Enabled only</option>
              <option value="false"${f.enabled === 'false' ? ' selected' : ''}>Disabled only</option>
            </select>
            <select id="projFilterDeprecated" style="width:auto" aria-label="Filter by deprecation">
              <option value="">Any deprecation</option>
              <option value="true"${f.deprecated === 'true' ? ' selected' : ''}>Deprecated</option>
              <option value="false"${f.deprecated === 'false' ? ' selected' : ''}>Not deprecated</option>
            </select>
            <input id="projFilterTag" placeholder="Tag" aria-label="Filter by tag" style="width:8rem" value="${escapeHtml(f.tag)}" />
            ${Object.values(f).some(Boolean) ? '<button class="ghost sm" type="button" data-action="clear-route-filters">Clear</button>' : ''}
          </div>
        </div>
        ${canEdit ? bulkBarHtml() : ''}
        ${routeTableHtml(canEdit)}
        ${paginationHtml(state.project.offset, state.project.limit, state.project.routesTotal, 'routes-page')}
      </section>`;

    wireProjectFilters();
    drawSeriesCharts('projUptimeChart', 'projLatencyChart', state.project.series);
  }

  function bulkBarHtml() {
    const count = state.project.selected.size;
    if (!count) return '';
    return `
      <div class="bulk-bar" role="region" aria-label="Bulk actions">
        <span><strong>${count}</strong> route${count === 1 ? '' : 's'} selected</span>
        <div class="row-actions">
          <button class="secondary sm" type="button" data-action="bulk-enable">Enable</button>
          <button class="secondary sm" type="button" data-action="bulk-disable">Disable</button>
          <button class="danger sm" type="button" data-action="bulk-delete">Delete</button>
          <button class="ghost sm" type="button" data-action="bulk-clear">Clear selection</button>
        </div>
      </div>`;
  }

  const ROUTE_COLUMNS = [
    { key: 'method', label: 'Method', sortable: true },
    { key: 'path', label: 'Path', sortable: true },
    { key: 'status', label: 'Health', sortable: true },
    { key: 'uptime', label: 'Uptime 24h', sortable: true },
    { key: 'latency', label: 'Avg latency', sortable: true },
    { key: null, label: 'Last code', sortable: false },
    { key: 'updated', label: 'Last check', sortable: true },
    { key: null, label: 'Actions', sortable: false },
  ];

  function routeTableHtml(canEdit) {
    const rows = state.project.routes;
    const colspan = ROUTE_COLUMNS.length + (canEdit ? 1 : 0);
    let body;
    if (!rows.length) {
      const filtering = Object.values(state.project.filters).some(Boolean);
      body = `<tr><td colspan="${colspan}">${
        filtering
          ? emptyPanel(ICON.route, 'No routes match these filters', 'Adjust or clear the filters to see more routes.')
          : emptyPanel(ICON.route, 'No routes yet', 'Add a route manually, paste a bulk list, or import an OpenAPI specification.')
      }</td></tr>`;
    } else {
      body = rows.map((r) => routeRowHtml(r, canEdit)).join('');
    }

    const allOnPageSelected = rows.length > 0 && rows.every((r) => state.project.selected.has(r.id));
    return `
      <div class="table-wrap">
        <table class="route-table">
          <thead>
            <tr>
              ${canEdit ? `<th class="col-check"><input type="checkbox" data-action="toggle-page-selection" aria-label="Select all routes on this page" ${allOnPageSelected ? 'checked' : ''} /></th>` : ''}
              ${ROUTE_COLUMNS.map((c) => {
                if (!c.sortable) return `<th>${escapeHtml(c.label)}</th>`;
                const active = state.project.sortBy === c.key;
                const direction = active ? (state.project.sortDir === 'desc' ? 'descending' : 'ascending') : 'none';
                return `<th class="sortable${active ? ` sort-active${state.project.sortDir === 'desc' ? ' sort-desc' : ''}` : ''}" aria-sort="${direction}"><button type="button" class="sort-button" data-action="sort-routes" data-sort="${c.key}">${escapeHtml(c.label)}<span class="sort-arrow" aria-hidden="true">&#9662;</span><span class="sr-only">${active ? `, sorted ${direction}` : ', not sorted'}</span></button></th>`;
              }).join('')}
            </tr>
          </thead>
          <tbody>${body}</tbody>
        </table>
      </div>`;
  }

  function routeRowHtml(r, canEdit) {
    const pid = state.project.id;
    return `
      <tr${r.enabled ? '' : ' class="is-disabled-row"'}>
        ${canEdit ? `<td class="col-check"><input type="checkbox" data-action="toggle-route-selection" data-id="${r.id}" aria-label="Select ${escapeHtml(r.method)} ${escapeHtml(r.path)}" ${state.project.selected.has(r.id) ? 'checked' : ''} /></td>` : ''}
        <td>${methodChip(r.method)}</td>
        <td class="path-cell">
          <a href="#/projects/${pid}/routes/${r.id}" title="${escapeHtml(r.path)}">${escapeHtml(r.path)}</a>
          ${r.deprecated ? '<span class="proj-chip is-deprecated">deprecated</span>' : ''}
          ${(r.tags || []).slice(0, 3).map((t) => `<span class="proj-chip is-tag">${escapeHtml(t)}</span>`).join('')}
        </td>
        <td>${healthBadge(r.status)}</td>
        <td class="mono">${pct(r.uptime24hPct)}</td>
        <td class="mono">${ms(r.avgLatency24hMs)}</td>
        <td class="mono">${r.lastStatusCode ? r.lastStatusCode : '–'}</td>
        <td class="mono">${r.lastCheckedAt ? escapeHtml(relativeTime(r.lastCheckedAt)) : 'never'}</td>
        <td>
          <div class="row-actions">
            <a class="btn-link" href="#/projects/${pid}/routes/${r.id}">Details</a>
            ${canEdit ? `<button class="secondary sm" type="button" data-action="edit-route" data-id="${r.id}">Edit</button>` : ''}
            ${canEdit ? `<button class="secondary sm" type="button" data-action="${r.enabled ? 'disable' : 'enable'}-route" data-id="${r.id}">${r.enabled ? 'Disable' : 'Enable'}</button>` : ''}
            ${canEdit ? `<button class="danger sm" type="button" data-action="delete-route" data-id="${r.id}">Delete</button>` : ''}
          </div>
        </td>
      </tr>`;
  }

  function incidentsListHtml(incidents, projectId, canEdit = false) {
    if (!incidents.length) {
      return emptyPanel(ICON.incident, 'No incidents recorded', 'An incident opens automatically after a route fails the configured number of consecutive checks.');
    }
    return `<ul class="list">${incidents
      .map((i) => {
        const resolved = i.state === 'resolved';
        const duration = resolved && i.resolvedAt ? formatDuration(new Date(i.resolvedAt) - new Date(i.startedAt)) : formatDuration(Date.now() - new Date(i.startedAt));
        return `
          <li class="list-item">
            <div class="list-item-main">
              <a class="list-item-title" href="#/projects/${projectId}/routes/${i.routeId}">Incident #${i.id} &middot; route #${i.routeId}</a>
              <span class="list-item-meta">
                started ${escapeHtml(relativeTime(i.startedAt))}
                &middot; ${resolved ? `resolved ${escapeHtml(relativeTime(i.resolvedAt))}` : 'ongoing'}
                &middot; ${escapeHtml(duration)}
                ${i.lastFailureReason ? `&middot; ${escapeHtml(i.lastFailureReason)}` : ''}
                ${i.source ? `&middot; source ${escapeHtml(i.source)}` : ''}
              </span>
            </div>
            <div class="row-actions"><span class="badge status-${resolved ? 'resolved' : 'open'}"><span class="lamp"></span>${escapeHtml(i.state)}</span>${canEdit && i.state === 'open' ? `<button class="secondary sm" type="button" data-action="acknowledge-incident" data-id="${i.id}">Acknowledge</button>` : ''}</div>
          </li>`;
      })
      .join('')}</ul>`;
  }

  function telemetryIngressHtml(records) {
    if (!records.length) {
      return emptyPanel(ICON.route, 'No telemetry received yet', 'Create a telemetry credential and point an OpenTelemetry exporter at this Argus instance.');
    }
    return `<ul class="list">${records.map((record) => `
      <li class="list-item">
        <div class="list-item-main">
          <span class="list-item-title">${escapeHtml(record.serviceName || 'Unnamed service')}</span>
          <span class="list-item-meta">${escapeHtml(record.deploymentEnvironment || 'Deployment not declared')} &middot; ${num(record.itemCount, '0')} item${record.itemCount === 1 ? '' : 's'} &middot; ${record.receivedAt ? escapeHtml(relativeTime(record.receivedAt)) : 'time unavailable'}</span>
        </div>
        <span class="badge ${record.signalType === 'traces' ? 'route-unknown' : 'status-up'}">${escapeHtml(record.signalType || 'signal')}</span>
      </li>`).join('')}</ul>`;
  }

  function telemetryMappingsHtml(mappings, canEdit) {
    if (!mappings.length) {
      return emptyPanel(ICON.route, 'No telemetry mappings yet', 'Map a trusted service and environment identity to a catalog route after telemetry is arriving.');
    }
    return `<div class="list">${mappings.map((mapping) => `<div class="list-item"><div class="list-item-main"><span class="list-item-title">${escapeHtml(mapping.serviceName)} → ${escapeHtml(mapping.httpMethod)} ${escapeHtml(mapping.routeTemplate)}</span><span class="list-item-meta">environment #${num(mapping.environmentId)}${mapping.deploymentEnvironment ? ` &middot; deployment ${escapeHtml(mapping.deploymentEnvironment)}` : ''} &middot; ${escapeHtml(mapping.source || 'manual')}</span></div>${canEdit ? `<button class="danger sm" type="button" data-action="delete-telemetry-mapping" data-id="${mapping.id}">Delete</button>` : ''}</div>`).join('')}</div>`;
  }

  function sloDefinitionsHtml(definitions) {
    if (!definitions.length) {
      return emptyPanel(ICON.incident, 'No SLOs configured', 'Create an availability or latency objective after connecting telemetry. Missing data will remain visible as no data.');
    }
    return `<div class="list">${definitions.map((definition) => {
      const windowDays = Math.round(definition.windowSeconds / 86400);
      const indicator = definition.sliKind === 'latency'
        ? `latency below ${num(definition.latencyThresholdMs)} ms`
        : 'availability';
      return `<div class="list-item"><div class="list-item-main"><span class="list-item-title">${escapeHtml(definition.name)}</span><span class="list-item-meta">${escapeHtml(indicator)} &middot; ${Number(definition.targetPercent).toFixed(3).replace(/\.?(0+)$/, '')}% target &middot; ${windowDays}-day window &middot; ${num(definition.minEvents, '0')} minimum events</span></div><span class="proj-chip is-muted">v${num(definition.version, '1')}</span></div>`;
    }).join('')}</div>`;
  }

  function heartbeatMonitorsHtml(monitors, canEdit) {
    if (!monitors.length) return emptyPanel(ICON.route, 'No heartbeat monitors configured', 'Create a heartbeat for a job or scheduled workload, then send its first signed ping.');
    return `<div class="list">${monitors.map((monitor) => `<div class="list-item"><div class="list-item-main"><span class="list-item-title">${escapeHtml(monitor.name)}</span><span class="list-item-meta">every ${num(Math.round(monitor.expectedIntervalSeconds / 60))} min + ${num(Math.round(monitor.gracePeriodSeconds / 60))} min grace &middot; ${monitor.lastReceivedAt ? `last seen ${escapeHtml(relativeTime(monitor.lastReceivedAt))}` : 'no signal received'}</span></div><div class="row-actions"><span class="badge ${monitor.lastOutcome === 'healthy' ? 'status-up' : monitor.lastOutcome === 'late' ? 'route-degraded' : 'route-unknown'}">${escapeHtml(monitor.lastOutcome || 'missing')}</span>${canEdit && !monitor.revokedAt ? `<button class="danger sm" type="button" data-action="revoke-heartbeat" data-id="${monitor.id}">Revoke</button>` : ''}</div></div>`).join('')}</div>`;
  }

  function privateAgentsHtml(agents, canEdit) {
    if (!agents.length) return emptyPanel(ICON.route, 'No private agents enrolled', 'Enroll an environment-local agent when a target cannot be safely reached from the public control plane.');
    return `<div class="list">${agents.map((agent) => {
      const tone = agent.status === 'healthy' ? 'status-up' : agent.status === 'stale' ? 'route-degraded' : agent.status === 'revoked' ? 'status-resolved' : 'route-unknown';
      const interval = agent.expectedIntervalSeconds >= 60 ? `${num(Math.round(agent.expectedIntervalSeconds / 60))} min` : `${num(agent.expectedIntervalSeconds)} sec`;
      return `<div class="list-item"><div class="list-item-main"><span class="list-item-title">${escapeHtml(agent.name)}</span><span class="list-item-meta">environment #${num(agent.environmentId)} &middot; every ${interval} &middot; ${agent.lastSeenAt ? `last seen ${escapeHtml(relativeTime(agent.lastSeenAt))}` : 'no signal received'}${agent.version ? ` &middot; version ${escapeHtml(agent.version)}` : ''}</span></div><div class="row-actions"><span class="badge ${tone}">${escapeHtml(agent.status || 'offline')}</span>${canEdit && !agent.revokedAt ? `<button class="danger sm" type="button" data-action="revoke-agent" data-id="${agent.id}">Revoke</button>` : ''}</div></div>`;
    }).join('')}</div>`;
  }

  function privateAgentAssignmentsHtml(assignments, canEdit) {
    if (!assignments.length) return emptyPanel(ICON.route, 'No private-agent assignments', 'Create an assignment through the authenticated API to permit a narrowly scoped local GET or HEAD check.');
    return `<div class="list">${assignments.map((assignment) => `<div class="list-item"><div class="list-item-main"><span class="list-item-title">${escapeHtml(assignment.name)}</span><span class="list-item-meta">${escapeHtml(assignment.method)} &middot; environment #${num(assignment.environmentId)} &middot; every ${num(assignment.intervalSeconds)} sec &middot; ${num(assignment.timeoutMs)} ms timeout</span></div><div class="row-actions"><span class="badge ${assignment.revokedAt || !assignment.enabled ? 'status-resolved' : 'status-up'}">${assignment.revokedAt || !assignment.enabled ? 'revoked' : 'enabled'}</span>${canEdit && !assignment.revokedAt ? `<button class="danger sm" type="button" data-action="revoke-agent-assignment" data-id="${assignment.id}">Revoke</button>` : ''}</div></div>`).join('')}</div>`;
  }

  function projectIncidentsHtml(incidents, canEdit) {
    if (!incidents.length) return emptyPanel(ICON.incident, 'No operational incidents', 'Agent, heartbeat, telemetry, and SLO problems will appear here with their source and evidence.');
    return `<div class="list">${incidents.map((incident) => {
      const tone = incident.state === 'resolved' ? 'status-resolved' : incident.state === 'acknowledged' ? 'route-degraded' : 'route-failing';
      let evidence = '';
      try {
        const parsed = JSON.parse(incident.evidence || '{}');
        if (parsed.livenessState) evidence = ` &middot; state ${escapeHtml(parsed.livenessState)}`;
      } catch (_) { /* Evidence stays intentionally opaque if it is not structured JSON. */ }
      return `<div class="list-item"><div class="list-item-main"><span class="list-item-title">${escapeHtml(incident.title || 'Operational incident')}</span><span class="list-item-meta">${escapeHtml(incident.source || 'unknown source')} &middot; opened ${incident.startedAt ? escapeHtml(relativeTime(incident.startedAt)) : 'at an unknown time'}${evidence}</span></div><div class="row-actions"><span class="badge ${tone}">${escapeHtml(incident.state || 'open')}</span>${canEdit && incident.state === 'open' ? `<button class="secondary sm" type="button" data-action="acknowledge-project-incident" data-id="${incident.id}">Acknowledge</button>` : ''}</div></div>`;
    }).join('')}</div>`;
  }

  function formatDuration(msTotal) {
    const secs = Math.max(0, Math.round(msTotal / 1000));
    if (secs < 60) return `${secs}s`;
    const mins = Math.round(secs / 60);
    if (mins < 60) return `${mins}m`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ${mins % 60}m`;
    return `${Math.floor(hrs / 24)}d ${hrs % 24}h`;
  }

  function wireProjectFilters() {
    const bind = (id, key, event = 'change') => {
      const node = document.getElementById(id);
      if (!node) return;
      const apply = () => {
        state.project.filters[key] = node.value.trim();
        state.project.offset = 0;
        state.project.selected.clear();
        loadProjectDetail({ silent: true });
      };
      node.addEventListener(event, event === 'input' ? debounce(apply, 250) : apply);
    };
    bind('projRouteSearch', 'search', 'input');
    bind('projFilterMethod', 'method');
    bind('projFilterStatus', 'status');
    bind('projFilterEnabled', 'enabled');
    bind('projFilterDeprecated', 'deprecated');
    bind('projFilterTag', 'tag', 'input');
  }

  /* ------------------------------------------------------- route detail view */

  async function loadRouteDetail({ silent = false } = {}) {
    const { projectId, routeId } = state.routeDetail;
    if (!projectId || !routeId || state.routeDetail.loading) return;
    state.routeDetail.loading = true;
    if (!silent && !state.routeDetail.route) {
      pel.viewRoute.innerHTML = `<section class="card"><div class="card-header"><h2>Loading route…</h2></div>${skeletonCards(2)}</section>`;
    }
    try {
      const [route, checks, incidents, series, project] = await Promise.all([
		apiProjects(`/route/catalog/${projectId}/${routeId}`),
		apiProjects(`/route/checks/${projectId}/${routeId}${qs({ limit: 100 })}`),
		apiProjects(`/route/incidents/${projectId}${qs({ routeId, limit: 20 })}`),
		apiProjects(`/route/metrics/${projectId}${qs({ range: state.routeDetail.range, routeId })}`),
		state.project.project && state.project.id === projectId ? Promise.resolve(state.project.project) : apiProjects(`/project/catalog/${projectId}`),
      ]);
      state.routeDetail.route = route;
      state.routeDetail.checks = checks || [];
      state.routeDetail.incidents = incidents || [];
      state.routeDetail.series = series;
      state.project.id = projectId;
      state.project.project = project;
      renderCrumbs();
      renderRouteDetail();
    } catch (err) {
      if (!(err instanceof SessionExpired)) {
        if (err.status === 404) {
          pel.viewRoute.innerHTML = errorPanel('Route not found', 'It may have been deleted, or it belongs to another project.', 'back-to-project');
        } else if (!silent) {
          pel.viewRoute.innerHTML = errorPanel('Could not load this route', err.message, 'reload-route');
        } else {
          reportError('Refresh failed', err);
        }
      }
    } finally {
      state.routeDetail.loading = false;
    }
  }

  function definitionRow(label, value, mono = false) {
    return `<div class="def-row"><dt>${escapeHtml(label)}</dt><dd${mono ? ' class="mono"' : ''}>${value}</dd></div>`;
  }

  function jsonBlock(label, raw) {
    if (!raw) return '';
    let pretty = raw;
    try {
      pretty = JSON.stringify(JSON.parse(raw), null, 2);
    } catch {
      // Leave the stored text as-is if it is not valid JSON.
    }
    return `
      <details class="spec-details">
        <summary>${escapeHtml(label)}</summary>
        <pre class="spec-json">${escapeHtml(pretty)}</pre>
      </details>`;
  }

  function statusCodeHistoryHtml(checks) {
    if (!checks.length) return emptyPanel(ICON.route, 'No status codes yet', 'Codes appear after the first completed check.');
    const counts = new Map();
    checks.forEach((c) => {
      const code = c.statusCode || 0;
      counts.set(code, (counts.get(code) || 0) + 1);
    });
    const total = checks.length;
    const sorted = [...counts.entries()].sort((a, b) => b[1] - a[1]);
    return `<div class="code-bars">${sorted
      .map(([code, count]) => {
        const share = (count / total) * 100;
        const tone = code === 0 ? 'down' : code < 400 ? 'ok' : code < 500 ? 'warn' : 'down';
        return `
          <div class="code-bar">
            <span class="code-label mono">${code === 0 ? 'no response' : code}</span>
            <span class="code-track"><span class="code-fill tone-${tone}" style="width:${share.toFixed(1)}%"></span></span>
            <span class="code-count mono">${count} &middot; ${share.toFixed(0)}%</span>
          </div>`;
      })
      .join('')}</div>`;
  }

  function renderRouteDetail() {
    const r = state.routeDetail.route;
    if (!r) return;
    const p = state.project.project;
    const canEdit = p && (p.viewerRole === 'owner' || p.viewerRole === 'editor');
    const checks = state.routeDetail.checks;

    pel.viewRoute.innerHTML = `
      <section class="card">
        <div class="card-header">
          <h2>${methodChip(r.method)} <span class="mono route-heading-path">${escapeHtml(r.path)}</span></h2>
          <div class="card-toolbar">
            ${healthBadge(r.status)}
            ${r.deprecated ? '<span class="proj-chip is-deprecated">deprecated</span>' : ''}
            <span class="proj-chip is-muted">${escapeHtml(r.source)}</span>
            ${canEdit ? `<button class="secondary sm" type="button" data-action="edit-route" data-id="${r.id}">Edit</button>` : ''}
            ${canEdit ? `<button class="secondary sm" type="button" data-action="${r.enabled ? 'disable' : 'enable'}-route" data-id="${r.id}">${r.enabled ? 'Disable' : 'Enable'}</button>` : ''}
            ${canEdit ? `<button class="danger sm" type="button" data-action="delete-route" data-id="${r.id}">Delete</button>` : ''}
          </div>
        </div>
        ${r.summary ? `<p class="card-subtitle">${escapeHtml(r.summary)}</p>` : ''}
        <div class="metric-row">
          ${metricTile(pct(r.uptime24hPct), 'uptime 24h')}
          ${metricTile(ms(r.avgLatency24hMs), 'avg latency 24h')}
          ${metricTile(num(r.checks24h, '0'), 'checks 24h')}
          ${metricTile(num(r.failures24h, '0'), 'failures 24h', r.failures24h > 0 ? 'down' : '')}
          ${metricTile(num(r.lastStatusCode, '–'), 'last status code')}
          ${metricTile(ms(r.lastLatencyMs), 'last latency')}
          ${metricTile(num(r.consecutiveFailures, '0'), 'consecutive failures', r.consecutiveFailures > 0 ? 'warn' : '')}
          ${metricTile(r.lastCheckedAt ? escapeHtml(relativeTime(r.lastCheckedAt)) : 'never', 'last check')}
        </div>
        ${r.lastFailureReason ? `<p class="proj-form-error">Last failure: ${escapeHtml(r.lastFailureReason)}</p>` : ''}
      </section>

      <section class="card">
        <div class="card-header"><h2>Configuration</h2></div>
        <dl class="def-list">
          ${definitionRow('Target URL', `<span class="mono">${escapeHtml((r.baseUrl || '') + r.path)}</span>`)}
          ${definitionRow('Monitoring', r.enabled ? 'enabled' : 'disabled')}
          ${definitionRow('Interval', `every ${num(r.monitorIntervalSeconds)}s`, true)}
          ${definitionRow('Timeout', `${num(r.timeoutMs)} ms`, true)}
          ${definitionRow('Retries', num(r.retries), true)}
          ${definitionRow('Expected status', escapeHtml(r.expectedStatusRange || '200-399'), true)}
          ${definitionRow('Incident rule', `opens after ${num(r.failureThreshold)} consecutive failures, resolves after ${num(r.recoverySuccesses)} successes`)}
          ${definitionRow('Next check', r.nextCheckAt ? escapeHtml(new Date(r.nextCheckAt).toLocaleString()) : '–', true)}
          ${definitionRow('Operation ID', r.operationId ? `<span class="mono">${escapeHtml(r.operationId)}</span>` : '–')}
          ${definitionRow('Tags', (r.tags || []).length ? (r.tags || []).map((t) => `<span class="proj-chip is-tag">${escapeHtml(t)}</span>`).join('') : '–')}
          ${definitionRow('Headers', r.headers ? `<span class="mono">${escapeHtml(r.headers)}</span>` : 'none')}
        </dl>
        ${r.description ? `<details class="spec-details"><summary>Description</summary><p class="spec-text">${escapeHtml(r.description)}</p></details>` : ''}
        ${jsonBlock('Parameters', r.parameters)}
        ${jsonBlock('Request body', r.requestBody)}
        ${jsonBlock('Responses', r.responses)}
        ${jsonBlock('Security requirements', r.security)}
      </section>

      <section class="card">
        <div class="card-header">
          <h2>Uptime &amp; response time</h2>
          <div class="card-toolbar">${rangePickerHtml(state.routeDetail.range, 'route-range')}</div>
        </div>
        <div class="chart-grid">
          <figure class="chart-box">
            <figcaption>Uptime %</figcaption>
            <canvas id="routeUptimeChart" height="150" role="img" aria-label="Route uptime over the selected range"></canvas>
          </figure>
          <figure class="chart-box">
            <figcaption>Response time (ms)</figcaption>
            <canvas id="routeLatencyChart" height="150" role="img" aria-label="Route response time over the selected range"></canvas>
          </figure>
        </div>
      </section>

      <section class="card">
        <div class="card-header"><h2>Status code history</h2><span class="card-subtitle">last ${checks.length} check${checks.length === 1 ? '' : 's'}</span></div>
        ${statusCodeHistoryHtml(checks)}
      </section>

      <section class="card">
        <div class="card-header"><h2>Recent checks</h2></div>
        <div class="table-wrap">
          <table>
            <thead><tr><th>Status</th><th>Code</th><th>Latency</th><th>Attempts</th><th>Checked at</th><th>Failure reason</th></tr></thead>
            <tbody>${
              checks.length
                ? checks
                    .map(
                      (c) => `
                        <tr>
                          <td><span class="badge status-${c.status === 'up' ? 'up' : 'down'}"><span class="lamp"></span>${escapeHtml(c.status)}</span></td>
                          <td class="mono">${c.statusCode ? c.statusCode : '–'}</td>
                          <td class="mono">${ms(c.latencyMs)}</td>
                          <td class="mono">${num(c.attempt, '1')}</td>
                          <td class="mono">${escapeHtml(new Date(c.checkedAt).toLocaleString())}</td>
                          <td>${c.failureReason ? escapeHtml(c.failureReason) : '–'}</td>
                        </tr>`
                    )
                    .join('')
                : `<tr><td colspan="6">${emptyPanel(ICON.route, 'No checks recorded yet', 'The background worker will check this route on its schedule.')}</td></tr>`
            }</tbody>
          </table>
        </div>
      </section>

      <section class="card">
        <div class="card-header"><h2>Incidents for this route</h2></div>
        ${incidentsListHtml(state.routeDetail.incidents, state.routeDetail.projectId, canEdit)}
      </section>`;

    drawSeriesCharts('routeUptimeChart', 'routeLatencyChart', state.routeDetail.series);
  }

  /* ------------------------------------------------------------ mini charts */

  function drawSeriesCharts(uptimeId, latencyId, series) {
    const points = (series && series.points) || [];
    drawChart(document.getElementById(uptimeId), points.map((p) => p.uptimePct), {
      labels: points.map((p) => p.bucketStart),
      min: 0,
      max: 100,
      suffix: '%',
      accent: 'ok',
      emptyText: 'No checks in this range yet.',
    });
    drawChart(document.getElementById(latencyId), points.map((p) => p.avgLatencyMs), {
      labels: points.map((p) => p.bucketStart),
      min: 0,
      suffix: ' ms',
      accent: 'signal',
      emptyText: 'No checks in this range yet.',
    });
  }

  function cssVar(name, fallback) {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || fallback;
  }

  /**
   * drawChart renders a small line/area chart on a canvas. Deliberately
   * hand-rolled: the rest of the frontend ships zero third-party JavaScript,
   * and a few dozen buckets do not justify a charting library.
   */
  function drawChart(canvas, values, opts = {}) {
    if (!canvas) return;
    const ratio = window.devicePixelRatio || 1;
    const cssWidth = canvas.clientWidth || canvas.parentElement.clientWidth || 320;
    const cssHeight = Number(canvas.getAttribute('height')) || 150;
    canvas.width = Math.round(cssWidth * ratio);
    canvas.height = Math.round(cssHeight * ratio);
    canvas.style.height = `${cssHeight}px`;

    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    ctx.clearRect(0, 0, cssWidth, cssHeight);

    const textFaint = cssVar('--text-faint', '#656b70');
    const line = cssVar('--line', '#24272c');
    const accentMap = { ok: cssVar('--ok', '#5fd97a'), signal: cssVar('--signal', '#2dd4c4'), down: cssVar('--down', '#ff5d6c') };
    const accent = accentMap[opts.accent] || accentMap.signal;

    const padLeft = 46;
    const padRight = 8;
    const padTop = 10;
    const padBottom = 20;
    const plotW = Math.max(1, cssWidth - padLeft - padRight);
    const plotH = Math.max(1, cssHeight - padTop - padBottom);

    if (!values.length) {
      ctx.fillStyle = textFaint;
      ctx.font = '12px var(--font-body), sans-serif';
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.fillText(opts.emptyText || 'No data', cssWidth / 2, cssHeight / 2);
      return;
    }

    let min = opts.min !== undefined ? opts.min : Math.min(...values);
    let max = opts.max !== undefined ? opts.max : Math.max(...values);
    if (max === min) max = min + (min === 0 ? 1 : min * 0.2);
    const span = max - min || 1;

    const x = (i) => padLeft + (values.length === 1 ? plotW / 2 : (i / (values.length - 1)) * plotW);
    const y = (v) => padTop + plotH - ((Math.min(Math.max(v, min), max) - min) / span) * plotH;

    // Gridlines + y labels.
    ctx.strokeStyle = line;
    ctx.fillStyle = textFaint;
    ctx.lineWidth = 1;
    ctx.font = '10px var(--font-mono), monospace';
    ctx.textAlign = 'right';
    ctx.textBaseline = 'middle';
    for (let i = 0; i <= 3; i += 1) {
      const value = min + (span * i) / 3;
      const gy = Math.round(y(value)) + 0.5;
      ctx.beginPath();
      ctx.moveTo(padLeft, gy);
      ctx.lineTo(padLeft + plotW, gy);
      ctx.stroke();
      ctx.fillText(`${Math.round(value)}${opts.suffix || ''}`, padLeft - 6, gy);
    }

    // Area fill.
    const gradient = ctx.createLinearGradient(0, padTop, 0, padTop + plotH);
    gradient.addColorStop(0, hexToRgba(accent, 0.28));
    gradient.addColorStop(1, hexToRgba(accent, 0));
    ctx.beginPath();
    ctx.moveTo(x(0), padTop + plotH);
    values.forEach((v, i) => ctx.lineTo(x(i), y(v)));
    ctx.lineTo(x(values.length - 1), padTop + plotH);
    ctx.closePath();
    ctx.fillStyle = gradient;
    ctx.fill();

    // Line.
    ctx.beginPath();
    values.forEach((v, i) => (i === 0 ? ctx.moveTo(x(i), y(v)) : ctx.lineTo(x(i), y(v))));
    ctx.strokeStyle = accent;
    ctx.lineWidth = 1.8;
    ctx.lineJoin = 'round';
    ctx.stroke();

    // Last point marker.
    ctx.beginPath();
    ctx.arc(x(values.length - 1), y(values[values.length - 1]), 2.6, 0, Math.PI * 2);
    ctx.fillStyle = accent;
    ctx.fill();

    // First/last time labels.
    const labels = opts.labels || [];
    if (labels.length) {
      ctx.fillStyle = textFaint;
      ctx.font = '10px var(--font-mono), monospace';
      ctx.textBaseline = 'top';
      ctx.textAlign = 'left';
      ctx.fillText(shortTime(labels[0]), padLeft, padTop + plotH + 5);
      ctx.textAlign = 'right';
      ctx.fillText(shortTime(labels[labels.length - 1]), padLeft + plotW, padTop + plotH + 5);
    }
  }

  function shortTime(iso) {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '';
    return d.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
  }

  function hexToRgba(color, alpha) {
    const hex = color.replace('#', '').trim();
    if (hex.length !== 6 && hex.length !== 3) return color;
    const full = hex.length === 3 ? hex.split('').map((c) => c + c).join('') : hex;
    const r = parseInt(full.slice(0, 2), 16);
    const g = parseInt(full.slice(2, 4), 16);
    const b = parseInt(full.slice(4, 6), 16);
    return `rgba(${r},${g},${b},${alpha})`;
  }

  /* -------------------------------------------------------- import wizard */

  function renderImportWizard() {
    showView('import');
    const step = state.importer.step;
    pel.viewImport.innerHTML = `
      <section class="card">
        <div class="card-header">
          <h2>Import an OpenAPI or Swagger specification</h2>
          <div class="card-toolbar"><a class="btn-link" href="#/projects/${state.importer.projectId}">Back to project</a></div>
        </div>
        <ol class="wizard-steps">
          ${[
            [1, 'Provide specification'],
            [2, 'Review and select'],
            [3, 'Result'],
          ]
            .map(
              ([n, label]) =>
                `<li class="wizard-step${step === n ? ' is-active' : ''}${step > n ? ' is-done' : ''}"><span class="wizard-step-num">${n}</span>${escapeHtml(label)}</li>`
            )
            .join('')}
        </ol>
        <div id="projImportBody">${step === 1 ? importStep1Html() : step === 2 ? importStep2Html() : importStep3Html()}</div>
      </section>`;

    if (step === 1) wireImportStep1();
    if (step === 2) wireImportStep2();
  }

  function importStep1Html() {
    return `
      <div class="wizard-body">
        <p class="card-subtitle">
          Supports OpenAPI 3.x and Swagger 2.0 in JSON or YAML, up to 10 MB and 5000 operations.
          References are resolved locally only — remote <code>$ref</code> URLs are never fetched.
        </p>
        <form id="projImportForm" class="stack">
          <div class="field">
            <label for="projImportFile">Upload a specification file</label>
            <input id="projImportFile" type="file" accept=".json,.yaml,.yml,application/json,text/yaml,application/x-yaml" />
          </div>
          <p class="wizard-or">or</p>
          <div class="field">
            <label for="projImportPaste">Paste the full specification</label>
            <textarea id="projImportPaste" rows="10" placeholder='{ "openapi": "3.0.0", ... }'></textarea>
          </div>
          <div class="field">
            <label for="projImportBaseUrl">Base URL override (optional)</label>
            <input id="projImportBaseUrl" placeholder="https://api.example.com" />
            <p class="hint">Leave blank to use the <code>servers</code> entry (OpenAPI 3) or <code>host</code> + <code>basePath</code> (Swagger 2).</p>
          </div>
          <p class="proj-form-error hidden" id="projImportError" role="alert"></p>
          <button type="submit" id="projImportValidate">Validate specification</button>
        </form>
      </div>`;
  }

  function wireImportStep1() {
    const form = document.getElementById('projImportForm');
    const fileInput = document.getElementById('projImportFile');
    const paste = document.getElementById('projImportPaste');
    const baseUrl = document.getElementById('projImportBaseUrl');
    const errorNode = document.getElementById('projImportError');
    const submit = document.getElementById('projImportValidate');

    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      hideFormError(errorNode);
      const file = fileInput.files && fileInput.files[0];
      const pastedText = paste.value.trim();
      if (!file && !pastedText) {
        showFormError(errorNode, 'Upload a file or paste a specification first.');
        return;
      }
      if (file && pastedText) {
        showFormError(errorNode, 'Provide either a file or pasted text, not both.');
        return;
      }
      // Fail fast in the browser too, so a huge file is not uploaded only to
      // be rejected; the server enforces the same limit authoritatively.
      const MAX_BYTES = 10 * 1024 * 1024;
      if (file && file.size > MAX_BYTES) {
        showFormError(errorNode, `That file is ${(file.size / 1048576).toFixed(1)} MB; the limit is 10 MB.`);
        return;
      }

      setButtonLoading(submit, true, 'Validating...');
      try {
        let job;
        if (file) {
          const fd = new FormData();
          fd.append('file', file);
          if (baseUrl.value.trim()) fd.append('baseUrlOverride', baseUrl.value.trim());
		  job = await apiProjects(`/import/validation/${state.importer.projectId}`, { method: 'POST', body: fd });
        } else {
		  job = await apiProjects(`/import/validation/${state.importer.projectId}`, {
            method: 'POST',
            body: JSON.stringify({ spec: pastedText, baseUrlOverride: baseUrl.value.trim() }),
          });
        }
        state.importer.job = job;
        state.importer.selections = new Map((job.items || []).map((item) => [item.key, !!item.selected]));
        state.importer.conflictFilter = 'all';
        state.importer.previewPage = 0;
        state.importer.step = 2;
        renderImportWizard();
      } catch (err) {
        if (!(err instanceof SessionExpired)) showFormError(errorNode, err.message);
      } finally {
        setButtonLoading(submit, false);
      }
    });
  }

  function previewBuckets(items) {
    return {
      all: items,
      create: items.filter((i) => i.action === 'create'),
      update: items.filter((i) => i.action === 'update'),
      unchanged: items.filter((i) => i.action === 'skip' && i.conflict === 'none'),
      duplicate: items.filter((i) => i.conflict === 'duplicate_in_spec'),
      removed: items.filter((i) => i.action === 'remove'),
    };
  }

  function importStep2Html() {
    const job = state.importer.job;
    const items = job.items || [];
    const buckets = previewBuckets(items);
    const filtered = buckets[state.importer.conflictFilter] || items;
    const selectedCount = [...state.importer.selections.values()].filter(Boolean).length;

    const pageCount = Math.max(1, Math.ceil(filtered.length / PREVIEW_PAGE_SIZE));
    const page = Math.min(state.importer.previewPage, pageCount - 1);
    const pageItems = filtered.slice(page * PREVIEW_PAGE_SIZE, (page + 1) * PREVIEW_PAGE_SIZE);

    const warnings = items.filter((i) => i.validationWarning);

    return `
      <div class="wizard-body">
        <div class="metric-row">
          ${metricTile(String(job.totalParsed), 'operations parsed')}
          ${metricTile(String(buckets.create.length), 'new', 'ok')}
          ${metricTile(String(buckets.update.length), 'changed', 'warn')}
          ${metricTile(String(buckets.unchanged.length), 'unchanged')}
          ${metricTile(String(buckets.duplicate.length), 'duplicates in spec', buckets.duplicate.length ? 'warn' : '')}
          ${metricTile(String(buckets.removed.length), 'missing from spec', buckets.removed.length ? 'down' : '')}
          ${metricTile(escapeHtml(job.specFormat === 'swagger2' ? 'Swagger 2.0' : 'OpenAPI 3.x'), 'format')}
        </div>

        ${
          buckets.removed.length
            ? `<p class="wizard-note">${buckets.removed.length} existing route${buckets.removed.length === 1 ? '' : 's'} no longer appear in this specification.
               Selecting one <strong>disables</strong> it — its history and settings are kept, and nothing is deleted.</p>`
            : ''
        }
        ${warnings.length ? `<p class="wizard-note is-warn">${warnings.length} row${warnings.length === 1 ? '' : 's'} could not be parsed and will be skipped.</p>` : ''}

        <div class="card-toolbar wizard-toolbar">
          <div class="filter-chips" role="group" aria-label="Filter preview rows">
            ${[
              ['all', `All (${items.length})`],
              ['create', `New (${buckets.create.length})`],
              ['update', `Changed (${buckets.update.length})`],
              ['unchanged', `Unchanged (${buckets.unchanged.length})`],
              ['duplicate', `Duplicates (${buckets.duplicate.length})`],
              ['removed', `Removed (${buckets.removed.length})`],
            ]
              .map(
                ([key, label]) =>
                  `<button type="button" class="filter-chip${state.importer.conflictFilter === key ? ' is-active' : ''}" data-action="preview-filter" data-filter="${key}">${escapeHtml(label)}</button>`
              )
              .join('')}
          </div>
          <div class="row-actions">
            <button class="secondary sm" type="button" data-action="preview-select-all">Select all shown</button>
            <button class="secondary sm" type="button" data-action="preview-select-none">Deselect all shown</button>
            <button class="ghost sm" type="button" data-action="preview-reset">Reset to recommended</button>
          </div>
        </div>

        <div class="table-wrap preview-table-wrap">
          <table>
            <thead>
              <tr>
                <th class="col-check"><input type="checkbox" data-action="preview-toggle-page" aria-label="Select all rows on this page" ${
                  pageItems.length && pageItems.every((i) => state.importer.selections.get(i.key)) ? 'checked' : ''
                } /></th>
                <th>Method</th><th>Path</th><th>Change</th><th>Summary</th><th>Notes</th>
              </tr>
            </thead>
            <tbody>
              ${
                pageItems.length
                  ? pageItems
                      .map(
                        (item) => `
                          <tr>
                            <td class="col-check"><input type="checkbox" data-action="preview-toggle" data-key="${escapeHtml(item.key)}" ${
                          state.importer.selections.get(item.key) ? 'checked' : ''
                        } ${item.action === 'skip' && item.conflict === 'duplicate_in_spec' ? 'disabled' : ''} aria-label="Include ${escapeHtml(item.key)}" /></td>
                            <td>${methodChip(item.method)}</td>
                            <td class="path-cell mono">${escapeHtml(item.path)}${item.deprecated ? '<span class="proj-chip is-deprecated">deprecated</span>' : ''}</td>
                            <td>${conflictBadge(item)}</td>
                            <td>${escapeHtml(item.summary || item.operationId || '')}</td>
                            <td>${item.validationWarning ? `<span class="proj-form-error inline">${escapeHtml(item.validationWarning)}</span>` : '–'}</td>
                          </tr>`
                      )
                      .join('')
                  : `<tr><td colspan="6">${emptyPanel(ICON.spec, 'Nothing in this category', 'Pick a different filter above.')}</td></tr>`
              }
            </tbody>
          </table>
        </div>

        ${
          pageCount > 1
            ? `<nav class="proj-pager" aria-label="Preview pagination">
                 <button class="secondary sm" type="button" data-action="preview-page" data-page="${page - 1}" ${page === 0 ? 'disabled' : ''}>Previous</button>
                 <span class="proj-pager-label">Page ${page + 1} of ${pageCount} &middot; ${filtered.length} rows</span>
                 <button class="secondary sm" type="button" data-action="preview-page" data-page="${page + 1}" ${page + 1 >= pageCount ? 'disabled' : ''}>Next</button>
               </nav>`
            : ''
        }

        <p class="proj-form-error hidden" id="projCommitError" role="alert"></p>
        <div class="wizard-actions">
          <button class="secondary" type="button" data-action="import-restart">Start over</button>
          <span class="wizard-summary"><strong>${selectedCount}</strong> of ${items.length} row${items.length === 1 ? '' : 's'} selected</span>
          <button type="button" id="projCommitBtn" data-action="import-commit" ${selectedCount === 0 ? 'disabled' : ''}>Apply ${selectedCount} change${selectedCount === 1 ? '' : 's'}</button>
        </div>
      </div>`;
  }

  function wireImportStep2() {
    // All controls are handled by the delegated click listener below.
  }

  function importStep3Html() {
    const job = state.importer.result;
    if (!job) return emptyPanel(ICON.spec, 'Nothing imported yet', 'Start again from step one.');
    const warnings = (job.items || []).filter((i) => i.validationWarning);
    return `
      <div class="wizard-body">
        <div class="metric-row">
          ${metricTile(String(job.createdRoutes), 'routes created', 'ok')}
          ${metricTile(String(job.updatedRoutes), 'metadata updated', 'warn')}
          ${metricTile(String(job.removedRoutes), 'routes disabled', job.removedRoutes ? 'down' : '')}
          ${metricTile(String(job.skippedRoutes), 'skipped')}
          ${metricTile(String(job.totalParsed), 'operations parsed')}
        </div>
        <p class="wizard-note">
          Imported metadata was refreshed on changed routes. Monitoring settings you had customised —
          interval, timeout, retries, expected status range, thresholds and headers — were left untouched.
        </p>
        ${
          warnings.length
            ? `<div class="table-wrap">
                 <table>
                   <thead><tr><th>Route</th><th>Warning</th></tr></thead>
                   <tbody>${warnings
                     .map((i) => `<tr><td class="mono">${escapeHtml(i.key || `${i.method} ${i.path}`)}</td><td>${escapeHtml(i.validationWarning)}</td></tr>`)
                     .join('')}</tbody>
                 </table>
               </div>`
            : '<p class="hint">No per-row warnings.</p>'
        }
        <div class="wizard-actions">
          <button class="secondary" type="button" data-action="import-restart">Import another specification</button>
          <a class="btn-link" href="#/projects/${state.importer.projectId}">Back to project dashboard</a>
        </div>
      </div>`;
  }

  async function commitImport() {
    const job = state.importer.job;
    if (!job || state.importer.busy) return;
    const errorNode = document.getElementById('projCommitError');
    const btn = document.getElementById('projCommitBtn');
    state.importer.busy = true;
    if (errorNode) hideFormError(errorNode);
    setButtonLoading(btn, true, 'Applying...');
    try {
      const selections = (job.items || []).map((item) => ({
        key: item.key,
        selected: !!state.importer.selections.get(item.key),
        action: item.action,
      }));
	  const result = await apiProjects(`/import/commit/${state.importer.projectId}/${job.id}`, {
        method: 'POST',
        body: JSON.stringify({ selections }),
      });
      state.importer.result = result;
      state.importer.step = 3;
      renderImportWizard();
      showToast(`Import complete: ${result.createdRoutes} created, ${result.updatedRoutes} updated, ${result.removedRoutes} disabled.`, 'success');
      // The project's cached counters change, so drop the stale copy.
      if (state.project.id === state.importer.projectId) state.project.project = null;
    } catch (err) {
      if (!(err instanceof SessionExpired)) {
        if (errorNode) showFormError(errorNode, err.message);
        else reportError('Import failed', err);
      }
    } finally {
      state.importer.busy = false;
      setButtonLoading(btn, false);
    }
  }

  /* ---------------------------------------------------------------- modals */

  function openModal(overlay) {
    modalReturnFocus = document.activeElement;
    overlay.classList.remove('hidden');
    syncModalBackground();
    const focusable = focusableIn(overlay)[0];
    if (focusable) focusable.focus();
  }

  function closeModal(overlay) {
    overlay.classList.add('hidden');
    syncModalBackground();
    if (modalReturnFocus && typeof modalReturnFocus.focus === 'function') modalReturnFocus.focus();
    modalReturnFocus = null;
  }

  function focusableIn(overlay) {
    return [...overlay.querySelectorAll('button:not([disabled]), [href], input:not([type="hidden"]):not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')]
      .filter((element) => !element.closest('[hidden], .hidden'));
  }

  function activeProjectModal() {
    return projectModals.find((overlay) => !overlay.classList.contains('hidden')) || null;
  }

  function syncModalBackground() {
    const hasProjectModal = Boolean(activeProjectModal());
    const topbar = document.querySelector('.topbar');
    const main = document.getElementById('main');
    if (topbar) topbar.inert = hasProjectModal;
    if (main) main.inert = hasProjectModal;
  }

  projectModals.forEach((overlay) => {
    overlay.addEventListener('click', (e) => {
      if (e.target !== overlay) return;
      if (overlay === pel.telemetrySecretModal) closeTelemetrySecret();
      else if (overlay === pel.heartbeatSecretModal) closeHeartbeatSecret();
      else closeModal(overlay);
    });
  });

  document.addEventListener('keydown', (e) => {
    const overlay = activeProjectModal();
    if (!overlay) return;
    if (e.key === 'Escape') {
      if (overlay === pel.telemetrySecretModal) closeTelemetrySecret();
      else if (overlay === pel.heartbeatSecretModal) closeHeartbeatSecret();
      else closeModal(overlay);
      return;
    }
    if (e.key !== 'Tab') return;
    const focusable = focusableIn(overlay);
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
  });

  function askConfirm({ title, body, confirmLabel = 'Confirm', danger = true }, onConfirm) {
    pel.confirmTitle.textContent = title;
    pel.confirmBody.textContent = body;
    pel.confirmOk.textContent = confirmLabel;
    pel.confirmOk.className = danger ? 'danger' : '';
    confirmAction = onConfirm;
    openModal(pel.confirmModal);
  }

  pel.confirmCancel.addEventListener('click', () => {
    confirmAction = null;
    closeModal(pel.confirmModal);
  });

  pel.confirmOk.addEventListener('click', async () => {
    const action = confirmAction;
    if (!action) return;
    setButtonLoading(pel.confirmOk, true, 'Working...');
    try {
      await action();
      closeModal(pel.confirmModal);
    } catch (err) {
      reportError('Action failed', err);
    } finally {
      confirmAction = null;
      setButtonLoading(pel.confirmOk, false);
    }
  });

  /* ------------------------------------------------------ project form modal */

  let editingProjectId = null;

  function readOnboardingDraft() {
    try {
      const draft = JSON.parse(localStorage.getItem(ONBOARDING_DRAFT_KEY) || '{}');
      return draft && typeof draft === 'object' ? draft : {};
    } catch (_) {
      return {};
    }
  }

  function saveOnboardingDraft() {
    if (editingProjectId || state.onboarding.createdProject) return;
    const selected = document.querySelector('input[name="projSource"]:checked');
    try {
      localStorage.setItem(ONBOARDING_DRAFT_KEY, JSON.stringify({
        name: document.getElementById('projFormName').value.trim(),
        description: document.getElementById('projFormDescription').value.trim(),
        environmentName: document.getElementById('projFormEnvironmentName').value.trim(),
        environmentBaseUrl: document.getElementById('projFormEnvironmentBaseUrl').value.trim(),
        source: selected ? selected.value : 'telemetry',
      }));
    } catch (_) {
      // Draft preservation is a convenience only; project creation remains server-authoritative.
    }
  }

  function clearOnboardingDraft() {
    try { localStorage.removeItem(ONBOARDING_DRAFT_KEY); } catch (_) { /* ignore unavailable storage */ }
  }

  function selectedOnboardingSource() {
    const selected = document.querySelector('input[name="projSource"]:checked');
    return selected ? selected.value : 'telemetry';
  }

  function sourceSummary(source) {
    return {
      telemetry: ['OpenTelemetry', 'Create an environment and issue a scoped OTLP credential. Argus will receive passive metrics; it will not probe your service.'],
      openapi: ['OpenAPI catalog', 'Import a specification after creation. It adds catalog entries only and sends zero requests to imported operations.'],
      synthetic: ['Synthetic check', 'Create a disabled GET or HEAD canary after creation, then review its target and request budget before enabling it.'],
      heartbeat: ['Heartbeat', 'Create a project first; heartbeat setup is the next recommended action for jobs and scheduled workloads.'],
      sample: ['Sample data', 'This guided evaluation option is non-production only. It does not connect to or report health for your service.'],
      later: ['Do this later', 'Create a project with no connected signal. You can connect telemetry, an OpenAPI catalog, a safe canary, or a heartbeat later.'],
    }[source] || ['Monitoring source', 'Connect a monitoring source after the project is created.'];
  }

  function setOnboardingStep(step) {
    state.onboarding.step = step;
    const isEdit = Boolean(editingProjectId);
    pel.projectSteps.classList.toggle('hidden', isEdit);
    pel.projectAdvanced.classList.toggle('hidden', !isEdit);
    pel.projectForm.querySelectorAll('[data-project-step]').forEach((section) => {
      section.classList.toggle('hidden', isEdit || Number(section.dataset.projectStep) !== step);
    });
    pel.projectSteps.querySelectorAll('.wizard-step').forEach((item, index) => {
      const current = index + 1 === step;
      item.classList.toggle('is-active', current);
      item.classList.toggle('is-done', index + 1 < step);
      if (current) item.setAttribute('aria-current', 'step');
      else item.removeAttribute('aria-current');
    });
    const created = Boolean(state.onboarding.createdProject);
    pel.projectBack.classList.toggle('hidden', isEdit || created || step <= 1 || step >= 7);
    pel.projectCancel.textContent = step === 7 ? 'Close' : 'Cancel';
    pel.projectSubmit.classList.toggle('hidden', step >= 4);
    pel.projectSubmit.textContent = isEdit ? 'Save project' : step === 3 ? 'Create project' : 'Continue';
    if (step === 3) {
      const [title, detail] = sourceSummary(selectedOnboardingSource());
      pel.projectReview.replaceChildren();
      const strong = document.createElement('strong');
      strong.textContent = title;
      const p = document.createElement('p');
      p.textContent = detail;
      pel.projectReview.append(strong, p);
    }
	if (!isEdit && step > 1) {
		const activeStep = pel.projectForm.querySelector(`[data-project-step="${step}"]`);
		const title = activeStep && activeStep.querySelector('legend, h4');
		if (title) {
			title.tabIndex = -1;
			title.focus();
		}
	}
  }

  function finishOnboarding(project) {
    const source = selectedOnboardingSource();
    const [title, detail] = sourceSummary(source);
    state.onboarding.createdProject = project;
    clearOnboardingDraft();
    pel.projectVerification.replaceChildren();
    const strong = document.createElement('strong');
    strong.textContent = `${project.name} was created. Signal verification is still pending.`;
    const p = document.createElement('p');
    p.textContent = `${detail} Argus does not treat this project as verified until it receives telemetry, a heartbeat, or an explicitly configured check.`;
    const link = document.createElement('a');
    link.className = 'btn-link';
    link.href = source === 'openapi' ? `#/projects/${project.id}/import` : `#/projects/${project.id}`;
    link.textContent = source === 'openapi' ? 'Import your OpenAPI catalog' : 'Open project dashboard to connect and verify';
    const sourceAction = {
      telemetry: ['Connect OpenTelemetry', openTelemetryCredentialModal],
      synthetic: ['Create disabled canary', () => openRouteModal(null)],
      heartbeat: ['Create heartbeat', openHeartbeatModal],
    }[source];
    let sourceButton;
    if (sourceAction) {
      const connect = document.createElement('button');
      connect.type = 'button';
      connect.className = 'secondary sm';
      connect.textContent = sourceAction[0];
      connect.addEventListener('click', () => openOnboardingSource(project, sourceAction[1]));
      sourceButton = connect;
    }
    const continueToSLO = document.createElement('button');
    continueToSLO.type = 'button';
    continueToSLO.className = 'secondary sm';
    continueToSLO.textContent = 'Continue to starter SLO';
    continueToSLO.addEventListener('click', () => setOnboardingStep(5));
    pel.projectVerification.append(strong, p, ...(sourceButton ? [sourceButton] : []), link, continueToSLO);

    pel.projectSLO.replaceChildren();
    const sloIntro = document.createElement('p');
    sloIntro.textContent = 'An availability objective gives the first verified signal a clear service-level target. It remains no data until eligible evidence arrives.';
    const starterSLO = document.createElement('button');
    starterSLO.type = 'button';
    starterSLO.className = 'secondary sm';
    starterSLO.textContent = 'Create starter availability SLO';
    starterSLO.addEventListener('click', async () => {
      setButtonLoading(starterSLO, true, 'Creating...');
      try {
        await apiProjects(`/slo/catalog/${project.id}`, {
          method: 'POST',
          body: JSON.stringify({
            name: 'Availability',
            sliKind: 'availability',
            targetPercent: 99.9,
            windowSeconds: 30 * 86400,
            minEvents: 100,
          }),
        });
        starterSLO.disabled = true;
        starterSLO.textContent = 'Starter availability SLO created';
        showToast('Starter availability SLO created. It will remain no data until eligible telemetry arrives.', 'success');
      } catch (err) {
        if (!(err instanceof SessionExpired)) showToast(`Could not create the starter SLO: ${err.message}`, 'error');
        setButtonLoading(starterSLO, false);
      }
    });
    const skipSLO = document.createElement('button');
    skipSLO.type = 'button';
    skipSLO.className = 'secondary sm';
    skipSLO.textContent = 'Continue without an SLO';
    skipSLO.addEventListener('click', () => setOnboardingStep(6));
    const continueAfterSLO = document.createElement('button');
    continueAfterSLO.type = 'button';
    continueAfterSLO.className = 'secondary sm';
    continueAfterSLO.textContent = 'Continue to notifications';
    continueAfterSLO.addEventListener('click', () => setOnboardingStep(6));
    pel.projectSLO.append(sloIntro, starterSLO, skipSLO, continueAfterSLO);

    pel.projectNotification.replaceChildren();
    const notificationIntro = document.createElement('p');
    notificationIntro.textContent = 'Connect a notification channel before relying on incidents. Open the Notifications tab to create a webhook, Slack, or email channel; no channel is silently enabled here.';
    const finish = document.createElement('button');
    finish.type = 'button';
    finish.className = 'secondary sm';
    finish.textContent = 'Finish setup';
    finish.addEventListener('click', () => setOnboardingStep(7));
    pel.projectNotification.append(notificationIntro, finish);

    pel.projectNext.replaceChildren();
    const complete = document.createElement('strong');
    complete.textContent = `${project.name} is ready for its first verified signal.`;
    const completeDetail = document.createElement('p');
    completeDetail.textContent = 'The dashboard keeps source freshness, SLO state, and incident evidence together. Return after connecting your source to confirm that Argus has received evidence.';
    pel.projectNext.append(complete, completeDetail, link.cloneNode(true));
    setOnboardingStep(4);
  }

  function openOnboardingSource(project, openDialog) {
    closeModal(pel.projectModal);
    navigate(`#/projects/${project.id}`);
    const deadline = Date.now() + 5000;
    const waitForProject = () => {
      if (state.project.id === project.id && state.project.project) {
        openDialog();
      } else if (Date.now() < deadline) {
        window.setTimeout(waitForProject, 50);
      } else {
        showToast('Project opened. Choose the recommended setup action from its dashboard.', 'info');
      }
    };
    waitForProject();
  }

  function openProjectModal(project) {
    editingProjectId = project ? project.id : null;
    pel.projectModalTitle.textContent = project ? `Edit ${project.name}` : 'New project';
    const draft = project ? {} : readOnboardingDraft();
    document.getElementById('projFormName').value = project ? project.name : draft.name || '';
    document.getElementById('projFormDescription').value = project ? project.description || '' : draft.description || '';
    document.getElementById('projFormEnvironmentName').value = project ? 'production' : draft.environmentName || 'production';
    document.getElementById('projFormEnvironmentBaseUrl').value = project ? '' : draft.environmentBaseUrl || '';
    document.getElementById('projFormInterval').value = project ? project.defaultIntervalSeconds : 300;
    document.getElementById('projFormTimeout').value = project ? project.defaultTimeoutMs : 5000;
    document.getElementById('projFormRetries').value = project ? project.defaultRetries : 1;
    document.getElementById('projFormFailureThreshold').value = project ? project.failureThreshold : 3;
    document.getElementById('projFormRecovery').value = project ? project.recoverySuccessThreshold : 1;
    const source = project ? 'telemetry' : draft.source || 'telemetry';
    const sourceInput = document.querySelector(`input[name="projSource"][value="${source}"]`);
    if (sourceInput) sourceInput.checked = true;
    state.onboarding.createdProject = null;
    setOnboardingStep(1);
    hideFormError(pel.projectFormError);
    openModal(pel.projectModal);
  }

  let editingEnvironmentId = null;
  function openEnvironmentModal(environment = null) {
    pel.environmentForm.reset();
    editingEnvironmentId = environment ? Number(environment.id) : null;
    document.getElementById('projEnvironmentModalTitle').textContent = environment ? `Edit ${environment.name}` : 'Add environment';
    pel.environmentSubmit.textContent = environment ? 'Save environment' : 'Create environment';
    if (environment) {
      pel.environmentName.value = environment.name || '';
      pel.environmentBaseURL.value = environment.canonicalBaseUrl || '';
    }
    hideFormError(pel.environmentFormError);
    openModal(pel.environmentModal);
  }

  pel.environmentCancel.addEventListener('click', () => closeModal(pel.environmentModal));
  pel.environmentForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.environmentFormError);
    setButtonLoading(pel.environmentSubmit, true, 'Creating...');
    try {
      await apiProjects(`/environment/catalog/${state.project.id}${editingEnvironmentId ? `/${editingEnvironmentId}` : ''}`, {
        method: editingEnvironmentId ? 'PUT' : 'POST',
        body: JSON.stringify({ name: pel.environmentName.value, baseUrl: pel.environmentBaseURL.value }),
      });
      closeModal(pel.environmentModal);
      showToast(editingEnvironmentId ? 'Environment updated.' : 'Environment created.', 'success');
      loadProjectDetail({ silent: true });
    } catch (err) {
      if (!(err instanceof SessionExpired)) showFormError(pel.environmentFormError, err.message);
    } finally {
      setButtonLoading(pel.environmentSubmit, false);
    }
  });

  function openTelemetryCredentialModal() {
    const environments = state.project.environments || [];
    pel.telemetryCredentialForm.reset();
    pel.telemetryCredentialExpiry.value = '90';
    pel.telemetryCredentialEnvironment.replaceChildren();
    environments.forEach((environment) => {
      const option = document.createElement('option');
      option.value = String(environment.id);
      option.textContent = environment.isDefault ? `${environment.name} (default)` : environment.name;
      pel.telemetryCredentialEnvironment.append(option);
    });
    pel.telemetryCredentialSubmit.disabled = environments.length === 0;
    hideFormError(pel.telemetryCredentialFormError);
    if (!environments.length) showFormError(pel.telemetryCredentialFormError, 'Create an environment before issuing a telemetry credential.');
    openModal(pel.telemetryCredentialModal);
  }

  function closeTelemetrySecret() {
    closeModal(pel.telemetrySecretModal);
    pel.telemetrySecretValue.value = '';
  }

  pel.telemetryCredentialCancel.addEventListener('click', () => closeModal(pel.telemetryCredentialModal));
  pel.telemetryCredentialForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.telemetryCredentialFormError);
    const expiresInDays = Number(pel.telemetryCredentialExpiry.value);
    if (!Number.isInteger(expiresInDays) || expiresInDays < 1 || expiresInDays > 365) {
      showFormError(pel.telemetryCredentialFormError, 'Expiry must be a whole number from 1 to 365 days.');
      return;
    }
    setButtonLoading(pel.telemetryCredentialSubmit, true, 'Creating...');
    try {
	  const issued = await apiProjects(`/telemetry/credentials/${state.project.id}`, {
        method: 'POST',
        body: JSON.stringify({
          name: pel.telemetryCredentialName.value.trim(),
          environmentId: Number(pel.telemetryCredentialEnvironment.value),
          expiresInDays,
        }),
      });
      if (!issued || !issued.token) throw new Error('The server did not return a telemetry secret.');
      closeModal(pel.telemetryCredentialModal);
      pel.telemetrySecretValue.value = issued.token;
      openModal(pel.telemetrySecretModal);
      showToast('Telemetry credential created. Save the secret before closing this dialog.', 'success');
    } catch (err) {
      if (!(err instanceof SessionExpired)) showFormError(pel.telemetryCredentialFormError, err.message);
    } finally {
      setButtonLoading(pel.telemetryCredentialSubmit, false);
    }
  });

  pel.telemetrySecretCopy.addEventListener('click', async () => {
    const secret = pel.telemetrySecretValue.value;
    if (!secret) return;
    try {
      if (!navigator.clipboard || !navigator.clipboard.writeText) throw new Error('Clipboard access is unavailable');
      await navigator.clipboard.writeText(secret);
      showToast('Telemetry secret copied. Keep it out of source control.', 'success');
    } catch (err) {
      pel.telemetrySecretValue.focus();
      pel.telemetrySecretValue.select();
      showToast('Select and copy the secret manually; browser clipboard access was unavailable.', 'info');
    }
  });
  pel.telemetrySecretClose.addEventListener('click', closeTelemetrySecret);

  function openTelemetryMappingModal() {
    const { environments, routes } = state.project;
    pel.telemetryMappingForm.reset();
    pel.telemetryMappingEnvironment.replaceChildren();
    pel.telemetryMappingRoute.replaceChildren();
    environments.forEach((environment) => {
      const option = document.createElement('option');
      option.value = String(environment.id);
      option.textContent = environment.isDefault ? `${environment.name} (default)` : environment.name;
      pel.telemetryMappingEnvironment.append(option);
    });
    routes.forEach((route) => {
      const option = document.createElement('option');
      option.value = String(route.id);
      option.textContent = `${route.method} ${route.path}`;
      pel.telemetryMappingRoute.append(option);
    });
    const unavailable = !environments.length || !routes.length;
    pel.telemetryMappingSubmit.disabled = unavailable;
    hideFormError(pel.telemetryMappingFormError);
    if (unavailable) showFormError(pel.telemetryMappingFormError, 'Create an environment and at least one catalog route before mapping telemetry.');
    openModal(pel.telemetryMappingModal);
  }

  pel.telemetryMappingCancel.addEventListener('click', () => closeModal(pel.telemetryMappingModal));
  pel.telemetryMappingForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.telemetryMappingFormError);
    const serviceName = pel.telemetryMappingService.value.trim();
    const environmentID = Number(pel.telemetryMappingEnvironment.value);
    const routeID = Number(pel.telemetryMappingRoute.value);
    if (!serviceName || !Number.isInteger(environmentID) || environmentID <= 0 || !Number.isInteger(routeID) || routeID <= 0) {
      showFormError(pel.telemetryMappingFormError, 'Choose an environment and catalog route, then provide the service name.');
      return;
    }
    setButtonLoading(pel.telemetryMappingSubmit, true, 'Creating...');
    try {
      await apiProjects(`/telemetry/mappings/${state.project.id}`, {
        method: 'POST',
        body: JSON.stringify({
          environmentId: environmentID,
          routeId: routeID,
          serviceName,
          deploymentEnvironment: pel.telemetryMappingDeployment.value.trim(),
        }),
      });
      closeModal(pel.telemetryMappingModal);
      showToast('Telemetry route mapping created.', 'success');
      loadProjectDetail({ silent: true });
    } catch (err) {
      if (!(err instanceof SessionExpired)) showFormError(pel.telemetryMappingFormError, err.message);
    } finally {
      setButtonLoading(pel.telemetryMappingSubmit, false);
    }
  });

  function syncSLOKindFields() {
    const latency = pel.sloKind.value === 'latency';
    pel.sloLatencyField.classList.toggle('hidden', !latency);
    pel.sloLatencyThreshold.required = latency;
  }

  function openSLOModal() {
    pel.sloForm.reset();
    pel.sloKind.value = 'availability';
    pel.sloTarget.value = '99.9';
    pel.sloWindowDays.value = '30';
    pel.sloMinEvents.value = '100';
    pel.sloLatencyThreshold.value = '500';
    syncSLOKindFields();
    hideFormError(pel.sloFormError);
    openModal(pel.sloModal);
  }

  pel.sloKind.addEventListener('change', syncSLOKindFields);
  pel.sloCancel.addEventListener('click', () => closeModal(pel.sloModal));
  pel.sloForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.sloFormError);
    const targetPercent = Number(pel.sloTarget.value);
    const windowDays = Number(pel.sloWindowDays.value);
    const minEvents = Number(pel.sloMinEvents.value);
    const latency = pel.sloKind.value === 'latency';
    const latencyThresholdMS = Number(pel.sloLatencyThreshold.value);
    if (!pel.sloName.value.trim() || !Number.isFinite(targetPercent) || targetPercent <= 0 || targetPercent >= 100 || !Number.isInteger(windowDays) || windowDays < 1 || windowDays > 90 || !Number.isInteger(minEvents) || minEvents < 0 || (latency && (!Number.isInteger(latencyThresholdMS) || latencyThresholdMS < 1))) {
      showFormError(pel.sloFormError, 'Provide a name, a target between 0% and 100%, a 1–90 day window, and valid event settings.');
      return;
    }
    setButtonLoading(pel.sloSubmit, true, 'Creating...');
    try {
      await apiProjects(`/slo/catalog/${state.project.id}`, {
        method: 'POST',
        body: JSON.stringify({
          name: pel.sloName.value.trim(),
          sliKind: pel.sloKind.value,
          targetPercent,
          windowSeconds: windowDays * 86400,
          latencyThresholdMs: latency ? latencyThresholdMS : 0,
          minEvents,
        }),
      });
      closeModal(pel.sloModal);
      showToast('SLO created. It will show no data until eligible telemetry arrives.', 'success');
      loadProjectDetail({ silent: true });
    } catch (err) {
      if (!(err instanceof SessionExpired)) showFormError(pel.sloFormError, err.message);
    } finally {
      setButtonLoading(pel.sloSubmit, false);
    }
  });

  function openHeartbeatModal() {
    pel.heartbeatForm.reset();
    pel.heartbeatInterval.value = '5';
    pel.heartbeatGrace.value = '5';
    pel.heartbeatEnvironment.replaceChildren();
    state.project.environments.forEach((environment) => {
      const option = document.createElement('option');
      option.value = String(environment.id);
      option.textContent = environment.name;
      pel.heartbeatEnvironment.append(option);
    });
    const unavailable = !state.project.environments.length;
    pel.heartbeatSubmit.disabled = unavailable;
    hideFormError(pel.heartbeatFormError);
    if (unavailable) showFormError(pel.heartbeatFormError, 'Create an environment before creating a heartbeat monitor.');
    openModal(pel.heartbeatModal);
  }
  function closeHeartbeatSecret() { closeModal(pel.heartbeatSecretModal); pel.heartbeatSecretValue.value = ''; }
  pel.heartbeatCancel.addEventListener('click', () => closeModal(pel.heartbeatModal));
  pel.heartbeatForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    const interval = Number(pel.heartbeatInterval.value); const grace = Number(pel.heartbeatGrace.value);
    if (!pel.heartbeatName.value.trim() || !Number.isInteger(interval) || interval < 1 || interval > 10080 || !Number.isInteger(grace) || grace < 1 || grace > 10080) {
      showFormError(pel.heartbeatFormError, 'Provide a name and intervals from 1 minute through 7 days.'); return;
    }
    setButtonLoading(pel.heartbeatSubmit, true, 'Creating...'); hideFormError(pel.heartbeatFormError);
    try {
      const issued = await apiProjects(`/heartbeat/catalog/${state.project.id}`, { method: 'POST', body: JSON.stringify({ name: pel.heartbeatName.value.trim(), environmentId: Number(pel.heartbeatEnvironment.value), expectedIntervalSeconds: interval * 60, gracePeriodSeconds: grace * 60 }) });
      closeModal(pel.heartbeatModal); pel.heartbeatSecretValue.value = issued.token; openModal(pel.heartbeatSecretModal); loadProjectDetail({ silent: true });
    } catch (err) { if (!(err instanceof SessionExpired)) showFormError(pel.heartbeatFormError, err.message); } finally { setButtonLoading(pel.heartbeatSubmit, false); }
  });
  pel.heartbeatSecretCopy.addEventListener('click', async () => { try { await navigator.clipboard.writeText(pel.heartbeatSecretValue.value); showToast('Heartbeat token copied.', 'success'); } catch (_) { pel.heartbeatSecretValue.select(); document.execCommand('copy'); showToast('Heartbeat token selected for copying.', 'info'); } });
  pel.heartbeatSecretClose.addEventListener('click', closeHeartbeatSecret);

  function openAgentModal() {
    pel.agentForm.reset();
    pel.agentInterval.value = '60';
    pel.agentEnvironment.replaceChildren();
    state.project.environments.forEach((environment) => {
      const option = document.createElement('option');
      option.value = String(environment.id);
      option.textContent = environment.name;
      pel.agentEnvironment.append(option);
    });
    const unavailable = !state.project.environments.length;
    pel.agentSubmit.disabled = unavailable;
    hideFormError(pel.agentFormError);
    if (unavailable) showFormError(pel.agentFormError, 'Create an environment before enrolling an agent.');
    openModal(pel.agentModal);
  }
  function closeAgentSecret() { closeModal(pel.agentSecretModal); pel.agentSecretValue.value = ''; }
  pel.agentCancel.addEventListener('click', () => closeModal(pel.agentModal));
  pel.agentForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    const interval = Number(pel.agentInterval.value);
    const environmentID = Number(pel.agentEnvironment.value);
    if (!pel.agentName.value.trim() || !Number.isInteger(environmentID) || environmentID <= 0 || !Number.isInteger(interval) || interval < 15 || interval > 86400) {
      showFormError(pel.agentFormError, 'Provide a name, environment, and a heartbeat interval from 15 seconds through 24 hours.'); return;
    }
    setButtonLoading(pel.agentSubmit, true, 'Creating...'); hideFormError(pel.agentFormError);
    try {
      const issued = await apiProjects(`/agent/catalog/${state.project.id}`, { method: 'POST', body: JSON.stringify({ name: pel.agentName.value.trim(), environmentId: environmentID, expectedIntervalSeconds: interval }) });
      closeModal(pel.agentModal); pel.agentSecretValue.value = issued.enrollmentToken; openModal(pel.agentSecretModal); loadProjectDetail({ silent: true });
    } catch (err) { if (!(err instanceof SessionExpired)) showFormError(pel.agentFormError, err.message); } finally { setButtonLoading(pel.agentSubmit, false); }
  });
  pel.agentSecretCopy.addEventListener('click', async () => { try { await navigator.clipboard.writeText(pel.agentSecretValue.value); showToast('Agent token copied. Keep it out of source control.', 'success'); } catch (_) { pel.agentSecretValue.select(); document.execCommand('copy'); showToast('Agent token selected for copying.', 'info'); } });
  pel.agentSecretClose.addEventListener('click', closeAgentSecret);

  function openAgentAssignmentModal() {
    pel.agentAssignmentForm.reset();
    pel.agentAssignmentInterval.value = '60'; pel.agentAssignmentTimeout.value = '5000';
    pel.agentAssignmentEnvironment.replaceChildren(); pel.agentAssignmentRoute.replaceChildren();
    state.project.environments.forEach((environment) => { const option = document.createElement('option'); option.value = environment.id; option.textContent = environment.name; pel.agentAssignmentEnvironment.append(option); });
    state.project.routes.filter((route) => route.method === 'GET' || route.method === 'HEAD').forEach((route) => { const option = document.createElement('option'); option.value = route.id; option.textContent = `${route.method} ${route.path}`; pel.agentAssignmentRoute.append(option); });
    const unavailable = !state.project.environments.length || !pel.agentAssignmentRoute.options.length;
    pel.agentAssignmentSubmit.disabled = unavailable; hideFormError(pel.agentAssignmentFormError);
    if (unavailable) showFormError(pel.agentAssignmentFormError, 'Create an environment and a GET or HEAD catalog route first.');
    openModal(pel.agentAssignmentModal);
  }
  pel.agentAssignmentCancel.addEventListener('click', () => closeModal(pel.agentAssignmentModal));
  pel.agentAssignmentForm.addEventListener('submit', async (event) => {
    event.preventDefault(); const interval = Number(pel.agentAssignmentInterval.value); const timeout = Number(pel.agentAssignmentTimeout.value);
    if (!pel.agentAssignmentName.value.trim() || !pel.agentAssignmentTarget.value.trim() || !Number.isInteger(interval) || interval < 15 || interval > 86400 || !Number.isInteger(timeout) || timeout < 200 || timeout > 60000) { showFormError(pel.agentAssignmentFormError, 'Provide a name, target, 15-second to 24-hour interval, and 200–60000 ms timeout.'); return; }
    setButtonLoading(pel.agentAssignmentSubmit, true, 'Creating...'); hideFormError(pel.agentAssignmentFormError);
    try { await apiProjects(`/agent/assignments/${state.project.id}`, { method: 'POST', body: JSON.stringify({ name: pel.agentAssignmentName.value.trim(), environmentId: Number(pel.agentAssignmentEnvironment.value), routeId: Number(pel.agentAssignmentRoute.value), method: pel.agentAssignmentMethod.value, target: pel.agentAssignmentTarget.value.trim(), intervalSeconds: interval, timeoutMs: timeout }) }); closeModal(pel.agentAssignmentModal); showToast('Private-agent assignment created.', 'success'); loadProjectDetail({ silent: true }); } catch (err) { if (!(err instanceof SessionExpired)) showFormError(pel.agentAssignmentFormError, err.message); } finally { setButtonLoading(pel.agentAssignmentSubmit, false); }
  });

  pel.projectCancel.addEventListener('click', () => {
    saveOnboardingDraft();
    closeModal(pel.projectModal);
  });
  pel.projectBack.addEventListener('click', () => setOnboardingStep(Math.max(1, state.onboarding.step - 1)));

  pel.projectForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.projectFormError);
    if (!editingProjectId && state.onboarding.step < 3) {
      const name = document.getElementById('projFormName').value.trim();
      if (!name) {
        showFormError(pel.projectFormError, 'A project name is required.');
        document.getElementById('projFormName').focus();
        return;
      }
      saveOnboardingDraft();
      setOnboardingStep(state.onboarding.step + 1);
      return;
    }
    const payload = {
      name: document.getElementById('projFormName').value.trim(),
      description: document.getElementById('projFormDescription').value.trim(),
      environmentName: document.getElementById('projFormEnvironmentName').value.trim(),
      environmentBaseUrl: document.getElementById('projFormEnvironmentBaseUrl').value.trim(),
      defaultIntervalSeconds: Number(document.getElementById('projFormInterval').value) || 0,
      defaultTimeoutMs: Number(document.getElementById('projFormTimeout').value) || 0,
      defaultRetries: Number(document.getElementById('projFormRetries').value) || 0,
      failureThreshold: Number(document.getElementById('projFormFailureThreshold').value) || 0,
      recoverySuccessThreshold: Number(document.getElementById('projFormRecovery').value) || 0,
    };
    if (!payload.name) {
      showFormError(pel.projectFormError, 'A project name is required.');
      return;
    }
    setButtonLoading(pel.projectSubmit, true, 'Saving...');
    try {
      if (editingProjectId) {
		await apiProjects(`/project/catalog/${editingProjectId}`, { method: 'PUT', body: JSON.stringify(payload) });
        showToast('Project updated.', 'success');
      } else {
		const created = await apiProjects('/project/catalog', { method: 'POST', body: JSON.stringify(payload) });
        showToast(`Project "${created.name}" created.`, 'success');
        finishOnboarding(created);
        return;
      }
      closeModal(pel.projectModal);
      if (state.route.name === 'list') loadProjectsList();
      else {
        state.project.project = null;
        loadProjectDetail();
      }
    } catch (err) {
      if (!(err instanceof SessionExpired)) showFormError(pel.projectFormError, err.message);
    } finally {
      setButtonLoading(pel.projectSubmit, false);
    }
  });

  document.querySelectorAll('input[name="projSource"]').forEach((input) => input.addEventListener('change', saveOnboardingDraft));

  /* -------------------------------------------------------- route form modal */

  let editingRouteId = null;

  function updateRouteBudgetPreview() {
    const interval = Number(document.getElementById('projRouteInterval').value);
    const retries = Math.max(0, Math.min(5, Number(document.getElementById('projRouteRetries').value) || 0));
    const enabled = document.getElementById('projRouteEnabled').checked;
    const target = document.getElementById('projRouteBudget');
    if (!enabled) {
      target.textContent = 'Synthetic monitoring is disabled: this route creates no outbound requests.';
      return;
    }
    if (!Number.isFinite(interval) || interval < 10) {
      target.textContent = 'Set an interval to preview this canary’s request cost. Activation is still constrained by project and global safety budgets.';
      return;
    }
    const checksPerDay = Math.ceil(86400 / interval);
    const requestsPerDay = checksPerDay * (retries + 1);
    target.textContent = `Estimated maximum: ${requestsPerDay.toLocaleString()} request attempt${requestsPerDay === 1 ? '' : 's'} per day (${checksPerDay.toLocaleString()} scheduled checks, up to ${retries + 1} attempt${retries ? 's' : ''} each). Project and global daily safety budgets can defer a run.`;
  }

  function openRouteModal(route) {
    editingRouteId = route ? route.id : null;
    const project = state.project.project;
    pel.routeModalTitle.textContent = route ? `Edit ${route.method} ${route.path}` : 'Add route';
    const set = (id, value) => {
      document.getElementById(id).value = value === null || value === undefined ? '' : value;
    };
    document.getElementById('projRouteMethod').value = route ? route.method : 'GET';
    document.getElementById('projRouteMethod').disabled = !!route; // identity is immutable
    document.getElementById('projRoutePath').readOnly = !!route;
    set('projRoutePath', route ? route.path : '');
    set('projRouteBaseUrl', route ? route.baseUrl : '');
    set('projRouteSummary', route ? route.summary : '');
    set('projRouteTags', route ? (route.tags || []).join(', ') : '');
    set('projRouteInterval', route ? route.monitorIntervalSeconds : project ? project.defaultIntervalSeconds : '');
    set('projRouteTimeout', route ? route.timeoutMs : project ? project.defaultTimeoutMs : '');
    set('projRouteRetries', route ? route.retries : project ? project.defaultRetries : '');
    set('projRouteExpected', route ? route.expectedStatusRange : '200-399');
    set('projRouteFailureThreshold', route ? route.failureThreshold : project ? project.failureThreshold : '');
    set('projRouteRecovery', route ? route.recoverySuccesses : project ? project.recoverySuccessThreshold : '');
    // Stored header values come back masked, so never echo them into an
    // editable field — a blank box means "keep what is stored".
    set('projRouteHeaders', '');
    document.getElementById('projRouteHeaders').placeholder = route && route.headers
      ? 'Stored headers are masked. Enter a full JSON object to replace them.'
      : '{"Authorization":"Bearer ..."}';
    document.getElementById('projRouteDeprecated').checked = route ? !!route.deprecated : false;
    document.getElementById('projRouteEnabled').checked = route ? !!route.enabled : true;
    updateRouteBudgetPreview();
    hideFormError(pel.routeFormError);
    openModal(pel.routeModal);
  }

  ['projRouteInterval', 'projRouteRetries', 'projRouteEnabled'].forEach((id) => {
    document.getElementById(id).addEventListener('input', updateRouteBudgetPreview);
    document.getElementById(id).addEventListener('change', updateRouteBudgetPreview);
  });

  pel.routeCancel.addEventListener('click', () => closeModal(pel.routeModal));

  pel.routeForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.routeFormError);
    const headersRaw = document.getElementById('projRouteHeaders').value.trim();
    if (headersRaw) {
      try {
        const parsed = JSON.parse(headersRaw);
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) throw new Error('not an object');
        if (Object.values(parsed).some((v) => typeof v !== 'string')) throw new Error('values must be strings');
      } catch (parseErr) {
        showFormError(pel.routeFormError, `Headers must be a JSON object of string values (${parseErr.message}).`);
        return;
      }
    }
    const tags = document.getElementById('projRouteTags').value.split(',').map((t) => t.trim()).filter(Boolean);
    const numOrZero = (id) => Number(document.getElementById(id).value) || 0;
    const payload = {
      method: document.getElementById('projRouteMethod').value,
      path: document.getElementById('projRoutePath').value.trim(),
      baseUrl: document.getElementById('projRouteBaseUrl').value.trim(),
      summary: document.getElementById('projRouteSummary').value.trim(),
      tags,
      deprecated: document.getElementById('projRouteDeprecated').checked,
      enabled: document.getElementById('projRouteEnabled').checked,
      monitorIntervalSeconds: numOrZero('projRouteInterval'),
      timeoutMs: numOrZero('projRouteTimeout'),
      retries: numOrZero('projRouteRetries'),
      expectedStatusRange: document.getElementById('projRouteExpected').value.trim(),
      failureThreshold: numOrZero('projRouteFailureThreshold'),
      recoverySuccesses: numOrZero('projRouteRecovery'),
    };
    if (headersRaw) payload.headers = headersRaw;

    setButtonLoading(pel.routeSubmit, true, 'Saving...');
    try {
      const pid = state.project.id;
      if (editingRouteId) {
		await apiProjects(`/route/catalog/${pid}/${editingRouteId}`, { method: 'PUT', body: JSON.stringify(payload) });
        showToast('Route updated.', 'success');
      } else {
		await apiProjects(`/route/catalog/${pid}`, { method: 'POST', body: JSON.stringify(payload) });
        showToast('Route added.', 'success');
      }
      closeModal(pel.routeModal);
      refreshCurrentView();
    } catch (err) {
      if (!(err instanceof SessionExpired)) showFormError(pel.routeFormError, err.message);
    } finally {
      setButtonLoading(pel.routeSubmit, false);
    }
  });

  /* --------------------------------------------------------- bulk add modal */

  function openBulkModal() {
    document.getElementById('projBulkBaseUrl').value = '';
    document.getElementById('projBulkRoutes').value = '';
    pel.bulkResult.innerHTML = '';
    hideFormError(pel.bulkFormError);
    openModal(pel.bulkModal);
  }

  pel.bulkCancel.addEventListener('click', () => {
    closeModal(pel.bulkModal);
    refreshCurrentView();
  });

  pel.bulkForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    hideFormError(pel.bulkFormError);
    pel.bulkResult.innerHTML = '';
    const baseUrl = document.getElementById('projBulkBaseUrl').value.trim();
    const lines = document.getElementById('projBulkRoutes').value.split('\n').map((l) => l.trim()).filter(Boolean);
    if (!lines.length) {
      showFormError(pel.bulkFormError, 'Enter at least one route.');
      return;
    }
    if (lines.length > 5000) {
      showFormError(pel.bulkFormError, `That is ${lines.length} rows; the limit is 5000 per submission.`);
      return;
    }
    const routes = lines.map((line) => {
      const [method, ...rest] = line.split(/\s+/);
      return { method: (method || '').toUpperCase(), path: rest.join(' ') || '', baseUrl };
    });

    setButtonLoading(pel.bulkSubmit, true, 'Adding...');
    try {
	  const result = await apiProjects(`/route/bulk/${state.project.id}`, {
        method: 'POST',
        body: JSON.stringify({ routes }),
      });
      const created = (result.created || []).length;
      const failed = result.failed || [];
      pel.bulkResult.innerHTML = `
        <div class="bulk-report">
          <p><strong>${created}</strong> route${created === 1 ? '' : 's'} added${failed.length ? `, <strong>${failed.length}</strong> rejected` : ''}.</p>
          ${
            failed.length
              ? `<div class="table-wrap"><table>
                   <thead><tr><th>Line</th><th>Route</th><th>Reason</th></tr></thead>
                   <tbody>${failed
                     .map((f) => `<tr><td class="mono">${f.index + 1}</td><td class="mono">${escapeHtml(f.route || '')}</td><td>${escapeHtml(f.error)}</td></tr>`)
                     .join('')}</tbody>
                 </table></div>`
              : ''
          }
        </div>`;
      if (created) showToast(`${created} route${created === 1 ? '' : 's'} added.`, 'success');
      if (failed.length && !created) showToast('No routes were added. See the report for details.', 'error');
    } catch (err) {
      if (!(err instanceof SessionExpired)) showFormError(pel.bulkFormError, err.message);
    } finally {
      setButtonLoading(pel.bulkSubmit, false);
    }
  });

  /* ------------------------------------------------------------- actions */

  function refreshCurrentView() {
    if (state.route.name === 'list') loadProjectsList();
    else if (state.route.name === 'project') loadProjectDetail({ silent: true });
    else if (state.route.name === 'route') loadRouteDetail({ silent: true });
  }

  async function findProject(id) {
    const cached = (state.list.data || []).find((p) => p.id === id);
    if (cached) return cached;
	return apiProjects(`/project/catalog/${id}`);
  }

  async function setRouteEnabled(routeId, enabled) {
	await apiProjects(`/route/${enabled ? 'enable' : 'disable'}/${state.project.id}/${routeId}`, { method: 'POST' });
    showToast(`Route ${enabled ? 'enabled' : 'disabled'}.`, 'success');
    refreshCurrentView();
  }

  async function runBulkRouteAction(action) {
    const ids = [...state.project.selected];
    if (!ids.length) return;
    const labels = { enable: 'Enable', disable: 'Disable', delete: 'Delete' };
    askConfirm(
      {
        title: `${labels[action]} ${ids.length} route${ids.length === 1 ? '' : 's'}?`,
        body:
          action === 'delete'
            ? 'The selected routes and their check history will be permanently removed. This cannot be undone.'
            : `Monitoring will be ${action}d for the selected routes.`,
        confirmLabel: labels[action],
        danger: action === 'delete',
      },
      async () => {
        if (action === 'delete') {
		  const res = await apiProjects(`/route/removal/${state.project.id}`, {
            method: 'POST',
            body: JSON.stringify({ ids }),
          });
          showToast(`${res.deleted} route${res.deleted === 1 ? '' : 's'} deleted.`, 'success');
        } else {
          // The API exposes enable/disable per route; run them with bounded
          // concurrency so a large selection cannot open hundreds of sockets.
          const failures = await mapWithLimit(ids, 6, async (id) => {
			await apiProjects(`/route/${action}/${state.project.id}/${id}`, { method: 'POST' });
          });
          if (failures.length) {
            showToast(`${ids.length - failures.length} updated, ${failures.length} failed.`, 'error');
          } else {
            showToast(`${ids.length} route${ids.length === 1 ? '' : 's'} ${action}d.`, 'success');
          }
        }
        state.project.selected.clear();
        loadProjectDetail({ silent: true });
      }
    );
  }

  /** mapWithLimit runs fn over items with bounded concurrency, collecting errors. */
  async function mapWithLimit(items, limit, fn) {
    const failures = [];
    let cursor = 0;
    const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
      while (cursor < items.length) {
        const index = cursor;
        cursor += 1;
        try {
          await fn(items[index]);
        } catch (err) {
          if (err instanceof SessionExpired) throw err;
          failures.push({ item: items[index], err });
        }
      }
    });
    await Promise.all(workers);
    return failures;
  }

  /* ------------------------------------------------- delegated interactions */

  pel.panel.addEventListener('click', async (e) => {
    const trigger = e.target.closest('[data-action]');
    if (!trigger) return;
    const action = trigger.dataset.action;
    const id = trigger.dataset.id ? Number(trigger.dataset.id) : null;

    // Checkbox actions are handled on 'change' instead.
    if (trigger.tagName === 'INPUT') return;

    try {
      switch (action) {
        case 'new-project':
          openProjectModal(null);
          break;
        case 'edit-project':
          openProjectModal(await findProject(id));
          break;
        case 'edit-current-project':
          openProjectModal(state.project.project);
          break;
        case 'archive-project':
        case 'unarchive-project': {
          const archiving = action === 'archive-project';
          askConfirm(
            {
              title: archiving ? 'Archive this project?' : 'Restore this project?',
              body: archiving
                ? 'Archived projects stay in the list and keep monitoring their routes; you can restore them at any time.'
                : 'The project will be marked active again.',
              confirmLabel: archiving ? 'Archive' : 'Restore',
              danger: false,
            },
            async () => {
			  await apiProjects(`/project/${archiving ? 'archive' : 'restore'}/${id}`, { method: 'POST' });
              showToast(archiving ? 'Project archived.' : 'Project restored.', 'success');
              loadProjectsList();
            }
          );
          break;
        }
        case 'delete-project':
          askConfirm(
            {
              title: 'Delete this project?',
              body: 'Every route, check, incident and import job in this project will be permanently deleted. This cannot be undone.',
              confirmLabel: 'Delete project',
            },
            async () => {
			  await apiProjects(`/project/catalog/${id}`, { method: 'DELETE' });
              showToast('Project deleted.', 'success');
              navigate('#/projects');
              loadProjectsList();
            }
          );
          break;
        case 'projects-page':
          state.list.offset = Number(trigger.dataset.offset) || 0;
          loadProjectsList();
          break;

        case 'add-route':
          openRouteModal(null);
          break;
        case 'bulk-add':
          openBulkModal();
          break;
        case 'open-import':
          navigate(`#/projects/${state.project.id}/import`);
          break;
        case 'create-environment': {
          openEnvironmentModal();
          break;
        }
        case 'edit-environment': {
          const environment = state.project.environments.find((item) => Number(item.id) === id);
          if (environment) openEnvironmentModal(environment);
          break;
        }
        case 'create-telemetry-credential':
          openTelemetryCredentialModal();
          break;
        case 'create-telemetry-mapping':
          openTelemetryMappingModal();
          break;
        case 'delete-telemetry-mapping':
          askConfirm(
            { title: 'Delete this telemetry mapping?', body: 'Future SLO evaluation will no longer attribute this telemetry identity to the selected route.', confirmLabel: 'Delete mapping' },
            async () => {
              await apiProjects(`/telemetry/mappings/${state.project.id}/${id}`, { method: 'DELETE' });
              showToast('Telemetry route mapping deleted.', 'success');
              loadProjectDetail({ silent: true });
            }
          );
          break;
        case 'create-slo':
          openSLOModal();
          break;
        case 'create-heartbeat':
          openHeartbeatModal();
          break;
        case 'revoke-heartbeat':
          askConfirm({ title: 'Revoke this heartbeat token?', body: 'The job using this token will no longer be able to report runs. Create a replacement heartbeat before changing the job configuration.', confirmLabel: 'Revoke heartbeat' }, async () => {
            await apiProjects(`/heartbeat/revoke/${state.project.id}/${id}`, { method: 'POST' });
            showToast('Heartbeat token revoked.', 'success'); loadProjectDetail({ silent: true });
          });
          break;
        case 'create-agent':
          openAgentModal();
          break;
        case 'create-agent-assignment':
          openAgentAssignmentModal();
          break;
        case 'revoke-agent':
          askConfirm({ title: 'Revoke this private-agent token?', body: 'The agent will no longer be able to report liveness. Create a replacement agent and update its local secret before revoking this one.', confirmLabel: 'Revoke agent' }, async () => {
            await apiProjects(`/agent/revoke/${state.project.id}/${id}`, { method: 'POST' });
            showToast('Private-agent token revoked.', 'success'); loadProjectDetail({ silent: true });
          });
          break;
        case 'revoke-agent-assignment':
          askConfirm({ title: 'Revoke this private-agent assignment?', body: 'Agents will stop receiving this signed local-check assignment after their next configuration refresh.', confirmLabel: 'Revoke assignment' }, async () => {
            await apiProjects(`/agent/assignments/revoke/${state.project.id}/${id}`, { method: 'POST' });
            showToast('Private-agent assignment revoked.', 'success'); loadProjectDetail({ silent: true });
          });
          break;
        case 'acknowledge-incident':
          await apiProjects(`/route/acknowledge/${state.project.id}/${id}`, { method: 'POST' });
          showToast('Incident acknowledged.', 'success');
          if (state.route.name === 'route') loadRouteDetail({ silent: true });
          else loadProjectDetail({ silent: true });
          break;
		case 'acknowledge-project-incident':
		  await apiProjects(`/incident/acknowledge/${state.project.id}/${id}`, { method: 'POST' });
		  showToast('Operational incident acknowledged.', 'success');
		  loadProjectDetail({ silent: true });
		  break;
        case 'edit-route': {
          const route = state.project.routes.find((r) => r.id === id) || (state.routeDetail.route && state.routeDetail.route.id === id ? state.routeDetail.route : null);
		  openRouteModal(route || (await apiProjects(`/route/catalog/${state.project.id}/${id}`)));
          break;
        }
        case 'enable-route':
          await setRouteEnabled(id, true);
          break;
        case 'disable-route':
          await setRouteEnabled(id, false);
          break;
        case 'delete-route':
          askConfirm(
            {
              title: 'Delete this route?',
              body: 'The route and its recorded check history will be permanently removed. This cannot be undone.',
              confirmLabel: 'Delete route',
            },
            async () => {
			  await apiProjects(`/route/catalog/${state.project.id}/${id}`, { method: 'DELETE' });
              showToast('Route deleted.', 'success');
              if (state.route.name === 'route') navigate(`#/projects/${state.project.id}`);
              else loadProjectDetail({ silent: true });
            }
          );
          break;
        case 'sort-routes': {
          const key = trigger.dataset.sort;
          if (state.project.sortBy === key) {
            state.project.sortDir = state.project.sortDir === 'asc' ? 'desc' : 'asc';
          } else {
            state.project.sortBy = key;
            state.project.sortDir = 'asc';
          }
          state.project.offset = 0;
          loadProjectDetail({ silent: true });
          break;
        }
        case 'routes-page':
          state.project.offset = Number(trigger.dataset.offset) || 0;
          state.project.selected.clear();
          loadProjectDetail({ silent: true });
          break;
        case 'clear-route-filters':
          state.project.filters = { search: '', method: '', status: '', tag: '', enabled: '', deprecated: '' };
          state.project.offset = 0;
          loadProjectDetail({ silent: true });
          break;
        case 'bulk-enable':
          await runBulkRouteAction('enable');
          break;
        case 'bulk-disable':
          await runBulkRouteAction('disable');
          break;
        case 'bulk-delete':
          await runBulkRouteAction('delete');
          break;
        case 'bulk-clear':
          state.project.selected.clear();
          renderProjectDetail();
          break;

        case 'project-range':
          state.project.range = trigger.dataset.range;
          loadProjectDetail({ silent: true });
          break;
        case 'route-range':
          state.routeDetail.range = trigger.dataset.range;
          loadRouteDetail({ silent: true });
          break;

        case 'preview-filter':
          state.importer.conflictFilter = trigger.dataset.filter;
          state.importer.previewPage = 0;
          renderImportWizard();
          break;
        case 'preview-page':
          state.importer.previewPage = Math.max(0, Number(trigger.dataset.page) || 0);
          renderImportWizard();
          break;
        case 'preview-select-all':
        case 'preview-select-none': {
          const select = action === 'preview-select-all';
          const buckets = previewBuckets(state.importer.job.items || []);
          (buckets[state.importer.conflictFilter] || []).forEach((item) => {
            if (item.conflict === 'duplicate_in_spec') return; // never selectable
            state.importer.selections.set(item.key, select);
          });
          renderImportWizard();
          break;
        }
        case 'preview-reset':
          state.importer.selections = new Map((state.importer.job.items || []).map((i) => [i.key, !!i.selected]));
          renderImportWizard();
          break;
        case 'import-commit':
          await commitImport();
          break;
        case 'import-restart':
          state.importer = { projectId: state.importer.projectId, step: 1, job: null, selections: new Map(), conflictFilter: 'all', previewPage: 0, busy: false, result: null };
          renderImportWizard();
          break;

        case 'reload-projects':
          loadProjectsList();
          break;
        case 'reload-project':
          loadProjectDetail();
          break;
        case 'reload-route':
          loadRouteDetail();
          break;
        case 'back-to-projects':
          navigate('#/projects');
          break;
        case 'back-to-project':
          navigate(`#/projects/${state.routeDetail.projectId}`);
          break;
        default:
          break;
      }
    } catch (err) {
      reportError('Action failed', err);
    }
  });

  pel.panel.addEventListener('change', (e) => {
    const input = e.target.closest('input[data-action]');
    if (!input) return;
    switch (input.dataset.action) {
      case 'toggle-route-selection': {
        const id = Number(input.dataset.id);
        if (input.checked) state.project.selected.add(id);
        else state.project.selected.delete(id);
        renderProjectDetail();
        break;
      }
      case 'toggle-page-selection':
        state.project.routes.forEach((r) => {
          if (input.checked) state.project.selected.add(r.id);
          else state.project.selected.delete(r.id);
        });
        renderProjectDetail();
        break;
      case 'preview-toggle':
        state.importer.selections.set(input.dataset.key, input.checked);
        renderImportWizard();
        break;
      case 'preview-toggle-page': {
        const buckets = previewBuckets(state.importer.job.items || []);
        const filtered = buckets[state.importer.conflictFilter] || [];
        const page = state.importer.previewPage;
        filtered.slice(page * PREVIEW_PAGE_SIZE, (page + 1) * PREVIEW_PAGE_SIZE).forEach((item) => {
          if (item.conflict === 'duplicate_in_spec') return;
          state.importer.selections.set(item.key, input.checked);
        });
        renderImportWizard();
        break;
      }
      default:
        break;
    }
  });

  /* ------------------------------------------------------------------- init */

  window.addEventListener('hashchange', handleRoute);

  // Redraw charts when the container width or the theme changes, since both
  // affect canvas rendering.
  window.addEventListener('resize', debounce(() => {
    if (state.route.name === 'project') drawSeriesCharts('projUptimeChart', 'projLatencyChart', state.project.series);
    if (state.route.name === 'route') drawSeriesCharts('routeUptimeChart', 'routeLatencyChart', state.routeDetail.series);
  }, 180));

  if (pel.tab) {
    pel.tab.addEventListener('click', () => {
      const parsed = parseHash();
      if (!parsed || parsed.name === 'auth') navigate('#/projects');
      else handleRoute();
    });
  }

  // Restore the sub-view on a hard refresh; otherwise stay on the legacy
  // dashboard and only initialise when the user opens the tab.
  if (parseHash()) {
    handleRoute();
  } else {
    syncAccountChrome();
    pel.globalAuthPanel.classList.add('hidden');
    pel.shell.classList.add('hidden');
  }
  restoreSession();
})();
