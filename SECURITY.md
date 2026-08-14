# Security

IaaS Platform handles authentication, multi-tenant organization data, and usage-based billing, so security is treated as a first-class concern. This document describes the current security model, known limitations, hardening guidance, and how to report a vulnerability.

## Security Model

### Authentication

| Mechanism | Details |
|-----------|---------|
| Passwords | Hashed with **bcrypt** (`golang.org/x/crypto/bcrypt`), cost configurable via `BCRYPT_COST` (default **12**, range 4–15; keep ≥ 10 in production). Passwords are never stored or returned as plaintext. |
| JWT | **HS256** signed tokens (`github.com/golang-jwt/jwt/v5`) carrying `user_id`, `email`, `role`, plus issuer/issued-at/expiry claims. Lifetime defaults to 24h (`JWT_EXPIRES_IN`). Only HMAC signing methods are accepted during validation. |
| API keys | Generated as 32 random bytes (`crypto/rand`), prefixed `iaas_`, returned **exactly once** at signup. Only a **SHA-256 hash** is stored (`users.api_key`, unique index). Login never exposes the key. |
| Password reset | One-time tokens prefixed `pwr_`, stored as SHA-256 digests, TTL default 24h (`PASSWORD_RESET_TTL_HOURS`), single-use (`used_at`), and enumeration-safe (unknown emails still return success). |
| Production guard | In any environment other than `development`, the server **refuses to boot** if `JWT_SECRET` is empty, a known placeholder, or shorter than 32 bytes (`internal/config/config.go`). |

API access is authenticated via `Authorization: Bearer <jwt>` or `X-API-Key: <key>`; both are accepted by the auth middleware.

### Authorization

- Multi-tenancy is enforced at the service layer, not just the router: every org-scoped operation looks up the caller's membership (`FindMember`) before acting.
- Roles are `admin` and `member`. Admin-only operations (invites, join-request accept/revoke, suspension, member removal) reject non-admins with `400`.
- Suspended members lose access automatically: the membership lookup excludes rows whose `suspended_until` is in the future. Suspensions are reversible and auto-expire.
- Members **cannot enumerate other members** — non-admin membership listings return only the caller's own record (no names, emails, or roles of others).
- Self-destructive actions are rejected (`ErrSelfAction`): a user cannot remove or suspend themselves.

### Transport & headers

- Baseline security headers are set on every response (`internal/middleware/security.go`):
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Referrer-Policy: no-referrer`
  - Content-Security-Policy restricting scripts/styles/images/connect sources to self
  - `Strict-Transport-Security` (2-year, includeSubDomains, preload) is emitted when TLS is enabled in-process.
- The server can terminate TLS itself (`TLS_CERT_FILE` / `TLS_KEY_FILE`), or sit behind a reverse proxy (recommended for production; see `docs/DEPLOYMENT.md`).

### Request hardening

- **Rate limiting:** a global in-memory token bucket (refill 1 token/sec, burst 10), keyed by client IP or `X-API-Key` when present. Exceeded requests get `429` with `Retry-After`.
- **Timeouts:** server-level `ReadTimeout`/`WriteTimeout`/`ReadHeaderTimeout`/`IdleTimeout` are set (15s/15s/10s/60s).

## Known Limitations

These are acknowledged and tracked; none block development, but production deployments should plan for them:

1. **No request-body size limit.** Large payloads are bounded only by server timeouts. Terminate TLS and enforce body limits at a reverse proxy (or see the open issue to add `http.MaxBytesReader`).
2. **No per-account rate limiting or login lockout.** Brute-force protection beyond bcrypt cost and the global limiter is future work.
3. **CORS allows all origins** (`Access-Control-Allow-Origin: *`). An allowlist configuration is planned.
4. **In-memory rate limiter** is per-process state — scale horizontally only with a shared store (e.g. Redis) or sticky sessions.
5. **No API-key rotation endpoint** yet — keys are valid until changed in the database.

## Configuration Hardening Checklist

- [ ] `ENV=production`
- [ ] `JWT_SECRET` set to a random value ≥ 32 bytes (the server refuses known placeholders in non-dev)
- [ ] `BCRYPT_COST=12` (or higher)
- [ ] Terminate TLS in-process **or** at a reverse proxy with HSTS enabled
- [ ] Use a dedicated database user with least privilege; never the superuser
- [ ] SMTP credentials provided via environment or secret store, never committed
- [ ] Run the gitleaks secret scan (`go run github.com/gitleaks/gitleaks/v8@latest detect --config .gitleaks.toml`) before every push; CI enforces it

## Data Protection

- Passwords: bcrypt (one-way).
- API keys and reset tokens: SHA-256 digests at rest (one-way).
- JWT secrets, SMTP passwords, and database credentials: environment-only; `.env` is gitignored and excluded from the Docker build context.
- CI runs **gitleaks** and **CodeQL** on every push/PR to catch accidental secret commits and code-level vulnerabilities.

## Reporting a Vulnerability

Please **do not open a public issue** for security problems.

- Privately report via the repository's **Security → Report a vulnerability** page (GitHub Security Advisories).
- If that is unavailable, open a private e-mail to the maintainer (address in the repository owner's profile).

Include a description of the issue, the affected version/branch, and a minimal reproduction. We will acknowledge within 5 business days and coordinate a fix before public disclosure.
