import {
  instancesFrom,
  escapeHtml as h,
  qrImage,
  maskedNumber,
  type Instance,
} from "./model.js";
import { operations, type Operation } from "./catalog.js";
import { ApiClient, ApiError } from "./api.js";
import {
  buildRequest,
  collectFields,
  redact,
  safeLink,
  type FormValues,
} from "./forms.js";
import { icon } from "./icons.js";
import {
  modules,
  moduleOps,
  empty,
  overview,
  instanceCard,
  toolOptions,
  operationForm,
  resultHtml,
  guideView,
} from "./views.js";

const root = document.querySelector<HTMLDivElement>("#app")!,
  api = new ApiClient();
const preview =
  document
    .querySelector('meta[name="console-preview"]')
    ?.getAttribute("content") === "true";
const readOnly =
  document
    .querySelector('meta[name="console-readonly"]')
    ?.getAttribute("content") === "true";
let key = "",
  rows: Instance[] = [],
  selectedId = "",
  view = "overview",
  opId = "",
  epoch = 0;
let busy = false,
  refreshing = false,
  lastSync = "",
  error = "",
  search = "",
  mobileMenu = false;
let activity: { time: string; title: string; state: string }[] = [];
let result: unknown = null,
  resultTitle = "",
  resultTime = "",
  resultPage = 0;
let values: FormValues = {},
  qrTimer: ReturnType<typeof setTimeout> | undefined;
