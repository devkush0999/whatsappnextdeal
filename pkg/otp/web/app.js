'use strict';
const $ = id => document.getElementById(id);
let adminToken = '', current = null, projects = [];
const notice = (message, error = false) => { $('notice').textContent = message; $('notice').classList.toggle('error', error); };
async function api(path, { method = 'GET', body, key = adminToken, idempotency } = {}) {
  const headers = { Authorization: `Bearer ${key}` };
  if (body) headers['Content-Type'] = 'application/json';
  if (idempotency) headers['Idempotency-Key'] = idempotency;
  const response = await fetch(path, { method, headers, body: body ? JSON.stringify(body) : undefined, cache: 'no-store', credentials: 'omit' });
  const data = await response.json();
  if (!response.ok) throw new Error(`${data.error?.message || 'Request failed'}${data.error?.retryAfter ? ` — retry in ${data.error.retryAfter}s` : ''}`);
  return data;
}
const bind = (id, event, fn) => $(id).addEventListener(event, async e => {
  e.preventDefault();
  const button = e.submitter || (e.currentTarget.tagName === 'BUTTON' ? e.currentTarget : null);
  if (button) button.disabled = true;
  try { await fn(e); } catch (error) { notice(error.message, true); } finally { if (button) button.disabled = false; }
});
function node(tag, text, className) { const element = document.createElement(tag); element.textContent = text; if (className) element.className = className; return element; }
function showKey(key) { $('key-output').value = key; $('new-key').hidden = false; }
function showChallenge(data) { $('challenge').hidden = false; $('challenge-id').textContent = data.challengeId; $('challenge-status').textContent = `${data.status} · ${data.deliveryStatus} · expires ${new Date(data.expiresAt).toLocaleTimeString()}`; $('sandbox-code').hidden = !data.sandbox || data.status !== 'ACTIVE'; $('resend').disabled = ['VERIFIED', 'SUPERSEDED', 'CANCELLED'].includes(data.status); $('result').textContent = JSON.stringify(data, null, 2); }
async function refresh() {
  const [overview, list, activity] = await Promise.all([api('/otp/admin/overview'), api('/otp/admin/projects'), api('/otp/admin/activity')]);
  projects = list.projects || [];
  for (const [id, value] of Object.entries({ projects: overview.projects, active: overview.active, verified: overview.verifiedToday, attention: overview.needsAttention })) $(`metric-${id}`).textContent = value;
  for (const option of $('provider').options) option.disabled = !overview.providers[option.value];
  if ($('provider').selectedOptions[0]?.disabled) { const available = [...$('provider').options].find(o => !o.disabled); if (available) $('provider').value = available.value; }
  providerFields();
  $('project-list').replaceChildren();
  if (!projects.length) $('project-list').append(node('p', 'No projects yet. Create one to start testing.', 'muted'));
  for (const p of projects) {
    const row = node('div', '', 'project-item'); row.append(node('strong', p.name), node('small', `${p.provider} · ${p.enabled ? 'Enabled' : 'Paused'} · ${p.dailyLimit}/day`), node('small', p.id));
    const rotate = node('button', 'Rotate API key', 'secondary'); rotate.onclick = async () => { if (!confirm('Rotate this key? The previous key will stop working immediately.')) return; try { const result = await api(`/otp/admin/projects/${p.id}/rotate`, { method: 'POST' }); showKey(result.apiKey); notice('Key rotated. Update the application backend.'); } catch (e) { notice(e.message, true); } };
    const toggle = node('button', p.enabled ? 'Pause project' : 'Enable project', 'secondary'); toggle.onclick = async () => { try { await api(`/otp/admin/projects/${p.id}`, { method: 'PATCH', body: { enabled: !p.enabled } }); await refresh(); } catch (e) { notice(e.message, true); } };
    row.append(rotate, toggle); $('project-list').append(row);
  }
  $('jobs').replaceChildren();
  for (const job of activity.jobs || []) { const row = document.createElement('tr'); for (const value of [new Date(job.createdAt).toLocaleString(), projects.find(p => p.id === job.projectId)?.name || job.projectId.slice(0, 8), job.challengeId.slice(0, 8), job.provider, job.status + (job.errorCode ? ` · ${job.errorCode}` : ''), job.attempts]) row.append(node('td', value)); $('jobs').append(row); }
  if (!activity.jobs?.length) { const row = document.createElement('tr'); const cell = node('td', 'No deliveries yet. Try the playground.'); cell.colSpan = 6; row.append(cell); $('jobs').append(row); }
  $('audits').replaceChildren(...(activity.audits || []).map(a => node('div', `${new Date(a.createdAt).toLocaleString()} · ${a.action} · ${a.resourceId || ''}`)));
}
function providerFields() { $('instance-field').hidden = $('provider').value !== 'evolution'; $('template-fields').hidden = $('provider').value !== 'meta'; }
$('provider').addEventListener('change', providerFields);
bind('login-form', 'submit', async e => { adminToken = new FormData(e.target).get('token'); try { await refresh(); } catch (err) { adminToken = ''; throw err; } e.target.reset(); $('login').hidden = true; $('workspace').hidden = false; $('logout').hidden = false; notice('Workspace unlocked.'); });
bind('logout', 'click', () => location.reload());
bind('refresh', 'click', refresh);
bind('project-form', 'submit', async e => { const data = new FormData(e.target); const body = { name: data.get('name'), provider: data.get('provider'), ttlSeconds: Number(data.get('ttlSeconds')), dailyLimit: Number(data.get('dailyLimit')) }; if (body.provider === 'evolution') body.instanceId = data.get('instanceId'); if (body.provider === 'meta') { body.template = data.get('template'); body.language = data.get('language'); } const result = await api('/otp/admin/projects', { method: 'POST', body }); await refresh(); showKey(result.apiKey); notice('Project created. Save the API key before leaving this page.'); });
bind('use-key', 'click', () => { $('project-key').value = $('key-output').value; $('playground').scrollIntoView({ behavior: 'smooth' }); });
bind('hide-key', 'click', () => { $('key-output').value = ''; $('new-key').hidden = true; });
bind('request-form', 'submit', async e => { const data = new FormData(e.target); const key = data.get('apiKey'); const body = { phone: data.get('phone'), purpose: data.get('purpose'), reference: data.get('reference'), consent: data.get('consent') === 'on' }; const projectId = key.slice(4).split('.')[0]; const project = projects.find(p => p.id === projectId); if (!project) throw new Error('Select a key for a project shown in this console.'); if (project.provider !== 'sandbox' && !confirm(`Send a real verification message to ${body.phone}?`)) return; const result = await api('/v1/otp/request', { method: 'POST', body, key, idempotency: crypto.randomUUID() }); current = { key, ...body, id: result.challengeId }; $('code').value = ''; showChallenge(result); notice(result.sandbox ? 'Sandbox challenge queued. No WhatsApp message will be sent.' : 'Verification message queued.'); await refresh(); });
bind('status', 'click', async () => { if (current) showChallenge(await api(`/v1/otp/challenges/${current.id}`, { key: current.key })); });
bind('resend', 'click', async () => { if (!current) return; if (!confirm('Request a replacement code? The previous code will stop working.')) return; const result = await api('/v1/otp/resend', { method: 'POST', body: { challengeId: current.id, consent: true }, key: current.key, idempotency: crypto.randomUUID() }); current.id = result.challengeId; $('code').value = ''; showChallenge(result); await refresh(); });
bind('sandbox-code', 'click', async () => { if (!current) return; const result = await api(`/otp/admin/sandbox/${current.id}/code`, { method: 'POST' }); $('code').value = result.code; notice('Sandbox code filled. This action is recorded in the audit trail.'); });
bind('verify-form', 'submit', async () => { if (!current) return; const result = await api('/v1/otp/verify', { method: 'POST', key: current.key, body: { challengeId: current.id, code: $('code').value, purpose: current.purpose, reference: current.reference } }); $('code').value = ''; showChallenge(await api(`/v1/otp/challenges/${current.id}`, { key: current.key })); $('result').textContent = JSON.stringify(result, null, 2); notice(result.sandbox ? 'Sandbox verification passed. This is not a real customer authentication.' : 'Verification passed. This code has been consumed.'); await refresh(); });
