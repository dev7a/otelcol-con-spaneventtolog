// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package metadata contains metadata for the spaneventtolog connector.
package metadata // import "github.com/dev7a/otelcol-con-spaneventtolog/spaneventtologconnector/internal/metadata"

import (
	"go.opentelemetry.io/collector/component"
)

// Type represents the component type.
var Type = component.MustNewType("spaneventtolog")

// Stability represents the component stability level.
const Stability = component.StabilityLevelAlpha
