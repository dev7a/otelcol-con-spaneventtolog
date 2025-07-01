// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spaneventtologconnector // import "github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector"

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/config"
)

// severityMappings defines the canonical mapping between severity numbers and text.
// This serves as the single source of truth for severity conversions.
var severityMappings = []struct {
	number plog.SeverityNumber
	text   string
}{
	{plog.SeverityNumberTrace, "trace"},
	{plog.SeverityNumberTrace2, "trace2"},
	{plog.SeverityNumberTrace3, "trace3"},
	{plog.SeverityNumberTrace4, "trace4"},
	{plog.SeverityNumberDebug, "debug"},
	{plog.SeverityNumberDebug2, "debug2"},
	{plog.SeverityNumberDebug3, "debug3"},
	{plog.SeverityNumberDebug4, "debug4"},
	{plog.SeverityNumberInfo, "info"},
	{plog.SeverityNumberInfo2, "info2"},
	{plog.SeverityNumberInfo3, "info3"},
	{plog.SeverityNumberInfo4, "info4"},
	{plog.SeverityNumberWarn, "warn"},
	{plog.SeverityNumberWarn2, "warn2"},
	{plog.SeverityNumberWarn3, "warn3"},
	{plog.SeverityNumberWarn4, "warn4"},
	{plog.SeverityNumberError, "error"},
	{plog.SeverityNumberError2, "error2"},
	{plog.SeverityNumberError3, "error3"},
	{plog.SeverityNumberError4, "error4"},
	{plog.SeverityNumberFatal, "fatal"},
	{plog.SeverityNumberFatal2, "fatal2"},
	{plog.SeverityNumberFatal3, "fatal3"},
	{plog.SeverityNumberFatal4, "fatal4"},
}

// severityToTextMap is generated from severityMappings for fast lookups.
var severityToTextMap = func() map[plog.SeverityNumber]string {
	m := make(map[plog.SeverityNumber]string, len(severityMappings))
	for _, mapping := range severityMappings {
		m[mapping.number] = mapping.text
	}
	return m
}()

// textToSeverityMap is generated from severityMappings for reverse lookups.
var textToSeverityMap = func() map[string]plog.SeverityNumber {
	m := make(map[string]plog.SeverityNumber, len(severityMappings))
	for _, mapping := range severityMappings {
		m[mapping.text] = mapping.number
		// Add common aliases
		switch mapping.text {
		case "warn":
			m["warning"] = mapping.number
		case "error":
			m["err"] = mapping.number
		}
	}
	return m
}()

// Connector is a span event to log connector.
type Connector struct {
	config       config.Config
	logsConsumer consumer.Logs
	logger       *zap.Logger
	eventNameSet map[string]struct{}
	tracer       trace.Tracer
}

var _ consumer.Traces = (*Connector)(nil)
var _ component.Component = (*Connector)(nil)

