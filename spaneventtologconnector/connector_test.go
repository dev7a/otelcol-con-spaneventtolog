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
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap/zaptest"

	"github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/config"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	cfgTyped, ok := cfg.(*config.Config) // Use config.Config
	assert.True(t, ok, "invalid config type")
	assert.True(t, cfgTyped.IncludeSpanContext, "default IncludeSpanContext should be true")
	assert.Equal(t, []string{"event.attributes"}, cfgTyped.LogAttributesFrom, "default LogAttributesFrom should include event.attributes")
	assert.Equal(t, map[string]string{"exception": "error"}, cfgTyped.SeverityByEventName, "default SeverityByEventName should map exception to error")
}

func TestConfig_Validate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *config.Config // Use config.Config
		expectedErr string
	}{
		{
			desc: "valid config",
			cfg: &config.Config{ // Use config.Config
				LogAttributesFrom: []string{"event.attributes", "span.attributes", "resource.attributes"},
				SeverityByEventName: map[string]string{
					"exception": "error",
					"info":      "info",
				},
			},
			expectedErr: "",
		},
		{
			desc: "invalid log attributes source",
			cfg: &config.Config{ // Use config.Config
				LogAttributesFrom: []string{"invalid.source"},
			},
			expectedErr: "invalid log attributes source: invalid.source",
		},
		{
			desc: "invalid severity level",
			cfg: &config.Config{ // Use config.Config
				SeverityByEventName: map[string]string{
					"exception": "invalid_severity",
				},
			},
			expectedErr: "invalid severity level for event exception: invalid_severity",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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

func TestMapSeverity(t *testing.T) {
	testCases := []struct {
		severity string
		expected plog.SeverityNumber
	}{
		{"trace", plog.SeverityNumberTrace},
		{"debug", plog.SeverityNumberDebug},
		{"info", plog.SeverityNumberInfo},
		{"warn", plog.SeverityNumberWarn},
		{"error", plog.SeverityNumberError},
		{"fatal", plog.SeverityNumberFatal},
		{"unknown", plog.SeverityNumberUnspecified},
	}

	for _, tc := range testCases {
		t.Run(tc.severity, func(t *testing.T) {
			result := config.MapSeverity(tc.severity) // Use config.MapSeverity
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConsumeTraces(t *testing.T) {
	// Create test traces with span events
	traces := createTestTraces()

	// Create a sink to capture the logs
	logsSink := new(consumertest.LogsSink)

	// Create the connector
	cfg := &config.Config{ // Use config.Config
		IncludeSpanContext: true,
		LogAttributesFrom:  []string{"event.attributes", "span.attributes", "resource.attributes"},
		SeverityByEventName: map[string]string{
			"exception": "error",
			"custom":    "info",
		},
	}
	connector := newConnector(zaptest.NewLogger(t), *cfg, logsSink) // Pass *cfg (value)

	// Consume traces
	err := connector.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)

	// Verify that we received logs
	allLogs := logsSink.AllLogs()
	require.Equal(t, 1, len(allLogs), "Expected exactly one batch of logs")

	logs := allLogs[0]
	require.Equal(t, 1, logs.ResourceLogs().Len(), "Expected exactly one resource logs")

	resourceLogs := logs.ResourceLogs().At(0)
	require.Equal(t, 1, resourceLogs.ScopeLogs().Len(), "Expected exactly one scope logs")

	scopeLogs := resourceLogs.ScopeLogs().At(0)
	require.Equal(t, 2, scopeLogs.LogRecords().Len(), "Expected exactly two log records")

	// Verify resource attributes were copied
	resAttrs := resourceLogs.Resource().Attributes()
	serviceName, exists := resAttrs.Get("service.name")
	assert.True(t, exists)
	assert.Equal(t, "test-service", serviceName.Str())

	// Check the first log record - exception event
	exceptionLog := scopeLogs.LogRecords().At(0)
	assert.Equal(t, "exception", exceptionLog.Body().Str())
	assert.Equal(t, plog.SeverityNumberError, exceptionLog.SeverityNumber())
	assert.Equal(t, "error", exceptionLog.SeverityText())

	// Verify trace context
	assert.NotEqual(t, pcommon.TraceID{}, exceptionLog.TraceID())
	assert.NotEqual(t, pcommon.SpanID{}, exceptionLog.SpanID())

	// Verify span attributes were copied
	spanName, exists := exceptionLog.Attributes().Get("span.name")
	assert.True(t, exists)
	assert.Equal(t, "test-span", spanName.Str())

	// Verify event attributes were copied
	errorType, exists := exceptionLog.Attributes().Get("exception.type")
	assert.True(t, exists)
	assert.Equal(t, "NullPointerException", errorType.Str())

	// Check the second log record - custom event
	customLog := scopeLogs.LogRecords().At(1)
	assert.Equal(t, "custom", customLog.Body().Str())
	assert.Equal(t, plog.SeverityNumberInfo, customLog.SeverityNumber())
	assert.Equal(t, "info", customLog.SeverityText())
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
