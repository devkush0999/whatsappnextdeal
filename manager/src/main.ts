import {
  instancesFrom,
  escapeHtml as h,
  qrImage,
  maskedNumber,
  type Instance,
} from "./model.js";
const root = document.querySelector<HTMLDivElement>("#app")!;
let key = "",
  rows: Instance[] = [],
  view = "overview",
  pending = false,
  loading = false,
  error = "",
  lastSync = "",
  generation = 0;
let activity: { time: string; message: string }[] = [];
let qrTimer: ReturnType<typeof setTimeout> | undefined;
const controllers = new Set<AbortController>();
function log(message: string) {
  activity.unshift({ time: new Date().toLocaleTimeString(), message });
  activity = activity.slice(0, 50);
}
async function api(
  path: string,
  token: string,
  body?: unknown,
): Promise<unknown> {
  const c = new AbortController();
  controllers.add(c);
  const timer = setTimeout(() => c.abort(), 30000);
  try {
    const r = await fetch(path, {
      method: body === undefined ? "GET" : "POST",
      headers: { apikey: token, "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
      signal: c.signal,
      cache: "no-store",
      redirect: "error",
    });
    if (!r.ok)
      throw new Error(
        r.status === 401 || r.status === 403
          ? "Access denied. Check the API key."
          : r.status === 429
            ? "Too many requests. Wait before retrying."
            : `Request failed (${r.status}). Check the service and try again.`,
      );
    return await r.json();
  } catch (e) {
    if (e instanceof DOMException)
      throw new Error(
        "Request timed out or was cancelled. Check connection status before retrying.",
      );
    throw e;
  } finally {
    clearTimeout(timer);
    controllers.delete(c);
  }
}
function login() {
  root.innerHTML = `<main class="login"><div class="brand"><span class="logo">N</span> NextDeal <span class="muted">/ WhatsApp</span></div><section class="login-card"><p class="eyebrow">OPERATIONS CONSOLE</p><h1>A clear view of<br>your connections.</h1><p class="muted">Manage your WhatsApp senders and keep NextDeal connected.</p><form id="login"><label for="key">Manager API key</label><input id="key" name="key" type="password" autocomplete="off" required placeholder="Enter your server manager key"><p class="hint">Your key stays in memory for this tab. Closing or refreshing the page locks the console.</p><button class="primary" type="submit">Open console <span>→</span></button><p id="login-error" role="alert"></p></form></section><p class="hint">NextDeal infrastructure · Authorized operators only</p></main>`;
  root.querySelector("form")!.onsubmit = async (e) => {
    e.preventDefault();
    const k = (document.querySelector("#key") as HTMLInputElement).value.trim();
    const b = root.querySelector("button")!;
    b.disabled = true;
    try {
      const data = await api("/instance/all", k);
      rows = instancesFrom(data);
      key = k;
      lastSync = new Date().toLocaleTimeString();
      log("Console unlocked");
      render();
    } catch (e) {
      document.querySelector("#login-error")!.textContent = (
        e as Error
      ).message;
      b.disabled = false;
    }
  };
}
function card(r: Instance) {
  return `<article class="instance"><div class="instance-top"><span class="avatar">W</span><span class="badge ${r.connected ? "good" : "warn"}">${r.connected ? "Connected" : "Disconnected"}</span></div><h3>${h(r.name)}</h3><p class="muted">${h(maskedNumber(r.jid))}</p><p class="instance-id">${h(r.id)}</p><div class="card-actions"><button data-action="connect" data-id="${h(r.id)}" ${pending || r.connected ? "disabled" : ""}>Connect</button><button data-action="qr" data-id="${h(r.id)}" ${pending || r.connected ? "disabled" : ""}>Link with QR</button></div></article>`;
}
function render() {
  if (!key) return;
  const connected = rows.filter((x) => x.connected).length;
  root.innerHTML = `<div class="shell"><aside><a class="brand" href="/manager"><span class="logo">N</span> NextDeal</a><div class="workspace">WHATSAPP WORKSPACE</div><nav aria-label="Main navigation">${[
    ["overview", "◫", "Overview"],
    ["instances", "◉", "Instances"],
    ["activity", "↗", "Activity"],
    ["guide", "☷", "Setup guide"],
  ]
    .map(
      ([id, icon, label]) =>
        `<button class="nav ${view === id ? "active" : ""}" data-view="${id}" ${view === id ? 'aria-current="page"' : ""}><span>${icon}</span>${label}</button>`,
    )
    .join(
      "",
    )}</nav><div class="aside-bottom"><div class="small-dot"></div> Self-hosted on your VPS<p>NextDeal Operations</p><button id="signout">Lock console</button></div></aside><div class="main"><header><span>Workspace <span class="muted">/ ${h(view)}</span></span><span class="operator">● &nbsp; Operator</span></header><main class="content"><div class="title"><div><p class="eyebrow">WHATSAPP OPERATIONS</p><h1>${({ overview: "Workspace overview", instances: "Your connections", activity: "Session activity", guide: "Connection playbook" } as Record<string, string>)[view]}</h1><p class="muted">${view === "overview" ? "Keep your senders connected. Know what needs attention." : view === "activity" ? "Actions in this browser session; this is not a server audit log." : view === "guide" ? "A practical guide to operating your WhatsApp service." : "Link and monitor WhatsApp sender accounts."}</p></div><button id="refresh" ${loading || pending ? "disabled" : ""}>${loading ? "Refreshing…" : "↻ Refresh"}</button></div>${error ? `<div class="alert" role="alert">${h(error)} Previous results may be stale.</div>` : ""}
 ${view === "overview" ? `<div class="stats"><section><span>Total instances</span><strong>${rows.length}</strong><small>Configured on this server</small></section><section><span>Connected</span><strong class="green">${connected}</strong><small>Live connection status</small></section><section><span>Needs attention</span><strong>${rows.length - connected}</strong><small>Disconnected senders</small></section></div><div class="notice"><span class="notice-icon">${rows.length === connected && rows.length ? "✓" : "!"}</span><div><b>${!rows.length ? "Start by adding a sender" : rows.length === connected ? "Your senders are connected" : "Some senders need attention"}</b><p>${!rows.length ? "Create an instance, then link WhatsApp with its QR code." : rows.length === connected ? "Connection status is healthy. Confirm delivery with an actual OTP when testing." : "Reconnect a saved session or link the sender with a fresh QR code."}</p></div></div>` : ""}
 ${view === "overview" || view === "instances" ? `<div class="section-title"><h2>Sender instances <span class="count">${rows.length}</span></h2><button class="primary" id="new-instance">+ New instance</button></div><div class="cards">${rows.length ? rows.map(card).join("") : '<div class="empty">No instances yet. Create your first sender to get started.</div>'}</div><p class="hint">Last synced: ${h(lastSync || "never")} · Refreshes every 20 seconds while this tab is visible.</p><div class="info-grid"><section><p class="eyebrow">DELIVERY</p><h3>Connection ≠ delivery</h3><p>Check the recipient inbox to confirm OTP delivery. Message delivery reporting is not configured.</p></section><section><p class="eyebrow">PRIVATE INTEGRATION</p><h3>Backend-owned OTPs</h3><p>NextDeal creates and verifies the code. This gateway delivers it. Keep transport keys on the backend.</p></section></div>` : ""}
 ${view === "activity" ? `<section class="panel">${activity.map((a) => `<div class="event"><time>${h(a.time)}</time><span>${h(a.message)}</span></div>`).join("") || "<p>No activity yet.</p>"}</section>` : ""}
 ${view === "guide" ? `<div class="guide-grid"><section class="panel"><h2>01 · Link a sender</h2><p>Create an instance or choose your existing one. Click Connect first. Then open Link with QR.</p><p>On the sender phone: WhatsApp → Settings → Linked devices → Link a device. Scan the QR displayed here.</p></section><section class="panel"><h2>02 · Recover a connection</h2><p>Click Connect once and refresh status. A saved session may reconnect without scanning. If the session is no longer valid, use a new QR.</p><p>A healthy HTTP server does not guarantee that WhatsApp is connected.</p></section><section class="panel"><h2>03 · Integrate safely</h2><p>Use the dedicated OTP transport from your trusted backend. Never place the manager key or transport key in a mobile app or website bundle.</p><p>New instances do not automatically become the NextDeal OTP sender. An operator must configure the intended instance on the server.</p></section><section class="panel"><h2>04 · Plan for outages</h2><p>Keep session backups and monitor disconnects. Daily backups and delivery monitoring are server operations; this screen does not guarantee them.</p><p>MSG91 fallback remains a separate backend setting. No provider change occurs when you link an instance here.</p></section></div>` : ""}
 </main><footer>NextDeal WhatsApp Console <span>Live API data · ${h(location.hostname)}</span></footer></div></div>`;
  root.querySelectorAll<HTMLButtonElement>("[data-view]").forEach(
    (b) =>
      (b.onclick = () => {
        view = b.dataset.view!;
        render();
      }),
  );
  document.querySelector("#signout")!.addEventListener("click", () => {
    generation++;
    controllers.forEach((c) => c.abort());
    key = "";
    rows = [];
    activity = [];
    error = "";
    clearTimeout(qrTimer);
    document.querySelector("dialog")?.remove();
    login();
  });
  document
    .querySelector("#refresh")!
    .addEventListener("click", () => void refresh());
  document
    .querySelector("#new-instance")
    ?.addEventListener("click", newInstance);
  root.querySelectorAll<HTMLButtonElement>("[data-action]").forEach(
    (b) =>
      (b.onclick = () => {
        const r = rows.find((r) => r.id === b.dataset.id);
        if (r) void act(b.dataset.action!, r);
      }),
  );
}
async function refresh() {
  if (!key || loading || pending) return;
  loading = true;
  const g = generation;
  try {
    const data = await api("/instance/all", key);
    if (g !== generation) return;
    rows = instancesFrom(data);
    error = "";
    lastSync = new Date().toLocaleTimeString();
  } catch (e) {
    if (g === generation) error = (e as Error).message;
  } finally {
    loading = false;
    if (g === generation) render();
  }
}
function dialog(title: string, html: string) {
  clearTimeout(qrTimer);
  document.querySelector("dialog")?.remove();
  const d = document.createElement("dialog");
  d.innerHTML = `<div class="dialog-title"><h2>${h(title)}</h2><button aria-label="Close dialog" id="close">×</button></div>${html}`;
  document.body.append(d);
  d.querySelector("#close")!.addEventListener("click", () => d.close());
  d.addEventListener("close", () => {
    clearTimeout(qrTimer);
    d.remove();
  });
  d.showModal();
  return d;
}
async function act(action: string, r: Instance) {
  if (action === "qr") {
    const d = dialog(
      "Link " + r.name,
      `<p>WhatsApp → Settings → Linked devices → Link a device</p><div id="qr" role="status">Loading QR…</div><p class="hint">QR refreshes every 15 seconds. Close this dialog after the sender connects.</p>`,
    );
    const update = async () => {
      try {
        const data = (await api("/instance/qr", r.token)) as {
          data?: { qrcode?: string; passkeyStage?: string };
        };
        if (!d.isConnected) return;
        const img = qrImage(data.data?.qrcode);
        d.querySelector("#qr")!.innerHTML = img
          ? `<img class="qr" alt="WhatsApp linking QR code" src="${img}">`
          : "No QR is ready. If already linked, close this dialog and refresh. Otherwise click Connect first.";
      } catch (e) {
        if (d.isConnected)
          d.querySelector("#qr")!.textContent = (e as Error).message;
      }
      if (d.isConnected) qrTimer = setTimeout(update, 15000);
    };
    void update();
    return;
  }
  let failure = "";
  pending = true;
  render();
  try {
    await api("/instance/connect", r.token, {});
    log(`Connection requested for ${r.name}`);
    error = "";
  } catch (e) {
    failure = (e as Error).message;
    log(`Connection request failed for ${r.name}`);
  } finally {
    pending = false;
    await refresh();
    if (failure) {
      error = failure;
      render();
    }
  }
}
function newInstance() {
  const d = dialog(
    "Create a sender",
    `<form id="create"><label for="name">Instance name</label><input id="name" maxlength="80" required placeholder="NextDeal OTP"><p class="hint">A secure instance key is generated automatically. Creating a sender does not change the active OTP configuration.</p><button class="primary">Create instance</button><p id="create-error" role="alert"></p></form>`,
  );
  d.querySelector("form")!.onsubmit = async (e) => {
    e.preventDefault();
    const name = (d.querySelector("#name") as HTMLInputElement).value.trim();
    if (!name) return;
    const b = d.querySelector("form button") as HTMLButtonElement;
    b.disabled = true;
    const token = Array.from(crypto.getRandomValues(new Uint8Array(32)), (b) =>
      b.toString(16).padStart(2, "0"),
    ).join("");
    try {
      await api("/instance/create", key, { name, token });
      log(`Created ${name}`);
      d.close();
      await refresh();
    } catch (e) {
      if (d.isConnected) {
        d.querySelector("#create-error")!.textContent = (e as Error).message;
        b.disabled = false;
      }
    }
  };
}
setInterval(() => {
  if (!document.hidden && !document.querySelector("dialog")) void refresh();
}, 20000);
login();
