# Span Events to Logs Connector

An OpenTelemetry Collector connector component that extracts span events and converts them into log records.  
This project enables unified telemetry ingestion by bridging trace events with log processing pipelines using only the tracing APIs.

## Features

- Converts span events to structured log records
- Flexible configuration for filtering, severity mapping, and context enrichment
- Designed for use as an external collector component

## Status

| Status                   |                       |
| ------------------------ | --------------------- |
| Stability                | [alpha]               |
| Supported pipeline types | See below             |
| Distributions            | [contrib]             |

[alpha]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#alpha
[contrib]: https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

## Supported Pipeline Types

| [Exporter Pipeline Type] | [Receiver Pipeline Type] | [Stability Level] |
| ------------------------ | ------------------------ | ----------------- |
| traces                   | logs                     | [alpha]           |

[Exporter Pipeline Type]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#exporter-pipeline-type
[Receiver Pipeline Type]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/connector/README.md#receiver-pipeline-type
[Stability Level]: https://github.com/open-telemetry/opentelemetry-collector/blob/main/docs/component-stability.md#stability-levels

## Configuration

The following settings are available:

- `include_event_names` (optional): The list of event names to include in the conversion from events to logs. If empty, all events will be included.
- `include_span_context` (optional, default: `true`): If true, span context (TraceID, SpanID, TraceFlags) will be included in the log records.
- `log_attributes_from` (optional, default: `["event.attributes", "resource.attributes"]`): The list of attribute sources to include in the log record. Valid values:
  - `event.attributes`: includes all attributes from the span event
  - `span.attributes`: includes all attributes from the parent span
  - `resource.attributes`: includes all resource attributes
- `severity_by_event_name` (optional): A mapping from event name to severity level. If the event name is not in the map, the default severity level (Info) will be used.
- `add_level` (optional, default: `false`): If true, adds a "level" attribute to the log record based on the severity text. This is useful for log systems that expect a "level" field instead of severity. If the event attributes already contain a "level" field, it will not be overwritten.

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
    add_level: true  # Add level attribute based on severity text

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

## Development

### Metadata Generation

This component uses the OpenTelemetry Collector's metadata generator (mdatagen) to generate metadata files. To regenerate these files:

1. **Install the mdatagen tool with the correct version**:
   ```bash
   go get go.opentelemetry.io/collector/cmd/mdatagen@v0.119.0
   ```

2. **Run the generator**:
   ```bash
   go run go.opentelemetry.io/collector/cmd/mdatagen ./metadata.yaml
   ```

   Or use the Makefile:
   ```bash
   make generate
   ```

> **Important**: Make sure to use mdatagen version v0.119.0 to match the OpenTelemetry Collector version used by this component. Using a different version may cause compatibility issues.

The generated files include:
- `internal/metadata/generated_status.go`: Component type and stability constants
- `internal/metadata/generated_config.go`: Resource attribute configurations
- `internal/metadata/generated_resource.go`: Resource attribute handling
- `generated_component_test.go`: Component lifecycle tests
- `generated_package_test.go`: Package-level tests
- `documentation.md`: Auto-generated documentation

### Running Tests

To run the tests:

```bash
make test
```

This will run all tests, including the generated component tests.

## License

Apache 2.0
