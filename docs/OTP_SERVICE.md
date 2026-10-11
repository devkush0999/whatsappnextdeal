# Go OTP service

This release implements the reusable OTP service discussed for CRM login and phone verification. It is part of the existing Go binary, uses Gin and the existing PostgreSQL connection, and embeds its English console. There is no Node.js, Prisma, Redis, npm build, or second backend. The larger CRM/chat/automation roadmap is separate work.

## Included

- Independent project API keys (one-way HMAC at rest), rotation, enable/pause, project isolation.
- Six-digit cryptographically random codes, expiry, bounded attempts, atomic single-use consumption.
- Purpose and login-attempt reference binding; successful verification returns the verified destination to the trusted backend.
- Explicit consent assertion, per-phone/project/IP rate limits, per-project daily budgets, resend cooldown.
- Durable PostgreSQL delivery queue, concurrent worker claiming with `SKIP LOCKED`, stale-worker recovery.
- Stable idempotency for requests/resends; replacing a code invalidates the previous code for that project/phone/purpose.
- Sandbox, direct linked-device Evolution delivery, and official Meta copy-code authentication templates.
- Signed Meta delivery webhooks, duplicate/out-of-order handling, and reconciliation when a receipt precedes the send response.
- English admin console with projects, key rotation, pause, OTP playground, delivery status, and audit history.
- Configurable retention, encrypted queued codes and destinations, no code/token/body logging in the OTP module.
- Go integration client, CI workflow, VPS Docker Compose and reverse-proxy examples.

## Local setup

Keep your existing Go/PostgreSQL setup. Add independent secrets to the root `.env` (or production secret environment). Generate them without displaying them in shared logs:

```sh
umask 077
go run ./cmd/otp-secrets > build/otp-secrets.env
```

Copy those entries into `.env`, then use these development settings:

```dotenv
OTP_ENABLED=true
OTP_ALLOW_SANDBOX=true
OTP_ALLOW_EVOLUTION=false
OTP_RETENTION_DAYS=30
SERVER_HOST=127.0.0.1
SERVER_PORT=8080
```

The generated file contains `OTP_ADMIN_TOKEN`, `OTP_HMAC_KEY`, and `OTP_ENCRYPTION_KEY`. HMAC and encryption keys must each be independent 32-byte random values encoded as 64 hex characters. Preserve them across restarts. Changing the HMAC key invalidates every project key and active code; changing the encryption key makes queued codes and destinations unreadable. Do not rotate these casually; drain jobs, expire challenges, and reissue project credentials during a planned migration.

```sh
go run ./cmd/evolution-go -dev
```

Open **http://localhost:8080/otp**. Unlock using `OTP_ADMIN_TOKEN`; this is separate from the manager's `GLOBAL_API_KEY`. Tokens remain only in browser memory; refreshing locks the console. The service fails startup if enabled with missing or malformed configuration. OTP tables are migrated automatically on startup; back up the database before application upgrades.

### Test the full flow

1. Create a sandbox project and save its one-time API key.
2. Click **Use in playground**.
3. Enter an E.164-format phone, purpose `login`, and a unique login-attempt reference. Confirm the consent checkbox.
4. Request a code. Wait approximately one second and check delivery; it should become `SIMULATED`.
5. Reveal the sandbox code (admin-only and audited), then verify.
6. A second verification must fail. A sandbox response contains `sandbox: true` and must never grant a real customer session.

No message is sent in sandbox mode. Production project APIs never expose the OTP itself. The only reveal endpoint is admin-only, explicitly enabled sandbox delivery.

## Providers

### Existing Evolution linked device

Set `OTP_ALLOW_EVOLUTION=true`, connect your WhatsApp account in the manager, and create an Evolution project using its instance UUID. The worker calls the Go whatsmeow service directly, with a 25-second context and stable message ID. This authentication-specific path does not run the gateway's generic text retry loop.

