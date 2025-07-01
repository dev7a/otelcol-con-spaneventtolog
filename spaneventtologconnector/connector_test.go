// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spaneventtologconnector // import "github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector"

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component" // Need component for ID, BuildInfo
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/connector" // Need connector for Settings type
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/config"
)

// TestTracingInstrumentationIntegration tests that tracing is properly integrated
func TestTracingInstrumentationIntegration(t *testing.T) {
	// Create test traces with span events
	traces := createTestTraces()

	// Create a sink to capture the logs
	logsSink := new(consumertest.LogsSink)

	// Create the connector (uses component TracerProvider internally)
	cfg := &config.Config{
		IncludeSpanContext: true,
		LogAttributesFrom:  []string{"event.attributes"},
		SeverityByEventName: map[string]string{
			"exception": "error",
		},
	}
	settings := createTestConnectorSettings(t)
	connector := newConnector(settings, *cfg, logsSink)

	// Verify tracer is set
	assert.NotNil(t, connector.tracer, "Tracer should be initialized")

	// Consume traces - this exercises the tracing code paths
	err := connector.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)

	// Verify logs were created (indicating the instrumented methods worked)
	allLogs := logsSink.AllLogs()
	require.Equal(t, 1, len(allLogs), "Expected logs to be created")

	logs := allLogs[0]
	require.Equal(t, 2, logs.LogRecordCount(), "Expected 2 log records")
}

// TestTracingInstrumentationWithError tests error handling in tracing
func TestTracingInstrumentationWithError(t *testing.T) {
	// Create test traces
	traces := createTestTraces()

	// Create a consumer that returns an error
	errorConsumer := &ErrorLogsConsumer{
		err: assert.AnError,
	}

	// Create the connector
	cfg := &config.Config{
		IncludeSpanContext: true,
		LogAttributesFrom:  []string{"event.attributes"},
	}
	settings := createTestConnectorSettings(t)
	connector := newConnector(settings, *cfg, errorConsumer)

	// Verify tracer is set
	assert.NotNil(t, connector.tracer, "Tracer should be initialized")

	// Consume traces - should return error
	err := connector.ConsumeTraces(context.Background(), traces)
	assert.Error(t, err)
	assert.ErrorIs(t, err, assert.AnError)
}

// TestTracingInstrumentationNoEvents tests tracing when no events are processed
func TestTracingInstrumentationNoEvents(t *testing.T) {
	// Create traces with no events
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans().AppendEmpty()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	span := scopeSpans.Spans().AppendEmpty()
	span.SetName("test-span")
	// No events added

	// Create a sink to capture the logs
	logsSink := new(consumertest.LogsSink)

	// Create the connector
	cfg := &config.Config{
		IncludeSpanContext: true,
		LogAttributesFrom:  []string{"event.attributes"},
	}
	settings := createTestConnectorSettings(t)
	connector := newConnector(settings, *cfg, logsSink)

	// Verify tracer is set
	assert.NotNil(t, connector.tracer, "Tracer should be initialized")

	// Consume traces
	err := connector.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)

	// Verify no logs consumer call was made (0 logs)
	allLogs := logsSink.AllLogs()
	assert.Equal(t, 0, len(allLogs), "Expected no logs")
}

// TestConnectorTracerInitialization tests that the tracer is properly initialized
func TestConnectorTracerInitialization(t *testing.T) {
	cfg := &config.Config{}
	logsSink := new(consumertest.LogsSink)

	settings := createTestConnectorSettings(t)
	connector := newConnector(settings, *cfg, logsSink)

	// Verify tracer is initialized with correct name
	assert.NotNil(t, connector.tracer, "Tracer should be initialized")

	// Test that we can start a span (basic functionality test)
	ctx, span := connector.tracer.Start(context.Background(), "test-span")
	assert.NotNil(t, ctx, "Context should be returned")
	assert.NotNil(t, span, "Span should be created")
	span.End()
}

