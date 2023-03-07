package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/codes"
)

// NilSpan
type NilSpan struct {
	ctx context.Context
}

// SetAttribute sets an attribute
func (s *NilSpan) SetAttribute(key string, value interface{}) {
}

// SetAttributes sets attributes
func (s *NilSpan) SetAttributes(attributes map[string]interface{}) {
}

func (s *NilSpan) SetStatus(code codes.Code, info string) {
}

// SetError sets the error
func (s *NilSpan) SetError(err error) {
}

// End ends the span
func (s *NilSpan) End() {
}

// context returns the span context
func (s *NilSpan) context() context.Context {
	return s.ctx
}

// NilTracer
type NilTracer struct {
	// context
	context context.Context
}

// Start starts a new span
func (t *NilTracer) Start(name string) Span {
	return &NilSpan{
		ctx: t.context,
	}
}

// StartWithParent starts a new span with a parent
func (t *NilTracer) StartWithParent(parent Span, name string) Span {
	return &NilSpan{
		ctx: t.context,
	}
}

// NilTracerProvider
type NilTracerProvider struct {
	// context
	context context.Context
}

// NewTracer creates a new tracer
func (p *NilTracerProvider) NewTracer(namespace string) Tracer {
	return &NilTracer{
		context: p.context,
	}
}

// Shutdown shuts down the tracer provider
func (p *NilTracerProvider) Shutdown() error {
	return nil
}

// NewNilTracerProvider creates a new trace provider
func NewNilTracerProvider(url string, service string) (TracerProvider, error) {
	return &NilTracerProvider{
		context: context.Background(),
	}, nil
}
