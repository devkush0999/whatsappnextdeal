# Codebase review — 17 September 2026

Follow-up: the external license runtime described below has since been removed.
See `pkg/core/README.md` for the standalone behavior and manager compatibility
routes. The findings below otherwise describe the original review snapshot.

The repository is a useful WhatsApp integration foundation, but needs security and concurrency fixes before public deployment or use with multiple independently authorized instances. This review does not change application behavior.

## Scope and verification

- Inventoried the repository and reviewed backend architecture, route/auth boundaries, instance lifecycle, message persistence, media handling, event transports, passkey flow, configuration, Docker and CI. Secondary feature services were reviewed for shared patterns; this is not an exhaustive line-by-line audit of every feature or generated bundle.
- `pkg` and `cmd` contain 56 Go files and approximately 20,669 lines, including four test files. Generated Swagger code is outside that count.
- `go test ./...` passed on local Go 1.27.1, darwin/arm64. Go compiled the backend packages. A compiler warning came from the bundled C source in `github.com/chai2010/webp`; this is not a vulnerability finding.
- The module and Docker builder declare Go 1.25.0; the container build was not executed or verified.
- No live WhatsApp pairing, external webhook delivery, PostgreSQL integration, browser interaction, load test, race-detector run, or dependency vulnerability scan was performed. Existing tests do not establish runtime concurrency safety.
- Did not read `.env` values or start the application. Starting it can initiate remote license calls and, when configured, WhatsApp connections.

## Existing architecture

```text
React manager build / external API consumers
                    |
             Gin routes + middleware
                    |
      Feature handlers -> services -> repositories
                    |                    |
             whatsmeow clients      PostgreSQL / GORM
                    |
            WhatsApp event handling
                    |
         Webhooks / WebSocket / RabbitMQ / NATS

Session persistence: PostgreSQL or SQLite
Optional media storage: MinIO-compatible object storage
Runtime activation and heartbeat: external license service
```

Feature areas include instances, messages, sending text/media/interactive messages, users, groups, chats, communities, newsletters, labels, polls and calls. `cmd/evolution-go/main.go` assembles dependencies.

Good foundations: feature-based packages, handler/service/repository separation, dependency interfaces, parameterized repository queries, database pool limits, rotating instance logs, a mutex-protected passkey ceremony store with expiring random tokens, and focused utility/media/referral regression tests.

## Priority findings

### P1 — Instance settings endpoints lack ownership checks

Evidence: `pkg/routes/routes.go:117`, `pkg/instance/handler/instance_handler.go:602` and `:632`; repository update at `pkg/instance/repository/instance_repository.go:195`.

The routes accept any valid instance API key. Both handlers trust the path `instanceId` without comparing it with the authenticated instance in Gin context. The service and repository also do not enforce ownership. A caller with instance A's key and instance B's UUID can read or change B's settings, including read-message and reject-call behavior.

Fix: derive the ID from the authenticated instance, or explicitly reject mismatched IDs. Keep any cross-instance administrator operation separately authorized. Add two-instance GET and PUT regression tests expecting denial and no database mutation.

### P1 — Message status data is not scoped to an instance

Evidence: `pkg/message/service/message_service.go:451`, `pkg/message/repository/message_repository.go:37`, `pkg/message/model/message_model.go:10`.

The service checks that the caller's client is connected, then looks up the supplied message ID globally. The table has no instance ID, and its conflict key is just `message_id`. A caller who knows another instance's stored message ID can retrieve that record, including status/source/referral information. Copies of the same message across instances cannot be represented independently.

Fix: add instance ownership to message records and use `(instance_id, message_id)` for lookup and uniqueness. Plan how legacy rows will be assigned or quarantined before enforcing the constraint. Test isolation and duplicate IDs across instances.

### P1 — Session maps are accessed concurrently without synchronization

Evidence: shared maps created in `cmd/evolution-go/main.go:101`; writes in `pkg/whatsmeow/service/whatsmeow.go:415`; reads/deletes in `pkg/instance/service/instance_service.go:245` and `:360`. Startup, HTTP requests and reconnect/event goroutines all reach this state.

Concurrent map reads/writes can terminate the process. Duplicate connection starts and channel send/close races are also possible because lifecycle transitions are not serialized. These are static findings, not reproduced crashes.

Fix: introduce one pointer-owned session registry with protected access and per-instance lifecycle coordination. A mutex around individual map operations alone will not make check-then-start sequences atomic. Use cancellation and explicit state transitions; test concurrent connect/send/disconnect/reconnect with `-race`.

### P1 — WebSocket output has concurrent writers and slow-client blocking

Evidence: `pkg/events/websocket/websocket_producer.go:105`; asynchronous callers in `pkg/whatsmeow/service/whatsmeow.go:2098` and output at `:2305`.