// ErrorLogsConsumer is a logs consumer that returns an error for testing
type ErrorLogsConsumer struct {
	err error
}

// Capabilities implements consumer.Logs.Capabilities
func (e *ErrorLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeLogs implements consumer.Logs.ConsumeLogs and returns an error
func (e *ErrorLogsConsumer) ConsumeLogs(ctx context.Context, logs plog.Logs) error {
	return e.err
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	cfgTyped, ok := cfg.(*config.Config) // Use config.Config
	assert.True(t, ok, "invalid config type")
	assert.True(t, cfgTyped.IncludeSpanContext, "default IncludeSpanContext should be true")
	assert.Equal(t, []string{"event.attributes", "resource.attributes"}, cfgTyped.LogAttributesFrom, "default LogAttributesFrom should include event.attributes and resource.attributes")
	assert.Equal(t, map[string]string{"exception": "error"}, cfgTyped.SeverityByEventName, "default SeverityByEventName should map exception to error")
	assert.False(t, cfgTyped.AddLevel, "default AddLevel should be false")
}

func TestCreateTracesToLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	// Manually create connector.Settings
	telSettings := componenttest.NewNopTelemetrySettings()
	telSettings.Logger = zaptest.NewLogger(t) // Keep the logger
	params := connector.Settings{
		ID:                component.MustNewIDWithName("spaneventtolog", "test"), // Use string literal
		TelemetrySettings: telSettings,
		BuildInfo:         component.NewDefaultBuildInfo(),
	}

	nextConsumer := consumertest.NewNop()
	connectorInstance, err := factory.CreateTracesToLogs(context.Background(), params, cfg, nextConsumer) // Use connectorInstance to avoid shadowing
	assert.NoError(t, err)
	assert.NotNil(t, connectorInstance) // Check connectorInstance
}

func createTestTraces() ptrace.Traces {
	traces := ptrace.NewTraces()

	// Add a resource
	resource := traces.ResourceSpans().AppendEmpty().Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	// Add a scope
	scope := traces.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Scope()
	scope.SetName("test-scope")
	scope.SetVersion("1.0.0")

	// Add a span
	span := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	span.Attributes().PutStr("http.method", "GET")
	span.Attributes().PutStr("http.url", "https://example.com")

	// Add an exception event
	exceptionEvent := span.Events().AppendEmpty()
	exceptionEvent.SetName("exception")
	exceptionEvent.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-30 * time.Second)))
	exceptionEvent.Attributes().PutStr("exception.type", "NullPointerException")
	exceptionEvent.Attributes().PutStr("exception.message", "Object was null")
	exceptionEvent.Attributes().PutStr("exception.stacktrace", "at com.example.Test.method(Test.java:42)")

	// Add a custom event
	customEvent := span.Events().AppendEmpty()
	customEvent.SetName("custom")
	customEvent.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-15 * time.Second)))
	customEvent.Attributes().PutStr("custom.key", "custom value")
	customEvent.Attributes().PutInt("custom.count", 42)

	return traces
}

func createTestConnectorSettings(t *testing.T) connector.Settings {
	telSettings := componenttest.NewNopTelemetrySettings()
	telSettings.Logger = zaptest.NewLogger(t)
	return connector.Settings{
		ID:                component.MustNewIDWithName("spaneventtolog", "test"),
		TelemetrySettings: telSettings,
		BuildInfo:         component.NewDefaultBuildInfo(),
	}
}

