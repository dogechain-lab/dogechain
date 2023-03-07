package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/codes"
)

type contextLabel string
type contextValue string

const (
	ContextNamespace contextLabel = "telemetry"
)

type Code codes.Code

const (
	// Unset is the default status code
	Unset Code = Code(codes.Unset)

	// Error indicates the operation contains an error
	Error Code = Code(codes.Error)

	// Ok indicates operation has been validated by an Application developers
	Ok Code = Code(codes.Ok)
)

type Span interface {
	// SetAttribute sets an attribute (base type)
	SetAttribute(label string, value interface{})

	// SetAttributes sets attributes
	SetAttributes(attributes map[string]interface{})

	// AddEvent adds an event
	AddEvent(name string, attributes map[string]interface{})

	// SetStatus sets the status
	SetStatus(code Code, info string)

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
	Shutdown(context.Context) error
}
