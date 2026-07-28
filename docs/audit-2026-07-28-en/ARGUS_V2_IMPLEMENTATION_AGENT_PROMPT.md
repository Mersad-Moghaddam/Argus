# Argus v2 — End-to-End Implementation Prompt

Copy the complete prompt below into a capable coding agent running at the root of the Argus repository.

---

## Prompt

You are the lead product engineer, software architect, SRE, security engineer, and design engineer responsible for delivering **Argus v2** from the existing repository to a complete, tested, documented, production-ready implementation.

Your assignment is to perform the work, not merely analyze it. Inspect the repository, establish the current baseline, implement every required change, migrate existing behavior and data safely, validate the complete system, update all documentation to match the as-built implementation, commit the intended changes, and push the completed branch to the remote repository.

All source code, UI copy, API messages, test names, comments, documentation, commit messages, branch names, and final reports must be written in English.

## 1. Mission and required outcome

Deliver Argus v2 as a secure, accessible, telemetry-first monitoring platform with:

- A global account entry point in the top-level header.
- Registration and login completed before a user can create or manage projects, endpoints, routes, monitors, integrations, or other private resources.
- Authentication removed from the project-creation interface and represented as its own global application flow.
- A coherent authenticated application shell in which authorized users can access every applicable feature.
- Centralized backend normalization and validation for every user-entered URL, route, endpoint, and target.
- A monitoring model that does not generate one broad recurring request per imported route.
- OpenTelemetry-based passive monitoring as the primary signal source.
- Explicit, budgeted, safe synthetic checks only where active validation is genuinely required.
- Heartbeats for scheduled or push-based workloads.
- SLO-based health, alerting, incident creation, and evidence.
- A production-capable approach for private targets that does not let the central control plane arbitrarily dial internal customer networks.
- Complete UI/UX, accessibility, security, migration, operational, and test coverage.

Do not stop after producing a plan, design, scaffold, partial migration, proof of concept, or collection of TODOs. Continue until the Definition of Done in this prompt is satisfied or a genuine external blocker prevents further work. If blocked by one item, complete all independent work before reporting the blocker.

Do not deploy to a production environment unless the user explicitly authorizes production deployment. Pushing the implementation branch and opening a draft pull request are required delivery actions and are not production deployment.

## 2. Operating rules

Before changing files:

1. Read the repository-level `AGENTS.md` and every instruction it references.
2. Follow repository-specific command conventions, including the required `rtk` command prefix if that instruction remains present.
3. Inspect the current branch, remote tracking state, worktree status, recent commits, repository structure, toolchain, runtime configuration, migrations, CI workflows, and tests.
4. Preserve all pre-existing user changes. Never discard, overwrite, reset, or silently rewrite unrelated work.
5. Fetch the remote and identify the correct integration base.
6. Create a new, clearly named implementation branch such as `feat/argus-v2`. If that name already exists, choose a unique descriptive variant. Do not reuse or overwrite an unrelated remote branch.
7. Record the starting commit and baseline test results.

Work autonomously and make safe, evidence-based decisions. Ask the user only when:

- Required credentials, secrets, infrastructure access, or external account decisions cannot be obtained from the repository or environment.
- Two materially different product choices have no documented resolution and choosing incorrectly would create significant rework or irreversible impact.
- An irreversible external action requires explicit authorization.

When clarification would only improve a detail, choose the safest reasonable default, document the decision, and continue.

Use incremental, logical commits throughout the implementation. Keep the branch buildable at meaningful checkpoints. Never use destructive Git operations to hide problems.

## 3. Authoritative documents

Read these documents completely before implementation and treat them as the primary Argus v2 product and engineering contract:

1. `docs/audit-2026-07-28-en/README.md`
2. `docs/audit-2026-07-28-en/ARGUS_TRANSFORMATION_BLUEPRINT.md`
3. `docs/audit-2026-07-28-en/SECURITY_REVIEW.md`
4. `Argus-threat-model.md`
5. `security_best_practices_report.md`
6. `animation-plans/README.md`
7. Every numbered plan in `animation-plans/`
8. `docs/audit-2026-07-28-en/SOURCES.md`
9. `docs/audit-2026-07-28-en/TOOLING_DECISION.md`
10. `docs/audit-2026-07-28-en/COMPLETION_MATRIX.md`

Then inspect all existing product, architecture, API, deployment, security, migration, roadmap, and user documentation, including files such as:

- `README.md`
- `PROJECT_MONITORING_PLAN.md`
- `REWRITE_ROADMAP.md`
- `USER_GUIDE.md`
- `SECURITY.md`
- Docker and Compose files
- Environment examples
- Database migrations and schema documentation
- CI/CD workflows
- API specifications

The English audit package defines the target intent, while the repository defines the implementation baseline. If documentation conflicts with executable reality:

1. Verify the behavior in code and tests.
2. Choose the solution that best satisfies the Argus v2 goals, current standards, security constraints, and backward compatibility.
3. Record the decision in an ADR or the appropriate architecture document.
4. Update superseded documentation so the repository has one coherent as-built story.