// TestNoEmptyLogsWhenNoMatchingEvents tests that no logs are created when no events match the filter criteria
func TestNoEmptyLogsWhenNoMatchingEvents(t *testing.T) {
	// Create test traces with span events that won't match our filter
	traces := createTestTraces()

	// Create a sink to capture the logs
	logsSink := new(consumertest.LogsSink)

	// Create connector with event name filtering that won't match any events
	cfg := &config.Config{
		IncludeEventNames: []string{"nonexistent_event"}, // This won't match any events in our test data
	}
	settings := createTestConnectorSettings(t)
	connector := newConnector(settings, *cfg, logsSink)

	// Consume traces
	err := connector.ConsumeTraces(context.Background(), traces)

	// Should not return an error
	assert.NoError(t, err)

	// Verify that no logs were created or sent to consumer
	assert.Equal(t, 0, logsSink.LogRecordCount(), "Expected no logs to be created when no events match filter")
	assert.Equal(t, 0, len(logsSink.AllLogs()), "Expected no log batches to be sent to consumer")
}

// TestAttributeMappings tests the new attribute mapping functionality
func TestAttributeMappings(t *testing.T) {
	tests := []struct {
		name                   string
		config                 config.Config
		expectedBody           string
		expectedSeverityNumber plog.SeverityNumber
		expectedSeverityText   string
		expectedEventNameAttr  string
		hasEventNameAttr       bool
	}{
		{
			name: "Complete attribute mapping",
			config: config.Config{
				LogAttributesFrom: []string{"event.attributes"},
				AttributeMappings: config.AttributeMappings{
					Body:           "event.body",
					SeverityNumber: "event.severity_number",
					SeverityText:   "event.severity_text",
					EventName:      "event.name",
				},
			},
			expectedBody:           "Successfully wrote TODO 5770916c-3838-4443-b4a8-f2b90366e235 to DynamoDB",
			expectedSeverityNumber: plog.SeverityNumber(9),
			expectedSeverityText:   "INFO",
			expectedEventNameAttr:  "backend.db.write_item.success",
			hasEventNameAttr:       true,
		},
		{
			name: "Partial mapping with fallback",
			config: config.Config{
				LogAttributesFrom: []string{"event.attributes"},
				AttributeMappings: config.AttributeMappings{
					Body:      "event.body",
					EventName: "event.name",
				},
				SeverityByEventName: map[string]string{
					"backend": "info",
				},
			},
			expectedBody:           "Successfully wrote TODO 5770916c-3838-4443-b4a8-f2b90366e235 to DynamoDB",
			expectedSeverityNumber: plog.SeverityNumberInfo,
			expectedSeverityText:   "info",
			expectedEventNameAttr:  "backend.db.write_item.success",
			hasEventNameAttr:       true,
		},
		{
			name: "Missing body attribute fallback",
			config: config.Config{
				LogAttributesFrom: []string{"event.attributes"},
				AttributeMappings: config.AttributeMappings{
					Body:           "missing.attribute",
					SeverityNumber: "event.severity_number",
					SeverityText:   "event.severity_text",
				},
			},
			expectedBody:           "backend.db.write_item.success", // Falls back to event name
			expectedSeverityNumber: plog.SeverityNumber(9),
			expectedSeverityText:   "INFO",
			hasEventNameAttr:       false,
		},
		{
			name: "No mappings - default behavior",
			config: config.Config{
				LogAttributesFrom: []string{"event.attributes"},
			},
			expectedBody:           "backend.db.write_item.success",
			expectedSeverityNumber: plog.SeverityNumberInfo,
			expectedSeverityText:   "info",
			hasEventNameAttr:       false,
		},
		{
			name: "Severity text mapping with parsing",
			config: config.Config{
				LogAttributesFrom: []string{"event.attributes"},
				AttributeMappings: config.AttributeMappings{
					SeverityText: "event.severity_text",
				},
			},
			expectedBody:           "backend.db.write_item.success",
			expectedSeverityNumber: plog.SeverityNumberInfo, // Parsed from "INFO"
			expectedSeverityText:   "info",                  // Canonical form
			hasEventNameAttr:       false,
		},
		{
			name: "Only severity number mapping - text should be derived",
			config: config.Config{
				LogAttributesFrom: []string{"event.attributes"},
				AttributeMappings: config.AttributeMappings{
					SeverityNumber: "event.severity_number",
				},
			},
			expectedBody:           "backend.db.write_item.success",
			expectedSeverityNumber: plog.SeverityNumber(9), // From event.severity_number
			expectedSeverityText:   "info",                 // Derived from number 9 (INFO)
			hasEventNameAttr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test traces with the structured event
			traces := createTestTracesWithStructuredEvent()

			// Create a sink to capture the logs
			logsSink := new(consumertest.LogsSink)

			// Create connector with the test configuration
			settings := createTestConnectorSettings(t)
			connector := newConnector(settings, tt.config, logsSink)

			// Consume traces
			err := connector.ConsumeTraces(context.Background(), traces)
			assert.NoError(t, err)

			// Verify logs were created
			allLogs := logsSink.AllLogs()
			require.Equal(t, 1, len(allLogs), "Expected logs to be created")

			logs := allLogs[0]
			require.Equal(t, 1, logs.LogRecordCount(), "Expected 1 log record")

			logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

			// Check body
			assert.Equal(t, tt.expectedBody, logRecord.Body().Str(), "Log body mismatch")

			// Check severity
			assert.Equal(t, tt.expectedSeverityNumber, logRecord.SeverityNumber(), "Severity number mismatch")
			assert.Equal(t, tt.expectedSeverityText, logRecord.SeverityText(), "Severity text mismatch")

			// Check event name attribute if expected
			if tt.hasEventNameAttr {
				eventNameAttr, exists := logRecord.Attributes().Get("event.name")
				assert.True(t, exists, "Expected event.name attribute to exist")
				assert.Equal(t, tt.expectedEventNameAttr, eventNameAttr.Str(), "Event name attribute mismatch")
			}
		})
	}
}

