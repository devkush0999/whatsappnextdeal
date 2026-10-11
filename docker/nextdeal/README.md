# NextDeal WhatsApp delivery deployment

This deployment keeps the existing NextDeal four-digit challenge and verification in Node. It does not call the independent six-digit `/v1/otp` service.

1. Generate three independent random 32-byte hex secrets for DB_PASSWORD, GLOBAL_API_KEY and NEXTDEAL_OTP_TRANSPORT_KEY in `.env` (chmod 600). Never commit it.
2. `docker compose --env-file .env up -d --build` from this directory.
3. Manager stays private on VPS loopback port 8091. Use `ssh -L 8091:127.0.0.1:8091 root@76.13.79.182` and open http://localhost:8091/manager. Authenticate with GLOBAL_API_KEY; connect a dedicated WhatsApp sender by QR. Do not expose manager/admin APIs publicly.
4. Set NEXTDEAL_OTP_INSTANCE_ID to the connected instance UUID and NEXTDEAL_OTP_TRANSPORT_ENABLED=true; recreate gateway.
5. Persistently attach the Node app to external network `nextdeal-whatsapp-delivery` in its Compose configuration. Set OTP_PROVIDER=whatsapp, WHATSAPP_OTP_BASE_URL=http://nextdeal-whatsapp:8080 and WHATSAPP_OTP_KEY to the transport key in the backend environment. Recreate only the app after backend build and tests pass.
6. Test one user-requested OTP end to end; do not switch customer traffic before receiving and verifying it. ACCEPTED does not mean delivered. Old mobile copy may still say SMS and SMS auto-read will not read WhatsApp. Users manually enter four digits. The current backend helper is also used by resend/profile flows; test each before enabling globally.

Transport is bound to a single sender, accepts only Indian individual numbers and four digits, redacts errors, enforces a 30s in-memory phone cooldown / 300 attempts per UTC day, and never retries uncertain sends. These limits reset on restart and assume one replica. Durable verification/expiry/rate controls must remain in NextDeal; this route does not replace them. Dedupe rejects repeated request IDs for one hour and does not replay success. A process restart clears its transport dedupe.

Keep debug logs, message persistence and event exports disabled. The existing whatsmeow event pipeline is separate from this endpoint and needs privacy review before enabling extra integrations. Linked-device disconnections can interrupt authentication; WhatsApp availability and account restrictions are not controlled by daily volume alone. This is not the official Meta Cloud API.

Back up this service database and sender sessions separately. Never run compose down -v against either project. Roll back the backend provider setting only to a tested provider; restoring MSG91 config does not fix its upstream delivery issue.

Tests: go test ./pkg/otptransport ./pkg/otp ./pkg/otpclient ./pkg/whatsmeow/service; go build ./cmd/evolution-go. DB-backed OTP integration tests require their explicit disposable test database. No live messages are sent by unit tests.
