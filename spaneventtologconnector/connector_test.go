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
	assert.Equal(t, []string{"event.attributes", "resource.attributes"}, cfgTyped.LogAttributesFrom, "default LogAttributesFrom should include event.attributes and resource.attributes")
	assert.Equal(t, map[string]string{"exception": "error"}, cfgTyped.SeverityByEventName, "default SeverityByEventName should map exception to error")
	assert.False(t, cfgTyped.AddLevel, "default AddLevel should be false")
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

func TestAddLevelAttribute(t *testing.T) {
	// Create test traces with span events
	traces := createTestTraces()

	// Create a sink to capture the logs
	logsSink := new(consumertest.LogsSink)

	// Create the connector with AddLevel set to true
	cfg := &config.Config{
		IncludeSpanContext: true,
		LogAttributesFrom:  []string{"event.attributes"},
		SeverityByEventName: map[string]string{
			"exception": "error",
			"custom":    "info",
		},
		AddLevel: true,
	}
	connector := newConnector(zaptest.NewLogger(t), *cfg, logsSink)

	// Consume traces
	err := connector.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)

	// Verify that we received logs with level attribute
	allLogs := logsSink.AllLogs()
	logs := allLogs[0]
	resourceLogs := logs.ResourceLogs().At(0)
	scopeLogs := resourceLogs.ScopeLogs().At(0)

	// Check the exception log record
	exceptionLog := scopeLogs.LogRecords().At(0)
	levelAttr, exists := exceptionLog.Attributes().Get("level")
	assert.True(t, exists, "level attribute should exist")
	assert.Equal(t, "error", levelAttr.Str(), "level should match severity text")

	// Check the custom log record
	customLog := scopeLogs.LogRecords().At(1)
	levelAttr, exists = customLog.Attributes().Get("level")
	assert.True(t, exists, "level attribute should exist")
	assert.Equal(t, "info", levelAttr.Str(), "level should match severity text")
}

func TestAddLevelAttributeWithExistingLevel(t *testing.T) {
	// Create test traces with span events that already have level attribute
	traces := createTestTracesWithLevelAttribute()

	// Create a sink to capture the logs
	logsSink := new(consumertest.LogsSink)

	// Create the connector with AddLevel set to true
	cfg := &config.Config{
		IncludeSpanContext: true,
		LogAttributesFrom:  []string{"event.attributes"},
		SeverityByEventName: map[string]string{
			"exception": "error",
		},
		AddLevel: true,
	}
	connector := newConnector(zaptest.NewLogger(t), *cfg, logsSink)

	// Consume traces
	err := connector.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)

	// Verify that we received logs with the original level attribute
	allLogs := logsSink.AllLogs()
	logs := allLogs[0]
	resourceLogs := logs.ResourceLogs().At(0)
	scopeLogs := resourceLogs.ScopeLogs().At(0)

	// Check the log record
	logRecord := scopeLogs.LogRecords().At(0)
	levelAttr, exists := logRecord.Attributes().Get("level")
	assert.True(t, exists, "level attribute should exist")
	assert.Equal(t, "critical", levelAttr.Str(), "level should be the original value from event attributes")
}

