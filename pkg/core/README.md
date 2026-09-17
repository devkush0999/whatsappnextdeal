# Standalone runtime

This fork removes the external license lifecycle: startup activation, registration
exchange, hardware-derived installation identity, request gating, usage heartbeat,
and shutdown deactivation. It no longer creates or reads `runtime_configs`.
Existing rows are left untouched; no database cleanup or credential deletion runs.

`GLOBAL_API_KEY` remains required for local administrator authentication. Instance
API keys still authorize instance routes. No license key or operator email is needed.

The bundled manager has no source in this checkout and checks `/license/status`
before authenticating against `/instance/all`. `RegisterManagerCompatibilityRoutes`
serves a local `status: active`, `mode: standalone`, `license_required: false`
response. Legacy register/activate URLs return the same response without network
calls, redirects, code exchange, or credentials in the response. These routes can
be retired when the frontend's activation flow is removed from its source.

Rebuild the server from source to use this behavior; an existing executable or
running process retains the old behavior. For example:

```sh
go build -o build/evolution-go ./cmd/evolution-go
./build/evolution-go --dev
```

Run from the repository root so `.env` and manager assets resolve. PostgreSQL and
WhatsApp connectivity are still needed for their respective application features.
If using SQLite session storage, its existing implementation locates `dbdata`
beside the executable; preserve that directory when changing the binary location.
The dormant `pkg/telemetry` package is separate and is not wired into startup.

Upstream `LICENSE`, `NOTICE`, and trademark documents remain unchanged. Removing
the runtime dependency does not replace those documents.
