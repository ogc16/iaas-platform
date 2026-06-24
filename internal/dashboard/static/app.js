const API = '/api/v1';

let state = {
  token: localStorage.getItem('token'),
  user: null,
  orgs: [],
  instances: [],
  currentOrg: null,
  view: 'dashboard',
};

function api(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json' };
  if (state.token) headers['Authorization'] = `Bearer ${state.token}`;
  return fetch(`${API}${path}`, { ...opts, headers })
    .then(r => r.json().then(body => ({ ok: r.ok, status: r.status, body })))
    .catch(() => ({ ok: false, body: { error: 'Network error' } }));
}

function toast(msg, type = 'success') {
  const el = document.createElement('div');
  el.className = `toast ${type}`;
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 3000);
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

function render() {
  if (!state.token) return renderLogin();
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="header">
      <h1>☁ IaaS Platform</h1>
      <nav>
        <span class="user-badge">${state.user ? state.user.email : ''}</span>
        <a href="#" data-view="dashboard" class="active">Dashboard</a>
        <a href="#" data-view="orgs">Organizations</a>
        <button onclick="logout()">Logout</button>
      </nav>
    </div>
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
}

async function renderDashboard() {
  const main = document.getElementById('main');
  main.innerHTML = '<div class="loading">Loading...</div>';

  const { ok, body: usersOrgs } = await api('/orgs');
  if (!ok) { main.innerHTML = '<div class="loading">Failed to load dashboard</div>'; return; }
  state.orgs = usersOrgs;

  let totalInstances = 0, runningInstances = 0;
  for (const org of state.orgs) {
    const { ok, body: instances } = await api(`/orgs/${org.id}/instances`);
    if (ok) {
      totalInstances += instances.length;
      runningInstances += instances.filter(i => i.status === 'running').length;
    }
  }

  const usage = state.orgs.length > 0
    ? (await api(`/orgs/${state.orgs[0].id}/billing/usage`)).body
    : null;

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
      ${instances.map(i => `<tr>
        <td><strong>${escape(i.name)}</strong></td>
        <td><span class="badge ${i.instance_type === 'vm' ? 'purple' : 'gray'}">${i.instance_type}</span></td>
        <td><span class="badge ${i.status === 'running' ? 'green' : i.status === 'stopped' ? 'yellow' : 'red'}">${i.status}</span></td>
        <td style="color:var(--muted);font-size:13px">${i.cpu_cores}vCPU · ${i.memory_mb}MB · ${i.disk_gb}GB</td>
        <td style="font-family:monospace;font-size:13px">${i.ip_address || '-'}</td>
        <td style="color:var(--muted)">${i.region}</td>
        <td style="display:flex;gap:4px">
          ${i.status === 'running' ? `<button class="btn btn-outline btn-sm" onclick="action('stop',${i.id})">Stop</button>` : ''}
          ${i.status === 'stopped' ? `<button class="btn btn-outline btn-sm" onclick="action('start',${i.id})">Start</button>` : ''}
          ${i.status !== 'terminated' ? `<button class="btn btn-danger btn-sm" onclick="action('terminate',${i.id})">Terminate</button>` : ''}
        </td>
      </tr>`).join('')}
    </tbody></table></div>`}`;
}

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
  main.innerHTML = '<div class="loading">Loading...</div>';
  const { ok: usageOk, body: usage } = await api(`/orgs/${state.currentOrg}/billing/usage`);
  const { ok: invOk, body: invoices } = await api(`/orgs/${state.currentOrg}/billing/invoices`);
  main.innerHTML = `
    <div class="stats">
      <div class="stat"><div class="label">CPU Hours</div><div class="value yellow">${usage ? usage.cpu_hours.toFixed(1) : '0'}</div></div>
      <div class="stat"><div class="label">Memory GB-hrs</div><div class="value blue">${usage ? usage.memory_gb_hours.toFixed(1) : '0'}</div></div>
      <div class="stat"><div class="label">Disk GB-hrs</div><div class="value green">${usage ? usage.disk_gb_hours.toFixed(1) : '0'}</div></div>
    </div>
    <h3 style="margin-bottom:12px">Invoices</h3>
    ${!invoices || invoices.length === 0 ? '<div class="card empty"><p>No invoices yet</p></div>' :
    `<div class="card"><table><thead><tr><th>Period</th><th>Amount</th><th>Status</th><th>Date</th></tr></thead><tbody>
      ${invoices.map(inv => `<tr>
        <td style="color:var(--muted)">${new Date(inv.period_start).toLocaleDateString()} - ${new Date(inv.period_end).toLocaleDateString()}</td>
        <td><strong>$${(inv.amount_cents/100).toFixed(2)}</strong></td>
        <td><span class="badge ${inv.status === 'paid' ? 'green' : inv.status === 'overdue' ? 'red' : 'yellow'}">${inv.status}</span></td>
        <td style="color:var(--muted)">${new Date(inv.created_at).toLocaleDateString()}</td>
      </tr>`).join('')}
    </tbody></table></div>`}`;
}