Use current official documentation for any library, framework, SDK, protocol, or cloud component whose API or recommended configuration may have changed. Prefer primary sources and standards. Use available repository-relevant skills and documentation tools where helpful, especially for current framework APIs, UI review, accessibility, security, threat modeling, frontend design, animation, and Go concurrency. Do not install speculative tools or plugins. Install an additional skill or plugin only when it has a concrete role in the work, comes from a trusted source, and its permissions and maintenance risk have been reviewed. Document any installation and why it was necessary.

## 4. Baseline and traceability

Before implementation, create a concise implementation tracking document in the repository that maps:

- Every requirement in this prompt.
- Every finding in both security reviews.
- Every threat and mitigation selected from the threat model.
- Every item in the completion matrix.
- Every motion plan.
- Every major user story and journey in the transformation blueprint.

Map each item to:

- Current behavior.
- Required change.
- Owning component.
- Migration impact.
- Tests and acceptance evidence.
- Implementation status.
- Final commit or pull-request evidence.

This matrix is a living delivery artifact, not a substitute for implementation. Update it as work lands and complete a final audit against it before delivery.

Capture baseline evidence:

- Existing unit, integration, end-to-end, race, lint, static-analysis, and vulnerability test results.
- Existing database schema and migration status.
- Current guest and authenticated UI states.
- Current monitoring request behavior and estimated request volume.
- Current route import behavior.
- Current authentication/session behavior.
- Current accessibility and keyboard behavior.
- Existing known failures, with proof that they predate the implementation.

## 5. Product and information architecture

Implement a clear separation between the public experience, identity flow, and authenticated control plane.

### 5.1 Public and guest experience

- Place account actions in the global header at the top of the page.
- Show **Register** as the primary guest action and **Log in** as the secondary guest action.
- Provide dedicated, routable registration and login experiences.
- Preserve a useful public landing page and, if supported by the product, public status pages.
- Do not expose private creation or management controls to guests.
- A guest must never see a contradictory mixture of authenticated project controls and authentication forms.
- Server authorization, not CSS visibility, must enforce access.
- Fix the hidden-state contract globally. Native `[hidden]`, the project’s `.hidden` utility, and component state must reliably remove hidden content from rendering, focus order, accessibility trees, and interaction.

### 5.2 Authentication flow

- A new user registers before creating a project.
- A returning user logs in before creating or managing private resources.
- Preserve a validated same-origin return destination so successful authentication can return the user to an intended private deep link.
- Prevent open redirects.
- Use clear validation, generic credential errors, password-manager-compatible fields, sensible autocomplete attributes, and accessible error summaries.
- Support logout, current-session inspection, revocation of other sessions, password change, and secure account recovery if the current product scope supports email delivery.
- If recovery cannot be completed without external email infrastructure, implement the secure internal workflow, configuration, tests, and documented integration boundary rather than insecure placeholders.

### 5.3 Authenticated application shell

After authentication, provide a consistent shell containing:

- Project switcher.
- Primary navigation.
- Context-aware create actions.
- Notifications or incident entry points where applicable.
- Account and session controls.
- Clear authorization feedback.

All features permitted by the user’s role must become available after authentication. Implement and enforce a documented authorization model such as owner, administrator, editor, and viewer. Enforce tenant and project scoping in every query, mutation, job, event, cache key, telemetry mapping, WebSocket/SSE stream, export, and background task.

### 5.4 Project creation and onboarding

Remove registration and login from the project area. Replace the current project flow with a focused authenticated onboarding journey:

1. Project identity and environment.
2. Monitoring source selection.
3. Signal connection or import.
4. Signal verification.
5. Starter SLO creation.
6. Notification or incident integration.
7. Completion summary and next recommended action.

Source selection must clearly explain:

- **OpenTelemetry — Recommended:** passive production telemetry through OTLP.
- **OpenAPI catalog:** imports endpoint definitions without making requests.
- **Synthetic check:** a deliberate active test with visible safety and cost controls.
- **Heartbeat:** a push signal for jobs and scheduled processes.
- **Sample data:** an optional guided evaluation path clearly marked as non-production data.

OpenAPI import must make **zero outbound requests** to the imported operations and must not silently enable synthetic checks.

Provide draft preservation, back/forward navigation, progress indication, safe cancellation, useful empty states, and recovery from refresh or an interrupted session.

## 6. Canonical URL, route, and endpoint model

Create one backend-owned normalization and validation pipeline. It must be the only authoritative path used by:

- Manual endpoint creation.
- Project base URL creation and updates.
- Bulk input.
- OpenAPI import.
- Legacy migration.
- API creation and update endpoints.
- Background jobs.
- Synthetic target creation.
- Any future agent or integration ingestion.

Frontend normalization may preview the result, but the backend must independently normalize and validate every input.

