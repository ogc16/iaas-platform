const API = '/api/v1';

let state = {
  token: localStorage.getItem('token'),
  user: null,
  orgs: [],
  instances: [],
  currentOrg: null,
  view: 'dashboard',
  currentInstance: null,
  notifications: [],
  activityLog: [],
  settings: {
    theme: localStorage.getItem('theme') || 'dark',
    emailNotifications: JSON.parse(localStorage.getItem('emailNotifications') ?? 'true'),
    autoRefresh: JSON.parse(localStorage.getItem('autoRefresh') ?? 'true'),
  },
};

function api(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (state.token) headers['Authorization'] = `Bearer ${state.token}`;
  return fetch(`${API}${path}`, { ...opts, headers })
    .then(r => r.json().then(body => ({ ok: r.ok, status: r.status, body })))
    .catch(() => ({ ok: false, body: { error: 'Network error' } }));
}

// Notification system with history
function notify(msg, type = 'success', duration = 3000) {
  const notification = {
    id: Date.now(),
    msg,
    type,
    timestamp: new Date(),
  };
  state.notifications.push(notification);
  
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.id = `notif-${notification.id}`;
  el.innerHTML = `<div style="display:flex;align-items:center;justify-content:space-between;gap:12px">
    <span>${msg}</span>
    <button onclick="document.getElementById('notif-${notification.id}').remove()" style="background:none;border:none;color:inherit;cursor:pointer;padding:0;font-size:16px">×</button>
  </div>`;
  document.body.appendChild(el);
  
  if (duration) setTimeout(() => el.remove(), duration);
  return notification.id;
}

function toast(msg, type = 'success') {
  notify(msg, type);
}

function modal(html) {
  const overlay = document.createElement('div');
  overlay.className = 'modal-overlay';
  overlay.innerHTML = `<div class="modal">${html}</div>`;
  overlay.addEventListener('click', e => { if (e.target === overlay) overlay.remove(); });
  document.body.appendChild(overlay);
  return overlay.querySelector('.modal');
}

function $(sel, parent = document) { return parent.querySelector(sel); }

function loadingSkeleton() {
  return `
    <div class="skeleton-container">
      <div class="skeleton-line" style="width:30%"></div>
      <div class="skeleton-line" style="width:100%;margin-top:12px"></div>
      <div class="skeleton-line" style="width:100%;margin-top:8px"></div>
      <div class="skeleton-line" style="width:80%;margin-top:8px"></div>
    </div>`;
}

function addActivityLog(action, details) {
  state.activityLog.unshift({
    id: Date.now(),
    action,
    details,
    timestamp: new Date(),
    user: state.user?.email || 'System',
  });
  if (state.activityLog.length > 50) state.activityLog.pop();
}