// TestAttributeMappingsPrecedence tests that attribute mappings take precedence over other configurations
func TestAttributeMappingsPrecedence(t *testing.T) {
	// Create test traces with the structured event
	traces := createTestTracesWithStructuredEvent()

	// Create a sink to capture the logs
	logsSink := new(consumertest.LogsSink)

	// Create connector with both attribute mappings and other severity configurations
	cfg := config.Config{
		LogAttributesFrom: []string{"event.attributes"},
		SeverityAttribute: "some.other.attribute", // This should be ignored
		SeverityByEventName: map[string]string{
			"backend": "error", // This should be ignored
		},
		AttributeMappings: config.AttributeMappings{
			SeverityNumber: "event.severity_number",
			SeverityText:   "event.severity_text",
		},
	}
	settings := createTestConnectorSettings(t)
	connector := newConnector(settings, cfg, logsSink)

	// Consume traces
	err := connector.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)

	// Verify logs were created
	allLogs := logsSink.AllLogs()
	require.Equal(t, 1, len(allLogs), "Expected logs to be created")

	logs := allLogs[0]
	require.Equal(t, 1, logs.LogRecordCount(), "Expected 1 log record")

	logRecord := logs.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)

	// Verify that attribute mappings took precedence
	assert.Equal(t, plog.SeverityNumber(9), logRecord.SeverityNumber(), "Attribute mapping should take precedence")
	assert.Equal(t, "INFO", logRecord.SeverityText(), "Attribute mapping should take precedence")
}

// createTestTracesWithStructuredEvent creates test traces with a span event that has the structured attributes
func createTestTracesWithStructuredEvent() ptrace.Traces {
	traces := ptrace.NewTraces()

	// Add a resource
	resource := traces.ResourceSpans().AppendEmpty().Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	// Add a scope
	scope := traces.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Scope()
	scope.SetName("test-scope")
	scope.SetVersion("1.0.0")

	// Add a span
	span := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-time.Minute)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Add a structured event matching the user's example
	event := span.Events().AppendEmpty()
	event.SetName("backend.db.write_item.success")
	event.SetTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-30 * time.Second)))
	event.Attributes().PutStr("event.body", "Successfully wrote TODO 5770916c-3838-4443-b4a8-f2b90366e235 to DynamoDB")
	event.Attributes().PutInt("event.severity_number", 9)
	event.Attributes().PutStr("event.severity_text", "INFO")

	return traces
}

