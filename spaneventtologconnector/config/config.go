// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config // import "github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/config"

import (
	"fmt"
)

// AttributeMappings defines how span event attributes should be mapped to log record fields.
type AttributeMappings struct {
	// Body specifies the event attribute name to use for the log record body.
	// If empty or the attribute doesn't exist, falls back to using the event name.
	Body string `mapstructure:"body"`

	// SeverityNumber specifies the event attribute name to use for the log record severity number.
	// If empty or the attribute doesn't exist, falls back to existing severity configuration.
	SeverityNumber string `mapstructure:"severity_number"`

	// SeverityText specifies the event attribute name to use for the log record severity text.
	// If empty or the attribute doesn't exist, falls back to existing severity configuration.
	SeverityText string `mapstructure:"severity_text"`

	// EventName specifies the log attribute name to store the original event name.
	// If empty, the event name won't be preserved as an attribute.
	EventName string `mapstructure:"event_name"`
}

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

	// AddLevel is a flag that indicates whether to add a "level" attribute to the log record
	// based on the severity text. If true and a "level" attribute doesn't already exist,
	// the severity text will be copied to a "level" attribute.
	AddLevel bool `mapstructure:"add_level"`

	// SeverityAttribute is the name of the event attribute to use for determining the severity level.
	// If set, this takes precedence over SeverityByEventName. The attribute value must be a string
	// matching one of the supported severity levels (case-insensitive).
	SeverityAttribute string `mapstructure:"severity_attribute"`

	// AttributeMappings defines how span event attributes should be mapped to log record fields.
	// These mappings take precedence over other configuration options and fall back to existing
	// behavior when the specified attributes don't exist.
	AttributeMappings AttributeMappings `mapstructure:"attribute_mappings"`

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
