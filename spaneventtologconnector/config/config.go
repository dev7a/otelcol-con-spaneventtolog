// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/config"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
)

// Config defines configuration for the span event to log connector.
type Config struct {
	// IncludeEventNames is the list of event names to include in the conversion from events to logs.
	// If empty, all events will be included.
	IncludeEventNames []string `mapstructure:"include_event_names"`

	// IncludeSpanContext is a flag that indicates whether to include span context in the log record.
	// If true, the following fields will be included in the log record:
	// - TraceID
	// - SpanID
	// - TraceFlags
	IncludeSpanContext bool `mapstructure:"include_span_context"`

	// LogAttributesFrom is a list of attribute sources to include in the log record.
	// Valid values are:
	// - "event.attributes": includes all attributes from the span event
	// - "span.attributes": includes all attributes from the parent span
	// - "resource.attributes": includes all resource attributes
	LogAttributesFrom []string `mapstructure:"log_attributes_from"`

	// SeverityByEventName is a map from event name to severity level.
	// If the event name is present in this map, the log record will have the mapped severity level.
	// If not, the default severity level (Info) will be used.
	SeverityByEventName map[string]string `mapstructure:"severity_by_event_name"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate checks if the connector configuration is valid.
func (c *Config) Validate() error {
	validSources := map[string]bool{
		"event.attributes":    true,
		"span.attributes":     true,
		"resource.attributes": true,
	}

	for _, source := range c.LogAttributesFrom {
		if !validSources[source] {
			return fmt.Errorf("invalid log attributes source: %s", source)
		}
	}

	validSeverities := map[string]bool{
		"trace":       true,
		"trace2":      true,
		"trace3":      true,
		"trace4":      true,
		"debug":       true,
		"debug2":      true,
		"debug3":      true,
		"debug4":      true,
		"info":        true,
		"info2":       true,
		"info3":       true,
		"info4":       true,
		"warn":        true,
		"warn2":       true,
		"warn3":       true,
		"warn4":       true,
		"error":       true,
		"error2":      true,
		"error3":      true,
		"error4":      true,
		"fatal":       true,
		"fatal2":      true,
		"fatal3":      true,
		"fatal4":      true,
		"unspecified": true,
	}

	for eventName, severity := range c.SeverityByEventName {
		if !validSeverities[severity] {
			return fmt.Errorf("invalid severity level for event %s: %s", eventName, severity)
		}
	}

	return nil
}

// MapSeverity maps a severity string to a plog.SeverityNumber.
func MapSeverity(severity string) plog.SeverityNumber { // Renamed to MapSeverity
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
