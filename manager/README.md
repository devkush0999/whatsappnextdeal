# NextDeal WhatsApp Console 2

Editable, modular TypeScript console for the existing Go gateway. No frontend runtime dependencies or CDN scripts. Existing legacy bundle files are retained; the maintained entry point is `src/main.ts`.

## Run and build

Node 22+ and the pinned TypeScript dependency are required.

```sh
cd manager
npm ci
npm run build
npm test
npm run preview
```

Synthetic preview: http://127.0.0.1:8092/manager — use `preview-key`. This local server cannot send a real message or change a WhatsApp account.

For real gateway data, first keep the existing private SSH tunnel to VPS port 8091 open. Start a separate local console:

```sh
MANAGER_PORT=8093 npm run dev
```

This mode is read-only. Use the server GLOBAL_API_KEY to unlock it. It forwards only catalogued API routes to http://127.0.0.1:8091; no credentials are stored in the proxy. To deliberately enable reviewed operator actions locally:

```sh
MANAGER_PORT=8093 npm run dev -- --allow-actions
```

The Go service serves the built console directly at `/manager`. A browser refresh locks it because credentials remain only in memory.

## Features and scope

The library inventories **85 currently mounted gateway operations**, grouped into connections, compose, contacts, groups, message tools, labels, channels, communities, profile/privacy, chat tools, polls and calls. Integrations, the operator guide and browser-session activity complete the workspace.

- Instance selection; masked sender numbers; connect/reconnect, QR and phone pairing; passkey-stage display; settings/proxy/log tools; secure creation tokens.
- Text, links, media URLs or files (up to 8 MB in the console), polls, stickers, locations, contact cards, interactive buttons, lists, carousels and status updates. Simple builders support reply buttons/list rows/contact cards; advanced nested payloads remain available for supported fields.
- Contact/group/channel/label reads displayed in bounded, paginated tables, with sanitized response inspection for other shapes.
- Required fields, numeric/range/enum checks; multipart boundaries; bounded network requests and responses; no automatic retry of actions.
- Review before sending/changing; typed confirmation for destructive operations; sender credential isolation; current preferences must load before behavior/privacy replacement.
- Keys, recipient identifiers and message bodies are not persisted to browser storage or activity logs. Loading a response can display that account’s data in the current tab.

Eight operations remain visible but disabled: six chat actions marked not working in Go, `/group/myall`, and `/message/status` until stored-message ownership is fixed. Route presence does not establish provider compatibility; real WhatsApp send/delivery and mutation journeys need a disposable instance.

This is **not** a persisted multi-agent inbox. Durable conversations, assignments, delivery analytics, human operator accounts/RBAC, persistent audit, reconnect alerts and automation still need backend work. No fabricated message history or delivery statistics are shown.

## Maintaining the inventory

`scripts/generate-catalog.py` reads mounted routes and Swagger DTOs, with explicit corrections for stale profile schemas and interactive form builders. Run from the repository checkout:

```sh
python3 manager/scripts/generate-catalog.py
cd manager && npm run build
```

Review the generated diff when handlers change; Swagger can be stale. The generated catalog is not an authorization boundary. Server middleware always authorizes the actual request.

## Deployment and security

The existing Dockerfile builds the console as part of the gateway image. Build/test before a release, preserve session/database volumes and retain the previous image. Sender settings ownership has a Go regression-tested guard in this source release; that guard requires a gateway rebuild/deploy to take effect on a running server.

Keep the full manager API private through a tunnel or trusted operator ingress. Existing public Caddy rules may permit instance controls only; a 404 on another feature can be the proxy allowlist, not missing source code. This change does not expand public routes or enable the internal OTP transport for browsers. Do not put GLOBAL_API_KEY or the dedicated NextDeal OTP transport key in app bundles or URLs.

Existing server limitations include incomplete session recovery, message-status ownership, remote-fetch controls, and single-process session coordination. A new dashboard does not resolve them. Preserve repository LICENSE, NOTICE and TRADEMARKS documents.

## Source map

- `src/main.ts`: session state, navigation, request/review orchestration.
- `src/views.ts`, `style.css`, `icons.ts`: responsive UI and escaped presentation.
- `src/catalog.ts`: generated operation/field definitions.
- `src/forms.ts`: DTO builders, file handling, validation and redaction.
- `src/api.ts`: cancellation, response limits and action error semantics.
- `scripts/serve.mjs`: loopback-only preview/private development proxy.
- `src/*.test.ts`: model, validation, multipart, credential/cancellation regression tests.
