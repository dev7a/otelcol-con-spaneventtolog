// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spaneventtologconnector // import "github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/connector"
	"go.opentelemetry.io/collector/consumer"

	"github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/config"
	"github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/internal/metadata"
)

// NewFactory creates a factory for the span event to log connector.
func NewFactory() connector.Factory {
	return connector.NewFactory(
		metadata.Type,
		createDefaultConfig,
		connector.WithTracesToLogs(createTracesToLogs, metadata.Stability),
	)
}

// createDefaultConfig creates the default configuration for the connector.
func createDefaultConfig() component.Config {
	return &config.Config{
		IncludeSpanContext: true,
		LogAttributesFrom:  []string{"event.attributes"},
		SeverityByEventName: map[string]string{
			"exception": "error",
		},
	}
}

// createTracesToLogs creates a traces to logs connector based on the config.
func createTracesToLogs(_ context.Context, params connector.Settings, cfg component.Config, nextConsumer consumer.Logs) (connector.Traces, error) {
	c := cfg.(*config.Config)
	return newConnector(params.Logger, *c, nextConsumer), nil
}
