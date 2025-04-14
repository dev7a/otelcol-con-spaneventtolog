// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spaneventtologconnector // import "github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/config"
)

// Connector is a span event to log connector.
type Connector struct {
	config       config.Config
	logsConsumer consumer.Logs
	logger       *zap.Logger
	eventNameSet map[string]struct{}
}

var _ consumer.Traces = (*Connector)(nil)
var _ component.Component = (*Connector)(nil)

// newConnector creates a new span event to log connector.
func newConnector(logger *zap.Logger, cfg config.Config, logsConsumer consumer.Logs) *Connector {
	c := &Connector{
		config:       cfg,
		logsConsumer: logsConsumer,
		logger:       logger,
	}

	// Create a map for fast lookup of included event names
	if len(cfg.IncludeEventNames) > 0 {
		c.eventNameSet = make(map[string]struct{}, len(cfg.IncludeEventNames))
		for _, name := range cfg.IncludeEventNames {
			c.eventNameSet[name] = struct{}{}
		}
	}

	return c
}

// Capabilities implements the consumer interface.
func (c *Connector) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements the consumer.Traces interface.
func (c *Connector) ConsumeTraces(ctx context.Context, traces ptrace.Traces) error {
	logs := c.extractLogsFromTraces(traces)

	if logs.LogRecordCount() > 0 {
		return c.logsConsumer.ConsumeLogs(ctx, logs)
	}

	return nil
}

// Start implements the component.Component interface.
func (c *Connector) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown implements the component.Component interface.
func (c *Connector) Shutdown(_ context.Context) error {
	return nil
}

// findOrCreateResourceLogs finds existing ResourceLogs or creates a new one.
// Returns the ResourceLogs and a boolean indicating if it was newly created.
func findOrCreateResourceLogs(logs plog.Logs, res pcommon.Resource) (plog.ResourceLogs, bool) {
	rls := logs.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		// Simple identity check, might need more robust comparison if attributes change
		if rl.Resource() == res {
			return rl, false
		}
	}
	newRl := rls.AppendEmpty()
	res.CopyTo(newRl.Resource())
	return newRl, true
}

// findOrCreateScopeLogs finds existing ScopeLogs or creates a new one within ResourceLogs.
// Returns the ScopeLogs.
func findOrCreateScopeLogs(rl plog.ResourceLogs, scope pcommon.InstrumentationScope) plog.ScopeLogs {
	sls := rl.ScopeLogs()
	for i := 0; i < sls.Len(); i++ {
		sl := sls.At(i)
		// Simple identity check
		if sl.Scope() == scope {
			return sl
		}
	}
	newSl := sls.AppendEmpty()
	scope.CopyTo(newSl.Scope())
	return newSl
}

// extractLogsFromTraces extracts logs from traces, grouping by resource and scope.
func (c *Connector) extractLogsFromTraces(traces ptrace.Traces) plog.Logs {
	logs := plog.NewLogs()

	if traces.ResourceSpans().Len() == 0 {
		return logs
	}

	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		resourceSpans := traces.ResourceSpans().At(i)
		resource := resourceSpans.Resource()

		// Find or create the ResourceLogs entry for this resource
		resourceLogs, createdRl := findOrCreateResourceLogs(logs, resource)
		if createdRl {
			// Copy resource attributes only if configured and only when ResourceLogs is first created
			if c.shouldCopyAttributes("resource.attributes") {
				resource.Attributes().CopyTo(resourceLogs.Resource().Attributes())
			} else {
				// Ensure resourceLogs has a resource object, even if empty
				resourceLogs.Resource().Attributes().Clear()
			}
		}

		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
			scopeSpans := resourceSpans.ScopeSpans().At(j)
			scope := scopeSpans.Scope()

			// Find or create the ScopeLogs entry for this scope within the current ResourceLogs
			scopeLogs := findOrCreateScopeLogs(resourceLogs, scope)

			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)

				// Process each event in the span
				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)

					// Skip if we're filtering by event name and this event is not in the list
					if c.eventNameSet != nil {
						if _, exists := c.eventNameSet[event.Name()]; !exists {
							continue
						}
					}

					// Create and append the log record to the correct ScopeLogs
					logRecord := scopeLogs.LogRecords().AppendEmpty()
					c.populateLogRecord(logRecord, event, span)
				}
			}
		}
	}

	return logs
}

