# Evolution Go

Self-hosted WhatsApp gateway with a Go OTP service for CRM and application authentication.

- [OTP service: setup, console, API, integration, and VPS deployment](docs/OTP_SERVICE.md)
- [Feature roadmap](MESSAGING_PLATFORM_ROADMAP.md)
- [Existing codebase review](CODEBASE_REVIEW.md)
- [Local privacy review](LOCAL_PRIVACY_REVIEW.md)

The OTP admin console is available at `/otp` when `OTP_ENABLED=true`. The WhatsApp manager remains at `/manager`.

Build with `go build ./cmd/evolution-go` and test with `go test ./...`. See the OTP guide for secrets, database configuration, sandbox testing, and live provider requirements.
