# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

No tagged releases exist yet; all work currently lives under **Unreleased**.

## [Unreleased]

### Added

- **Password reset** — enumeration-safe `POST /auth/forgot-password` and one-time `POST /auth/reset-password` flows with expiring (24h) tokens delivered via SMTP or logged to stdout when SMTP is unconfigured
- **Member suspension** — admin-controlled suspensions with auto-expiry (`suspended_until`), dashboard reminder banner for expirations within 2 days, and automatic access revocation while suspended
- **Org join requests** — users can sign up with an `org_slug` and request membership; admins accept or revoke requests
- **Member display names** — org membership listings now include the user's name and email
- **Member data scoping** — non-admin members see only their own membership record, never other members' details
- **SMTP mailer** — `net/smtp` STARTTLS delivery with a `LogMailer` fallback; configurable host, port, credentials, and from-address
- **API key hashing at rest** — API keys stored as SHA-256 digests (migration 010), raw key shown only once at signup
- **Pagination** — `limit`/`offset` query params on list endpoints with `X-Total-Count`
- **Health probes** — `/healthz` liveness and `/readyz` readiness endpoints
- **In-process TLS** — optional `TLS_CERT_FILE`/`TLS_KEY_FILE` with HSTS
- **Security headers** — nosniff, frame-deny, referrer policy, CSP on every response
- **Configurable bcrypt cost** via `BCRYPT_COST` (default 12)
- **Secret scan** — gitleaks + CodeQL in CI; `.gitleaks.toml` allowlist
- **Docker + Makefile + CI** — multi-stage distroless Dockerfile, build/format/vet/test targets, and a GitHub Actions pipeline
- **MIT License**

### Changed

- Suspended members are excluded from the universal membership access gate (`FindMember`), not merely from the dashboard
- Invoice generation amounts expressed in cents with line items
- Rate limiting keyed by `X-API-Key` when present, IP otherwise

### Fixed

- Null slice responses coerced to `[]` (dashboard empty-list crash)
- Password-reset link token extraction now correctly cuts at the closing quote inside the email anchor
- Migration 010 idempotently rehashes legacy plaintext API keys

### Security

- Production boot refuses placeholder/weak `JWT_SECRET` values (non-`development` environments)
- Enumeration-safe password reset (unknown emails still succeed)