// TestSeverityNumberToText tests the severityNumberToText helper function
func TestSeverityNumberToText(t *testing.T) {
	tests := []struct {
		severityNumber plog.SeverityNumber
		expectedText   string
	}{
		{plog.SeverityNumberTrace, "trace"},
		{plog.SeverityNumberDebug, "debug"},
		{plog.SeverityNumberInfo, "info"},
		{plog.SeverityNumberWarn, "warn"},
		{plog.SeverityNumberError, "error"},
		{plog.SeverityNumberFatal, "fatal"},
		{plog.SeverityNumberInfo2, "info2"},
		{plog.SeverityNumberError3, "error3"},
		{plog.SeverityNumberUnspecified, "info"}, // Default case
		{plog.SeverityNumber(999), "info"},       // Unknown number
	}

	for _, tt := range tests {
		t.Run(tt.expectedText, func(t *testing.T) {
			result := severityNumberToText(tt.severityNumber)
			assert.Equal(t, tt.expectedText, result)
		})
	}
}

// TestMapSeverity tests the mapSeverity function comprehensively
func TestMapSeverity(t *testing.T) {
	tests := []struct {
		input          string
		expectedNumber plog.SeverityNumber
		expectedText   string
	}{
		// Basic levels
		{"trace", plog.SeverityNumberTrace, "trace"},
		{"debug", plog.SeverityNumberDebug, "debug"},
		{"info", plog.SeverityNumberInfo, "info"},
		{"warn", plog.SeverityNumberWarn, "warn"},
		{"error", plog.SeverityNumberError, "error"},
		{"fatal", plog.SeverityNumberFatal, "fatal"},

		// Numbered variants
		{"trace1", plog.SeverityNumberTrace, "trace"},
		{"debug1", plog.SeverityNumberDebug, "debug"},
		{"info1", plog.SeverityNumberInfo, "info"},
		{"warn1", plog.SeverityNumberWarn, "warn"},
		{"error1", plog.SeverityNumberError, "error"},
		{"fatal1", plog.SeverityNumberFatal, "fatal"},

		{"trace2", plog.SeverityNumberTrace2, "trace2"},
		{"debug2", plog.SeverityNumberDebug2, "debug2"},
		{"info2", plog.SeverityNumberInfo2, "info2"},
		{"warn2", plog.SeverityNumberWarn2, "warn2"},
		{"error2", plog.SeverityNumberError2, "error2"},
		{"fatal2", plog.SeverityNumberFatal2, "fatal2"},

		// Case insensitive
		{"TRACE", plog.SeverityNumberTrace, "trace"},
		{"DEBUG", plog.SeverityNumberDebug, "debug"},
		{"INFO", plog.SeverityNumberInfo, "info"},
		{"WARN", plog.SeverityNumberWarn, "warn"},
		{"ERROR", plog.SeverityNumberError, "error"},
		{"FATAL", plog.SeverityNumberFatal, "fatal"},

		// Aliases
		{"warning", plog.SeverityNumberWarn, "warn"},
		{"err", plog.SeverityNumberError, "error"},
		{"warning2", plog.SeverityNumberWarn2, "warn2"},
		{"warning3", plog.SeverityNumberWarn3, "warn3"},

		// Invalid cases
		{"invalid", plog.SeverityNumberUnspecified, ""},
		{"", plog.SeverityNumberUnspecified, ""},
		{"unknown", plog.SeverityNumberUnspecified, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			number, text := mapSeverity(tt.input)
			assert.Equal(t, tt.expectedNumber, number, "Severity number mismatch for input: %s", tt.input)
			assert.Equal(t, tt.expectedText, text, "Severity text mismatch for input: %s", tt.input)
		})
	}
}