// populateLogRecord populates a log record based on a span event.
func (c *Connector) populateLogRecord(
	logRecord plog.LogRecord,
	event ptrace.SpanEvent,
	span ptrace.Span,
) {

	// Set timestamp from event
	logRecord.SetTimestamp(event.Timestamp())

	// Set observed timestamp to current time
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set severity level if configured
	if severity, ok := c.config.SeverityByEventName[event.Name()]; ok {
		severityNumber := mapSeverity(severity)
		logRecord.SetSeverityNumber(severityNumber)
		logRecord.SetSeverityText(severity)
	} else {
		logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
		logRecord.SetSeverityText("info")
	}

	// Set body to event name
	logRecord.Body().SetStr(event.Name())

	// Copy event attributes if configured
	if c.shouldCopyAttributes("event.attributes") {
		event.Attributes().CopyTo(logRecord.Attributes())
	}

	// Add level attribute if configured and not already present
	if c.config.AddLevel {
		// Check if level attribute already exists in log record attributes
		_, hasLevel := logRecord.Attributes().Get("level")
		if !hasLevel {
			// Add level attribute based on severity text
			logRecord.Attributes().PutStr("level", logRecord.SeverityText())
		}
	}

	// Copy span attributes if configured
	if c.shouldCopyAttributes("span.attributes") {
		span.Attributes().Range(func(k string, v pcommon.Value) bool {
			v.CopyTo(logRecord.Attributes().PutEmpty(k))
			return true
		})
	}

	// Add trace and span ID fields if configured
	if c.config.IncludeSpanContext {
		logRecord.SetTraceID(span.TraceID())
		logRecord.SetSpanID(span.SpanID())

		// Set flags
		if span.TraceState().AsRaw() != "" {
			logRecord.Attributes().PutStr("trace.state", span.TraceState().AsRaw())
		}

		// Add span name
		logRecord.Attributes().PutStr("span.name", span.Name())

		// Add span kind
		logRecord.Attributes().PutStr("span.kind", span.Kind().String())
	}
}

// shouldCopyAttributes determines if attributes should be copied from the specified source.
func (c *Connector) shouldCopyAttributes(source string) bool {
	for _, s := range c.config.LogAttributesFrom {
		if s == source {
			return true
		}
	}
	return false
}

// mapSeverity maps a severity string to a plog.SeverityNumber.
func mapSeverity(severity string) plog.SeverityNumber {
	switch severity {
	case "trace", "trace1":
		return plog.SeverityNumberTrace
	case "trace2":
		return plog.SeverityNumberTrace2
	case "trace3":
		return plog.SeverityNumberTrace3
	case "trace4":
		return plog.SeverityNumberTrace4
	case "debug", "debug1":
		return plog.SeverityNumberDebug
	case "debug2":
		return plog.SeverityNumberDebug2
	case "debug3":
		return plog.SeverityNumberDebug3
	case "debug4":
		return plog.SeverityNumberDebug4
	case "info", "info1":
		return plog.SeverityNumberInfo
	case "info2":
		return plog.SeverityNumberInfo2
	case "info3":
		return plog.SeverityNumberInfo3
	case "info4":
		return plog.SeverityNumberInfo4
	case "warn", "warn1":
		return plog.SeverityNumberWarn
	case "warn2":
		return plog.SeverityNumberWarn2
	case "warn3":
		return plog.SeverityNumberWarn3
	case "warn4":
		return plog.SeverityNumberWarn4
	case "error", "error1":
		return plog.SeverityNumberError
	case "error2":
		return plog.SeverityNumberError2
	case "error3":
		return plog.SeverityNumberError3
	case "error4":
		return plog.SeverityNumberError4
	case "fatal", "fatal1":
		return plog.SeverityNumberFatal
	case "fatal2":
		return plog.SeverityNumberFatal2
	case "fatal3":
		return plog.SeverityNumberFatal3
	case "fatal4":
		return plog.SeverityNumberFatal4
	default:
		return plog.SeverityNumberUnspecified
	}
}