### 6.1 Required representations

Keep the following concepts explicit:

- **Input representation:** what the user supplied, retained only where useful and safe.
- **Display representation:** readable, escaped text shown to a user.
- **Canonical identity:** stable normalized data used for equality, uniqueness, and mapping.
- **Fetch target:** the final parsed URL used for a controlled outbound request after safety validation.

Never use display text as a fetch target and never construct a URL by string concatenation.

### 6.2 Normalization requirements

Implement a standards-based canonicalizer using the runtime’s structured URL APIs and RFC 3986 semantics where applicable:

- Trim ordinary surrounding whitespace.
- Reject hidden Unicode controls, dangerous whitespace, null bytes, invalid UTF-8, and ambiguous separators.
- Require an explicit supported scheme for absolute targets.
- Lowercase scheme and host.
- Normalize internationalized domain names using current IDNA processing.
- Correctly parse DNS names, IPv4 literals, and bracketed IPv6 literals.
- Reject user information in URLs.
- Reject fragments.
- Normalize or remove default ports.
- Validate non-default ports.
- remove dot segments without changing legitimate path meaning.
- Apply one documented percent-encoding policy.
- Reject malformed percent escapes.
- Define and test the policy for encoded `/`, encoded `\`, literal backslashes, repeated separators, and ambiguous path traversal.
- Preserve path case.
- Preserve meaningful trailing-slash semantics unless product rules explicitly unify them.
- Parse query parameters separately from the path.
- Define whether query order, duplicates, and empty values participate in endpoint identity.
- Validate route templates and parameter placeholders.
- Prevent a path template from being confused with a concrete fetch URL.
- Resolve relative references through structured URL resolution such as Go’s `url.ResolveReference`, never string concatenation.

Return stable machine-readable validation error codes plus clear English messages.

### 6.3 Preview and user feedback

Create an authenticated backend preview endpoint and corresponding UI that shows, before save:

- The canonical endpoint identity.
- The effective fetch target, when applicable.
- Environment and base URL resolution.
- Any transformed fields.
- Duplicate detection.
- Safety restrictions.
- Whether the item creates no traffic, passive traffic ingestion, a heartbeat, or an active request.
- The expected synthetic request frequency and approximate daily request count.

### 6.4 Data model and uniqueness

Introduce explicit environments and endpoint identities. Design the uniqueness key around the actual product contract, including fields such as:

- Project.
- Environment.
- Method where relevant.
- Canonical scheme/host/port/path.
- Route template identity.
- Query identity policy.
- Monitoring source or check identity where required.

Use a versioned canonical hash so normalization rules can evolve safely. Implement dual-write, backfill, conflict reporting, and reversible migration steps. Never silently merge or delete legacy records when canonical forms collide.

### 6.5 SSRF and egress safety

Normalization is not an SSRF defense. Retain and strengthen independent outbound safety controls:

- Scheme allowlist.
- Port policy.
- DNS and IP classification.
- Private, loopback, link-local, multicast, metadata, and reserved range policy.
- DNS rebinding resistance.
- Validation at creation time and immediately before dialing.
- Redirect validation on every hop.
- Redirect limit.
- Egress proxy, network policy, or equivalent infrastructure boundary.
- Response-size limits.
- Connect, TLS, header, and total timeouts.
- Sanitized headers.
- Secret redaction.
- Audit events.

Cover IPv4, IPv6, mixed encodings, alternative numeric forms, DNS changes, redirects, and race conditions in tests.

## 7. Monitoring v2 architecture

Implement a telemetry-first hybrid architecture. The endpoint catalog and monitoring execution must be separate concepts.

### 7.1 Endpoint catalog

An endpoint catalog describes operations, ownership, environments, and mapping metadata. Catalog entries do not automatically generate requests.

- OpenAPI import populates the catalog.
- Imported operations default to no active monitoring.
- Users can map telemetry to route templates.
- Users can explicitly create selected synthetic checks.
- Show source, signal status, mapping confidence, last seen time, and data freshness.

### 7.2 OpenTelemetry ingestion

Add production-grade OpenTelemetry ingestion supporting OTLP over gRPC and HTTP as appropriate.

Required controls:

- Authenticated, scoped ingestion credentials.
- Project and environment attribution that cannot be forged across tenants.
- Credential rotation and revocation.
- Per-project quotas and rate limits.
- Payload and request-size limits.
- Attribute allowlists.
- Sensitive attribute and header redaction.
- Cardinality limits.
- Rejection metrics and clear diagnostics.
- Backpressure.
- Bounded queues.
- Retry with bounded exponential backoff.
- Persistent buffering or WAL where supported.
- Health, readiness, and self-observability for the pipeline.

Provide a secure collector configuration including at minimum:

- Memory limiting.
- Batching.
- Retry and queue controls.
- Authentication.
- Tenant attribution.
- Redaction/filtering.
- Resource normalization.
- Export health metrics.

Instrument the Argus Go/Fiber application with the current compatible OpenTelemetry SDK and middleware. Capture low-cardinality server metrics and traces using normalized route templates, never raw URLs, unbounded IDs, full query strings, secrets, or arbitrary user-controlled values as metric labels.

### 7.3 Metrics and telemetry storage

Select and implement a self-hosted Prometheus-compatible time-series backend appropriate for this repository’s operational scale and deployment model. Record the decision in an ADR.

The implementation must:

- Work in local Docker Compose.
- Have retention and resource settings.
- Provide health checks.
- Support backup/restore or durable-volume guidance.
- Expose query paths needed by the SLO engine and UI.
- Avoid storing high-volume time-series samples in the transactional MySQL route-check table.

Do not introduce Kafka or another major distributed dependency unless repository evidence proves it is necessary. Prefer the smallest production-worthy architecture that meets durability, throughput, tenant isolation, and operability requirements.

### 7.4 Signal mapping

Map telemetry to endpoint catalog entries using stable low-cardinality attributes:

- Service identity.
- Deployment environment.
- HTTP method.
- Framework route template.
- Optional explicit Argus endpoint identifier.

Implement mapping diagnostics, unmapped-signal views, conflicts, and safe manual overrides. Protect against cross-project mappings.

### 7.5 SLI and SLO engine

Implement an SLO model with versioned definitions and evaluation records.

Support at minimum:

- Availability/error-rate SLIs.
- Latency threshold or distribution-based SLIs.
- Configurable rolling windows.
- Error budgets.
- Multi-window, multi-burn-rate alerting.
- Maintenance windows.
- No-data and stale-data states.
- Low-traffic safeguards.
- Delayed or partial telemetry handling.
- Evaluation provenance.
- Audit history.

Do not silently treat missing telemetry as success. Distinguish healthy, unhealthy, no data, stale data, paused, maintenance, and configuration error states throughout the backend and UI.

### 7.6 Safe synthetic monitoring

Retain active checks only as explicit, selected canaries.

Defaults and protections:

- GET and HEAD are the safe defaults.
- TRACE is prohibited.
- State-changing methods are disabled by default.
- Unsafe checks require explicit authorization, prominent warnings, a documented use case, idempotency controls, test fixtures, and cleanup behavior.
- Never retry a non-idempotent request automatically.
- Apply project-level and global request budgets.
- Apply concurrency caps, jitter, minimum intervals, timeout ceilings, redirect limits, and response-size limits.
- Prevent synchronized bursts.
- Display request cost before activation.
- Pause or shed work safely under pressure.
- Record scheduler lag, skipped runs, queue depth, and outcome provenance.
- Use the hardened outbound transport for every check.

The scheduler must no longer scan and request every imported route. Separate Asynq queues by purpose and priority. Jobs must be idempotent and observable, with bounded retries and dead-letter handling.

### 7.7 Private targets

Do not let the central Argus service arbitrarily probe private customer addresses.

Implement a production-capable customer-side or environment-local agent model with:

- Scoped one-time enrollment.
- Short-lived or rotatable credentials.
- Outbound-only control or results communication where practical.
- Signed configuration.
- Project and environment binding.
- Least privilege.
- Restricted target policy.
- Upgrade and version visibility.
- Heartbeat and last-seen status.
- Revocation.
- Audit trail.
- Replay protection.
- Bounded local concurrency and resource use.
- Clear offline and stale-agent states.

If a fully packaged agent is too large for one component, deliver a minimal production-capable agent, protocol, packaging, integration tests, operational documentation, and explicit extension boundaries. Do not substitute an insecure server-side private-network dialer.

### 7.8 Heartbeats

Implement project-scoped heartbeat monitors for jobs and scheduled workloads:

- Secure generated endpoint or token.
- Rotatable and revocable secrets.
- Expected interval and grace period.
- Last-seen state.
- Late and missing states.
- Optional run metadata with strict size and privacy limits.
- Idempotency and replay considerations.
- Examples for common CLI and application clients.

### 7.9 Incidents and notifications

Create incidents from SLO, synthetic, heartbeat, agent, and pipeline signals through a source-aware, idempotent incident model.

- Deduplicate repeated evaluations.
- Preserve source and evidence.
- Track state transitions.
- Support acknowledgement and resolution.
- Use a transactional outbox for notifications and external integrations.
- Make notification delivery idempotent, retryable, observable, and tenant-scoped.
- Record failures without losing the underlying incident.

### 7.10 Monitor Argus itself

Expose operational signals for:

- OTLP acceptance and rejection.
- Queue depth and retry.
- WAL or buffer utilization.
- Exporter failures.
- Signal mapping coverage.
- Last-seen and staleness.
- SLO evaluation lag.
- Synthetic scheduler lag.
- Request budget use.
- Agent last seen and version.
- Notification outbox backlog.
- Database, cache, worker, and time-series backend health.

Provide useful dashboards, alert rules, and runbooks for these failure modes.

## 8. UX and visual design

Perform the implementation as a complete product flow, not a collection of disconnected screens.

### 8.1 Core journeys

Implement and test at least:

- New visitor → register → create project → select OpenTelemetry → connect → verify → create starter SLO → reach dashboard.
- Returning user → log in → return to intended project.
- Authenticated user → import OpenAPI → review catalog → observe zero generated requests → map telemetry.
- Authenticated user → create a safe synthetic canary → review canonical target and request budget → activate.
- Authenticated user → create a heartbeat → send first heartbeat → observe status.
- User without permission → receive a clear, accessible authorization response with no data leak.
- User with stale or missing telemetry → understand the state and corrective action.
- User with a canonicalization conflict → resolve it without losing data.
- Operator → diagnose an ingestion, queue, exporter, agent, or notification failure.

### 8.2 Interface principles

- Preserve valuable existing visual identity while making information architecture coherent.
- Use progressive disclosure.
- Prefer clear labels over unexplained icons.
- Keep primary actions visually distinct.
- Show cause, effect, safety, and request cost before activation.
- Make empty, loading, stale, error, partial, maintenance, paused, and unauthorized states explicit.
- Keep destructive actions separated and confirm material consequences.
- Avoid framework migration unless measured evidence shows the current frontend cannot safely support the target.
- Use efficient server-sent events, WebSockets, or bounded adaptive polling for live updates. Pause or reduce work in hidden tabs and give users control where appropriate.
- Do not use continuous broad client polling as a substitute for a coherent update model.

### 8.3 Required views

Implement coherent views for:

- Public landing and status surfaces.
- Registration, login, sessions, and account security.
- Project list, project creation, switching, and settings.
- Environment management.
- Source setup and connection verification.
- Endpoint catalog and endpoint detail.
- Telemetry mapping and unmapped signals.
- Synthetic checks and budgets.
- Heartbeats.
- SLOs and error budgets.
- Incidents and evidence.
- Integrations and delivery health.
- Private agents.
- Audit log.
- Operational health.

Use responsive layouts and retain understandable hierarchy from mobile through wide desktop.

## 9. Accessibility and motion

Meet WCAG 2.2 AA across all affected flows.

At minimum:

- Use native semantic elements and controls.
- Ensure full keyboard operation with visible focus.
- Provide a skip link and logical heading/landmark structure.
- Use real buttons for sortable headers and expose `aria-sort`.
- Use accessible names, descriptions, field instructions, autocomplete values, validation summaries, `aria-invalid`, and `aria-describedby`.
- Move focus to the first relevant invalid field after failed submission.
- Use narrow, intentional live regions; do not announce entire dashboards repeatedly.
- Ensure dialogs have labeling, focus containment, background inertness, Escape behavior where safe, and focus restoration.
- Ensure status is never conveyed by color alone.
- Meet contrast requirements, including focus indicators and non-text UI.
- Provide at least WCAG minimum target sizes and prefer comfortable 44-by-44 CSS-pixel targets for primary touch controls.
- Support 320 CSS-pixel width, 200% zoom, reflow, text spacing, screen readers, keyboard-only use, touch, high contrast, and forced-colors mode.
- Provide data-table or textual alternatives for canvas-only charts.
- Make timestamps understandable and expose exact values where relative time is used.
- Avoid drag-only interactions.

Implement every plan in `animation-plans/`:

- Remove perpetual decorative brand looping.
- Make frequent tab navigation immediate.
- Replace blanket animation disabling with semantic reduced-motion behavior.
- Make refresh animation state-driven and stop it when work completes.
- Gate hover behavior to fine pointers and preserve interruptible, understandable toast feedback.

Motion must be causal, restrained, interruptible, and safe under `prefers-reduced-motion`. Do not delay high-frequency navigation for decorative transitions. Test animation state cleanup and focus behavior.

## 10. Security requirements

Resolve every critical and high-severity finding in:

- `docs/audit-2026-07-28-en/SECURITY_REVIEW.md`
- `security_best_practices_report.md`
- `Argus-threat-model.md`

Resolve medium findings that are touched by the implementation and all feasible remaining findings required for production readiness. If an item genuinely cannot be resolved in code, document the specific residual risk, compensating control, owner, and due condition. Do not silently accept risk.

### 10.1 Browser and session security

- Replace browser-stored management credentials with server-managed opaque sessions in `HttpOnly`, `Secure`, appropriately scoped cookies.
- Apply an explicit `SameSite` policy.
- Add CSRF protection to state-changing browser requests.
- Rotate session identifiers at authentication and privilege changes.
- Implement idle and absolute expiry.
- Store session tokens only as secure hashes where applicable.
- Support revocation, logout, and revoke-other-sessions.
- Coalesce last-used writes rather than writing on every request.
- Never store API keys, bearer tokens, passwords, or session secrets in `localStorage` or `sessionStorage`.
- Remove or safely migrate the legacy browser API-key flow.
- Use scoped, hashed, rotatable automation tokens for non-browser access.

### 10.2 Authentication abuse controls

- Rate-limit registration, login, recovery, verification, token creation, and other sensitive endpoints.
- Use generic authentication errors.
- Enforce a current password policy without arbitrary composition rules.
- Use a modern supported password hash with calibrated parameters.
- Prevent account enumeration.
- Add audit events for sensitive identity and authorization actions.
- Define trusted proxy behavior before relying on forwarded client addresses.

### 10.3 Authorization and tenancy

- Require authentication for every management endpoint.
- Fail secure when production secrets or identity configuration are absent.
- Enforce project/tenant scope at the data-access boundary.
- Add negative cross-tenant tests for reads, writes, jobs, streams, ingestion, exports, mappings, incidents, and tokens.
- Do not rely on hidden UI for authorization.

### 10.4 Input, output, browser, and secret safety

- Remove inline event handlers and executable HTML string interpolation.
- Use DOM APIs or framework escaping for untrusted content.
- Establish a strict Content Security Policy without relying on unsafe inline script execution.
- Configure HSTS where TLS termination and deployment architecture permit it.
- Add appropriate security headers.
- Use endpoint-specific body-size limits.
- Configure explicit HTTP server read, header, write, and idle timeouts.
- Parse and validate headers strictly.
- Encrypt stored endpoint and integration secrets with an explicit key-management boundary, or store external secret references.
- Redact secrets from logs, traces, metrics, UI, errors, exports, and audit payloads.
- Pin and scan dependencies and container images.

Update the threat model to describe the as-built v2 architecture and mitigations, especially for telemetry ingestion, private agents, URL handling, synthetics, sessions, tenant isolation, and notification integrations.

## 11. Database, migrations, and compatibility

Use additive, ordered, reversible database migrations suitable for both fresh installation and upgrade from the current repository state.

Model the required v2 concepts, including:

- Accounts and memberships.
- Sessions and scoped automation tokens.
- Projects and environments.
- Endpoint catalog and canonical identities.
- Encrypted secret references.
- Synthetic definitions and execution metadata.
- Telemetry credentials and mappings.
- SLO definitions, versions, and evaluations.
- Heartbeats.
- Private agents and enrollment.
- Incidents and evidence.
- Notification outbox.
- Audit events.
- Migration conflicts and backfill checkpoints.

Requirements:

- Preserve existing user and project data.
- Do not silently delete legacy routes or checks.
- Provide deterministic backfills.
- Make backfills restartable and observable.
- Use dual-read or dual-write only where needed and define the cutover condition.
- Provide rollback steps and known limitations for each release phase.
- Run migrations successfully on a fresh database and on a representative legacy snapshot or fixture.
- Keep backward compatibility until the documented cutover.
- Stop writing high-volume monitoring samples to transactional tables after cutover.
- Retire obsolete paths only after parity and migration evidence exist.

## 12. Workers, reliability, and operations

Retain Asynq where it remains appropriate for control-plane background work, such as:

- Notifications.
- Explicit synthetic checks.
- Backfills.
- Cleanup.
- Scheduled SLO evaluation if the selected architecture requires it.
- Integration delivery.

Separate queues by workload and priority. Add:

- Idempotency.
- Timeouts.
- Bounded retry.
- Dead-letter behavior.
- Queue metrics.
- Trace propagation.
- Tenant/project context.
- Graceful shutdown.
- Overload behavior.

The v2 path must stop scanning and probing every endpoint after migration.

Provide a complete local and test deployment in Docker Compose containing the services required by the selected architecture, with:

- Health checks.
- Readiness dependencies.
- Persistent volumes where needed.
- Resource guidance.
- Non-secret environment examples.
- Secure defaults.
- Bootstrap instructions.
- Upgrade instructions.
- Backup and restore instructions.
- Troubleshooting guidance.

Add runbooks for ingestion outage, exporter failure, full queue/WAL, high cardinality, SLO evaluation lag, synthetic overload, private agent offline, notification backlog, database migration failure, and rollback.

## 13. Implementation sequence

Use a risk-first incremental sequence, while continuing through all phases:

### Phase 0 — Baseline and delivery controls

- Establish the branch, baseline, traceability matrix, architecture decisions, feature flags, and test harnesses.

### Phase 1 — Identity and global shell

- Secure session model, authentication lifecycle, authorization, global header, dedicated identity pages, route protection, project scoping, and guest/authenticated UI separation.

### Phase 2 — Canonical resource model

- Environments, centralized URL normalization, preview API, endpoint catalog, canonical hashing, SSRF-safe fetch construction, legacy dual-write/backfill, and conflict handling.

### Phase 3 — Telemetry foundation

- Application instrumentation, OTLP ingestion, collector configuration, time-series storage, tenant-safe signal mapping, pipeline self-monitoring, and local deployment.

### Phase 4 — Monitoring products

- SLO engine, safe selected synthetics, heartbeats, private agent, incidents, outbox, dashboards, and operational alerts.

### Phase 5 — Complete product experience

- Authenticated onboarding, source selection, catalog, mapping, SLO, synthetic, heartbeat, incident, agent, audit, account, and operations UI.
- Complete responsive behavior, accessibility, and motion plans.

### Phase 6 — Migration and compatibility

- Backfill legacy data, run dual paths where required, measure parity, migrate representative deployments, and validate rollback.

### Phase 7 — Cutover and hardening

- Disable route-wide probing, remove obsolete credential paths after migration, complete security remediation, load and failure testing, documentation, and evidence audit.

Do not call a phase complete until its acceptance tests pass. Keep a documented fallback and rollback boundary during migration.

## 14. Required test and verification program

Use the repository’s actual commands and CI configuration. Add missing test infrastructure where necessary.

### 14.1 Backend quality gates

Run and pass, as applicable:

- Formatting checks such as `gofmt`.
- `go test ./...`.
- Race tests with `go test -race ./...`.
- `go vet ./...`.
- `staticcheck ./...`.
- `govulncheck ./...`.
- Migration tests.
- Repository lint and generated-file checks.

Do not ignore failures. Fix regressions. If an unrelated baseline failure remains, prove it existed at the starting commit and report it precisely.

### 14.2 Frontend quality gates

Run and pass all applicable:

- Unit and component tests.
- Lint.
- Type checks if a typed frontend is introduced or already present.
- Production build.
- Accessibility automation.
- Browser end-to-end tests.

Do not introduce a large frontend framework solely to obtain testing support.

### 14.3 End-to-end browser coverage

Use Playwright or the repository’s supported real-browser runner. Cover:

- Registration, login, logout, session expiry, and return-to behavior.
- Guest and authenticated header states.
- No private controls in guest state.
- Project onboarding.
- Source selection and signal verification.
- OpenAPI import with proof of zero outbound operation requests.
- URL normalization preview and validation.
- Canonical conflict handling.
- Telemetry mapping.
- Starter SLO creation.
- Safe synthetic creation and visible request budget.
- Heartbeat creation and first signal.
- Role restrictions and cross-project navigation.
- Keyboard navigation and focus restoration.
- Dialog behavior.
- Sortable tables.
- Mobile layout.
- 200% zoom and narrow reflow.
- Reduced motion.
- No browser storage of credentials.

### 14.4 Integration coverage

Bring up the full local stack and test:

- Fresh database migrations.
- Upgrade migrations from representative legacy data.
- Rollback boundaries.
- OTLP HTTP and gRPC ingestion.
- Invalid and revoked ingestion credentials.
- Tenant isolation.
- Cardinality and quota enforcement.
- Collector retry and persistent buffering during storage outage.
- Mapping and unmapped signals.
- SLO evaluation and burn-rate alerts.
- No-data, stale, maintenance, and low-traffic behavior.
- Synthetic scheduling, jitter, budgets, skips, and overload.
- Heartbeat late/missing transitions.
- Private-agent enrollment, revocation, replay rejection, and offline state.
- Idempotent incident and outbox behavior.
- Notification retries.
- Service health and graceful shutdown.

### 14.5 Security and robustness coverage

Add:

- URL canonicalization unit tables and fuzz tests.
- OpenAPI parser/import fuzz tests.
- SSRF tests for address classes, encodings, DNS changes, rebinding, and redirect hops.
- XSS regression tests.
- CSRF tests.
- Session fixation and revocation tests.
- Authentication rate-limit tests.
- Cross-tenant negative tests.
- Secret-redaction tests.
- Oversized-body tests.
- HTTP timeout tests.
- Worker idempotency and concurrency tests.
- Race tests for schedulers, session touches, mappings, and outbox processing.

### 14.6 Performance and resilience

Measure and document:

- Request reduction relative to the legacy route-wide polling model.
- OTLP ingestion throughput and resource use at a representative load.
- Cardinality protection behavior.
- Queue and buffer recovery after downstream outage.
- SLO evaluation latency.
- Synthetic scheduler fairness and lag.
- Database load before and after high-volume sample removal.
- UI load and update behavior for large endpoint catalogs.

The OpenAPI import path must produce zero synthetic operation requests. The default v2 architecture should reduce active monitoring requests by at least 90% for representative imported APIs unless a documented workload makes that threshold inapplicable. No unsafe method may be probed by default.

## 15. Documentation deliverables

Update the repository documentation to match the completed implementation. Include:

- Argus v2 architecture and component diagram.
- Data-flow and trust-boundary diagram.
- User journeys and updated wireframes or implemented-screen references.
- Authentication, session, and authorization model.
- Canonical URL and endpoint contract with examples.
- Monitoring source decision guide.
- OTLP connection guide.
- Collector configuration guide.
- SLO and alerting guide.
- Synthetic safety and request-budget guide.
- Heartbeat guide.
- Private-agent installation, enrollment, upgrade, and revocation guide.
- API documentation.
- Database migration and rollback guide.
- Operations and troubleshooting runbooks.
- Backup and restore guide.
- Security model and updated threat model.
- Accessibility statement and verification record.
- Final implementation and evidence matrix.
- Release notes and upgrade guide from v1.

Remove or clearly mark obsolete documentation. Do not leave planning language that falsely describes completed behavior as future work. Preserve useful decision history, but distinguish proposals from the as-built system.

## 16. Definition of Done

Argus v2 is complete only when all of the following are true:

- Every explicit requirement in this prompt is implemented or has a documented, evidence-backed external blocker.
- Registration and login are global top-level flows, not embedded in project creation.
- Guests cannot create or manage private resources.
- Authenticated users can access every feature allowed by their role.
- Every management API is authenticated and tenant-scoped.
- Browser credentials are no longer stored in Web Storage.
- Central backend normalization handles every URL and route ingestion path.
- Canonical identity, fetch target, validation errors, and preview behavior are tested.
- SSRF protections operate independently from normalization.
- OpenAPI import makes zero operation requests and enables no synthetic check by default.
- OpenTelemetry is the primary monitoring signal path.
- SLOs, no-data handling, safe synthetics, heartbeats, incidents, and pipeline health are functional.
- The scheduler no longer broadly probes every catalog endpoint.
- Private targets use the secure customer-side agent model.
- All critical and high security findings are resolved.
- WCAG 2.2 AA requirements and all animation plans are implemented and verified.
- Fresh install, legacy upgrade, and rollback boundaries are tested.
- Unit, integration, race, static, vulnerability, browser, security, and performance gates pass.
- Documentation describes the actual implementation.
- No required path is a mock, dead UI, placeholder, untracked TODO, or plan-only artifact.
- The traceability matrix contains implementation and test evidence for every requirement.
- The worktree contains only intentional changes and is clean after the final commit.
- The implementation branch is pushed and its remote commit is verified.

## 17. Prohibited shortcuts

Do not:

- Deliver only a report, plan, scaffold, or UI mockup.
- Hide unavailable features behind non-functional controls.
- Treat CSS visibility as authorization.
- Store credentials in browser Web Storage.
- Use normalization as a substitute for SSRF protection.
- Build or compare URLs with unsafe string concatenation.
- Put raw URLs, arbitrary IDs, query strings, or user input in metric labels.
- Automatically probe imported endpoints.
- Enable mutating synthetic requests by default.
- Retry non-idempotent requests automatically.
- Treat missing telemetry as healthy.
- Use unbounded queues, retries, concurrency, cardinality, bodies, redirects, or timeouts.
- Put high-volume time-series data in the transactional route-check table.
- Add major infrastructure without evidence and an ADR.
- Delete or merge legacy data silently.
- Skip tenant-isolation tests.
- weaken security to preserve obsolete behavior.
- Claim a test passed without running it.
- Claim completion while required TODOs, placeholders, disabled tests, or unexplained failures remain.
- Commit secrets, generated credentials, local environment files, database dumps, or sensitive logs.
- Force-push or rewrite shared history.
- Deploy production without explicit authorization.

## 18. Git and pull-request delivery

When implementation and verification are complete:

1. Review the full diff and confirm every change belongs to Argus v2.
2. Ensure generated artifacts are intentional and no secrets are present.
3. Run the complete final verification suite from a clean state.
4. Update the traceability and completion evidence.
5. Create logical English commits using descriptive messages.
6. Confirm the worktree is clean.
7. Push the implementation branch and configure its upstream.
8. Verify that the remote branch SHA exactly matches the local final commit.
9. Open a draft pull request when GitHub tooling or the connected repository integration is available.
10. Include in the pull request:
    - Product outcome.
    - Architecture summary.
    - Security changes.
    - Database migrations.
    - Compatibility and rollout plan.
    - Rollback plan.
    - Exact test commands and results.
    - Performance evidence.
    - Screenshots or recordings of important responsive flows where practical.
    - Known residual risks or external blockers.
    - Documentation links.

Do not merge the pull request unless the user explicitly asks you to merge it or repository policy clearly delegates that action.

## 19. Final response contract

Your final response must be concise but evidence-based and must include:

- The completed outcome.
- Branch name.
- Final local and verified remote commit SHA.
- Draft pull-request link, if created.
- Major product and architecture changes.
- Security findings resolved.
- Migration and rollback status.
- Exact validation commands run and their results.
- Request-reduction and relevant performance results.
- Accessibility and browser coverage.
- Key documentation paths.
- Any genuine residual blocker, with impact and the exact action needed from the user.

Do not describe unfinished work as complete. If no blocker remains, state that Argus v2 satisfies the Definition of Done and is ready for review and controlled rollout.

Begin now by reading the repository instructions and authoritative documents, inspecting the baseline, creating the implementation branch, and then continue through implementation, validation, documentation, commit, push, and draft pull-request delivery.