window.showCreateOrg = function() {
  const m = modal(`
    <h2>New Organization</h2>
    <div class="form-group"><label>Name</label><input type="text" id="org-name" placeholder="My Cloud"></div>
    <div class="form-group"><label>Slug</label><input type="text" id="org-slug" placeholder="my-cloud"></div>
    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-primary" onclick="createOrg()">Create</button>
    </div>`);
};

window.createOrg = async function() {
  const name = document.getElementById('org-name').value;
  const slug = document.getElementById('org-slug').value;
  if (!name) return toast('Name is required', 'error');
  const { ok, body } = await api('/orgs', { method: 'POST', body: JSON.stringify({ name, slug }) });
  if (!ok) return toast(body.error || 'Failed to create org', 'error');
  document.querySelector('.modal-overlay').remove();
  toast('Organization created');
  renderView();
};

window.showCreateInstance = function() {
  const m = modal(`
    <h2>New Instance</h2>
    <div class="form-group"><label>Name</label><input type="text" id="inst-name" placeholder="web-server"></div>
    <div class="form-row">
      <div class="form-group"><label>Type</label><select id="inst-type"><option value="vm">VM</option><option value="container">Container</option></select></div>
      <div class="form-group"><label>Region</label><select id="inst-region"><option value="us-east">US East</option><option value="us-west">US West</option><option value="eu-west">EU West</option></select></div>
    </div>
    <div class="form-row">
      <div class="form-group"><label>CPU Cores</label><input type="number" id="inst-cpu" value="1" min="1"></div>
      <div class="form-group"><label>Memory (MB)</label><input type="number" id="inst-memory" value="1024" min="512"></div>
    </div>
    <div class="form-group"><label>Disk (GB)</label><input type="number" id="inst-disk" value="10" min="1"></div>
    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-primary" onclick="createInstance()">Create</button>
    </div>`);
};

window.createInstance = async function() {
  const name = document.getElementById('inst-name').value;
  const instance_type = document.getElementById('inst-type').value;
  const region = document.getElementById('inst-region').value;
  const cpu_cores = parseInt(document.getElementById('inst-cpu').value) || 1;
  const memory_mb = parseInt(document.getElementById('inst-memory').value) || 1024;
  const disk_gb = parseInt(document.getElementById('inst-disk').value) || 10;
  if (!name) return toast('Name is required', 'error');
  const { ok, body } = await api(`/orgs/${state.currentOrg}/instances`, {
    method: 'POST',
    body: JSON.stringify({ name, instance_type, region, cpu_cores, memory_mb, disk_gb })
  });
  if (!ok) return toast(body.error || 'Failed to create instance', 'error');
  document.querySelector('.modal-overlay').remove();
  toast('Instance created');
  renderOrgDetail(state.orgs.find(o => o.id === state.currentOrg));
};

window.showInviteMember = function() {
  const m = modal(`
    <h2>Invite Member</h2>
    <div class="form-group"><label>Email</label><input type="email" id="invite-email" placeholder="colleague@company.com"></div>
    <div class="form-group"><label>Role</label><select id="invite-role"><option value="member">Member</option><option value="admin">Admin</option></select></div>
    <div class="modal-actions">
      <button class="btn btn-outline" onclick="this.closest('.modal-overlay').remove()">Cancel</button>
      <button class="btn btn-primary" onclick="inviteMember()">Invite</button>
    </div>`);
};

window.inviteMember = async function() {
  const email = document.getElementById('invite-email').value;
  const role = document.getElementById('invite-role').value;
  if (!email) return toast('Email is required', 'error');
  const { ok, body } = await api(`/orgs/${state.currentOrg}/members`, { method: 'POST', body: JSON.stringify({ email, role }) });
  if (!ok) return toast(body.error || 'Failed to invite', 'error');
  document.querySelector('.modal-overlay').remove();
  toast('Member invited');
  renderMembersTab();
};

window.action = async function(action, instanceId) {
  const { ok, body } = await api(`/orgs/${state.currentOrg}/instances/${instanceId}/${action}`, { method: 'POST' });
  if (!ok) return toast(body.error || 'Action failed', 'error');
  toast(`Instance ${action}ed`);
  renderOrgDetail(state.orgs.find(o => o.id === state.currentOrg));
};

window.selectOrg = function(id) {
  state.currentOrg = id;
  renderOrgs();
};

window.backToOrgs = function() {
  state.currentOrg = null;
  renderOrgs();
};

function escape(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

async function init() {
  if (state.token) {
    const { ok, body } = await api('/me');
    if (ok) state.user = body;
    else { state.token = null; localStorage.removeItem('token'); }
  }
  render();
}

init();
