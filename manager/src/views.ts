import { escapeHtml as h, maskedNumber, type Instance } from "./model.js";
import { icon } from "./icons.js";
import { redact, type FormValues } from "./forms.js";
import { operations, type Operation, type Field } from "./catalog.js";

export const modules = [
  ["overview", "grid", "Overview", "Your WhatsApp workspace, at a glance."],
  [
    "instance",
    "phone",
    "Connections",
    "Link senders and manage their sessions.",
  ],
  ["compose", "send", "Compose", "Send a message from the selected sender."],
  [
    "contacts",
    "people",
    "Contacts",
    "Look up contacts and manage blocked numbers.",
  ],
  [
    "group",
    "people",
    "Groups",
    "Group information, invitations and member controls.",
  ],
  [
    "message",
    "chat",
    "Message tools",
    "Reactions, read receipts and message actions.",
  ],
  ["label", "tag", "Labels", "Organize chats and messages with labels."],
  [
    "newsletter",
    "radio",
    "Channels",
    "Manage subscriptions and channel information.",
  ],
  [
    "community",
    "people",
    "Communities",
    "Create communities and manage linked groups.",
  ],
  [
    "profile",
    "shield",
    "Profile & privacy",
    "Sender profile and visibility preferences.",
  ],
  [
    "chat",
    "chat",
    "Chat tools",
    "History requests and recovery status for chat actions.",
  ],
  [
    "polls",
    "activity",
    "Poll results",
    "Look up results for a known poll message.",
  ],
  ["call", "phone", "Calls", "Reject a specified incoming call."],
  [
    "integrations",
    "code",
    "Integrations",
    "Event subscriptions, webhooks and OTP integration.",
  ],
  [
    "library",
    "book",
    "Feature library",
    "Every mounted operation and its recovery status.",
  ],
  [
    "activity",
    "activity",
    "Activity",
    "Actions in this tab. Message bodies are not recorded.",
  ],
  [
    "guide",
    "book",
    "Operator guide",
    "Connection, credentials and deployment essentials.",
  ],
] as const;
export const moduleOps = (view: string) =>
  operations.filter(
    (o) =>
      view === "library" ||
      (view === "integrations"
        ? o.path === "/instance/connect"
        : o.category === view),
  );
export const empty = (title: string, description: string) =>
  `<div class="empty"><span class="tile-icon">${icon("grid")}</span><h3>${h(title)}</h3><p>${h(description)}</p></div>`;
