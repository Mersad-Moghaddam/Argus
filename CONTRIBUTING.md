# Contributing to Argus

Thank you for helping make Argus more dependable. Contributions are welcome as issue reports, design discussions, documentation, tests, and focused pull requests.

## Before you begin

1. Search existing issues and pull requests.
2. For a large feature or architectural change, open an issue first and describe the operator problem, proposed behavior, failure modes, and migration impact.
3. Review `PROJECT_MONITORING_PLAN.md` for work already sequenced on the multi-project subsystem.
4. Follow the `CODE_OF_CONDUCT.md` and report security issues through `SECURITY.md`, not a public issue.

## Local development

```bash
git clone https://github.com/Mersad-Moghaddam/Argus.git
cd Argus
docker compose up -d
go test ./...
go run ./cmd/api
```

The dashboard is available at `http://localhost:8080`. MySQL and Redis use ports `3306` and `6379`.

## Design principles

- Keep domain policies pure and framework-independent.
- Define external dependencies as ports; implement them in adapters.
- Preserve the existing website-monitoring API unless a change is explicitly breaking and documented.
- Make background jobs bounded, retry-safe, and idempotent.
- Treat URLs, uploaded specifications, headers, and webhook targets as untrusted input.
- Prefer additive, reversible database migrations.
- Keep the frontend dependency-free unless a proposal establishes a clear maintenance benefit.
- Add tests for state transitions, authorization boundaries, parsing limits, and failure behavior.

## Quality checks

Run these before submitting:

```bash
gofmt -w $(git ls-files '*.go')
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

If `revive` is installed:

```bash
revive -config revive.toml ./...
```

## Pull requests

- Branch from the latest `main`.
- Keep each commit and pull request focused.
- Use an imperative, descriptive title.
- Explain what changed, why, how it was tested, and any operational or migration implications.
- Include screenshots for dashboard changes and request/response examples for API changes.
- Update the README, user guide, and roadmap when behavior or maturity changes.
- Do not commit secrets, local `.env` files, database dumps, or generated dependency caches.

Maintainers may ask for a smaller scope, additional tests, or a design issue before merging. By contributing, you agree that your work will be licensed under the MIT License.
