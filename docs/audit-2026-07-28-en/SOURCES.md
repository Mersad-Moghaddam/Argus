# Research and Standards Register

Accessed 2026-07-28. Primary or official sources were preferred.

## Product and accessibility

- [WCAG 2.2](https://www.w3.org/TR/WCAG22/) — AA target, including Focus Not
  Obscured, Target Size (Minimum), Accessible Authentication, and control over
  auto-updating content.
- [Vercel Web Interface Guidelines](https://raw.githubusercontent.com/vercel-labs/web-interface-guidelines/main/command.md)
  — current implementation checklist for forms, focus, controls, URL state,
  reduced motion, and responsive interaction.
- [The 2020 Scrum Guide](https://scrumguides.org/scrum-guide.html) — Product
  Goal, Product Backlog, Sprint, Increment, inspection, and adaptation.

## Monitoring and reliability

- [OpenTelemetry signals](https://opentelemetry.io/docs/concepts/signals/)
- [OpenTelemetry metrics](https://opentelemetry.io/docs/concepts/signals/metrics/)
- [OpenTelemetry specification](https://opentelemetry.io/docs/specs/otel/)
- [OpenTelemetry semantic conventions](https://opentelemetry.io/docs/specs/semconv/)
- [OpenTelemetry HTTP metrics](https://opentelemetry.io/docs/specs/semconv/http/http-metrics/)
- [OpenTelemetry Metrics SDK](https://opentelemetry.io/docs/specs/otel/metrics/sdk/)
- [OpenTelemetry Collector configuration](https://opentelemetry.io/docs/collector/configuration/)
- [OpenTelemetry Collector resiliency](https://opentelemetry.io/docs/collector/resiliency/)
- [Prometheus instrumentation practices](https://prometheus.io/docs/practices/instrumentation/)
- [Prometheus alerting practices](https://prometheus.io/docs/practices/alerting/)
- [Prometheus metric naming](https://prometheus.io/docs/practices/naming/)
- [Prometheus Blackbox Exporter](https://github.com/prometheus/blackbox_exporter)
- [Google SRE: Monitoring Distributed Systems](https://sre.google/sre-book/monitoring-distributed-systems/)
- [Google SRE Workbook: Alerting on SLOs](https://sre.google/workbook/alerting-on-slos/)

## URL and security

- [RFC 3986: URI Generic Syntax](https://www.rfc-editor.org/rfc/rfc3986)
- [OWASP SSRF overview](https://owasp.org/www-community/attacks/Server_Side_Request_Forgery)
- [OWASP SSRF Prevention Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Server_Side_Request_Forgery_Prevention_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)

## Current library documentation

- [Fiber v2 OpenTelemetry middleware](https://pkg.go.dev/github.com/gofiber/contrib/otelfiber/v2)
- [Asynq periodic tasks](https://github.com/hibiken/asynq/wiki/Periodic-Tasks)
- [Asynq unique tasks](https://github.com/hibiken/asynq/wiki/Unique-Tasks)
- [Asynq queue priority](https://github.com/hibiken/asynq/wiki/Queue-Priority)

The Context7 documentation connector was used to confirm the current
OpenTelemetry Go provider/view pattern, Fiber v2 middleware options, and Asynq
scheduler, uniqueness, retry, priority, and shutdown behavior.
