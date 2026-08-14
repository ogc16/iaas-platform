## Summary

<!-- What does this change do and why? Keep it focused. -->

## Type of change

- [ ] Bug fix
- [ ] New feature
- [ ] Refactor / internal
- [ ] Documentation
- [ ] CI / build / tooling

## Test plan

<!-- How was this verified? Run the relevant checks locally:

gofmt -l .
go vet ./...
go test -race -count=1 ./...        # integration tests need -tags integration + a Postgres
go run github.com/gitleaks/gitleaks/v8@latest detect --config .gitleaks.toml
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
-->

- [ ] `gofmt -l .` is clean
- [ ] `go vet ./...` passes
- [ ] `go test ./...` passes
- [ ] New tests cover the change

## Checklist

- [ ] Branch follows the `<type>/<short-slug>` convention (never commit to `master`)
- [ ] Single commit per change set, conventional message (`type(scope): summary`)
- [ ] No secrets, binaries, logs, or `.env` committed
- [ ] CHANGELOG.md updated if user-visible
- [ ] openapi.yaml updated if the API surface changed
- [ ] Relevant docs updated (README, docs/, SECURITY.md)

## Related

<!-- Issues this closes, e.g. Closes #12 -->
