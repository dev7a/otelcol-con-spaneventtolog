# Span Event to Log Connector

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [alpha]               |
| Supported pipeline types | See below             |
| Distributions            | [contrib]             |

The span event to log connector extracts events from spans and converts them into log records. This enables unified telemetry by bridging trace events with log processing pipelines.

## Supported Pipeline Types

| [Exporter Pipeline Type] | [Receiver Pipeline Type] | [Stability Level] |
| ------------------------ | ------------------------ | ----------------- |
| traces                   | logs                     | [alpha]           |

[Exporter Pipeline Type]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#exporter-pipeline-type
[Receiver Pipeline Type]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#receiver-pipeline-type
[Stability Level]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#stability-levels
[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

## Configuration

The following settings are available:

- `include_event_names` (optional): The list of event names to include in the conversion from events to logs. If empty, all events will be included.
- `include_span_context` (optional, default: `true`): If true, span context (TraceID, SpanID, TraceFlags) will be included in the log records.
- `log_attributes_from` (optional, default: `["event.attributes"]`): The list of attribute sources to include in the log record. Valid values:
  - `event.attributes`: includes all attributes from the span event
  - `span.attributes`: includes all attributes from the parent span
  - `resource.attributes`: includes all resource attributes
- `severity_by_event_name` (optional): A mapping from event name to severity level. If the event name is not in the map, the default severity level (Info) will be used.

### Example Configuration

```yaml
connectors:
  span_event_to_log:
    include_event_names: ["exception", "retry", "db.query"]
    include_span_context: true
    log_attributes_from: ["event.attributes", "span.attributes"]
    severity_by_event_name:
      exception: error
      retry: warning
      db.query: info

receivers:
  otlp:
    protocols:
      grpc:

exporters:
  loki:
    endpoint: http://your-loki-endpoint:3100

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [span_event_to_log]
    logs:
      receivers: [span_event_to_log]
      exporters: [loki]
```

## Use Cases

### Exception Tracking

When instrumenting applications, exception events are often added to spans. This connector can automatically convert these events to error logs for better visibility in your logging system.

### Database Query Monitoring

For spans that include database query events, you can convert these to logs to retain the SQL queries for later analysis.

### Custom Event Processing

Any custom event added to spans can be selectively converted to logs and assigned appropriate severity levels.

[Connectors README]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md