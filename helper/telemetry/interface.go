package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/codes"
)

type Span interface {
	// SetAttribute sets an attribute (base type)
	SetAttribute(label string, value interface{})

	// SetAttributes sets attributes
	SetAttributes(attributes map[string]interface{})

	// SetStatus sets the status
	SetStatus(code codes.Code, info string)

	// SetError sets the error
	SetError(err error)

	// End ends the span
	End()

	// context returns the span context
	context() context.Context
}

// Tracer provides a tracer
type Tracer interface {
	// Start starts a new span
	Start(name string) Span

	// StartWithParent starts a new span with a parent
	StartWithParent(parent Span, name string) Span
}

type TracerProvider interface {
	// NewTracer creates a new tracer
	NewTracer(namespace string) Tracer

	// Shutdown shuts down the tracer provider
	Shutdown() error
}
