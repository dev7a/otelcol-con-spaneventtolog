# Span Events to Logs Connector

An OpenTelemetry Collector connector component that extracts span events and converts them into log records.  
This project enables unified telemetry ingestion by bridging trace events with log processing pipelines using only the tracing APIs.

## Features

- Converts span events to structured log records
- Flexible configuration for filtering, severity mapping, and context enrichment
- Designed for use as an external collector component

## Motivation: Bridging Traces and Logs for Simplified Instrumentation

OpenTelemetry defines distinct signals for traces, metrics, and logs, each serving a crucial role in observability. However, developers often find themselves needing to record significant, contextual events *during* a traced operation – information that traditionally might be sent via a separate logging API. Manually correlating these logs with the corresponding trace ID can sometimes be cumbersome.

This connector offers a pragmatic approach to streamline this process for certain use cases. By leveraging the `AddEvent` function within the OpenTelemetry Tracing API, developers can record structured events directly onto spans. The `spaneventtologconnector` then automatically transforms these span events into standard OpenTelemetry Log records.

The primary benefits of this approach include:

*   **Automatic Context:** Log records generated from span events automatically inherit the `TraceID` and `SpanID`, ensuring they are perfectly correlated with the trace without manual effort.
*   **Simplified Developer Experience:** For events occurring within a traced operation, developers can focus on using the Tracing API, potentially reducing the need to switch between tracing and logging APIs for instrumentation within the same code block.
*   **Inherited Sampling:** Logs derived from span events automatically adhere to the sampling decisions applied to their parent trace. This is particularly beneficial when using head or tail-based sampling strategies, ensuring that logs associated with sampled (e.g., interesting or erroneous) traces are retained along with the trace context.
*   **Unified Telemetry Pipeline:** Applications emit traces enriched with events, and the collector handles the transformation, allowing downstream processing using standard log exporters and analysis tools.

This connector acts as a bridge, enabling a workflow where the rich context of traces can be seamlessly combined with the need for structured, event-based logging, offering a convenient instrumentation strategy for developers working within traced code paths. It complements the standard OpenTelemetry logging API, which remains essential for logs generated outside the context of a specific trace.

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
- `severity_attribute` (optional, default: `\"\"`): The name of the event attribute to use for determining the severity level.
  - The attribute value must be a string representing a severity level (e.g., `INFO`, `warn`, `Error`), case-insensitive.
  - If this attribute is set on an event and contains a valid severity string, it takes **precedence** over `severity_by_event_name`.
  - If empty, not present on the event, or invalid, the connector falls back to other methods.
- `severity_by_event_name` (optional): A mapping from **event name substring** to severity level (e.g., `trace`, `debug`, `info`, `warn`, `error`, `fatal`).
  - Matching is case-insensitive.
  - If an event name contains multiple configured substrings (e.g., config has `error: error` and `connection error: fatal`, event name is `database connection error`), the **longest matching substring** takes precedence (`connection error` in the example).
  - This mapping is applied only if `severity_attribute` is not configured or does not yield a valid severity.
  - If no match is found via attribute or substring, the default severity level (Info) will be used.
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
      database: debug
      retry: warning
    severity_attribute: "log.level"
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

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.