package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
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
	// SetAttribute set attribute (base type)
	SetAttribute(label string, value interface{})

	// SetAttributes set attributes
	SetAttributes(attributes map[string]interface{})

	// AddEvent adds an event
	AddEvent(name string, attributes map[string]interface{})

	// SetStatus set status
	SetStatus(code Code, info string)

	// RecordError will record err as an exception span event for this span. An
	// additional call to SetStatus is required if the Status of the Span should
	// be set to Error, as this method does not change the Span status. If this
	// span is not being recorded or err is nil then this method does nothing.
	// (wrapping otel.Span.RecordError)
	RecordError(err error)

	// End ends the span
	End()

	// SpanContext returns the span context
	SpanContext() trace.SpanContext

	// Context returns the context.Context (span warrapped)
	Context() context.Context
}

// Tracer provides a tracer
type Tracer interface {
	// Start starts a new span
	Start(name string) Span

	// StartWithParent starts a new span with a parent
	StartWithParent(parent trace.SpanContext, name string) Span

	// StartWithContext starts a new span with a parent from context
	StartWithContext(ctx context.Context, name string) Span

	// GetTraceProvider returns the trace provider
	GetTraceProvider() TracerProvider
}

type TracerProvider interface {
	// NewTracer creates a new tracer
	NewTracer(namespace string) Tracer

	// Shutdown shuts down the tracer provider
	Shutdown(context.Context) error
}
