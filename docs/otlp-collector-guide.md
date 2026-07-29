# OpenTelemetry Collector connection guide

Argus accepts OTLP metrics and traces. Create a project/environment-scoped
credential in **Telemetry signals** and treat its one-time value as a secret.
The credential—not resource attributes—selects the Argus project and
environment.

## Choose a transport

Use OTLP/HTTP when the Argus HTTPS origin is available. It is the simplest
deployment because the Collector exporter appends `/v1/metrics` and
`/v1/traces` itself. Use OTLP/gRPC only when the operator has explicitly set
`OTLP_GRPC_ADDR`, terminated TLS, and restricted network access to that
dedicated listener. Both transports accept metrics and traces, use the same
credential scope and quota, and do not accept logs as an Argus signal.

Never place the credential directly in a committed Collector YAML file. Inject
it from the deployment secret store as an environment variable.

## OTLP/HTTP example

This example uses the standard Collector component names in current releases.
Keep the `endpoint` as the Argus origin: the `otlphttp` exporter adds the
signal-specific `/v1/*` paths.

```yaml
receivers:
  otlp:
    protocols:
      grpc: {}
      http: {}

processors:
  memory_limiter:
    check_interval: 1s
    limit_mib: 256
  batch: {}

exporters:
  otlphttp/argus:
    endpoint: https://argus.example.com
    headers:
      Authorization: Bearer ${env:ARGUS_OTLP_TOKEN}
    retry_on_failure:
      enabled: true
    sending_queue:
      enabled: true

service:
  pipelines:
    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp/argus]
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlphttp/argus]
```

## OTLP/gRPC example

Enable this only after the Argus operator has published a TLS-protected gRPC
listener. `endpoint` is host and port, without an HTTP path. Configure the
appropriate CA/certificate settings for your deployment; do not use insecure
transport for a network-reachable listener.

```yaml
exporters:
  otlp/argus:
    endpoint: argus-otlp.example.com:4317
    headers:
      Authorization: Bearer ${env:ARGUS_OTLP_TOKEN}
    tls:
      insecure: false
    retry_on_failure:
      enabled: true
    sending_queue:
      enabled: true
```

Add `otlp/argus` to the metrics and traces pipelines shown above. Component
names can differ in older Collector distributions; use the Collector version's
component inventory and keep the same endpoint, Authorization header, TLS, and
queue semantics.

## Verify safely

1. Export one metric or trace from the service.
2. Open the project’s **Telemetry signals** card and confirm a new bounded
   ingestion record for the expected service and deployment environment.
3. Map that service identity to a catalog route only after reviewing it.

Do not rely on `argus.project.id`, `argus.environment.id`, URLs, span names, or
other resource attributes for attribution: Argus intentionally ignores them.
It retains only bounded service/deployment diagnostics.

## Troubleshooting

- `401` or gRPC `Unauthenticated`: the bearer credential is missing, invalid,
  expired, or revoked. Issue a new credential; never reuse a displayed secret.
- `403` or gRPC `PermissionDenied`: the credential lacks the signal scope.
- `429` or gRPC `ResourceExhausted`: reduce export volume or raise the
  credential’s configured per-minute rate deliberately.
- gRPC connection failure: confirm `OTLP_GRPC_ADDR` is enabled, TLS reaches
  the listener, and network policy permits the Collector.
- No telemetry signal record: verify the Collector metrics/traces pipeline,
  then inspect its exporter retry/queue telemetry before rotating credentials.