let settingsLoadedFor = "";
let attachedFile: File | undefined;
const selected = () => rows.find((r) => r.id === selectedId);
const secrets = () => [key, ...rows.map((r) => r.token)];
const operation = () => operations.find((o) => o.id === opId);
function log(title: string, state = "success") {
  activity.unshift({ time: new Date().toLocaleTimeString(), title, state });
  activity = activity.slice(0, 100);
}
function clearResult() {
  attachedFile = undefined;
  result = null;
  resultTitle = "";
  resultTime = "";
  resultPage = 0;
}
function showError(message: string) {
  error = message;
  const e = root.querySelector<HTMLElement>("#global-error");
  if (e) {
    e.textContent = message;
    e.hidden = !message;
  }
}
function lock() {
  epoch++;
  api.cancelAll();
  clearTimeout(qrTimer);
  document.querySelector("dialog")?.remove();
  key = "";
  rows = [];
  activity = [];
  selectedId = "";
  values = {};
  busy = false;
  refreshing = false;
  error = "";
  clearResult();
  login();
}
function login() {
  root.innerHTML = `<main class="login"><div class="brand"><span class="logo">N<span>·</span></span>NextDeal <small>WHATSAPP</small></div><div class="login-layout"><section class="login-story"><p class="eyebrow">YOUR MESSAGING WORKSPACE</p><h1>One workspace.<br>More possibilities.</h1><p>Connect your senders. Work with contacts, messages and groups. Keep every operation in view.</p><div class="login-capabilities">${["Connections", "Messaging", "Contacts", "Groups", "Labels", "Privacy"].map((x) => `<span>${icon("check")}${x}</span>`).join("")}</div><p class="hint">Powered by your self-hosted Evolution Go gateway.</p></section><section class="login-card"><span class="badge">${preview ? "SYNTHETIC PREVIEW" : "AUTHORIZED OPERATORS"}</span><h2>Open your workspace</h2><p class="muted">Use the manager key from your server.</p><form id="login-form"><label for="manager-key">Manager API key</label><input id="manager-key" type="password" autocomplete="off" required placeholder="Enter manager key"><button class="primary wide" type="submit">Open console ${icon("arrow")}</button><p id="login-error" role="alert"></p></form><p class="hint">${icon("lock")} Credentials stay in this tab’s memory. Refreshing locks the console.</p>${preview ? '<p class="notice">Fictional data only. No WhatsApp action can be sent.</p>' : ""}</section></div></main>`;
  root.querySelector<HTMLFormElement>("form")!.onsubmit = async (e) => {
    e.preventDefault();
    const candidate = root
        .querySelector<HTMLInputElement>("#manager-key")!
        .value.trim(),
      g = ++epoch;
    const b = root.querySelector<HTMLButtonElement>("button")!;
    b.disabled = true;
    try {
      const data = instancesFrom(await api.request("/instance/all", candidate));
      if (g !== epoch) return;
      rows = data;
      key = candidate;
      selectedId = rows[0]?.id || "";
      view = "overview";
      lastSync = new Date().toLocaleTimeString();
      log("Workspace unlocked", "read");
      render();
    } catch (e) {
      if (g === epoch) {
        root.querySelector("#login-error")!.textContent = (e as Error).message;
        b.disabled = false;
      }
    }
  };
}
function senderPicker() {
  return `<div class="sender-strip"><div class="sender-heading">${icon("phone")}<div><label for="sender">Active sender</label><small>All sender tools use this account</small></div></div><select id="sender" aria-label="Active sender" ${busy ? "disabled" : ""}>${rows.length ? rows.map((r) => `<option value="${h(r.id)}" ${r.id === selectedId ? "selected" : ""}>${h(r.name)} · ${h(maskedNumber(r.jid))}</option>`).join("") : '<option value="">No senders configured</option>'}</select><span id="sender-status" class="badge ${selected()?.connected ? "good" : "warn"}">${selected()?.connected ? "Connected" : "Disconnected"}</span></div>`;
}
function renderResult() {
  return result === null
    ? ""
    : resultHtml(
        result,
        resultTitle,
        resultTime,
        selected()?.name || "Server",
        resultPage,
        secrets(),
      );
}
function tools() {
  const op = operation(),
    list = moduleOps(view);
  return `<div class="tools-layout"><section class="tool-list"><label class="search-box">${icon("search")}<input id="tool-search" aria-label="Search tools" placeholder="Search ${view === "library" ? "all features" : "tools"}…" value="${h(search)}"></label><div id="tool-options">${toolOptions(list, search, opId)}</div></section><section class="tool-detail" id="tool-detail">${op && list.some((o) => o.id === op.id) ? operationForm(op, values, busy, !!selected(), renderResult()) : empty("Choose a tool", "Start with an action on the left. Review its fields before running it.")}</section></div>`;
}
function content() {
  if (view === "overview") return overview(rows);
  if (view === "guide") return guideView();
  if (view === "activity")
    return `<section class="panel"><div class="section-title"><h2>Session activity</h2><span class="badge">${activity.length} events</span></div><p class="hint">Cleared on lock or refresh. Message bodies, recipients and credentials are not recorded here.</p>${activity.map((e) => `<div class="event"><span class="event-icon ${e.state}">${icon(e.state === "failure" ? "close" : "check")}</span><div><b>${h(e.title)}</b><small>${h(e.time)}</small></div><span class="badge">${h(e.state)}</span></div>`).join("") || empty("Nothing recorded yet", "Actions in this tab will appear here.")}</section>`;
  if (view === "instance")
    return `<div class="section-title"><div><h2>Sender connections</h2><p class="muted">Manage each linked account independently.</p></div><button class="primary" data-op="POST /instance/create">${icon("plus")} New sender</button></div><div class="cards">${rows.length ? rows.map(instanceCard).join("") : empty("Create your first sender", "Then link the intended WhatsApp phone.")}</div><div class="section-title"><h2>Connection tools</h2><span class="hint">Session changes require review.</span></div>${tools()}`;
  const intro =
    view === "library"
      ? `<div class="library-summary"><b>${operations.length} mounted operations</b><span> · ${operations.filter((o) => !o.blocked).length} available to request · ${operations.filter((o) => o.blocked).length} recovery holds</span><p class="hint">A mounted route is not proof of provider support. Required fields are checked before requests; WhatsApp may still reject an operation.</p></div>`
      : view === "message"
        ? '<div class="notice"><p>These tools act on known message IDs. A searchable shared inbox and durable delivery analytics require additional server-side persistence.</p></div>'
        : view === "integrations"
          ? `<div class="notice">${icon("code")}<p><b>Backend-owned OTPs.</b> NextDeal creates and verifies its four-digit codes. The gateway only delivers them. Creating a sender does not switch the active OTP sender.</p></div><div class="info-grid"><section><h3>Event integrations</h3><p>Configure an explicit webhook and event subscriptions below. Brokers also need server configuration.</p></section><section><h3>Optional OTP projects</h3><p>The independent six-digit OTP console at /otp, when enabled, is separate from the NextDeal login flow. Transport keys remain on the backend.</p></section></div>`
          : "";
  return intro + tools();
}
function render() {
  if (!key) return;
  const mod = modules.find((m) => m[0] === view) || modules[0];
  root.innerHTML = `<div class="shell ${mobileMenu ? "menu-open" : ""}"><aside><a class="brand" href="#overview"><span class="logo">N<span>·</span></span>NextDeal</a><div class="workspace"><span class="small-dot"></span> WHATSAPP WORKSPACE</div><nav aria-label="Workspace navigation">${modules.map((m) => `<button data-view="${m[0]}" class="nav ${view === m[0] ? "active" : ""}" ${view === m[0] ? 'aria-current="page"' : ""}>${icon(m[1])}<span>${m[2]}</span>${m[0] === "library" ? `<small>${operations.length}</small>` : ""}</button>`).join("")}</nav><div class="aside-bottom"><p>Evolution Go · Self-hosted</p><button id="lock">${icon("lock")} Lock workspace</button></div></aside><div class="main"><header><div><button class="icon-button mobile-toggle" id="menu" aria-label="Toggle navigation">${icon("grid")}</button><span class="breadcrumb">Workspace <span>/ ${h(mod[2])}</span></span></div><div class="header-actions"><span class="secure-label">${icon("shield")} Private workspace</span><button class="icon-button" id="header-lock" aria-label="Lock workspace">${icon("lock")}</button></div></header>${preview ? '<div class="preview-banner">PREVIEW · Fictional sender data. No action can reach WhatsApp.</div>' : ""}<main class="content"><div class="title"><div><p class="eyebrow">${view === "overview" ? "THE CONTROL ROOM" : "WHATSAPP OPERATIONS"}</p><h1>${view === "overview" ? "Your workspace, connected." : h(mod[2])}</h1><p class="muted">${h(mod[3])}</p></div><button id="refresh" ${busy || refreshing ? "disabled" : ""}>${icon("refresh")} Refresh senders</button></div><div id="global-error" class="alert" role="alert" ${error ? "" : "hidden"}>${h(error)}</div>${!["activity", "guide"].includes(view) ? senderPicker() : ""}<div id="workspace-body">${content()}</div></main><footer><span>NextDeal WhatsApp Console <b>2.0</b></span><span id="sync-status">Synced ${h(lastSync || "never")} · ${h(location.hostname)}</span></footer></div></div>`;
  if (readOnly)
    root
      .querySelector("header")
      ?.insertAdjacentHTML(
        "afterend",
        '<div class="preview-banner">LOCAL READ-ONLY CONNECTION · Sender data is real. Outbound actions and setting changes are disabled.</div>',
      );
  if (["/send/media", "/send/status/media"].includes(operation()?.path || "")) {
    root
      .querySelector("#operation-form fieldset")
      ?.insertAdjacentHTML(
        "afterbegin",
        `<div class="media-picker"><label for="media-file">Attach a file <small>or use a media URL below</small></label><input type="file" id="media-file"><p id="file-status" class="hint">${attachedFile ? h(attachedFile.name) + " · " + Math.ceil(attachedFile.size / 1024) + " KB" : "Choose a photo, document, audio or video up to 8 MB."}</p><button type="button" id="clear-file">Remove attachment</button></div>`,
      );
    const url = root.querySelector<HTMLInputElement>('[name="url"]');
    if (url) url.required = !attachedFile;
  }
  if (operation()?.path === "/user/privacy" && operation()?.method === "POST")
    root
      .querySelector("#operation-form fieldset")
      ?.insertAdjacentHTML(
        "afterbegin",
        '<button type="button" id="load-privacy">Load current privacy first</button><p class="hint">Start with current account preferences before reviewing changes.</p>',
      );
  bind();
}
function navigate(next: string) {
  if (busy || !modules.some((m) => m[0] === next)) return;
  view = next;
  search = "";
  opId = "";
  values = {};
  clearResult();
  mobileMenu = false;
  error = "";
  render();
}
function openOperation(id: string) {
  if (busy) return;
  const op = operations.find((o) => o.id === id);
  if (!op) return;
  opId = id;
  values = {};
  clearResult();
  error = "";
  if (!moduleOps(view).some((o) => o.id === id)) view = op.category;
  settingsLoadedFor = "";
  if (op.path === "/instance/create")
    values.token = Array.from(crypto.getRandomValues(new Uint8Array(32)), (b) =>
      b.toString(16).padStart(2, "0"),
    ).join("");
  render();
  if (op.path === "/instance/qr") void openQr();
}
function bindOperations() {
  root
    .querySelectorAll<HTMLButtonElement>("[data-op]")
    .forEach((b) => (b.onclick = () => openOperation(b.dataset.op!)));
}
function bindPagination() {
  root.querySelectorAll<HTMLButtonElement>("[data-page]").forEach(
    (b) =>
      (b.onclick = () => {
        resultPage = Number(b.dataset.page);
        root.querySelector("#result")!.innerHTML = renderResult();
        bindPagination();
      }),
  );
}
function remember() {
  const op = operation(),
    form = root.querySelector<HTMLFormElement>("#operation-form");
  if (op && form) values = collectFields(form, op.fields);
}
function bind() {
  root
    .querySelector("#load-privacy")
    ?.addEventListener("click", () => void loadPrivacy());
  root
    .querySelector<HTMLInputElement>("#media-file")
    ?.addEventListener("change", (e) => {
      const file = (e.target as HTMLInputElement).files?.[0];
      if (file && (file.size === 0 || file.size > 8 * 1024 * 1024)) {
        (e.target as HTMLInputElement).value = "";
        showError("Choose a non-empty file up to 8 MB.");
        return;
      }
      attachedFile = file;
      const url = root.querySelector<HTMLInputElement>('[name="url"]');
      if (url) url.required = !file;
      root.querySelector("#file-status")!.textContent = file
        ? file.name + " · " + Math.ceil(file.size / 1024) + " KB"
        : "No attachment selected.";
    });
  root.querySelector("#clear-file")?.addEventListener("click", () => {
    attachedFile = undefined;
    const input = root.querySelector<HTMLInputElement>("#media-file");
    if (input) input.value = "";
    const url = root.querySelector<HTMLInputElement>('[name="url"]');
    if (url) url.required = true;
    root.querySelector("#file-status")!.textContent = "No attachment selected.";
  });
  root
    .querySelectorAll<HTMLButtonElement>("[data-view]")
    .forEach((b) => (b.onclick = () => navigate(b.dataset.view!)));
  root.querySelector(".brand")?.addEventListener("click", (e) => {
    e.preventDefault();
    navigate("overview");
  });
  root.querySelector("#lock")?.addEventListener("click", lock);
  root.querySelector("#header-lock")?.addEventListener("click", lock);
  root.querySelector("#menu")?.addEventListener("click", () => {
    mobileMenu = !mobileMenu;
    root.querySelector(".shell")?.classList.toggle("menu-open", mobileMenu);
  });
  root
    .querySelector<HTMLSelectElement>("#sender")
    ?.addEventListener("change", (e) => {
      epoch++;
      api.cancelAll();
      selectedId = (e.target as HTMLSelectElement).value;
      busy = false;
      refreshing = false;
      values = {};
      error = "";
      clearResult();
      render();
    });
  root
    .querySelector("#refresh")
    ?.addEventListener("click", () => void refreshInstances(true));
  bindOperations();
  bindPagination();
  root.querySelectorAll<HTMLButtonElement>("[data-command]").forEach(
    (b) =>
      (b.onclick = () => {
        if (busy) return;
        epoch++;
        api.cancelAll();
        refreshing = false;
        selectedId = b.dataset.instance!;
        clearResult();
        values = {};
        if (b.dataset.command === "qr") {
          render();
          void openQr();
        } else {
          view = "instance";
          openOperation(
            b.dataset.command === "status"
              ? "GET /instance/status"
              : "GET /instance/info/:instanceId",
          );
        }
      }),
  );
  root
    .querySelector<HTMLInputElement>("#tool-search")
    ?.addEventListener("input", (e) => {
      search = (e.target as HTMLInputElement).value;
      root.querySelector("#tool-options")!.innerHTML = toolOptions(
        moduleOps(view),
        search,
        opId,
      );
      bindOperations();
    });
  root
    .querySelector<HTMLFormElement>("#operation-form")
    ?.addEventListener("submit", (e) => {
      e.preventDefault();
      void submit();
    });
  root
    .querySelector("#load-settings")
    ?.addEventListener("click", () => void loadSettings(false));
  root
    .querySelector("#load-integrations")
    ?.addEventListener("click", () => void loadSettings(true));
}
async function refreshInstances(forceRender = false) {
  if (!key || busy || refreshing) return;
  refreshing = true;
  const g = epoch;
  try {
    const data = instancesFrom(await api.request("/instance/all", key));
    if (g !== epoch) return;
    rows = data;
    lastSync = new Date().toLocaleTimeString();
    if (!rows.some((r) => r.id === selectedId)) {
      selectedId = rows[0]?.id || "";
      clearResult();
      remember();
      render();
    }
    for (const [id, value] of [
      ["stat-total", rows.length],
      ["stat-connected", rows.filter((r) => r.connected).length],
    ]) {
      const e = root.querySelector("#" + id);
      if (e) e.textContent = String(value);
    }
    const status = root.querySelector("#sender-status");
    if (status) {
      status.className = `badge ${selected()?.connected ? "good" : "warn"}`;
      status.textContent = selected()?.connected ? "Connected" : "Disconnected";
    }
    const sync = root.querySelector("#sync-status");
    if (sync) sync.textContent = `Synced ${lastSync} · ${location.hostname}`;
    showError("");
    if (forceRender) {
      remember();
      render();
    }
  } catch (e) {
    if (g === epoch)
      showError((e as Error).message + " Previous sender data may be stale.");
  } finally {
    if (g === epoch) refreshing = false;
  }
}
function modal(title: string, html: string) {
  clearTimeout(qrTimer);
  document.querySelector("dialog")?.remove();
  const d = document.createElement("dialog");
  d.innerHTML = `<div class="dialog-title"><h2>${h(title)}</h2><button class="icon-button close-dialog" aria-label="Close dialog">${icon("close")}</button></div>${html}`;
  document.body.append(d);
  d.querySelector(".close-dialog")!.addEventListener("click", () => d.close());
  d.addEventListener("close", () => {
    clearTimeout(qrTimer);
    d.remove();
  });
  d.showModal();
  return d;
}
function review(
  op: Operation,
  body: Record<string, unknown> | undefined,
  sender: Instance | undefined,
): Promise<boolean> {
  return new Promise((resolve) => {
    const destructive = op.effect === "destructive",
      mustType = destructive ? sender?.name || "CONFIRM" : "";
    const d = modal(
      op.effect === "send" ? "Review your message" : "Review this action",
      `<p><b>${h(op.title)}</b> ${sender ? "using <b>" + h(sender.name) + "</b>" : "on this server"}.</p>${op.effect === "send" ? "<p>A real message will be sent when confirmed. Check status before repeating an interrupted request.</p>" : ""}${destructive ? '<p class="alert">This may remove data, revoke a session or interrupt the active OTP sender.</p>' : ""}<pre>${h(JSON.stringify(redact(body || {}, secrets()), null, 2))}</pre>${mustType ? `<label for="confirm-name">Type ${h(mustType)} to continue</label><input id="confirm-name" autocomplete="off">` : ""}<div class="review-actions"><button id="cancel">Cancel</button><button id="confirm" class="${destructive ? "danger" : "primary"}" ${mustType ? "disabled" : ""}>${op.effect === "send" ? "Send message" : "Confirm action"}</button></div>`,
    );
    let decided = false;
    const finish = (v: boolean) => {
      if (decided) return;
      decided = true;
      resolve(v);
      d.close();
    };
    d.querySelector("#cancel")!.addEventListener("click", () => finish(false));
    d.querySelector("#confirm")!.addEventListener("click", () => finish(true));
    d.querySelector<HTMLInputElement>("#confirm-name")?.addEventListener(
      "input",
      (e) => {
        (d.querySelector("#confirm") as HTMLButtonElement).disabled =
          (e.target as HTMLInputElement).value !== mustType;
      },
    );
    d.addEventListener("close", () => {
      if (!decided) {
        decided = true;
        resolve(false);
      }
    });
  });
}
async function submit() {
  const op = operation(),
    sender = selected(),
    form = root.querySelector<HTMLFormElement>("#operation-form");
  if (!op || !form || busy || op.blocked) return;
  const errorNode = form.querySelector("#form-error")!,
    g = epoch;
  if (
    op.path === "/user/privacy" &&
    op.method === "POST" &&
    settingsLoadedFor !== selectedId
  ) {
    errorNode.textContent =
      "Load current privacy first to preserve existing preferences.";
    return;
  }
  if (readOnly && op.effect !== "read") {
    errorNode.textContent =
      "This local connection is read-only. Enable operator actions when starting the development proxy to make changes.";
    return;
  }
  if (
    op.path.endsWith("/advanced-settings") &&
    op.method === "PUT" &&
    settingsLoadedFor !== selectedId
  ) {
    errorNode.textContent =
      "Load current behavior first to preserve existing sender preferences.";
    return;
  }
  const connectionAction = op.path.startsWith("/instance/");
  if (op.auth === "instance" && !connectionAction && !sender?.connected) {
    errorNode.textContent = "Connect the selected sender first.";
    return;
  }
  remember();
  let request: ReturnType<typeof buildRequest>;
  try {
    request = buildRequest(op, values, sender?.id || "", attachedFile);
  } catch (e) {
    errorNode.textContent = (e as Error).message;
    return;
  }
  if (op.effect !== "read" && !(await review(op, request.review, sender)))
    return;
  if (g !== epoch || !key) return;
  busy = true;
  render();
  try {
    const data = await api.request(
      request.path,
      op.auth === "admin" ? key : sender!.token,
      op.method,
      request.body,
    );
    if (g !== epoch) return;
    result = data;
    resultTitle = op.title;
    resultTime = new Date().toLocaleTimeString();
    resultPage = 0;
    error = "";
    log(op.title, op.effect === "read" ? "read" : "success");
  } catch (e) {
    if (g === epoch) {
      const err = e as ApiError;
      error = err.message;
      log(
        op.title + " failed" + (err.uncertain ? " · outcome uncertain" : ""),
        "failure",
      );
    }
  } finally {
    if (g === epoch) {
      busy = false;
      render();
      if (connectionAction && op.effect !== "read") void refreshInstances(true);
    }
  }
}
async function loadPrivacy() {
  const sender = selected();
  if (busy || !sender) return;
  const g = epoch;
  remember();
  busy = true;
  render();
  try {
    const r = (await api.request("/user/privacy", sender.token)) as {
      data?: Record<string, unknown>;
    };
    if (g !== epoch) return;
    const data = r.data || {};
    for (const field of operation()!.fields) {
      const entry = Object.entries(data).find(
        ([name]) => name.toLowerCase() === field.name.toLowerCase(),
      );
      if (entry) values[field.name] = String(entry[1]);
    }
    settingsLoadedFor = sender.id;
    log("Loaded saved privacy", "read");
  } catch (e) {
    if (g === epoch) error = (e as Error).message;
  } finally {
    if (g === epoch) {
      busy = false;
      render();
    }
  }
}
async function loadSettings(integration: boolean) {
  if (busy || !selected()) return;
  const sender = selected()!,
    g = epoch,
    id = opId;
  remember();
  busy = true;
  render();
  try {
    const response = (await api.request(
      integration
        ? `/instance/info/${encodeURIComponent(sender.id)}`
        : `/instance/${encodeURIComponent(sender.id)}/advanced-settings`,
      integration ? key : sender.token,
    )) as { data?: unknown };
    if (g !== epoch || opId !== id) return;
    const data = (
      response && typeof response === "object" && "data" in response
        ? response.data
        : response
    ) as Record<string, unknown>;
    if (!data || typeof data !== "object")
      throw Error("Unexpected settings response.");
    if (integration)
      values = {
        ...values,
        webhookUrl: String(data.webhook ?? ""),
        subscribe: String(data.events ?? "")
          .split(",")
          .filter(Boolean)
          .join("\n"),
        rabbitmqEnable: String(data.rabbitmqEnable ?? ""),
        websocketEnable: String(data.websocketEnable ?? ""),
        natsEnable: String(data.natsEnable ?? ""),
      };
    else {
      for (const field of operation()!.fields)
        if (data[field.name] !== undefined)
          values[field.name] = String(data[field.name]);
      settingsLoadedFor = sender.id;
    }
    log(
      "Loaded saved " +
        (integration ? "connection settings" : "sender behavior"),
      "read",
    );
  } catch (e) {
    if (g === epoch) error = (e as Error).message;
  } finally {
    if (g === epoch) {
      busy = false;
      render();
    }
  }
}
async function openQr() {
  const sender = selected();
  if (!sender) return;
  const g = epoch;
  const d = modal(
    "Link " + sender.name,
    '<p>WhatsApp → Settings → Linked devices → Link a device.</p><div id="qr" role="status">Loading linking information…</div><p class="hint">Refreshes every 15 seconds while open. This does not log out or reset a sender.</p>',
  );
  const update = async () => {
    try {
      const response = (await api.request("/instance/qr", sender.token)) as {
        data?: Record<string, unknown>;
      };
      if (!d.isConnected || g !== epoch) return;
      const data = response.data || {},
        image = qrImage(data.qrcode),
        link = safeLink(data.passkeyOpenUrl);
      d.querySelector("#qr")!.innerHTML = image
        ? `<img class="qr" src="${image}" alt="WhatsApp sender linking QR code">`
        : data.passkeyStage
          ? `<p>Passkey step: ${h(data.passkeyStage)}</p>${data.passkeyCode ? `<p>Confirmation code: <b>${h(data.passkeyCode)}</b></p>` : ""}${link ? `<a href="${h(link)}" target="_blank" rel="noopener noreferrer">Continue passkey verification</a>` : "<p>Complete the linked-device step using the configured passkey helper.</p>"}`
          : "<p>No QR is ready. If connected, close this dialog. Otherwise Connect once and reopen QR.</p>";
    } catch (e) {
      if (d.isConnected && g === epoch)
        d.querySelector("#qr")!.textContent = (e as Error).message;
    }
    if (d.isConnected && g === epoch) qrTimer = setTimeout(update, 15000);
  };
  void update();
}
setInterval(() => {
  if (!document.hidden && !document.querySelector("dialog"))
    void refreshInstances();
}, 20000);
login();