`Produce` uses a shared read lock while calling `WriteJSON`; multiple producers can acquire that lock and write the same connection concurrently. Broadcast clients make this possible even when events belong to different instances. There are no write deadlines, so a slow client can hold the registry lock and delay connection removal and other deliveries.

Also, `AddClient` replaces an existing connection for an instance, while the old connection's cleanup removes the entry by instance ID. Closing an old browser tab can therefore unregister the newer tab.

Fix: a bounded outbound queue and single writer per connection, deadlines, and connection-specific removal. Test simultaneous broadcasts, slow readers and reconnecting tabs.

### P1 — Media storage initialization attempts to make the whole bucket public

Evidence: `pkg/storage/minio/media_storage.go:24` and `:82`.

When enabled and the storage credentials permit policy changes, initialization grants anonymous `s3:GetObject` on every object in the configured bucket. Presigned URL expiration does not protect an object that is separately readable anonymously. A shared bucket would expose unrelated objects as well. Generated presigned URLs are also printed to stdout.

Fix: private bucket by default, no application startup policy mutation, narrowly scoped credentials and short-lived authorized download links. Remove signed URLs from logs. Verify anonymous GET fails in a storage integration test.

### P1 — User-controlled remote downloads allow internal-network requests

Evidence: `pkg/sendMessage/service/send_service.go:1241` fetches `data.Url` using `http.Get`, followed by unlimited `io.ReadAll` at `:1250`. Similar paths exist for link previews, stickers, status media and carousel assets. Instance connection settings also accept a webhook URL.

An authenticated caller can cause the server to request arbitrary reachable HTTP endpoints, including internal services; default redirects can change the destination. Unbounded reads and missing timeouts can consume memory or hold requests indefinitely. Exploitability depends on server network access and which response types a feature processes.

Fix: centralize outbound fetching with permitted schemes/destinations, DNS and redirect checks, private-address restrictions where appropriate, total deadlines, status validation and byte limits. Test loopback/private targets, redirects, oversized bodies and stalled responses with controlled fixtures.

### P1 — Instance API tokens are written to logs

Evidence: `pkg/whatsmeow/service/whatsmeow.go:1961` and `:1975` log the full instance token during connection failure/disconnection. `cmd/evolution-go/main.go:264` logs rejected WebSocket tokens; the WebSocket API also places the global credential in the URL query. `pkg/config/config.go:147` sends the database DSN to debug logging.

Log access or forwarding can expose working credentials. The compiled manager reads its API key from `localStorage` (`evolution-auth`), increasing the impact of any future same-origin script injection; no injection was demonstrated here.

Fix: redact credentials centrally, log instance IDs, and replace long-lived WebSocket URL credentials with short-lived tickets. Separate browser sessions from the global administrator key. Store instance API keys as verifiable hashes when plaintext recovery is unnecessary.

### P2 — ForceReconnect dereferences a missing client

Evidence: `pkg/instance/service/instance_service.go:699`.

The function calls methods on `i.clientPointer[instanceId]` before checking whether a client exists or the instance is valid. Reconnecting a newly created or unloaded instance can panic and yield a recovered HTTP 500 instead of starting a session or returning an appropriate error.

Fix: look up the instance first, retrieve the client safely and handle the absent-client state explicitly. Test existing-but-unloaded and unknown instance IDs.

### P2 — Webhook retries are unbounded in concurrency and not durable

Evidence: `pkg/events/webhook/webhook_producer.go:44` and `:77`.

Each event can start two goroutines. The HTTP client has no timeout and responses are read without a size limit; a hung attempt never reaches the next retry. Pending deliveries disappear on restart. Requests are not signed, so receivers have no built-in authenticity verification.

Fix: a bounded worker queue with request deadlines, response-size limits, retry backoff and jitter. Add stable event IDs and signature verification support. Use an outbox if reliable delivery matters, and record delivery state rather than implying exactly-once delivery.

### P2 — RabbitMQ success is reported without observing publisher confirms

Evidence: `pkg/events/rabbitmq/rabbitmq_producer.go:126` and `:200`.

The channel enables confirms, but the implementation never consumes confirmations or waits for an acknowledgement. A nil `Publish` error is treated as delivered. Concurrent reconnects also replace `p.conn` without a shared synchronization boundary, and retry-created channels are not closed by the outer defer for the original channel.

Fix: serialize reconnection, explicitly handle broker acknowledgement/nack and timeout, and close all owned channels. Test disconnects during publishing. For a personal MVP, enable only the transports actually needed.

### P2 — HTTP input/resource limits and shutdown coverage are incomplete

Evidence: `cmd/evolution-go/main.go:410`, `pkg/middleware/jid_validation_middleware.go:41`, `pkg/routes/routes.go:47`.

