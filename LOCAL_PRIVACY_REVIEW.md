# Local WhatsApp privacy review

Reviewed the current source, compiled manager, local launch configuration, saved instance integration settings, and a point-in-time socket listing. This is a scoped review, not a guarantee against every vulnerability or a historical packet-capture audit.

## Current configuration

- The single saved instance has no webhook URL or proxy. RabbitMQ, NATS and WebSocket event forwarding are disabled.
- `.env` has no global webhook, broker URL, external audio converter, proxy host or public passkey API URL. MinIO is disabled. Both PostgreSQL URLs point to localhost.
- The test launcher additionally clears global webhook/broker URLs and disables MinIO. Detailed WhatsApp DEBUG logging was disabled after pairing diagnostics.
- External license activation was removed from startup. The license compatibility endpoints return local JSON. `pkg/telemetry` still contains an external telemetry URL, but no application startup code imports or registers this module.
- The manager sends API requests to the API URL chosen at login. The API key is stored in browser localStorage. Only enter a trusted server URL; an untrusted server would receive that key.
- Manager assets are served locally. No automatically loaded third-party analytics or external-font endpoint was found in the inspected bundle/CSS. Documentation/community links are external navigation; interactive carousel examples fetch images from picsum.photos when those examples are sent.
- The passkey helper sends WebAuthn responses to the API base URL carried in the pairing link. Its inspected code has no separate analytics destination. This helper was not needed for the successful QR pairing.
- At the socket inspection, the gateway had local PostgreSQL connections and its API listener; there was no established external TCP connection. The instance was then marked disconnected. This snapshot does not prove historical absence of outbound traffic.

## What scanning does

Scanning authorizes this server as a linked WhatsApp device. WhatsApp/Meta connectivity is required for pairing and messaging. Session credentials are stored in the local auth database. This server can receive/decrypt WhatsApp events and media permitted by the linked-device protocol; self-hosting does not mean that no system processes message content.

I found no active configuration forwarding the tested instance's data to an unrelated operator. That conclusion applies to this checked configuration, not every deployment of this project.

## Risks still present

1. Webhook/event payloads include `instanceToken` in several paths. Turning on an external integration can disclose both event data and credentials to its operator.
2. Local logs include QR/pairing material, tokens, identifiers and, depending on events/log level, message data. Earlier diagnostic logs remain on disk. Treat logs and database backups as sensitive.
3. The gateway listens on all interfaces on port 8080. Most API routes require keys, but this is not localhost-only network binding. API keys and the host/database must be protected.
4. The sample `.env.example` contains a third-party webhook collector URL. It is not active in the checked `.env`; do not copy it into deployment configuration.
5. If MinIO is enabled, current storage code attempts to grant public object-read access. Use private bucket policy before using it for real documents.
6. The previous review found cross-instance authorization gaps. Public multi-tenant deployment needs those fixes before exposing customer accounts.

## Recommended next changes

Bind development servers to loopback; redact credentials and message content from logs; remove instance keys from event payloads; sign webhooks and restrict their destinations; keep media private; enforce tenant ownership in API handlers. Add regression tests for these boundaries before production.