// newConnector creates a new span event to log connector.
func newConnector(settings connector.Settings, cfg config.Config, logsConsumer consumer.Logs) *Connector {
	c := &Connector{
		config:       cfg,
		logsConsumer: logsConsumer,
		logger:       settings.Logger,
		tracer:       settings.TracerProvider.Tracer(settings.ID.String()),
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
	ctx, span := c.tracer.Start(ctx, "connector/spaneventtolog/ConsumeTraces",
		trace.WithAttributes(
			attribute.Int("input_spans", traces.SpanCount()),
			attribute.Int("resource_spans", traces.ResourceSpans().Len()),
		),
	)
	defer span.End()

	logs := c.extractLogsFromTraces(ctx, traces)

	if logs.LogRecordCount() > 0 {
		span.SetAttributes(attribute.Int("output_logs", logs.LogRecordCount()))
		err := c.logsConsumer.ConsumeLogs(ctx, logs)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return err
		}
	} else {
		span.SetAttributes(attribute.Int("output_logs", 0))
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
func (c *Connector) extractLogsFromTraces(ctx context.Context, traces ptrace.Traces) plog.Logs {
	_, otelSpan := c.tracer.Start(ctx, "connector/spaneventtolog/ExtractLogs")
	defer otelSpan.End()

	logs := plog.NewLogs()

	if traces.ResourceSpans().Len() == 0 {
		otelSpan.SetAttributes(attribute.String("result", "no_resource_spans"))
		return logs
	}

	totalEvents := 0
	processedEvents := 0

	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		resourceSpans := traces.ResourceSpans().At(i)
		resource := resourceSpans.Resource()

		for j := 0; j < resourceSpans.ScopeSpans().Len(); j++ {
			scopeSpans := resourceSpans.ScopeSpans().At(j)
			scope := scopeSpans.Scope()

			for k := 0; k < scopeSpans.Spans().Len(); k++ {
				span := scopeSpans.Spans().At(k)

				// Process each event in the span
				for l := 0; l < span.Events().Len(); l++ {
					event := span.Events().At(l)
					totalEvents++

					// Skip if we're filtering by event name and this event is not in the list
					if c.eventNameSet != nil {
						if _, exists := c.eventNameSet[event.Name()]; !exists {
							continue
						}
					}

					processedEvents++

					// LAZY CREATION: Only create ResourceLogs and ScopeLogs when we have an event to process
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

					// Find or create the ScopeLogs entry for this scope within the current ResourceLogs
					scopeLogs := findOrCreateScopeLogs(resourceLogs, scope)

					// Create and append the log record to the correct ScopeLogs
					logRecord := scopeLogs.LogRecords().AppendEmpty()
					c.populateLogRecord(logRecord, event, span)
				}
			}
		}
	}

	otelSpan.SetAttributes(
		attribute.Int("total_events_found", totalEvents),
		attribute.Int("events_processed", processedEvents),
		attribute.Int("logs_created", logs.LogRecordCount()),
	)

	return logs
}

// populateLogRecord populates a log record based on a span event.
func (c *Connector) populateLogRecord(
	logRecord plog.LogRecord,
	event ptrace.SpanEvent,
	span ptrace.Span,
) {
	// Default severity
	severityNumber := plog.SeverityNumberInfo
	severityText := "info"
	severityFound := false

	// 1. Check AttributeMappings for severity (Highest Precedence)
	if c.config.AttributeMappings.SeverityNumber != "" || c.config.AttributeMappings.SeverityText != "" {
		if c.config.AttributeMappings.SeverityNumber != "" {
			if attrValue, exists := event.Attributes().Get(c.config.AttributeMappings.SeverityNumber); exists {
				if attrValue.Type() == pcommon.ValueTypeInt {
					severityNumber = plog.SeverityNumber(attrValue.Int())
					// Derive severity text from the mapped number to keep them in sync
					severityText = severityNumberToText(severityNumber)
					severityFound = true
				}
			}
		}
		if c.config.AttributeMappings.SeverityText != "" {
			if attrValue, exists := event.Attributes().Get(c.config.AttributeMappings.SeverityText); exists && attrValue.Type() == pcommon.ValueTypeStr {
				severityText = attrValue.Str()
				// If we don't have severity number from attribute mapping, try to parse from text
				if !severityFound {
					parsedNumber, parsedText := mapSeverity(severityText)
					if parsedNumber != plog.SeverityNumberUnspecified {
						severityNumber = parsedNumber
						severityText = parsedText
					}
				}
				severityFound = true
			}
		}
	}

	// 2. Check SeverityAttribute (High Precedence)
	if !severityFound && c.config.SeverityAttribute != "" {
		if attrValue, exists := event.Attributes().Get(c.config.SeverityAttribute); exists && attrValue.Type() == pcommon.ValueTypeStr {
			parsedNumber, parsedText := mapSeverity(attrValue.Str())
			if parsedNumber != plog.SeverityNumberUnspecified {
				severityNumber = parsedNumber
				severityText = parsedText
				severityFound = true
			}
		}
	}

	// 3. Check SeverityByEventName (Substring Match, Longest Precedence)
	if !severityFound && len(c.config.SeverityByEventName) > 0 {
		lowerEventName := strings.ToLower(event.Name())
		longestMatchKeyLen := 0
		matchedSeverityText := ""

		for key, configuredSeverity := range c.config.SeverityByEventName {
			lowerKey := strings.ToLower(key)
			if strings.Contains(lowerEventName, lowerKey) {
				if len(key) > longestMatchKeyLen {
					// Check if the configuredSeverity is valid before accepting it
					parsedNumber, parsedText := mapSeverity(configuredSeverity)
					if parsedNumber != plog.SeverityNumberUnspecified {
						longestMatchKeyLen = len(key)
						matchedSeverityText = parsedText // Use the canonical text from mapSeverity
					}
				}
			}
		}

		if matchedSeverityText != "" {
			severityNumber, severityText = mapSeverity(matchedSeverityText) // Remap to get both Number and Text
			severityFound = true
		}
	}

	// Set timestamp from event
	logRecord.SetTimestamp(event.Timestamp())

	// Set observed timestamp to current time
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))

	// Set the determined severity (or default if not found)
	logRecord.SetSeverityNumber(severityNumber)
	logRecord.SetSeverityText(severityText)

	// Set body using attribute mapping or fallback to event name
	bodySet := false
	if c.config.AttributeMappings.Body != "" {
		if attrValue, exists := event.Attributes().Get(c.config.AttributeMappings.Body); exists && attrValue.Type() == pcommon.ValueTypeStr {
			logRecord.Body().SetStr(attrValue.Str())
			bodySet = true
		}
	}
	if !bodySet {
		// Fallback to event name
		logRecord.Body().SetStr(event.Name())
	}

	// Copy event attributes if configured
	if c.shouldCopyAttributes("event.attributes") {
		event.Attributes().CopyTo(logRecord.Attributes())
	}

	// Preserve event name as attribute if configured
	if c.config.AttributeMappings.EventName != "" {
		logRecord.Attributes().PutStr(c.config.AttributeMappings.EventName, event.Name())
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

// mapSeverity maps a severity string (case-insensitive) to a plog.SeverityNumber and its canonical text.
// Returns SeverityNumberUnspecified and an empty string if the input is not a valid severity.
func mapSeverity(severity string) (plog.SeverityNumber, string) {
	lowerSeverity := strings.ToLower(severity)

	// Handle direct mappings and aliases
	if severityNumber, exists := textToSeverityMap[lowerSeverity]; exists {
		return severityNumber, severityToTextMap[severityNumber]
	}

	// Handle numbered variants (e.g., "trace1", "debug1", "warning2")
	if strings.HasSuffix(lowerSeverity, "1") {
		baseText := strings.TrimSuffix(lowerSeverity, "1")
		if severityNumber, exists := textToSeverityMap[baseText]; exists {
			return severityNumber, severityToTextMap[severityNumber]
		}
	}

	// Handle numbered warning variants (e.g., "warning2", "warning3")
	if strings.HasPrefix(lowerSeverity, "warning") && len(lowerSeverity) > 7 {
		numberSuffix := lowerSeverity[7:] // Extract number after "warning"
		warnVariant := "warn" + numberSuffix
		if severityNumber, exists := textToSeverityMap[warnVariant]; exists {
			return severityNumber, severityToTextMap[severityNumber]
		}
	}

	return plog.SeverityNumberUnspecified, ""
}

// severityNumberToText maps a plog.SeverityNumber to its canonical text representation.
// Returns "info" as default for unspecified or unknown severity numbers.
func severityNumberToText(severityNumber plog.SeverityNumber) string {
	if text, exists := severityToTextMap[severityNumber]; exists {
		return text
	}
	return "info"
}