The HTTP server has no configured read-header/read/write/idle timeouts, and validation middleware reads request bodies without a hard byte limit. CORS is duplicated and combines wildcard origin with credentials. HTTP shutdown does not explicitly drain event workers, close hijacked WebSockets or stop all WhatsApp clients.

Fix: endpoint-appropriate timeouts/body limits, rate limits, one explicit CORS policy, and an owned shutdown sequence. Pass request contexts into operations where cancellation is meaningful; persistent WhatsApp connections need a separate application lifecycle context.

## Personal-project constraints

1. **Frontend source is missing.** `manager/` contains only `dist` HTML/CSS/JavaScript; no source tree or package manifest is present. The UI can be served but cannot be maintained through a normal React build workflow from this checkout. Obtain source or create a new TypeScript UI.
2. **Runtime activation is external.** `pkg/core/c0.go:441` uses a saved license or tries remote activation with `GLOBAL_API_KEY`. On that fresh-install path the administrator key is sent as `X-Api-Key` to the license service. Without successful activation the gate returns 503 for application APIs (`:638`). A saved license activates locally and remote notification failure is non-blocking; this is not a claim that every startup requires a successful network call. Heartbeat code sends instance ID, version, uptime and message counters (`:924`). The older `pkg/telemetry` middleware is not wired into the inspected startup.
3. **Repository licensing includes extra text.** The opening of `LICENSE` describes frontend logo/copyright restrictions and an Evolution Go usage notification; `NOTICE` and `TRADEMARKS.md` are also present. Preserve these documents and account for their stated requirements before rebranding. This review records repository contents, not a legal determination.
4. **Current data model is an integration model, not a complete inbox.** Message rows contain IDs/status/source/referral, not a full conversation and message-body schema. A personal chat UI needs additional persistence, pagination and retention design.
5. **Single-process state limits scaling.** Clients, ceremony tokens and event connections are in memory. Multiple replicas need explicit instance ownership/routing and duplicate-session prevention; adding replicas alone is not a scaling solution.

## Maintainability and delivery

- `send_service.go` is 3,380 lines and `whatsmeow.go` is 2,895 lines. Split by session lifecycle, message type, media processing and event routing, while keeping feature boundaries.
- Connection checks and fixed sleeps are repeated across services. Move connection resolution behind a shared interface and signal readiness instead of guessing with sleeps.
- `Producer.Produce` uses positional strings with different meanings for each transport. Replace with a typed event envelope and explicit destination configuration.
- Use typed request/response DTOs and consistent error mapping. Several handlers expose raw internal errors; the poll handler serializes the context instance through JSON instead of using its known Go type.
- Startup uses `AutoMigrate`; introduce versioned migrations before ownership/schema changes. Separate database provisioning permissions from routine application permissions.
- `README.md` currently contains only `devesh`. Restore setup, environment names, architectural context and troubleshooting instructions.
- The only CI workflow builds/publishes on `main`; it does not test or vet PRs. Its image target remains `evoapicloud/evolution-go`.
- Docker runs as root and copies only the binary, manager build and VERSION. Review runtime asset needs. Compose examples expose databases/brokers with sample credentials; the full example references `nats` without defining that service and does not provision its media bucket.
- Existing tests cover utils, thumbnails, referral extraction and generated message-upsert SQL. Add real repository integration tests, HTTP authorization tests and lifecycle concurrency tests before relying on the backend for private data.

## Suggested implementation order

1. **Stabilize:** fix cross-instance access, secret logging, public bucket policy and outbound request controls; add focused regression tests. Resolve frontend source and activation/licensing expectations before rebranding.
2. **Make sessions reliable:** synchronized registry, per-instance lifecycle coordination, WebSocket writers, bounded delivery workers, cancellation and graceful shutdown.
3. **Define the personal MVP:** proposed scope is one administrator, one or a few WhatsApp connections, QR/status, contacts, text/media sending and basic activity history. The product scope is a proposal, not an assumed user decision.
4. **Build the interface:** TypeScript React dashboard or Expo app based on the intended use. Retain the Go WhatsApp integration; a complete Node.js rewrite is not justified by this review. Add a separate business API only when requirements warrant it.
5. **Ship reproducibly:** owned image namespace, PR test/vet checks, race tests for lifecycle paths, tested migrations/backups, private infrastructure ports and a non-root runtime. Start with one Go service and PostgreSQL; optional brokers/storage should follow actual needs.

For a future React Native client, keep the global administrator key on the server, use app-specific authentication, and add Android/iOS build and release checks when that client exists. Redux Toolkit can hold client workflow state; remote API data should have one consistent query-cache owner rather than duplicate Redux and query caches.
