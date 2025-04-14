# Span Event to Log Connector Implementation Plan

## Overview
This plan outlines the steps to implement a new connector for the OpenTelemetry Collector that extracts span events and converts them into log records. The connector will enable unified telemetry ingestion by bridging trace events with log processing pipelines.

## Goals
- **Unified Telemetry:** Convert span events to structured log records.
- **Flexible Configuration:** Allow filtering, severity mapping, and context enrichment.
- **Pipeline Decoupling:** Maintain clear separation between trace and log pipelines.
- **Performance:** Ensure efficient processing in high-throughput environments.

## Implementation Steps

### 1. Requirements & Design
- **Gather Use Cases:**
  - Identify common event types (e.g., `exception`, `retry`, `db.query`) for conversion.
  - Determine necessary span context fields (e.g., trace ID, span ID) to include.
- **Define Configuration Options:**
  - `include_event_names`: List of event names to filter.
  - `include_span_context`: Boolean flag to include span context in logs.
  - `log_attributes_from`: Fields to be mapped from event and span attributes.
  - `severity_by_event_name`: Map event names to log severity levels.
  - `add_level`: Boolean flag to add a "level" attribute based on severity text.
- **Design Considerations:**
  - Ensure minimal performance overhead.
  - Decide on batching or rate-limiting if necessary.
  - Architect the connector to clearly separate trace ingestion from log emission.

### 2. Familiarize with Collector Architecture
- Review the connector helper framework in the OpenTelemetry Collector.
- Study existing connectors and transformation components (especially those related to metrics) for design inspiration.
- Understand how trace and log pipelines are structured and where the connector fits in.

### 3. Set Up the Development Environment
- **Repository Preparation:**
  - Fork/clone the [opentelemetry-collector-contrib](https://github.com/open-telemetry/opentelemetry-collector-contrib) repository.
- **Project Skeleton:**
  - Create a new package/module, e.g., `connector/spaneventtolog`.
  - Set up local build and test configurations.

### 4. Connector Implementation
- **Configuration Handling:**
  - Define the configuration structures in Go.
  - Implement YAML parsing and validation for user-specified settings.
- **Event Extraction & Transformation:**
  - Iterate over spans received from the trace pipeline.
  - For each span, filter events based on the `include_event_names` configuration.
  - Transform matching events into log records:
    - Copy over event attributes.
    - Optionally add span context (trace ID, span ID, etc.).
    - Map event names to log severity using `severity_by_event_name`.
- **Pipeline Integration:**
  - Ensure the connector outputs log records in a format consumable by log exporters.
  - Allow the connector to operate in both trace and log pipelines.
- **Error Handling & Batching:**
  - Implement error handling for malformed events or configuration issues.
  - Consider batching mechanisms for high-throughput scenarios.

### 5. Testing
- **Unit Testing:**
  - Write tests to cover the transformation logic from span events to log records.
  - Validate configuration options and edge cases.
- **Integration Testing:**
  - Deploy the connector in a full Collector pipeline.
  - Verify that logs are correctly generated and routed to log exporters.
- **Performance Testing:**
  - Simulate high-load scenarios to assess performance impact.
  - Optimize based on profiling results.

### 6. Documentation & Examples
- **User Documentation:**
  - Create a README or user guide detailing the connector’s purpose, configuration, and usage examples.
  - Include sample YAML configurations.
- **Developer Documentation:**
  - Comment code extensively for maintainability.
  - Document contribution guidelines and design decisions.

### 7. Community Feedback & Iteration
- Publish the design and initial implementation as a GitHub issue/RFC.
- Solicit feedback from the community and maintainers.
- Iterate on the design and implementation based on feedback.

### 8. Finalization & Merge
- Conduct thorough code reviews with experienced maintainers.
- Address any performance, usability, or integration issues.
- Merge the connector into the main branch and update release notes.

## Example Configuration
```yaml
connectors:
  span_event_to_log:
    include_event_names: ["exception", "retry", "db.query"]
    include_span_context: true
    log_attributes_from: ["event.attributes", "span.attributes"]
    severity_by_event_name:
      exception: error
      retry: warning
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
      connectors: [span_event_to_log]
      exporters: [otlp]
    logs:
      connectors: [span_event_to_log]
      exporters: [loki]