A send error is marked `UNKNOWN` because the provider may already have received it. No automatic retry runs for uncertain submissions. The linked-device adapter currently reports acceptance, not delivery/read receipts. WhatsApp can disconnect or require relinking; self-hosting does not guarantee account availability. Existing gateway WhatsApp debug logs and optional event integrations are outside the OTP module: keep debug logging, third-party webhooks, and message exports disabled for authentication instances.

### Official Meta WhatsApp Business Platform

Configure:

```dotenv
OTP_ALLOW_SANDBOX=false
OTP_META_ACCESS_TOKEN=<server-side token>
OTP_META_PHONE_ID=<numeric sender phone-number ID>
OTP_META_VERSION=<supported Graph API version, e.g. v25.0>
OTP_META_APP_SECRET=<Meta application secret>
OTP_META_VERIFY_TOKEN=<independent random verification token>
```

Create and obtain approval for a **copy-code authentication template** in Meta, then configure its name and language on the project. Match the template's displayed expiry and message delivery TTL to the project's code lifetime. Template approval, business eligibility, sender registration, billing, and live delivery require your Meta account; they cannot be completed by local code alone. One-tap/zero-tap Android SDK integration is not part of this release.

Set the webhook callback to `https://your-domain/otp/webhooks/meta`, verify using `OTP_META_VERIFY_TOKEN`, and subscribe to the relevant WhatsApp message events. Incoming payload signatures are checked against `OTP_META_APP_SECRET`; events for another phone-number ID are ignored. Reverse proxies must not log the verification URL query string.

