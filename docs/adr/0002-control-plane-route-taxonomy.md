# ADR 0002: Control-plane route taxonomy

## Status

Accepted — 2026-07-28.

## Decision

Argus control-plane endpoints use this shape:

```
/family/purpose[/optional]
```

The first segment identifies the bounded capability family (`identity`,
`project`, `route`, `telemetry`, `monitor`, `system`, `notification`,
`status`, `import`, `environment`, or `slo`). The second segment identifies
the operation or resource purpose. Remaining segments carry the scoped ID or
explicit action only when needed. The old `/api/*` prefix is removed and is
not mounted as a compatibility alias.

OTLP remains at `/v1/metrics` and `/v1/traces`, because those are standard
protocol paths rather than Argus control-plane routes.

## Consequences

- Route templates shown in telemetry, logs, and documentation describe the
  capability directly instead of the generic `api` prefix.
- Browser clients and automation must migrate to the documented taxonomy.
- There is no silent fallback to the old paths, making stale integrations
  visible instead of masking an incomplete migration.
