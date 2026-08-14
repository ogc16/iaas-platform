# Roadmap

IaaS Platform is a Go-based control plane and billing platform. The compute backend is currently a durable simulation; the control-plane surface (auth, multi-tenancy, quotas, billing, async lifecycle) is production-shaped.

This roadmap reflects maintainer intent, not commitments. Items are ordered by priority and depend on contributor and maintainer capacity.

## Near Term (v1.0)

Gate for declaring **v1.0** (no tags or releases exist yet):

- [ ] **Database transactions** for multi-step writes (instance creation, org creation, invoice generation) — see `docs/ENTERPRISE_READINESS.md`
- [ ] **Prometheus metrics export** (HTTP request latency/errors, per-endpoint) for production observability
- [ ] **CORS allowlist** configuration instead of `*`
- [ ] **Login lockout** on repeated failed authentication
- [ ] **Idempotency keys** for instance creation and invoice generation
- [ ] **API-key rotation endpoint**
- [ ] **golangci-lint** wired into CI
- [x] **Request-body size limit** (`http.MaxBytesReader`)
- [ ] **Integration tests** against a real PostgreSQL (via docker-compose) in CI
- [ ] First tagged release + published container image

## v2.0

- **Real compute backends** — implement the `compute.Provider` seam against a concrete backend:
  - Docker / Podman for self-hosted compute
  - One hyperscaler (AWS EC2 or GCE) as the first native provider
- **Webhook notifications** for instance lifecycle events (created, running, failed, terminated) and billing events
- **Per-account rate limiting and quotas** beyond the global token bucket
- **Custom compute providers** — SaaS/datacenter integrations plugged in via config, not code
- **Control-plane dashboard v2** — the current embedded SPA is intentionally minimal
- **Audit log** — immutable, per-tenant record of administrative actions
- **Usage aggregation jobs** — move billing aggregation out of the request path into a scheduled worker

## Longer Term (Post v2.0)

- **Multi-region control plane** — read replicas, horizontal scaling of stateless servers with a shared rate-limiter store
- **Organization plans and tiered billing** (flat rate + usage)
- **Team roles** beyond admin/member (e.g. billing-admin, read-only)
- **SSO / OIDC federation**
- **Infrastructure-as-code template** (Terraform) to deploy the control plane itself

## Explicit Non-Goals

- Building a hypervisor or orchestrator — the control plane delegates to providers
- On-premise management agents (until the provider seam is exercised by a real backend)
- Multi-currency billing beyond USD

---

Contributions toward any item are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) and the open [good-first-issues](https://github.com/ogc16/iaas-platform/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).