function escape(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

function render() {
  if (!state.token) return renderLogin();
  const app = document.getElementById('app');
  const unreadCount = state.notifications.filter(n => !n.read).length;
  app.innerHTML = `
    <div class="header">
      <h1>☁ IaaS Platform</h1>
      <nav>
        <span class="user-badge">${state.user ? state.user.email : ''}</span>
        <a href="#" data-view="dashboard" class="active">Dashboard</a>
        <a href="#" data-view="orgs">Organizations</a>
        <button class="notification-btn ${unreadCount > 0 ? 'has-notifications' : ''}" onclick="toggleNotificationCenter()" title="Notifications">
          🔔
          ${unreadCount > 0 ? `<span class="notification-badge">${unreadCount}</span>` : ''}
        </button>
        <button class="header-btn" onclick="goToSettings()" title="Settings">⚙️</button>
        <button class="header-btn" onclick="logout()" title="Logout">🚪</button>
      </nav>
    </div>
    <div id="notification-center" class="notification-center"></div>
    <div class="main" id="main"></div>`;
  app.querySelectorAll('[data-view]').forEach(el => {
    el.addEventListener('click', e => {
      e.preventDefault();
      app.querySelectorAll('[data-view]').forEach(a => a.classList.remove('active'));
      el.classList.add('active');
      state.view = el.dataset.view;
      renderView();
    });
  });
  renderView();
}

window.goToSettings = function() {
  state.view = 'settings';
  renderSettings();
  document.querySelectorAll('[data-view]').forEach(a => a.classList.remove('active'));
};

window.toggleNotificationCenter = function() {
  const center = document.getElementById('notification-center');
  if (center.style.display === 'block') {
    center.style.display = 'none';
  } else {
    renderNotificationCenter();
    center.style.display = 'block';
  }
};

function renderNotificationCenter() {
  const center = document.getElementById('notification-center');
  const items = state.notifications.slice(0, 10).map(n => `
    <div class="notification-item ${n.type}">
      <div style="flex:1">
        <div class="notification-msg">${n.msg}</div>
        <div class="notification-time">${n.timestamp.toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'})}</div>
      </div>
      <button onclick="dismissNotification(${n.id})" style="background:none;border:none;color:inherit;cursor:pointer">×</button>
    </div>
  `).join('');
  center.innerHTML = `
    <div class="notification-panel">
      <div class="notification-header">
        <h3>Notifications</h3>
        <button onclick="clearAllNotifications()" class="btn btn-sm btn-outline">Clear All</button>
      </div>
      ${state.notifications.length === 0 ? '<div style="padding:20px;text-align:center;color:var(--muted)">No notifications</div>' : items}
    </div>`;
}

window.dismissNotification = function(id) {
  const notif = state.notifications.find(n => n.id === id);
  if (notif) notif.read = true;
  renderNotificationCenter();
};

window.clearAllNotifications = function() {
  state.notifications = [];
  renderNotificationCenter();
};

function renderLogin() {
  document.getElementById('app').innerHTML = `
    <div class="login-page">
      <div class="login-card">
        <h1>IaaS Platform</h1>
        <p class="subtitle">Sign in to manage your infrastructure</p>
        <div id="login-form">
          <div class="form-group">
            <label>Email</label>
            <input type="email" id="login-email" placeholder="you@example.com">
          </div>
          <div class="form-group">
            <label>Password</label>
            <input type="password" id="login-password" placeholder="password">
          </div>
          <button class="btn btn-primary" style="width:100%;justify-content:center" onclick="login()">Sign In</button>
          <p style="text-align:center;margin-top:16px;font-size:13px;color:var(--muted)">
            No account? <a href="#" onclick="showSignup()">Sign up</a>
          </p>
        </div>
      </div>
    </div>`;
}

window.showSignup = function() {
  document.getElementById('login-form').innerHTML = `
    <div class="form-group"><label>Name</label><input type="text" id="signup-name" placeholder="Your name"></div>
    <div class="form-group"><label>Email</label><input type="email" id="signup-email" placeholder="you@example.com"></div>
    <div class="form-group"><label>Password</label><input type="password" id="signup-password" placeholder="password"></div>
    <div class="form-group"><label>Organization</label><input type="text" id="signup-org" placeholder="Company name"></div>
    <button class="btn btn-primary" style="width:100%;justify-content:center" onclick="signup()">Create Account</button>
    <p style="text-align:center;margin-top:16px;font-size:13px;color:var(--muted)">
      Already have an account? <a href="#" onclick="render()">Sign in</a>
    </p>`;
};

window.login = async function() {
  const email = document.getElementById('login-email').value;
  const password = document.getElementById('login-password').value;
  if (!email || !password) return toast('Fill in all fields', 'error');
  const { ok, body } = await api('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) });
  if (!ok) return toast(body.error || 'Login failed', 'error');
  state.token = body.token;
  state.user = body.user;
  localStorage.setItem('token', body.token);
  render();
};

window.signup = async function() {
  const name = document.getElementById('signup-name').value;
  const email = document.getElementById('signup-email').value;
  const password = document.getElementById('signup-password').value;
  const org = document.getElementById('signup-org').value;
  if (!name || !email || !password) return toast('Fill in all fields', 'error');
  const { ok, body } = await api('/auth/signup', { method: 'POST', body: JSON.stringify({ name, email, password, organization: org }) });
  if (!ok) return toast(body.error || 'Signup failed', 'error');
  state.token = body.token;
  state.user = body.user;
  localStorage.setItem('token', body.token);
  render();
};

window.logout = function() {
  state.token = null;
  state.user = null;
  localStorage.removeItem('token');
  render();
};

function renderView() {
  if (state.view === 'dashboard') renderDashboard();
  else if (state.view === 'orgs') renderOrgs();
  else if (state.view === 'settings') renderSettings();
}

async function renderDashboard() {
  const main = document.getElementById('main');
  main.innerHTML = '<div class="loading">Loading...</div>';

  try {
    const { ok, body } = await api('/orgs');
    if (!ok) { main.innerHTML = '<div class="loading">Failed to load dashboard</div>'; return; }
    // API may return either an array or an object containing the array.
    // Handler currently writes `orgs` directly (array), but this makes the UI resilient.
    state.orgs = Array.isArray(body) ? body : (body?.orgs || body?.data || []);


    // Fetch instance lists concurrently to avoid one failing request blocking rendering
    const instanceResults = await Promise.all(
      state.orgs.map(async (org) => {
        const { ok, body: instances } = await api(`/orgs/${org.id}/instances`);
        return ok ? instances : [];
      })
    );

    let totalInstances = 0, runningInstances = 0;
    instanceResults.forEach(instances => {
      totalInstances += instances.length;
      runningInstances += instances.filter(i => i.status === 'running').length;
    });

    // Billing usage is optional for rendering; fallback safely if it fails
    let usage = null;
    if (state.orgs.length > 0) {
      const usageResp = await api(`/orgs/${state.orgs[0].id}/billing/usage`);
      usage = usageResp.ok ? usageResp.body : null;
    }

    main.innerHTML = `
      <div class="stats">
        <div class="stat"><div class="label">Organizations</div><div class="value blue">${state.orgs.length}</div></div>
        <div class="stat"><div class="label">Total Instances</div><div class="value">${totalInstances}</div></div>
        <div class="stat"><div class="label">Running</div><div class="value green">${runningInstances}</div></div>
        <div class="stat"><div class="label">CPU Hours (30d)</div><div class="value yellow">${usage ? usage.cpu_hours.toFixed(1) : '0'}</div></div>
      </div>
      <div class="card">
        <div class="card-header">
          <h2>Organizations</h2>
          <button class="btn btn-primary btn-sm" onclick="showCreateOrg()">+ New Org</button>
        </div>
        ${state.orgs.length === 0 ? '<div class="empty"><p>No organizations yet</p><button class="btn btn-primary" onclick="showCreateOrg()">Create your first organization</button></div>' :
        `<table><thead><tr><th>Name</th><th>Slug</th><th>Created</th><th></th></tr></thead><tbody>
          ${state.orgs.map(o => `<tr>
            <td><strong>${escape(o.name)}</strong></td>
            <td style="color:var(--muted)">${escape(o.slug)}</td>
            <td style="color:var(--muted)">${new Date(o.created_at).toLocaleDateString()}</td>
            <td><button class="btn btn-outline btn-sm" onclick="selectOrg(${o.id})">Manage</button></td>
          </tr>`).join('')}
        </tbody></table>`}
      </div>`;
  } catch (e) {
    main.innerHTML = '<div class="loading">Failed to load dashboard</div>';
  }
}

async function renderOrgs() {
  const main = document.getElementById('main');
  main.innerHTML = '<div class="loading">Loading...</div>';

  const { ok, body: orgs } = await api('/orgs');
  if (!ok) { main.innerHTML = '<div class="loading">Failed to load</div>'; return; }
  state.orgs = orgs;

  if (state.currentOrg) {
    const org = state.orgs.find(o => o.id === state.currentOrg);
    if (!org) { state.currentOrg = null; return renderOrgs(); }
    return renderOrgDetail(org);
  }

  main.innerHTML = `
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:20px">
      <h2 style="font-size:20px">Organizations</h2>
      <button class="btn btn-primary" onclick="showCreateOrg()">+ New Organization</button>
    </div>
    ${orgs.length === 0 ? '<div class="card empty"><p>No organizations yet</p></div>' :
    `<div class="card"><table><thead><tr><th>Name</th><th>Slug</th><th>Created</th><th></th></tr></thead><tbody>
      ${orgs.map(o => `<tr>
        <td><strong>${escape(o.name)}</strong></td>
        <td style="color:var(--muted)">${escape(o.slug)}</td>
        <td style="color:var(--muted)">${new Date(o.created_at).toLocaleDateString()}</td>
        <td><button class="btn btn-primary btn-sm" onclick="selectOrg(${o.id})">Open</button></td>
      </tr>`).join('')}
    </tbody></table></div>`}`;
}

async function renderOrgDetail(org) {
  const { ok: instOk, body: instances } = await api(`/orgs/${org.id}/instances`);
  if (instOk) state.instances = instances;

  const { ok: membOk, body: members } = await api(`/orgs/${org.id}/members`);
  const { ok: usageOk, body: usage } = await api(`/orgs/${org.id}/billing/usage`);
  const { ok: invOk, body: invoices } = await api(`/orgs/${org.id}/billing/invoices`);

  const running = instances.filter(i => i.status === 'running').length;
  const stopped = instances.filter(i => i.status === 'stopped').length;

  const main = document.getElementById('main');
  main.innerHTML = `
    <div style="display:flex;align-items:center;gap:12px;margin-bottom:24px">
      <button class="btn btn-outline btn-sm" onclick="backToOrgs()">← Back</button>
      <h2 style="font-size:20px">${escape(org.name)}</h2>
      <span class="badge purple">${escape(org.slug)}</span>
    </div>

    <div class="stats">
      <div class="stat"><div class="label">Instances</div><div class="value blue">${instances.length}</div></div>
      <div class="stat"><div class="label">Running</div><div class="value green">${running}</div></div>
      <div class="stat"><div class="label">Stopped</div><div class="value yellow">${stopped}</div></div>
      <div class="stat"><div class="label">Members</div><div class="value">${members ? members.length : 0}</div></div>
    </div>

    <div class="tabs">
      <button class="tab active" data-tab="instances" onclick="switchTab(this)">Instances</button>
      <button class="tab" data-tab="members" onclick="switchTab(this)">Members</button>
      <button class="tab" data-tab="billing" onclick="switchTab(this)">Billing</button>
    </div>

    <div id="tab-content">
      ${renderInstancesTab(instances)}
    </div>`;
}

window.switchTab = function(btn) {
  document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
  btn.classList.add('active');
  const tab = btn.dataset.tab;
  const main = document.getElementById('tab-content');
  if (tab === 'instances') main.innerHTML = renderInstancesTab(state.instances);
  else if (tab === 'members') renderMembersTab();
  else if (tab === 'billing') renderBillingTab();
};

function renderInstancesTab(instances) {
  return `
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <h3>Compute Instances</h3>
      <button class="btn btn-primary btn-sm" onclick="showCreateInstance()">+ New Instance</button>
    </div>
    ${instances.length === 0 ? '<div class="card empty"><p>No instances. Create your first one!</p></div>' :
    `<div class="card"><table><thead><tr><th>Name</th><th>Type</th><th>Status</th><th>Specs</th><th>IP</th><th>Region</th><th></th></tr></thead><tbody>
      ${instances.map(i => `<tr style="cursor:pointer" onclick="showInstanceDetail(${i.id})">
        <td><strong>${escape(i.name)}</strong></td>
        <td><span class="badge ${i.instance_type === 'vm' ? 'purple' : 'gray'}">${i.instance_type}</span></td>
        <td><span class="badge ${i.status === 'running' ? 'green' : i.status === 'stopped' ? 'yellow' : 'red'}">${i.status}</span></td>
        <td style="color:var(--muted);font-size:13px">${i.cpu_cores}vCPU · ${i.memory_mb}MB · ${i.disk_gb}GB</td>
        <td style="font-family:monospace;font-size:13px;color:${i.ip_address ? 'var(--text)' : 'var(--muted)'}">${i.ip_address || '-'}</td>
        <td style="color:var(--muted)">${i.region}</td>
        <td style="display:flex;gap:4px" onclick="event.stopPropagation()">
          ${i.status === 'running' ? `<button class="btn btn-outline btn-sm" onclick="action('stop',${i.id})" title="Stop instance">⏸</button>` : ''}
          ${i.status === 'stopped' ? `<button class="btn btn-outline btn-sm" onclick="action('start',${i.id})" title="Start instance">▶</button>` : ''}
          ${i.status !== 'terminated' ? `<button class="btn btn-danger btn-sm" onclick="confirmAction('terminate',${i.id})" title="Terminate instance">🗑</button>` : ''}
        </td>
      </tr>`).join('')}
    </tbody></table></div>`}`;
}

window.showInstanceDetail = function(instanceId) {
  const instance = state.instances.find(i => i.id === instanceId);
  if (!instance) return toast('Instance not found', 'error');
  
  modal(`
    <div style="display:flex;justify-content:space-between;align-items:start">
      <div>
        <h2>${escape(instance.name)}</h2>
        <p style="color:var(--muted);font-size:13px">ID: ${instance.id}</p>
      </div>
      <span class="badge ${instance.status === 'running' ? 'green' : instance.status === 'stopped' ? 'yellow' : 'red'}" style="margin-top:4px">${instance.status}</span>
    </div>
    
    <div style="margin-top:24px">
      <h3 style="margin-bottom:12px">Instance Details</h3>
      <div class="details-grid">
        <div class="detail-item">
          <div class="detail-label">Type</div>
          <div class="detail-value">${instance.instance_type}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">Region</div>
          <div class="detail-value">${instance.region}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">CPU Cores</div>
          <div class="detail-value">${instance.cpu_cores}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">Memory</div>
          <div class="detail-value">${instance.memory_mb}MB</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">Disk</div>
          <div class="detail-value">${instance.disk_gb}GB</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">IP Address</div>
          <div class="detail-value" style="font-family:monospace">${instance.ip_address || 'Not assigned'}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">Created</div>
          <div class="detail-value">${new Date(instance.created_at).toLocaleDateString()}</div>
        </div>
        <div class="detail-item">
          <div class="detail-label">Uptime</div>
          <div class="detail-value">${instance.status === 'running' ? 'Running' : 'Stopped'}</div>
        </div>
      </div>
    </div>

    <div style="margin-top:24px">
      <h3 style="margin-bottom:12px">Actions</h3>
      <div style="display:flex;gap:8px;flex-wrap:wrap">
        ${instance.status === 'running' ? `<button class="btn btn-outline" onclick="action('stop',${instance.id}); document.querySelector('.modal-overlay').remove()">⏸ Stop</button>` : ''}
        ${instance.status === 'stopped' ? `<button class="btn btn-outline" onclick="action('start',${instance.id}); document.querySelector('.modal-overlay').remove()">▶ Start</button>` : ''}
        <button class="btn btn-outline" onclick="copyToClipboard('${instance.ip_address || 'N/A'}')">📋 Copy IP</button>
        ${instance.status !== 'terminated' ? `<button class="btn btn-danger" onclick="confirmAction('terminate',${instance.id}); document.querySelector('.modal-overlay').remove()">🗑 Terminate</button>` : ''}
      </div>
    </div>

    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Close</button>
    </div>
  `);
};

window.copyToClipboard = function(text) {
  navigator.clipboard.writeText(text).then(() => {
    toast('Copied to clipboard');
  }).catch(() => {
    toast('Failed to copy', 'error');
  });
};

window.confirmAction = function(action, instanceId) {
  modal(`
    <h2>⚠️ Confirm Action</h2>
    <p style="color:var(--muted);margin:16px 0">Are you sure you want to ${action} this instance? This action cannot be undone.</p>
    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-danger" onclick="action('${action}',${instanceId}); document.querySelector('.modal-overlay').remove()">Yes, ${action}</button>
    </div>
  `);
};

async function renderMembersTab() {
  const main = document.getElementById('tab-content');
  main.innerHTML = '<div class="loading">Loading...</div>';
  const { ok, body: members } = await api(`/orgs/${state.currentOrg}/members`);
  if (!ok) { main.innerHTML = '<div class="empty">Failed to load members</div>'; return; }
  main.innerHTML = `
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <h3>Team Members</h3>
      <button class="btn btn-primary btn-sm" onclick="showInviteMember()">+ Invite</button>
    </div>
    <div class="card"><table><thead><tr><th>User ID</th><th>Role</th><th>Joined</th></tr></thead><tbody>
      ${members.map(m => `<tr><td>#${m.user_id}</td><td><span class="badge ${m.role === 'admin' ? 'purple' : 'gray'}">${m.role}</span></td><td style="color:var(--muted)">${new Date(m.created_at).toLocaleDateString()}</td></tr>`).join('')}
    </tbody></table></div>`;
}

async function renderBillingTab() {
  const main = document.getElementById('tab-content');
  main.innerHTML = '<div class="loading">Loading billing data...</div>';
  const { ok: usageOk, body: usage } = await api(`/orgs/${state.currentOrg}/billing/usage`);
  const { ok: invOk, body: invoices } = await api(`/orgs/${state.currentOrg}/billing/invoices`);
  
  if (!usageOk || !invOk) {
    main.innerHTML = '<div class="card empty"><p>Failed to load billing data</p></div>';
    return;
  }

  const totalCost = invoices?.reduce((sum, inv) => sum + (inv.amount_cents / 100), 0) || 0;
  const avgMonthlyCost = totalCost / Math.max(invoices?.length || 1, 1);
  
  main.innerHTML = `
    <div class="billing-container">
      <div class="stats">
        <div class="stat">
          <div class="label">Est. Monthly Cost</div>
          <div class="value yellow">$${avgMonthlyCost.toFixed(2)}</div>
          <div style="font-size:12px;color:var(--muted);margin-top:4px">based on ${invoices?.length || 0} invoices</div>
        </div>
        <div class="stat">
          <div class="label">CPU Hours (30d)</div>
          <div class="value blue">${(usage?.cpu_hours || 0).toFixed(1)}</div>
        </div>
        <div class="stat">
          <div class="label">Memory GB-hrs</div>
          <div class="value green">${(usage?.memory_gb_hours || 0).toFixed(1)}</div>
        </div>
        <div class="stat">
          <div class="label">Disk GB-hrs</div>
          <div class="value">${(usage?.disk_gb_hours || 0).toFixed(1)}</div>
        </div>
      </div>

      <div class="card">
        <h3>Usage Breakdown</h3>
        <div class="usage-chart">
          <div class="chart-bar">
            <div class="bar-label">CPU</div>
            <div class="bar" style="width:${Math.min((usage?.cpu_hours || 0) / (usage?.cpu_hours || 1) * 100, 100)}%;background:linear-gradient(90deg,#fbbf24,#f59e0b)"></div>
            <div class="bar-value">${(usage?.cpu_hours || 0).toFixed(1)}h</div>
          </div>
          <div class="chart-bar">
            <div class="bar-label">Memory</div>
            <div class="bar" style="width:${Math.min((usage?.memory_gb_hours || 0) / Math.max(usage?.cpu_hours || 1, 1) * 100, 100)}%;background:linear-gradient(90deg,#60a5fa,#3b82f6)"></div>
            <div class="bar-value">${(usage?.memory_gb_hours || 0).toFixed(1)}GB-h</div>
          </div>
          <div class="chart-bar">
            <div class="bar-label">Disk</div>
            <div class="bar" style="width:${Math.min((usage?.disk_gb_hours || 0) / Math.max(usage?.cpu_hours || 1, 1) * 100, 100)}%;background:linear-gradient(90deg,#4ade80,#22c55e)"></div>
            <div class="bar-value">${(usage?.disk_gb_hours || 0).toFixed(1)}GB-h</div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h3>Recent Invoices</h3>
          <button class="btn btn-outline btn-sm" onclick="exportInvoices()">📥 Export</button>
        </div>
        ${!invoices || invoices.length === 0 ? '<div class="empty"><p>No invoices yet</p></div>' :
        `<div class="invoice-table"><table><thead><tr><th>Period</th><th>Amount</th><th>Status</th><th>Date</th><th></th></tr></thead><tbody>
          ${invoices.map(inv => `<tr>
            <td style="color:var(--muted);font-size:13px">${new Date(inv.period_start).toLocaleDateString()} - ${new Date(inv.period_end).toLocaleDateString()}</td>
            <td><strong>$${(inv.amount_cents/100).toFixed(2)}</strong></td>
            <td><span class="badge ${inv.status === 'paid' ? 'green' : inv.status === 'overdue' ? 'red' : 'yellow'}">${inv.status}</span></td>
            <td style="color:var(--muted);font-size:13px">${new Date(inv.created_at).toLocaleDateString()}</td>
            <td><button class="btn btn-outline btn-sm" onclick="viewInvoice(${inv.id})">View</button></td>
          </tr>`).join('')}
        </tbody></table></div>`}
      </div>
    </div>`;
}

window.exportInvoices = function() {
  toast('Invoice export feature coming soon', 'info');
};

window.viewInvoice = function(invoiceId) {
  modal(`
    <h2>Invoice #${invoiceId}</h2>
    <div class="form-group" style="color:var(--muted);line-height:1.8;margin:20px 0">
      <p><strong>Invoice Details:</strong></p>
      <p>Invoice ID: #${invoiceId}</p>
      <p>Organization: ${escape(state.orgs.find(o => o.id === state.currentOrg)?.name || 'Unknown')}</p>
      <p>Generated: ${new Date().toLocaleDateString()}</p>
      <p style="margin-top:20px"><strong>Status:</strong> <span class="badge green">Available</span></p>
    </div>
    <div class="modal-actions">
      <button class="btn btn-primary" onclick="downloadInvoice(${invoiceId})">📥 Download PDF</button>
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Close</button>
    </div>`);
};

window.downloadInvoice = function(invoiceId) {
  toast('Invoice download feature coming soon', 'info');
};

window.showCreateOrg = function() {
  const m = modal(`
    <h2>🏢 Create Organization</h2>
    <p style="color:var(--muted);font-size:13px;margin-bottom:16px">Create a new organization to manage your infrastructure</p>
    <div class="form-group">
      <label>Organization Name *</label>
      <input type="text" id="org-name" placeholder="e.g., Acme Corp Cloud" maxlength="50">
      <div style="font-size:12px;color:var(--muted);margin-top:4px" id="name-hint"></div>
    </div>
    <div class="form-group">
      <label>Slug *</label>
      <input type="text" id="org-slug" placeholder="e.g., acme-corp" maxlength="30">
      <div style="font-size:12px;color:var(--muted);margin-top:4px">Used in URLs - lowercase letters, numbers, and hyphens only</div>
    </div>
    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-primary" onclick="createOrg()">Create Organization</button>
    </div>
  `);
  
  // Auto-generate slug from name
  document.getElementById('org-name').addEventListener('input', function() {
    const slug = this.value.toLowerCase().replace(/[^a-z0-9-]/g, '').replace(/--+/g, '-');
    document.getElementById('org-slug').value = slug;
    document.getElementById('name-hint').textContent = `${this.value.length}/50 characters`;
  });
};

window.createOrg = async function() {
  const name = document.getElementById('org-name').value.trim();
  const slug = document.getElementById('org-slug').value.trim();
  
  if (!name) return toast('Organization name is required', 'error');
  if (!slug) return toast('Slug is required', 'error');
  if (!/^[a-z0-9-]+$/.test(slug)) return toast('Slug must contain only lowercase letters, numbers, and hyphens', 'error');
  
  const { ok, body } = await api('/orgs', { method: 'POST', body: JSON.stringify({ name, slug }) });
  if (!ok) return toast(body.error || 'Failed to create organization', 'error');
  
  addActivityLog('org.create', `Created organization "${name}"`);
  document.querySelector('.modal-overlay').remove();
  toast('✅ Organization created successfully', 'success');
  renderView();
};

window.showCreateInstance = function() {
  const m = modal(`
    <h2>🚀 Launch Compute Instance</h2>
    <p style="color:var(--muted);font-size:13px;margin-bottom:16px">Create a new compute instance with your desired specifications</p>
    
    <div class="form-group">
      <label>Instance Name *</label>
      <input type="text" id="inst-name" placeholder="e.g., web-server-01" maxlength="50">
      <div style="font-size:12px;color:var(--muted);margin-top:4px">Lowercase letters, numbers, and hyphens</div>
    </div>
    
    <div class="form-row">
      <div class="form-group">
        <label>Instance Type *</label>
        <select id="inst-type" onchange="updateInstancePresets()">
          <option value="">Select type...</option>
          <option value="vm">Virtual Machine</option>
          <option value="container">Container</option>
        </select>
      </div>
      <div class="form-group">
        <label>Region *</label>
        <select id="inst-region">
          <option value="">Select region...</option>
          <option value="us-east">US East (us-east-1)</option>
          <option value="us-west">US West (us-west-2)</option>
          <option value="eu-west">EU West (eu-west-1)</option>
          <option value="ap-south">Asia Pacific (ap-south-1)</option>
        </select>
      </div>
    </div>

    <div class="form-group">
      <label>Resource Preset</label>
      <select id="inst-preset" onchange="applyPreset()">
        <option value="custom">Custom</option>
        <option value="micro">Micro (1vCPU, 512MB)</option>
        <option value="small">Small (2vCPU, 2GB)</option>
        <option value="medium">Medium (4vCPU, 8GB)</option>
        <option value="large">Large (8vCPU, 16GB)</option>
      </select>
    </div>

    <div class="form-row">
      <div class="form-group">
        <label>CPU Cores *</label>
        <select id="inst-cpu">
          <option value="1">1 Core</option>
          <option value="2">2 Cores</option>
          <option value="4">4 Cores</option>
          <option value="8">8 Cores</option>
          <option value="16">16 Cores</option>
        </select>
      </div>
      <div class="form-group">
        <label>Memory (GB) *</label>
        <select id="inst-memory">
          <option value="512">512 MB</option>
          <option value="1024">1 GB</option>
          <option value="2048">2 GB</option>
          <option value="4096">4 GB</option>
          <option value="8192">8 GB</option>
          <option value="16384">16 GB</option>
        </select>
      </div>
    </div>

    <div class="form-group">
      <label>Disk (GB) *</label>
      <select id="inst-disk">
        <option value="10">10 GB</option>
        <option value="20">20 GB</option>
        <option value="50">50 GB</option>
        <option value="100">100 GB</option>
        <option value="250">250 GB</option>
        <option value="500">500 GB</option>
      </select>
    </div>

    <div style="background:var(--bg);padding:12px;border-radius:6px;margin:16px 0;font-size:12px;color:var(--muted)">
      <strong>Estimated Monthly Cost:</strong> $<span id="cost-estimate">0.00</span>
    </div>

    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-primary" onclick="createInstance()">Launch Instance</button>
    </div>
  `);

  // Set up event listeners for cost estimation
  ['inst-cpu', 'inst-memory', 'inst-disk'].forEach(id => {
    document.getElementById(id).addEventListener('change', updateCostEstimate);
  });
  updateCostEstimate();
};

window.updateInstancePresets = function() {
  const type = document.getElementById('inst-type').value;
  if (!type) {
    document.getElementById('inst-preset').value = 'custom';
  }
};

window.applyPreset = function() {
  const preset = document.getElementById('inst-preset').value;
  const presets = {
    micro: { cpu: '1', memory: '512', disk: '10' },
    small: { cpu: '2', memory: '1024', disk: '20' },
    medium: { cpu: '4', memory: '2048', disk: '50' },
    large: { cpu: '8', memory: '4096', disk: '100' },
  };
  
  if (presets[preset]) {
    const p = presets[preset];
    document.getElementById('inst-cpu').value = p.cpu;
    document.getElementById('inst-memory').value = p.memory;
    document.getElementById('inst-disk').value = p.disk;
    updateCostEstimate();
  }
};

window.updateCostEstimate = function() {
  const cpu = parseInt(document.getElementById('inst-cpu').value) || 0;
  const memory = parseInt(document.getElementById('inst-memory').value) / 1024 || 0;
  const disk = parseInt(document.getElementById('inst-disk').value) || 0;
  
  // Simple cost calculation: $0.01 per vCPU-hour, $0.01 per GB-hour, $0.001 per disk-GB-hour
  const hourly = (cpu * 0.01) + (memory * 0.01) + (disk * 0.001);
  const monthly = (hourly * 730).toFixed(2);
  
  document.getElementById('cost-estimate').textContent = monthly;
};

window.createInstance = async function() {
  const name = document.getElementById('inst-name').value.trim();
  const instance_type = document.getElementById('inst-type').value;
  const region = document.getElementById('inst-region').value;
  const cpu_cores = parseInt(document.getElementById('inst-cpu').value) || 1;
  const memory_mb = parseInt(document.getElementById('inst-memory').value) || 1024;
  const disk_gb = parseInt(document.getElementById('inst-disk').value) || 10;
  
  if (!name) return toast('Instance name is required', 'error');
  if (!instance_type) return toast('Instance type is required', 'error');
  if (!region) return toast('Region is required', 'error');
  
  const { ok, body } = await api(`/orgs/${state.currentOrg}/instances`, {
    method: 'POST',
    body: JSON.stringify({ name, instance_type, region, cpu_cores, memory_mb, disk_gb })
  });
  
  if (!ok) return toast(body.error || 'Failed to launch instance', 'error');
  
  addActivityLog('instance.create', `Launched instance "${name}" (${cpu_cores}vCPU, ${memory_mb}MB)`);
  document.querySelector('.modal-overlay').remove();
  toast('✅ Instance launching...', 'success');
  
  setTimeout(() => {
    const org = state.orgs.find(o => o.id === state.currentOrg);
    if (org) renderOrgDetail(org);
  }, 1000);
};

window.showInviteMember = function() {
  const m = modal(`
    <h2>👥 Invite Team Member</h2>
    <p style="color:var(--muted);font-size:13px;margin-bottom:16px">Add a new member to your organization</p>
    
    <div class="form-group">
      <label>Email Address *</label>
      <input type="email" id="invite-email" placeholder="colleague@company.com">
      <div style="font-size:12px;color:var(--muted);margin-top:4px">We'll send them an invitation link</div>
    </div>
    
    <div class="form-group">
      <label>Role *</label>
      <select id="invite-role">
        <option value="member">Member (Can view and manage instances)</option>
        <option value="admin">Admin (Full access)</option>
      </select>
    </div>

    <div style="background:var(--bg);padding:12px;border-radius:6px;margin:16px 0;font-size:12px;color:var(--muted)">
      <strong>Role Permissions:</strong>
      <div id="role-info" style="margin-top:8px">
        <div>✓ View organization and instances</div>
        <div>✓ Start/stop instances</div>
        <div>✗ Delete organization</div>
        <div>✗ Manage members</div>
      </div>
    </div>

    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-primary" onclick="inviteMember()">Send Invitation</button>
    </div>
  `);

  document.getElementById('invite-role').addEventListener('change', function() {
    const info = document.getElementById('role-info');
    if (this.value === 'admin') {
      info.innerHTML = '<div>✓ Full access to organization</div><div>✓ Manage team members</div><div>✓ Delete organization</div>';
    } else {
      info.innerHTML = '<div>✓ View organization and instances</div><div>✓ Start/stop instances</div><div>✗ Delete organization</div><div>✗ Manage members</div>';
    }
  });
};

window.inviteMember = async function() {
  const email = document.getElementById('invite-email').value.trim();
  const role = document.getElementById('invite-role').value;
  
  if (!email) return toast('Email is required', 'error');
  if (!email.includes('@')) return toast('Please enter a valid email', 'error');
  if (!role) return toast('Role is required', 'error');
  
  const { ok, body } = await api(`/orgs/${state.currentOrg}/members`, {
    method: 'POST',
    body: JSON.stringify({ email, role })
  });
  
  if (!ok) return toast(body.error || 'Failed to send invitation', 'error');
  
  addActivityLog('member.invite', `Invited ${email} as ${role}`);
  document.querySelector('.modal-overlay').remove();
  toast(`✅ Invitation sent to ${email}`, 'success');
  renderMembersTab();
};

window.action = async function(action, instanceId) {
  const { ok, body } = await api(`/orgs/${state.currentOrg}/instances/${instanceId}/${action}`, { method: 'POST' });
  if (!ok) return toast(body.error || 'Action failed', 'error');
  
  const actionVerb = action === 'terminate' ? 'terminated' : action === 'stop' ? 'stopped' : 'started';
  toast(`Instance ${actionVerb} successfully`, 'success');
  addActivityLog(`instance.${action}`, `Instance #${instanceId} was ${actionVerb}`);
  
  // Refresh the current view
  if (state.currentOrg) {
    const org = state.orgs.find(o => o.id === state.currentOrg);
    if (org) renderOrgDetail(org);
  }
};

window.selectOrg = function(id) {
  state.currentOrg = id;
  renderOrgs();
};

window.backToOrgs = function() {
  state.currentOrg = null;
  renderOrgs();
};

// Settings Page
function renderSettings() {
  const main = document.getElementById('main');
  main.innerHTML = `
    <div style="max-width:600px">
      <h2 style="font-size:20px;margin-bottom:24px">Settings</h2>

      <div class="card">
        <h3>Account Settings</h3>
        <div style="margin-top:16px">
          <div class="form-group">
            <label>Email Address</label>
            <input type="email" value="${escape(state.user?.email || '')}" disabled>
          </div>
          <button class="btn btn-outline" onclick="showChangePasswordModal()">Change Password</button>
        </div>
      </div>

      <div class="card">
        <h3>Preferences</h3>
        <div style="margin-top:16px;display:flex;flex-direction:column;gap:12px">
          <label style="display:flex;align-items:center;gap:8px;cursor:pointer;color:var(--text)">
            <input type="checkbox" ${state.settings.emailNotifications ? 'checked' : ''} onchange="updateSetting('emailNotifications', this.checked)">
            <span>Email me about important updates</span>
          </label>
          <label style="display:flex;align-items:center;gap:8px;cursor:pointer;color:var(--text)">
            <input type="checkbox" ${state.settings.autoRefresh ? 'checked' : ''} onchange="updateSetting('autoRefresh', this.checked)">
            <span>Auto-refresh dashboard (every 30 seconds)</span>
          </label>
        </div>
      </div>

      <div class="card">
        <h3>Activity Log</h3>
        <div style="margin-top:16px">
          ${state.activityLog.length === 0 ? 
            '<div class="empty"><p>No activity yet</p></div>' :
            `<div class="activity-log">
              ${state.activityLog.slice(0, 20).map(log => `
                <div class="activity-item">
                  <div class="activity-time">${log.timestamp.toLocaleTimeString([], {hour:'2-digit', minute:'2-digit'})}</div>
                  <div class="activity-content">
                    <div class="activity-action">${log.action}</div>
                    <div class="activity-details">${log.details}</div>
                    <div class="activity-user">${log.user}</div>
                  </div>
                </div>
              `).join('')}
            </div>`
          }
        </div>
      </div>

      <div class="card">
        <h3>Danger Zone</h3>
        <button class="btn btn-danger" onclick="showDeleteAccountModal()">Delete Account</button>
      </div>
    </div>`;
}

window.showChangePasswordModal = function() {
  modal(`
    <h2>Change Password</h2>
    <div class="form-group">
      <label>Current Password</label>
      <input type="password" id="current-pwd" placeholder="••••••••">
    </div>
    <div class="form-group">
      <label>New Password</label>
      <input type="password" id="new-pwd" placeholder="••••••••">
    </div>
    <div class="form-group">
      <label>Confirm New Password</label>
      <input type="password" id="confirm-pwd" placeholder="••••••••">
    </div>
    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-primary" onclick="changePassword()">Update Password</button>
    </div>
  `);
};

window.changePassword = async function() {
  const currentPwd = document.getElementById('current-pwd').value;
  const newPwd = document.getElementById('new-pwd').value;
  const confirmPwd = document.getElementById('confirm-pwd').value;
  
  if (!currentPwd || !newPwd || !confirmPwd) {
    return toast('All fields are required', 'error');
  }
  if (newPwd !== confirmPwd) {
    return toast('Passwords do not match', 'error');
  }
  if (newPwd.length < 8) {
    return toast('Password must be at least 8 characters', 'error');
  }
  
  toast('Password change feature coming soon', 'info');
  document.querySelector('.modal-overlay').remove();
};

window.showDeleteAccountModal = function() {
  modal(`
    <h2>⚠️ Delete Account</h2>
    <p style="color:var(--danger);margin:16px 0">
      This action is permanent and cannot be undone. All your data will be lost.
    </p>
    <div class="form-group">
      <label>Type your email to confirm:</label>
      <input type="email" id="delete-confirm" placeholder="${state.user?.email}">
    </div>
    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-danger" onclick="deleteAccount()">Delete Account</button>
    </div>
  `);
};

window.deleteAccount = function() {
  const input = document.getElementById('delete-confirm').value;
  if (input !== state.user?.email) {
    return toast('Email does not match', 'error');
  }
  toast('Account deletion feature coming soon', 'info');
};

window.updateSetting = function(key, value) {
  state.settings[key] = value;
  localStorage.setItem(key, JSON.stringify(value));
  toast('Setting updated');
};

async function init() {
  if (state.token) {
    const { ok, body } = await api('/me');
    if (ok) state.user = body;
    else { state.token = null; localStorage.removeItem('token'); }
  }
  render();
}

init();
