// Backend Auth tester. All server data is written with textContent or escaped
// before it reaches innerHTML, so API responses can never inject markup.
(function () {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const TOKEN_KEY = 'backend-auth.token';
  const MAX_LOG_ENTRIES = 50;
  const STATUS_TEXT = {
    0: 'Network error', 200: 'OK', 201: 'Created', 204: 'No Content', 400: 'Bad Request',
    401: 'Unauthorized', 403: 'Forbidden', 404: 'Not Found', 405: 'Method Not Allowed',
    409: 'Conflict', 413: 'Payload Too Large', 500: 'Internal Server Error', 503: 'Service Unavailable',
  };

  // ── Small helpers ────────────────────────────────────────
  function el(tag, attrs, children) {
    const node = document.createElement(tag);
    if (attrs) {
      for (const [k, v] of Object.entries(attrs)) {
        if (v == null || v === false) continue;
        if (k === 'class') node.className = v;
        else if (k === 'text') node.textContent = v;
        else node.setAttribute(k, v === true ? '' : v);
      }
    }
    for (const child of [].concat(children || [])) {
      if (child == null) continue;
      node.append(child instanceof Node ? child : document.createTextNode(String(child)));
    }
    return node;
  }

  function icon(name, cls) {
    const ns = 'http://www.w3.org/2000/svg';
    const svg = document.createElementNS(ns, 'svg');
    svg.setAttribute('class', 'icon' + (cls ? ' ' + cls : ''));
    svg.setAttribute('aria-hidden', 'true');
    const use = document.createElementNS(ns, 'use');
    use.setAttribute('href', '#i-' + name);
    svg.append(use);
    return svg;
  }

  function storage(fn) {
    try { return fn(window.sessionStorage); } catch (e) { return null; }
  }

  function initials(name) {
    const parts = String(name || '').trim().split(/\s+/).filter(Boolean);
    if (!parts.length) return '?';
    const first = Array.from(parts[0])[0] || '';
    const last = parts.length > 1 ? Array.from(parts[parts.length - 1])[0] || '' : '';
    return (first + last).toUpperCase();
  }

  function errorMessage(data, status) {
    if (data && typeof data === 'object' && typeof data.error === 'string') {
      const msg = data.error;
      return msg.charAt(0).toUpperCase() + msg.slice(1) + '.';
    }
    if (status === 0) return 'Could not reach the server. Check that it is running.';
    return 'Request failed with status ' + status + ' ' + (STATUS_TEXT[status] || '') + '.';
  }

  let toastTimer;
  function toast(message) {
    const t = $('toast');
    t.textContent = message;
    t.classList.add('show');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => t.classList.remove('show'), 2600);
  }

  // ── Theme ────────────────────────────────────────────────
  const themeToggle = $('theme-toggle');
  function currentTheme() {
    const explicit = document.documentElement.getAttribute('data-theme');
    if (explicit) return explicit;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  function syncThemeButton() {
    const next = currentTheme() === 'dark' ? 'light' : 'dark';
    themeToggle.setAttribute('aria-label', 'Switch to ' + next + ' theme');
    themeToggle.title = 'Switch to ' + next + ' theme';
  }
  themeToggle.addEventListener('click', () => {
    const next = currentTheme() === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    try { localStorage.setItem('theme', next); } catch (e) { /* storage blocked */ }
    syncThemeButton();
  });
  syncThemeButton();

  // ── Health check ─────────────────────────────────────────
  $('health-host').textContent = window.location.host;
  async function checkHealth() {
    const pill = $('health');
    try {
      const res = await fetch('/healthz', { cache: 'no-store' });
      pill.dataset.state = res.ok ? 'ok' : 'down';
      $('health-label').textContent = res.ok ? 'API online' : 'API degraded';
    } catch (e) {
      pill.dataset.state = 'down';
      $('health-label').textContent = 'API unreachable';
    }
  }

  // ── Tabs ─────────────────────────────────────────────────
  const tabs = Array.from(document.querySelectorAll('[role="tab"]'));
  function selectTab(name, focus) {
    tabs.forEach((tab) => {
      const selected = tab.dataset.tab === name;
      tab.setAttribute('aria-selected', String(selected));
      tab.tabIndex = selected ? 0 : -1;
      $(tab.getAttribute('aria-controls')).hidden = !selected;
      if (selected && focus) tab.focus();
    });
    if (window.location.hash !== '#' + name) history.replaceState(null, '', '#' + name);
    if (name === 'profile') renderProfileIdle();
    if (name === 'users') renderUsersIdle();
  }
  tabs.forEach((tab, i) => {
    tab.addEventListener('click', () => selectTab(tab.dataset.tab, false));
    tab.addEventListener('keydown', (e) => {
      let next = null;
      if (e.key === 'ArrowRight') next = tabs[(i + 1) % tabs.length];
      else if (e.key === 'ArrowLeft') next = tabs[(i - 1 + tabs.length) % tabs.length];
      else if (e.key === 'Home') next = tabs[0];
      else if (e.key === 'End') next = tabs[tabs.length - 1];
      if (next) { e.preventDefault(); selectTab(next.dataset.tab, true); }
    });
  });

  // ── Response log ─────────────────────────────────────────
  function escapeHTML(str) {
    return String(str)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }

  // Escapes first, then highlights, so server data can never inject markup.
  function highlight(value) {
    const json = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
    return escapeHTML(json).replace(/&quot;/g, '"').replace(
      /("(\\u[\da-fA-F]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g,
      (m) => {
        let cls = 'json-num';
        if (m.charAt(0) === '"') cls = /:$/.test(m) ? 'json-key' : 'json-str';
        else if (/true|false|null/.test(m)) cls = 'json-bool';
        return '<span class="' + cls + '">' + m + '</span>';
      });
  }

  const log = $('log');
  const logEmpty = $('log-empty');
  function addLog({ method, path, status, data, ms, local }) {
    if (logEmpty.isConnected) logEmpty.remove();
    const kind = local ? 'local' : status >= 200 && status < 300 ? 'ok' : 'fail';
    const statusLabel = local ? 'Not sent' : (status || '—') + ' ' + (STATUS_TEXT[status] || '');
    const body = el('div', { class: 'entry-body' });
    body.innerHTML = highlight(data === undefined ? '' : data);
    const entry = el('article', { class: 'entry ' + kind, 'aria-label': method + ' ' + path + ': ' + statusLabel }, [
      el('div', { class: 'entry-meta' }, [
        el('span', { class: 'badge badge-' + method.toLowerCase(), text: method }),
        el('span', { class: 'entry-path', text: path }),
        el('span', { class: 'entry-time', text: new Date().toLocaleTimeString() + (ms != null ? ' · ' + ms + ' ms' : '') }),
        el('span', { class: 'entry-status', text: statusLabel }),
      ]),
      body,
    ]);
    log.append(entry);
    while (log.querySelectorAll('.entry').length > MAX_LOG_ENTRIES) log.querySelector('.entry').remove();
    log.scrollTop = log.scrollHeight;
    $('btn-clear').disabled = false;
  }
  $('btn-clear').addEventListener('click', () => {
    log.replaceChildren(logEmpty);
    $('btn-clear').disabled = true;
    toast('Response log cleared');
  });

  // ── API ──────────────────────────────────────────────────
  async function api(method, path, body, withAuth) {
    const headers = {};
    if (body) headers['Content-Type'] = 'application/json';
    if (withAuth && session.token) headers.Authorization = 'Bearer ' + session.token;
    const started = performance.now();
    let status = 0;
    let data;
    try {
      const res = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined });
      status = res.status;
      const text = await res.text();
      try { data = text ? JSON.parse(text) : ''; } catch (e) { data = text; }
    } catch (err) {
      data = { error: 'network error: ' + err.message };
    }
    addLog({ method, path, status, data, ms: Math.round(performance.now() - started) });
    return { ok: status >= 200 && status < 300, status, data };
  }

  function setBusy(btn, busy) {
    btn.disabled = busy;
    if (busy) btn.setAttribute('aria-busy', 'true');
    else btn.removeAttribute('aria-busy');
  }

  // ── Session (JWT) ────────────────────────────────────────
  const session = { token: null, claims: null };

  function decodeClaims(token) {
    try {
      const part = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/');
      const json = decodeURIComponent(Array.from(atob(part), (c) => '%' + c.charCodeAt(0).toString(16).padStart(2, '0')).join(''));
      return JSON.parse(json);
    } catch (e) {
      return null;
    }
  }

  function formatExpiry(exp) {
    if (!exp) return '';
    const ms = exp * 1000 - Date.now();
    if (ms <= 0) return 'expired';
    const mins = Math.round(ms / 60000);
    if (mins < 60) return 'expires in ' + mins + ' min';
    const hours = Math.round(mins / 60);
    return 'expires in ' + hours + ' h';
  }

  function setSession(token) {
    const claims = token ? decodeClaims(token) : null;
    if (token && (!claims || (claims.exp && claims.exp * 1000 <= Date.now()))) token = null;
    session.token = token;
    session.claims = token ? claims : null;
    storage((s) => (token ? s.setItem(TOKEN_KEY, token) : s.removeItem(TOKEN_KEY)));
    renderSession();
  }

  function renderSession() {
    const box = $('session');
    const avatar = $('session-avatar');
    const tokenEl = $('session-token');
    if (!session.token) {
      box.dataset.state = 'out';
      avatar.replaceChildren(icon('lock'));
      $('session-title').textContent = 'Not signed in';
      $('session-meta').textContent = 'Log in to get a JWT for the protected endpoints.';
      tokenEl.hidden = true;
      tokenEl.textContent = '';
      return;
    }
    const c = session.claims || {};
    box.dataset.state = 'in';
    avatar.replaceChildren(icon(c.role === 'admin' ? 'users' : 'user'));
    $('session-title').textContent = 'Signed in as ' + (c.role || 'unknown role') + (c.user_id != null ? ' · user #' + c.user_id : '');
    $('session-meta').textContent = 'Bearer token ' + formatExpiry(c.exp) + '.';
    tokenEl.hidden = false;
    tokenEl.textContent = session.token;
    tokenEl.title = session.token;
  }

  $('btn-copy-token').addEventListener('click', async () => {
    if (!session.token) return;
    try {
      await navigator.clipboard.writeText(session.token);
      toast('Token copied to clipboard');
    } catch (e) {
      const range = document.createRange();
      range.selectNodeContents($('session-token'));
      const sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
      toast('Press Ctrl+C (or ⌘C) to copy the selected token');
    }
  });
  $('btn-logout').addEventListener('click', () => {
    setSession(null);
    profileState = null;
    usersState = null;
    renderProfileIdle();
    renderUsersIdle();
    toast('Signed out');
  });

  // ── Form validation ──────────────────────────────────────
  const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  const validators = {
    name: (v) => (!v.trim() ? 'Enter your name.' : Array.from(v.trim()).length > 100 ? 'Name must be 100 characters or fewer.' : ''),
    email: (v) => (!v.trim() ? 'Enter your email address.' : !EMAIL_RE.test(v.trim()) ? 'Enter a valid email address, like name@example.com.' : ''),
    newPassword: (v) => {
      if (!v) return 'Choose a password.';
      if (Array.from(v).length < 8) return 'Password must be at least 8 characters.';
      if (new TextEncoder().encode(v).length > 72) return 'Password must be at most 72 bytes.';
      return '';
    },
    password: (v) => (!v ? 'Enter your password.' : ''),
  };

  function setFieldError(input, message) {
    const err = $(input.id + '-error');
    err.replaceChildren();
    if (message) err.append(icon('alert'), el('span', { text: message }));
    if (message) input.setAttribute('aria-invalid', 'true');
    else input.removeAttribute('aria-invalid');
  }

  function wireForm(form, rules) {
    const inputs = Object.keys(rules).map((id) => $(id));
    let attempted = false;
    inputs.forEach((input) => {
      const check = () => setFieldError(input, rules[input.id](input.value));
      input.addEventListener('blur', () => { if (attempted || input.value) check(); });
      input.addEventListener('input', () => { if (input.getAttribute('aria-invalid') === 'true') check(); });
    });
    return function validate() {
      attempted = true;
      let firstInvalid = null;
      inputs.forEach((input) => {
        const msg = rules[input.id](input.value);
        setFieldError(input, msg);
        if (msg && !firstInvalid) firstInvalid = input;
      });
      if (firstInvalid) firstInvalid.focus();
      return !firstInvalid;
    };
  }

  function showAlert(box, kind, title, message, actions) {
    const iconName = kind === 'success' ? 'check' : kind === 'error' ? 'alert' : 'info';
    const body = el('div', { class: 'alert-body' }, [
      el('strong', { text: title }), message ? ' ' + message : null,
      actions && actions.length ? el('div', { class: 'alert-actions' }, actions) : null,
    ]);
    box.className = 'alert alert-' + kind;
    box.replaceChildren(icon(iconName), body);
    box.hidden = false;
  }
  function hideAlert(box) { box.hidden = true; box.replaceChildren(); }

  document.querySelectorAll('.reveal').forEach((btn) => {
    btn.addEventListener('click', () => {
      const input = $(btn.getAttribute('aria-controls'));
      const show = input.type === 'password';
      input.type = show ? 'text' : 'password';
      btn.setAttribute('aria-pressed', String(show));
      btn.setAttribute('aria-label', show ? 'Hide password' : 'Show password');
    });
  });

  // ── Sign up ──────────────────────────────────────────────
  const validateSignup = wireForm($('form-signup'), {
    'signup-name': validators.name,
    'signup-email': validators.email,
    'signup-password': validators.newPassword,
  });
  $('form-signup').addEventListener('submit', async (e) => {
    e.preventDefault();
    const alertBox = $('signup-alert');
    hideAlert(alertBox);
    if (!validateSignup()) return;
    const btn = $('btn-signup');
    const payload = {
      name: $('signup-name').value.trim(),
      email: $('signup-email').value.trim(),
      password: $('signup-password').value,
    };
    setBusy(btn, true);
    const res = await api('POST', '/signup', payload);
    setBusy(btn, false);
    if (res.ok) {
      const goLogin = el('button', { class: 'btn btn-sm btn-primary', type: 'button', text: 'Continue to log in' });
      goLogin.addEventListener('click', () => {
        $('login-email').value = payload.email;
        selectTab('login', false);
        $('login-password').focus();
      });
      showAlert(alertBox, 'success', 'Account created.', 'You can now log in as ' + payload.email + '.', [goLogin]);
      $('form-signup').reset();
      return;
    }
    if (res.status === 409) setFieldError($('signup-email'), 'An account with this email already exists.');
    showAlert(alertBox, 'error', 'Could not create the account.', errorMessage(res.data, res.status));
  });

  // ── Log in ───────────────────────────────────────────────
  const validateLogin = wireForm($('form-login'), {
    'login-email': validators.email,
    'login-password': validators.password,
  });
  $('form-login').addEventListener('submit', async (e) => {
    e.preventDefault();
    const alertBox = $('login-alert');
    hideAlert(alertBox);
    if (!validateLogin()) return;
    const btn = $('btn-login');
    setBusy(btn, true);
    const res = await api('POST', '/login', {
      email: $('login-email').value.trim(),
      password: $('login-password').value,
    });
    setBusy(btn, false);
    if (res.ok && res.data && res.data.token) {
      setSession(res.data.token);
      profileState = null;
      usersState = null;
      $('login-password').value = '';
      const role = (session.claims && session.claims.role) || 'user';
      const toProfile = el('button', { class: 'btn btn-sm btn-primary', type: 'button', text: 'View my profile' });
      toProfile.addEventListener('click', () => { selectTab('profile', true); loadProfile(); });
      const actions = [toProfile];
      if (role === 'admin') {
        const toUsers = el('button', { class: 'btn btn-sm', type: 'button', text: 'List all users' });
        toUsers.addEventListener('click', () => { selectTab('users', true); loadUsers(); });
        actions.push(toUsers);
      }
      showAlert(alertBox, 'success', 'Signed in.', 'Your ' + role + ' token is ready.', actions);
      return;
    }
    showAlert(alertBox, 'error', res.status === 401 ? 'Wrong email or password.' : 'Could not log in.',
      res.status === 401 ? 'Check your details and try again.' : errorMessage(res.data, res.status));
  });

  // ── Shared result renderers ──────────────────────────────
  function emptyState(iconName, title, text, action) {
    return el('div', { class: 'empty' }, [icon(iconName), el('p', { class: 'empty-title', text: title }), el('p', { text }), action || null]);
  }
  function skeleton(label) {
    return el('div', { class: 'skeleton', role: 'status', 'aria-label': label }, [
      el('div', { class: 'skeleton-line' }), el('div', { class: 'skeleton-line' }), el('div', { class: 'skeleton-line' }),
    ]);
  }
  function goToLoginButton() {
    const b = el('button', { class: 'btn btn-sm btn-primary', type: 'button', text: 'Go to log in' });
    b.addEventListener('click', () => { selectTab('login', false); $('login-email').focus(); });
    return b;
  }
  function errorAlert(title, message) {
    const box = el('div', { role: 'alert' });
    showAlert(box, 'error', title, message);
    return box;
  }
  function roleBadge(role) {
    return el('span', { class: 'badge badge-role-' + (role === 'admin' ? 'admin' : 'user'), text: role });
  }

  // ── Profile ──────────────────────────────────────────────
  let profileState = null; // last rendered result node, kept across tab switches
  function renderProfileIdle() {
    const out = $('profile-result');
    if (!session.token) {
      out.replaceChildren(emptyState('lock', 'Log in to see your profile', 'This endpoint needs a Bearer token.', goToLoginButton()));
    } else if (profileState) {
      out.replaceChildren(profileState);
    } else {
      out.replaceChildren(emptyState('user', 'Profile not loaded yet', 'Press "Load my profile" to call the endpoint with your token.'));
    }
  }

  async function loadProfile() {
    const out = $('profile-result');
    if (!session.token) {
      addLog({ method: 'GET', path: '/profile', local: true, data: { error: 'no token: log in first' } });
      renderProfileIdle();
      return;
    }
    const btn = $('btn-profile');
    setBusy(btn, true);
    out.replaceChildren(skeleton('Loading profile'));
    const res = await api('GET', '/profile', null, true);
    setBusy(btn, false);
    if (res.ok && res.data && typeof res.data === 'object') {
      const u = res.data;
      profileState = el('div', { class: 'profile' }, [
        el('span', { class: 'avatar', 'aria-hidden': 'true', text: initials(u.name) }),
        el('div', null, [
          el('p', { class: 'profile-name', text: u.name }),
          el('p', { class: 'profile-email', text: u.email }),
          el('div', { class: 'profile-tags' }, [roleBadge(u.role), el('span', { class: 'badge', text: 'ID ' + u.id })]),
        ]),
      ]);
      out.replaceChildren(profileState);
      return;
    }
    if (res.status === 401) {
      setSession(null);
      out.replaceChildren(errorAlert('Your session has expired.', 'Log in again to get a new token.'));
      return;
    }
    out.replaceChildren(errorAlert('Could not load the profile.', errorMessage(res.data, res.status)));
  }
  $('btn-profile').addEventListener('click', loadProfile);

  // ── Users ────────────────────────────────────────────────
  let usersState = null;
  function renderUsersIdle() {
    const out = $('users-result');
    if (!session.token) {
      out.replaceChildren(emptyState('lock', 'Admins only', 'Log in with the bootstrap admin account to list users.', goToLoginButton()));
    } else if (usersState) {
      out.replaceChildren(usersState);
    } else if (session.claims && session.claims.role !== 'admin') {
      const box = el('div');
      showAlert(box, 'warning', 'Your token has the user role.', 'This request will be rejected with 403. Try it to see the error, or log in as an admin.');
      out.replaceChildren(box);
    } else {
      out.replaceChildren(emptyState('users', 'User list not loaded yet', 'Press "Load all users" to call the endpoint.'));
    }
  }

  function usersTable(users) {
    const tbody = el('tbody');
    const rows = users.map((u) => {
      const tr = el('tr', null, [
        el('td', { class: 'num', text: '#' + u.id }),
        el('td', { class: 'name', text: u.name }),
        el('td', { class: 'email', text: u.email }),
        el('td', { class: 'role' }, roleBadge(u.role)),
      ]);
      tr.dataset.search = (u.name + ' ' + u.email + ' ' + u.role).toLowerCase();
      return tr;
    });
    tbody.append(...rows);
    const count = el('span', { class: 'muted', 'aria-live': 'polite', text: users.length + (users.length === 1 ? ' user' : ' users') });
    const noMatch = el('p', { class: 'empty', text: 'No users match your search.', hidden: true });
    const search = el('input', { class: 'input search', type: 'search', placeholder: 'Filter by name, email or role', 'aria-label': 'Filter users' });
    search.addEventListener('input', () => {
      const q = search.value.trim().toLowerCase();
      let shown = 0;
      rows.forEach((tr) => { const hit = tr.dataset.search.includes(q); tr.hidden = !hit; if (hit) shown++; });
      count.textContent = q ? shown + ' of ' + users.length + ' users' : users.length + (users.length === 1 ? ' user' : ' users');
      noMatch.hidden = shown !== 0;
    });
    return el('div', { class: 'table-wrap' }, [
      el('div', { class: 'table-caption' }, [count, search]),
      el('div', { class: 'table-scroll' }, [
        el('table', { class: 'users-table' }, [
          el('caption', { class: 'sr-only', text: 'Registered users' }),
          el('thead', null, el('tr', null, [
            el('th', { scope: 'col', text: 'ID' }), el('th', { scope: 'col', text: 'Name' }),
            el('th', { scope: 'col', text: 'Email' }), el('th', { scope: 'col', text: 'Role' }),
          ])),
          tbody,
        ]),
      ]),
      noMatch,
    ]);
  }

  async function loadUsers() {
    const out = $('users-result');
    if (!session.token) {
      addLog({ method: 'GET', path: '/users', local: true, data: { error: 'no token: log in first' } });
      renderUsersIdle();
      return;
    }
    const btn = $('btn-users');
    setBusy(btn, true);
    out.replaceChildren(skeleton('Loading users'));
    const res = await api('GET', '/users', null, true);
    setBusy(btn, false);
    if (res.ok && Array.isArray(res.data)) {
      usersState = res.data.length
        ? usersTable(res.data)
        : emptyState('users', 'No users yet', 'Accounts created through sign-up will show up here.');
      out.replaceChildren(usersState);
      return;
    }
    if (res.status === 401) {
      setSession(null);
      out.replaceChildren(errorAlert('Your session has expired.', 'Log in again to get a new token.'));
      return;
    }
    if (res.status === 403) {
      out.replaceChildren(errorAlert('Admin access required.', 'Your token has the user role. Log in with the admin account configured by ADMIN_EMAIL.'));
      return;
    }
    out.replaceChildren(errorAlert('Could not load users.', errorMessage(res.data, res.status)));
  }
  $('btn-users').addEventListener('click', loadUsers);

  // ── Boot ─────────────────────────────────────────────────
  setSession(storage((s) => s.getItem(TOKEN_KEY)));
  const initial = window.location.hash.slice(1);
  selectTab(tabs.some((t) => t.dataset.tab === initial) ? initial : 'signup', false);
  checkHealth();
})();