func TestConsumeTraces_SeverityLogic(t *testing.T) {
	testCases := []struct {
		desc            string
		cfg             *config.Config
		eventName       string
		eventAttrs      map[string]interface{}
		expectedNum     plog.SeverityNumber
		expectedText    string
		expectLevelAttr bool // Only check if AddLevel is true in cfg
	}{
		{
			desc: "SeverityAttribute (present, valid, case-insensitive)",
			cfg: &config.Config{
				SeverityAttribute: "event.level",
			},
			eventName:    "test event",
			eventAttrs:   map[string]interface{}{"event.level": "Warn"},
			expectedNum:  plog.SeverityNumberWarn,
			expectedText: "warn",
		},
		{
			desc: "SeverityAttribute (present, valid, alias)",
			cfg: &config.Config{
				SeverityAttribute: "log.severity",
			},
			eventName:    "test event",
			eventAttrs:   map[string]interface{}{"log.severity": "ERR"},
			expectedNum:  plog.SeverityNumberError,
			expectedText: "error",
		},
		{
			desc: "SeverityAttribute (takes precedence over event name map)",
			cfg: &config.Config{
				SeverityAttribute:   "level",
				SeverityByEventName: map[string]string{"test": "fatal"},
			},
			eventName:    "test event",
			eventAttrs:   map[string]interface{}{"level": "DEBUG"},
			expectedNum:  plog.SeverityNumberDebug,
			expectedText: "debug",
		},
		{
			desc: "SeverityAttribute (present but invalid value, fallback to event name)",
			cfg: &config.Config{
				SeverityAttribute:   "level",
				SeverityByEventName: map[string]string{"event": "error"},
			},
			eventName:    "test event",
			eventAttrs:   map[string]interface{}{"level": "invalid"},
			expectedNum:  plog.SeverityNumberError, // Falls back to event name match
			expectedText: "error",
		},
		{
			desc: "SeverityAttribute (present but invalid value, fallback to default)",
			cfg: &config.Config{
				SeverityAttribute:   "level",
				SeverityByEventName: map[string]string{}, // No matching event name
			},
			eventName:    "test event",
			eventAttrs:   map[string]interface{}{"level": "invalid"},
			expectedNum:  plog.SeverityNumberInfo, // Falls back to default
			expectedText: "info",
		},
		{
			desc: "SeverityAttribute (configured but not present, fallback to event name)",
			cfg: &config.Config{
				SeverityAttribute:   "level",
				SeverityByEventName: map[string]string{"test": "fatal"},
			},
			eventName:    "test event",
			eventAttrs:   map[string]interface{}{"other.attr": "value"},
			expectedNum:  plog.SeverityNumberFatal, // Falls back to event name match
			expectedText: "fatal",
		},
		{
			desc: "SeverityAttribute (configured but non-string type, fallback to event name)",
			cfg: &config.Config{
				SeverityAttribute:   "level",
				SeverityByEventName: map[string]string{"test": "fatal"},
			},
			eventName:    "test event",
			eventAttrs:   map[string]interface{}{"level": 123},
			expectedNum:  plog.SeverityNumberFatal, // Falls back to event name match
			expectedText: "fatal",
		},
		{
			desc: "EventNameMap (simple substring, case-insensitive)",
			cfg: &config.Config{
				SeverityByEventName: map[string]string{"exception": "error"},
			},
			eventName:    "An Exception Occurred",
			eventAttrs:   map[string]interface{}{},
			expectedNum:  plog.SeverityNumberError,
			expectedText: "error",
		},
		{
			desc: "EventNameMap (longest match precedence)",
			cfg: &config.Config{
				SeverityByEventName: map[string]string{
					"error":            "error",
					"connection error": "fatal",
				},
			},
			eventName:    "Database connection error",
			eventAttrs:   map[string]interface{}{},
			expectedNum:  plog.SeverityNumberFatal, // "connection error" is longer
			expectedText: "fatal",
		},
		{
			desc: "EventNameMap (longest match precedence, case-insensitive keys)",
			cfg: &config.Config{
				SeverityByEventName: map[string]string{
					"ERROR":            "error",
					"Connection Error": "fatal",
				},
			},
			eventName:    "Database connection error",
			eventAttrs:   map[string]interface{}{},
			expectedNum:  plog.SeverityNumberFatal,
			expectedText: "fatal",
		},
		{
			desc: "EventNameMap (no match, fallback to default)",
			cfg: &config.Config{
				SeverityByEventName: map[string]string{"exception": "error"},
			},
			eventName:    "Normal event",
			eventAttrs:   map[string]interface{}{},
			expectedNum:  plog.SeverityNumberInfo,
			expectedText: "info",
		},
		{
			desc: "EventNameMap (invalid severity value in map, fallback to default)",
			cfg: &config.Config{
				SeverityByEventName: map[string]string{"test": "invalid"},
			},
			eventName:    "test event",
			eventAttrs:   map[string]interface{}{},
			expectedNum:  plog.SeverityNumberInfo,
			expectedText: "info",
		},
		{
			desc: "Default (no attribute, no map match)",
			cfg: &config.Config{
				SeverityByEventName: map[string]string{}, // Empty map
			},
			eventName:    "Some event",
			eventAttrs:   map[string]interface{}{},
			expectedNum:  plog.SeverityNumberInfo,
			expectedText: "info",
		},
		{
			desc: "SeverityAttribute with AddLevel=true",
			cfg: &config.Config{
				SeverityAttribute: "level",
				AddLevel:          true,
			},
			eventName:       "test event",
			eventAttrs:      map[string]interface{}{"level": "FATAL"},
			expectedNum:     plog.SeverityNumberFatal,
			expectedText:    "fatal",
			expectLevelAttr: true,
		},
		{
			desc: "EventNameMap with AddLevel=true",
			cfg: &config.Config{
				SeverityByEventName: map[string]string{"critical": "fatal"},
				AddLevel:            true,
			},
			eventName:       "critical error",
			eventAttrs:      map[string]interface{}{},
			expectedNum:     plog.SeverityNumberFatal,
			expectedText:    "fatal",
			expectLevelAttr: true,
		},
		{
			desc: "Default with AddLevel=true",
			cfg: &config.Config{
				AddLevel: true,
			},
			eventName:       "default event",
			eventAttrs:      map[string]interface{}{},
			expectedNum:     plog.SeverityNumberInfo,
			expectedText:    "info",
			expectLevelAttr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Create trace with the specified event
			traces := ptrace.NewTraces()
			span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
			event := span.Events().AppendEmpty()
			event.SetName(tc.eventName)
			attrs := event.Attributes()
			for k, v := range tc.eventAttrs {
				switch val := v.(type) {
				case string:
					attrs.PutStr(k, val)
				case int:
					attrs.PutInt(k, int64(val))
					// Add other types if needed
				}
			}

			// Ensure default config values don't interfere unless specified
			if tc.cfg.IncludeSpanContext == false && !mapContains(tc.cfg.LogAttributesFrom, "span.attributes") {
				tc.cfg.IncludeSpanContext = true // Need span context for the test setup
			}
			if !mapContains(tc.cfg.LogAttributesFrom, "event.attributes") {
				tc.cfg.LogAttributesFrom = append(tc.cfg.LogAttributesFrom, "event.attributes") // Needed for SeverityAttribute
			}

			logsSink := new(consumertest.LogsSink)
			connector := newConnector(zaptest.NewLogger(t), *tc.cfg, logsSink)

			err := connector.ConsumeTraces(context.Background(), traces)
			assert.NoError(t, err)

			allLogs := logsSink.AllLogs()
			require.Equal(t, 1, len(allLogs), "Expected exactly one batch of logs")
			logs := allLogs[0]
			require.Equal(t, 1, logs.ResourceLogs().Len())
			resourceLogs := logs.ResourceLogs().At(0)
			require.Equal(t, 1, resourceLogs.ScopeLogs().Len())
			scopeLogs := resourceLogs.ScopeLogs().At(0)
			require.Equal(t, 1, scopeLogs.LogRecords().Len(), "Expected exactly one log record")

			logRecord := scopeLogs.LogRecords().At(0)

			// Assert SeverityNumber and SeverityText
			assert.Equal(t, tc.expectedNum, logRecord.SeverityNumber(), "SeverityNumber mismatch")
			assert.Equal(t, tc.expectedText, logRecord.SeverityText(), "SeverityText mismatch")

			// Assert level attribute only when AddLevel is configured to true
			if tc.cfg.AddLevel {
				levelAttr, exists := logRecord.Attributes().Get("level")
				if tc.expectLevelAttr { // We expect AddLevel to have added it (or it existed already)
					assert.True(t, exists, "level attribute should exist when AddLevel=true and severity was determined")
					// Check if the original event had a level attribute. If so, it shouldn't be overwritten.
					if origLevelVal, origExists := tc.eventAttrs["level"]; origExists {
						if levelStr, ok := origLevelVal.(string); ok {
							assert.Equal(t, levelStr, levelAttr.Str(), "level attribute should retain original value if present in event")
						} else {
							// If original level wasn't a string, it shouldn't have prevented adding the derived one.
							assert.Equal(t, tc.expectedText, levelAttr.Str(), "level attribute should match canonical severity text")
						}
					} else {
						// Original event didn't have 'level', so AddLevel should have added the canonical text.
						assert.Equal(t, tc.expectedText, levelAttr.Str(), "level attribute should match canonical severity text")
					}
				} else {
					// This case might occur if AddLevel is true, but severity was Unspecified (though defaults prevent this currently)
					// Or if the original event had a non-string 'level' attribute.
					// Let's assume for now if expectLevelAttr is false, it shouldn't exist.
					assert.False(t, exists, "level attribute should not exist when AddLevel=true but expectLevelAttr=false")
				}
			} else {
				// If AddLevel is false, we don't assert about the 'level' attribute's presence,
				// as it might have been copied from the original event attributes.
			}
		})
	}
}

// Helper function to check if a string slice contains a string
func mapContains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// Helper function to create test traces with level attribute
func createTestTracesWithLevelAttribute() ptrace.Traces {
	traces := ptrace.NewTraces()

	// Add a resource
	resource := traces.ResourceSpans().AppendEmpty().Resource()
	resource.Attributes().PutStr("service.name", "test-service")

	// Add a scope
	scope := traces.ResourceSpans().At(0).ScopeSpans().AppendEmpty().Scope()
	scope.SetName("test-scope")

	// Add a span
	span := traces.ResourceSpans().At(0).ScopeSpans().At(0).Spans().AppendEmpty()
	span.SetName("test-span")
	span.SetTraceID(pcommon.TraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}))
	span.SetSpanID(pcommon.SpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))

	// Add an exception event with level attribute
	exceptionEvent := span.Events().AppendEmpty()
	exceptionEvent.SetName("exception")
	exceptionEvent.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	exceptionEvent.Attributes().PutStr("exception.type", "NullPointerException")
	exceptionEvent.Attributes().PutStr("level", "critical") // Pre-existing level attribute

	return traces
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