export function instanceCard(r: Instance) {
  return `<article class="instance"><div class="instance-top"><span class="avatar">${h(r.name[0] || "W")}</span><span class="badge ${r.connected ? "good" : "warn"}">${r.connected ? "Connected" : "Needs attention"}</span></div><h3>${h(r.name)}</h3><p class="muted">${h(maskedNumber(r.jid))}</p><p class="instance-id">${h(r.id)}</p><div class="card-actions"><button data-instance="${h(r.id)}" data-command="status">Status</button><button data-instance="${h(r.id)}" data-command="qr" ${r.connected ? "disabled" : ""}>Link QR</button><button data-instance="${h(r.id)}" data-command="manage">Manage ${icon("arrow")}</button><button class="danger" data-instance="${h(r.id)}" data-command="delete" aria-label="Delete ${h(r.name)} connection">Delete</button></div></article>`;
}
export function overview(rows: Instance[]) {
  return `<section class="welcome"><div><span class="badge soft">YOUR OPERATIONS, TOGETHER</span><h2>A little less switching.<br>A lot more control.</h2><p>Your gateway’s tools, brought into one place.</p><button class="primary" data-view="compose">Compose a message ${icon("arrow")}</button><button class="text-button" data-view="library">Explore all features</button></div><div class="connection-orbit" aria-hidden="true"><div class="orbit-line"></div><span class="orbit-item orbit-a">${icon("chat")}</span><span class="orbit-item orbit-b">${icon("people")}</span><span class="orbit-item orbit-c">${icon("tag")}</span><div class="orbit-center">${icon("phone")}<span>Connected ideas.</span></div></div></section><div class="stats"><section><span>Sender instances</span><strong id="stat-total">${rows.length}</strong><small>Configured on this server</small></section><section><span>Connected now</span><strong class="green" id="stat-connected">${rows.filter((r) => r.connected).length}</strong><small>Connection is separate from delivery</small></section><section><span>Gateway tools</span><strong>${operations.length}</strong><small>${new Set(operations.map((o) => o.category)).size} feature areas</small></section><section><span>Recovery holds</span><strong>${operations.filter((o) => o.blocked).length}</strong><small>Visible with a reason for each</small></section></div><div class="section-title"><div><h2>What would you like to do?</h2><p class="muted">A direct route to your everyday tools.</p></div><button class="text-button" data-view="library">All features ${icon("arrow")}</button></div><div class="quick-grid">${[
    [
      "compose",
      "send",
      "Start a conversation",
      "Text, media, polls and contact cards.",
    ],
    [
      "contacts",
      "people",
      "Find your contacts",
      "WhatsApp availability and profiles.",
    ],
    [
      "group",
      "chat",
      "Manage your groups",
      "Members, invitations and settings.",
    ],
    ["profile", "settings", "Make it yours", "Sender profile and privacy."],
  ]
    .map(
      ([v, i, t, d]) =>
        `<button class="quick-card" data-view="${v}"><span class="tile-icon">${icon(i)}</span><h3>${t}</h3><p>${d}</p>${icon("arrow")}</button>`,
    )
    .join(
      "",
    )}</div><div class="section-title"><h2>Connection snapshot</h2><button class="text-button" data-view="instance">Manage connections ${icon("arrow")}</button></div><div class="cards">${rows.length ? rows.slice(0, 3).map(instanceCard).join("") : empty("No connections yet", "Create your first sender to get started.")}</div><div class="notice">${icon("activity")}<p><b>Know what this screen measures.</b> Counts describe connections and available tools. Delivery analytics and a shared inbox require additional server-side event storage.</p></div>`;
}
export function toolOptions(list: Operation[], search: string, id: string) {
  const q = search.toLowerCase();
  const filtered = list.filter((o) =>
    `${o.title} ${o.path} ${o.category}`.toLowerCase().includes(q),
  );
  return filtered.length
    ? filtered
        .map(
          (o) =>
            `<button class="tool-option ${o.id === id ? "selected" : ""}" data-op="${h(o.id)}"><span><b>${h(o.title)}</b><small>${h(o.category)} · ${o.blocked ? "Recovery hold" : o.effect === "read" ? "Read information" : o.effect === "send" ? "Outbound message" : o.effect === "destructive" ? "Careful action" : "Change settings"}</small></span>${o.blocked ? '<span class="hold-dot"></span>' : icon("arrow")}</button>`,
        )
        .join("")
    : '<p class="hint empty">No matching tools.</p>';
}
export function fieldHtml(f: Field, values: FormValues) {
  const val = values[f.name] ?? String(f.default ?? "");
  const attrs = `id="field-${h(f.name)}" name="${h(f.name)}" ${f.required ? "required" : ""}`;
  let control = "";
  if (f.type === "boolean" || f.options) {
    const opts =
      f.type === "boolean"
        ? [
            ["true", "Yes"],
            ["false", "No"],
          ]
        : f.options!.map((o) => [String(o), String(o)]);
    control = `<select ${attrs}><option value="">${f.required ? "Choose an option" : "Omit / leave unchanged"}</option>${opts.map(([v, l]) => `<option value="${h(v)}" ${v === val ? "selected" : ""}>${h(l)}</option>`).join("")}</select>`;
  } else if (
    [
      "lines",
      "json-array",
      "json-object",
      "reply-buttons",
      "list-rows",
      "contact-card",
    ].includes(f.type) ||
    [
      "text",
      "message",
      "description",
      "caption",
      "body",
      "image",
      "sticker",
    ].includes(f.name)
  ) {
    const hints: Record<string, string> = {
      lines: "One entry per line",
      "json-array": "JSON array, e.g. []",
      "json-object": "JSON object, e.g. {}",
      "reply-buttons":
        "One reply label per line · up to 3. For copy/link/call buttons, use a JSON array with the gateway button fields.",
      "list-rows":
        "One option title per line, or a JSON sections array for descriptions and custom row IDs.",
      "contact-card":
        "Full name\nPhone with country code\nOrganization (optional)",
    };
    control = `<textarea ${attrs} rows="${f.type.startsWith("json") ? 6 : 4}" maxlength="100000" placeholder="${h(hints[f.type] || "Enter " + f.label.toLowerCase())}">${h(val)}</textarea>${hints[f.type] ? `<small class="hint">${h(hints[f.type]).replace(/\n/g, " · ")}</small>` : ""}`;
  } else
    control = `<input ${attrs} type="${["integer", "number"].includes(f.type) ? "number" : /password|token/.test(f.name) ? "password" : "text"}" ${f.type === "number" ? 'step="any"' : f.type === "integer" ? 'step="1"' : ""} ${f.min !== undefined ? `min="${f.min}"` : ""} ${f.max !== undefined ? `max="${f.max}"` : ""} value="${h(val)}" maxlength="4096" autocomplete="off" placeholder="${h(["number", "phone"].includes(f.name) ? "Country code + number, e.g. 91…" : f.label)}">`;
  return `<div class="field ${["text", "description", "body", "caption"].includes(f.name) || f.type.startsWith("json") ? "full" : ""}"><label for="field-${h(f.name)}">${h(f.label)} ${f.required ? '<span aria-label="required">*</span>' : "<small>optional</small>"}</label>${control}</div>`;
}
export function operationForm(
  op: Operation,
  values: FormValues,
  busy: boolean,
  hasSender: boolean,
  result: string,
) {
  const primary = op.fields.filter((f) => !f.advanced),
    advanced = op.fields.filter((f) => f.advanced);
  return `<div class="detail-heading"><span class="badge ${op.blocked ? "warn" : "soft"}">${op.blocked ? "RECOVERY HOLD" : h(op.category.toUpperCase())}</span><h2>${h(op.title)}</h2><p class="muted">${op.effect === "send" ? "Review the recipient and content. Acceptance is separate from delivery." : op.effect === "destructive" ? "This action can remove data or revoke a connection. Review carefully." : op.effect === "read" ? "Read information from the selected sender." : "Changes apply to the selected WhatsApp account."}</p></div>${op.blocked ? `<div class="alert">${h(op.blocked)}</div>` : ""}${op.note ? `<p class="inline-note">${h(op.note)}</p>` : ""}<form id="operation-form"><fieldset ${busy || op.blocked ? "disabled" : ""}><div class="fields">${primary.map((f) => fieldHtml(f, values)).join("") || '<p class="hint">No additional input needed.</p>'}</div>${advanced.length ? `<details class="advanced"><summary>Additional options <small>${advanced.length}</small></summary><div class="fields">${advanced.map((f) => fieldHtml(f, values)).join("")}</div></details>` : ""}${op.path.includes("advanced-settings") && op.method === "PUT" ? '<button type="button" id="load-settings">Load current behavior first</button><p class="hint">Start with saved preferences to preserve settings you do not intend to change.</p>' : ""}${op.path === "/instance/connect" ? '<button type="button" id="load-integrations">Load saved connection settings</button>' : ""}<div class="submit-row"><button class="${op.effect === "destructive" ? "danger" : "primary"}" type="submit" ${op.blocked || (!hasSender && !["/instance/create", "/instance/all"].includes(op.path)) ? "disabled" : ""}>${busy ? "Working…" : op.effect === "read" ? "Load information" : op.effect === "send" ? "Review message" : op.effect === "destructive" ? "Review action" : "Review changes"} ${icon("arrow")}</button></div><p id="form-error" class="form-error" role="alert"></p></fieldset></form><details class="api-details"><summary>API details</summary><code>${h(op.method)} ${h(op.path)}</code><p class="hint">${op.auth === "admin" ? "Server manager" : "Selected sender"} authentication · No automatic retry for actions.</p></details><div id="result">${result}</div>`;
}
export function resultHtml(
  value: unknown,
  title: string,
  time: string,
  sender: string,
  pageNumber: number,
  secrets: string[],
) {
  const clean = redact(value, secrets);
  let data: unknown = clean;
  if (
    data &&
    typeof data === "object" &&
    !Array.isArray(data) &&
    "data" in data
  )
    data = (data as { data: unknown }).data;
  let records: Record<string, unknown>[] = [];
  const toRecords = (v: unknown[]): Record<string, unknown>[] =>
    v.filter(
      (x): x is Record<string, unknown> =>
        !!x && typeof x === "object" && !Array.isArray(x),
    );
  if (Array.isArray(data)) records = toRecords(data);
  else if (data && typeof data === "object") {
    const arr = Object.values(data).find((v) => Array.isArray(v));
    if (Array.isArray(arr)) records = toRecords(arr);
  }
  const keys = [...new Set(records.slice(0, 25).flatMap(Object.keys))]
    .filter(
      (k) =>
        !/(token|secret|password|qrcode|proxy|webhook|authorization)/i.test(k),
    )
    .slice(0, 5);
  const page = records.slice(pageNumber * 20, pageNumber * 20 + 20),
    cell = (v: unknown) =>
      typeof v === "object" ? JSON.stringify(v) : String(v ?? "—");
  return `<section class="result-panel"><span class="badge good">RESPONSE RECEIVED</span><h3>${h(title)}</h3><p class="hint">${h(time)} · ${h(sender)}</p>${records.length ? `<div class="table-scroll"><table><thead><tr>${keys.map((k) => `<th>${h(k.replace(/_/g, " "))}</th>`).join("")}</tr></thead><tbody>${page.map((r) => `<tr>${keys.map((k) => `<td title="${h(cell(r[k]))}">${h(cell(r[k]).slice(0, 180))}</td>`).join("")}</tr>`).join("")}</tbody></table></div><div class="pagination"><span>${pageNumber * 20 + 1}–${Math.min((pageNumber + 1) * 20, records.length)} of ${records.length}${records.length === 500 ? " (display capped)" : ""}</span><button data-page="${pageNumber - 1}" ${!pageNumber ? "disabled" : ""}>Previous</button><button data-page="${pageNumber + 1}" ${(pageNumber + 1) * 20 >= records.length ? "disabled" : ""}>Next</button></div>` : ""}<details ${records.length ? "" : "open"}><summary>${records.length ? "View sanitized response" : "Response details"}</summary><pre tabindex="0">${h((JSON.stringify(clean, null, 2) || "null").slice(0, 100000))}</pre></details></section>`;
}
export function guideView() {
  return `<div class="guide-grid">${[
    [
      "01",
      "Link the intended sender",
      "Create a sender, then Connect once. Open its QR or phone pairing tool. Use WhatsApp → Settings → Linked devices on the intended phone. Do not log out a working OTP sender to troubleshoot the dashboard.",
    ],
    [
      "02",
      "Choose before you act",
      "The active sender applies to instance tools. Manager and sender keys are different. Keys stay in memory, never in URLs or browser storage. Lock when finished.",
    ],
    [
      "03",
      "Send deliberately",
      "Review sender, recipient and content before confirming. No bulk broadcast or automatic retry is added. Check the recipient inbox to prove delivery.",
    ],
    [
      "04",
      "Understand the limits",
      "The library inventories mounted routes. Unfinished chat controls remain on recovery hold. A searchable conversation inbox needs additional persistence.",
    ],
    [
      "05",
      "Keep integrations private",
      "The public proxy may expose connection endpoints only. The private tunnel offers the existing gateway API. NextDeal’s OTP transport key belongs on the backend, not in this browser.",
    ],
    [
      "06",
      "Operate with evidence",
      "Back up sessions and databases. Never remove volumes to fix reconnection. Activity here is not a durable server audit. Test writes on disposable instances first.",
    ],
  ]
    .map(
      ([n, t, d]) =>
        `<section class="panel"><span class="eyebrow">${n}</span><h2>${t}</h2><p>${d}</p></section>`,
    )
    .join("")}</div>`;
}
