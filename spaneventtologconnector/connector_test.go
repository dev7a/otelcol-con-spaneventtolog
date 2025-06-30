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

	// Create the connector (uses otel.Tracer internally)
	cfg := &config.Config{
		IncludeSpanContext: true,
		LogAttributesFrom:  []string{"event.attributes"},
		SeverityByEventName: map[string]string{
			"exception": "error",
		},
	}
	connector := newConnector(zaptest.NewLogger(t), *cfg, logsSink)

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
	connector := newConnector(zaptest.NewLogger(t), *cfg, errorConsumer)

	// Verify tracer is set
	assert.NotNil(t, connector.tracer, "Tracer should be initialized")

	// Consume traces - should return error
	err := connector.ConsumeTraces(context.Background(), traces)
	assert.Error(t, err)
	assert.Equal(t, assert.AnError, err)
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
	connector := newConnector(zaptest.NewLogger(t), *cfg, logsSink)

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

	connector := newConnector(zaptest.NewLogger(t), *cfg, logsSink)

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