Authoritative references: [Meta authentication products](https://whatsappbusiness.com/products/conversation-categories/authentication/), [Meta template API collection](https://www.postman.com/meta/whatsapp-business-platform/overview), [WhatsApp pricing](https://whatsappbusiness.com/products/platform-pricing/).

## API contract

All project routes require `Authorization: Bearer <project-api-key>`. Call these from the CRM **backend**. Never embed project/admin credentials in React Native, browser apps, public bundles, URLs, or analytics. Your backend associates the challenge with its own pending login session; only it decides whether to grant access after verification. Do not accept a customer-supplied user ID as proof of account ownership. For account recovery, apply your application's additional checks. Sandbox verification is not identity proof.

### Request

`POST /v1/otp/request`

Headers: `Content-Type: application/json`, `Idempotency-Key: <unique 8–128 character identifier>`.

```json
{
  "phone": "+919876543210",
  "purpose": "login",
  "reference": "server-generated-login-attempt-id",
  "consent": true
}
```

Response `202`:

```json
{
  "challengeId": "a UUID",
  "status": "ACTIVE",
  "deliveryStatus": "QUEUED",
  "destination": "••••3210",
  "expiresAt": "2026-10-11T12:05:00Z",
  "resendAfter": "2026-10-11T12:01:00Z",
  "sandbox": false
}
```

The consent assertion is audited; your CRM must preserve the underlying customer request/consent evidence. Repeating the same project/key/payload returns the same challenge. Reusing a key with different input returns `409 IDEMPOTENCY_CONFLICT`. Idempotency records last for the configured retention period; never reuse keys intentionally.

### Verify

`POST /v1/otp/verify`

```json
{
  "challengeId": "challenge UUID",
  "code": "123456",
  "purpose": "login",
  "reference": "server-generated-login-attempt-id"
}
```

Response `200`: `verified`, `challengeId`, `phone`, `purpose`, `reference`, `sandbox`. A code is accepted only once, including simultaneous requests from different server replicas. Wrong purpose/reference counts as a failed attempt. Verification failures deliberately share `INVALID_OTP` so they do not disclose the reason to an unauthenticated customer. If a successful verify response is lost, start a new challenge; verification is not replayable.

### Resend

`POST /v1/otp/resend` with a **new** idempotency key:

```json
{"challengeId":"latest challenge UUID","consent":true}
```

Returns a replacement challenge. The old code stops working. Repeating the same resend/key returns the same replacement. Wait for the cooldown; verified, superseded, or cancelled challenges cannot initiate a new resend. Destinations are erased one hour after expiry, after which request a fresh challenge using the known phone rather than resend.

### Status

`GET /v1/otp/challenges/{id}` returns the request response shape and is project-scoped.

Challenge states: `ACTIVE`, `VERIFIED`, `LOCKED`, `SUPERSEDED`, `EXPIRED`, `CANCELLED`.

Delivery states: `QUEUED`, `SENDING`, `ACCEPTED`, `DELIVERED`, `READ`, `FAILED`, `UNKNOWN`, `EXPIRED`, `SIMULATED`, `CANCELLED`.

`ACCEPTED` is not proof of delivery. `DELIVERED` is not proof of identity; only successful OTP verification authorizes a next application step.

### Errors and limits

Errors use `{"error":{"code":"COOLDOWN","message":"Wait before requesting another code","retryAfter":30}}`. Rate/cooldown responses are HTTP 429 with `Retry-After`. Validation is 400, authentication 401, missing resources 404, conflicting requests 409, expired retained destination 410. Unexpected internal failures return a generic 500 without credentials or database details.

Default policy:

| Control | Default | Project creation range |
|---|---:|---:|
| Code lifetime | 300 seconds | 60–600 |
| Incorrect attempts | 5 | 1–10 |
| Resend cooldown | 60 seconds | 30–300 |
| Requests per phone/project/hour | 5 | 1–10 |
| Requests per project/day | 500 | 1–10,000 |

Additional fixed limits: 10 requests per phone/hour across projects, 60 requests per project/calling IP/minute, 120 verification requests per project/calling IP/minute, 300 HTTP requests per calling IP/minute. These use UTC fixed windows, so a window boundary permits bursts. The service uses the immediate peer IP and ignores spoofable forwarding headers. Behind the supplied proxy, that is the proxy IP; implement additional end-user IP/device abuse protection in your CRM or trusted edge. Volume settings are application controls, not WhatsApp account allowances.

Request/resend limits count new logical challenges, not idempotent repeats. A project pause invalidates active challenges and cancels queued jobs. Already submitted/in-flight messages cannot be recalled. Project provider/policy fields are immutable in this release; create a replacement project to change them.

### Admin API

Use the separate OTP admin token, restrict network access, and prefer the console:

| Method | Path | Purpose |
|---|---|---|
| GET | `/otp/admin/overview` | Counts and configured providers |
| GET | `/otp/admin/projects` | Latest 200 projects, excluding key hashes |
| POST | `/otp/admin/projects` | Create a project; return API key once |
| PATCH | `/otp/admin/projects/{id}` | `{"enabled":false}` to pause |
| POST | `/otp/admin/projects/{id}/rotate` | Revoke old key and return replacement once |
| GET | `/otp/admin/activity?projectId=...` | Latest 100 jobs; latest 100 global admin audit events |
| POST | `/otp/admin/sandbox/{challengeId}/code` | Reveal active simulated code only |
| GET | `/otp/health` | Database readiness |

Project creation fields: `name`, `provider` (`sandbox`, `evolution`, `meta`), `instanceId` (Evolution), `template` and `language` (Meta), `ttlSeconds`, `maxAttempts`, `cooldownSeconds`, `phoneHourlyLimit`, `dailyLimit`.

## Go client

Use `pkg/otpclient` from a trusted backend. Every method takes a context, and the client has a 15-second HTTP timeout and refuses redirects. It deliberately performs no implicit retries. Reuse the same idempotency key when explicitly retrying request/resend after a network error.

```go
client, err := otpclient.New(os.Getenv("OTP_BASE_URL"), os.Getenv("OTP_PROJECT_KEY"))
if err != nil { return err }
challenge, err := client.Request(ctx, otpclient.Request{
    Phone: customerPhone, Purpose: "login", Reference: loginAttemptID, Consent: true,
}, requestID)
// Persist challenge.ID against loginAttemptID in your CRM.
// Later, verify using that server-side association:
result, err := client.Verify(ctx, otpclient.Verify{
    ChallengeID: challenge.ID, Code: submittedCode,
    Purpose: "login", Reference: loginAttemptID,
})
// Require err == nil, result.Verified, !result.Sandbox, and the expected phone.
// Create the CRM session only after all these checks succeed.
```

REST works equally with other backend languages. This release does not modify or integrate any CRM yet.

## Queue, recovery, and data lifecycle

The challenge and delivery job are inserted in one database transaction before returning 202. Project row locks serialize code replacement and verification. Workers claim jobs with row locks and persist `SENDING` before contacting a provider. Only an explicit Meta HTTP 429 is automatically retried, at most three attempts, and only while enough validity remains. Timeouts, transport failures, and 5xx responses are marked `UNKNOWN`. A worker interrupted after claiming is marked `UNKNOWN` after two minutes, preventing blind duplicates.

No system can guarantee exactly-once network delivery across a crash. Request idempotency prevents duplicate logical jobs, and verification consumption is atomic. Customers can explicitly request a replacement after the cooldown. Provider acceptance and final delivery remain separate states.

Queued/sandbox codes use AES-256-GCM with challenge-bound associated data. The verification digest uses keyed HMAC-SHA256; a plain hash would allow offline guessing of the small code space. Codes are erased after submission/verification/expiry, except active sandbox codes retained for admin testing. Encrypted phone numbers are erased one hour after expiry. Challenge/job/audit records default to 30-day retention. Backups may retain older ciphertext; apply the same retention controls to backups. OTP destinations necessarily pass through WhatsApp/Meta and the sending account; self-hosting does not remove that dependency.

## VPS deployment

The Compose file builds this fork's source and publishes the gateway on **VPS loopback only**. PostgreSQL is not publicly exposed. Runtime logs are capped; the database and logs use persistent volumes. No Redis is needed.

1. Copy `docker/otp/.env.example` to `docker/otp/.env` and set file permissions to 600.
2. Generate independent OTP secrets. Generate independent `DB_PASSWORD` and `GLOBAL_API_KEY` as random hex values; hex avoids URL-escaping problems in the database DSN.
3. Configure your provider. Keep `OTP_ALLOW_SANDBOX=false` in production.
4. From `docker/otp`, run `docker compose --env-file .env up -d --build`.
5. Configure an HTTPS reverse proxy using `docker/otp/nginx.conf.example`. Only project APIs and signed Meta callbacks are public.
6. Access admin UI via `ssh -L 8080:127.0.0.1:8080 user@vps`, then open `http://localhost:8080/otp` on your computer.
7. Back up PostgreSQL and the encryption/HMAC secrets securely; test a restore before onboarding customers.

Do not expose the existing whole gateway/manager publicly without reviewing its separate authorization and media-handling risks. Use a firewall, disable WhatsApp debug logs and unwanted event sinks, and keep secrets out of Git. The example database role owns the application database and creates the auth database; a stricter production setup should pre-provision both databases and restrict runtime DDL after migrations.

At 300 OTP messages/day, one Go process with PostgreSQL is a reasonable starting architecture. This is not a benchmark or provider limit. Monitor request latency, oldest queued job, failed/unknown deliveries, expired codes, database connections, and disk growth. Keep a recovery/fallback authentication method in the CRM; no SMS/email fallback provider is configured by this release.

## Validation

```sh
go test ./...
OTP_TEST_DATABASE_URL='postgres://user:password@localhost:5432/testdb?sslmode=disable' \
  go test -race -count=1 ./pkg/otp ./pkg/otpclient
```

Integration tests create unique `otp_test_*` schemas and remove only those generated schemas. The test database user needs schema creation permission. Without `OTP_TEST_DATABASE_URL`, PostgreSQL tests skip explicitly. CI supplies a disposable PostgreSQL service.

Tests cover code encryption/context binding, secret redaction, config validation, project isolation/key rotation/pause, idempotency, resend replacement/cooldown, attempt lockout, expiry, concurrent single-use verification, concurrent job claiming, interrupted worker recovery, expired-job suppression, retention, signed webhooks, receipt ordering, HTTP contracts, and provider payloads. Live Meta credentials and an approved template are required for a separate end-to-end delivery acceptance test; local tests do not establish production delivery availability.
