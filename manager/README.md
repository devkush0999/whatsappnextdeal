# NextDeal WhatsApp Console

Editable TypeScript UI served at `/manager` by the existing Go API. No frontend runtime dependency or external CDN.

## Build

Use Node 22+ and npm: `npm ci`, `npm run build`, `npm test`. Compiled files stay in manager/dist so existing Go deployments continue to work. Docker builds the UI in a separate Node stage.

## Features

Live instance overview; connection counts; create an instance with a cryptographically generated token; connect an existing session; QR linking with bounded polling; in-memory session activity; operator guide; responsive navigation; memory-only API key. All network operations use existing server authentication. Lock aborts in-flight requests and clears keys.

No delivery statistics are inferred from connection state. Activity is browser-session-only, not a durable audit trail. Creating an instance does not update the backend OTP sender configuration.

## Public hosting

Use HTTPS and a dedicated subdomain. Proxy only manager/assets and the required authenticated instance endpoints. Block public access to internal OTP transport and message sending endpoints. Never embed keys in the bundle or URLs. The backend retains its private Docker transport network.

## Next iterations

Durable delivery event storage with retention, role-based operator accounts, audited operator actions, connection recovery alerts, and integration tests against disposable WhatsApp instances. Current underlying reconnect failure can require gateway restart then Connect; it is not solved by this UI.
