package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// nilSpan
type nilSpan struct {
}

// SetAttribute sets an attribute
func (s *nilSpan) SetAttribute(key string, value interface{}) {
}

// SetAttributes sets attributes
func (s *nilSpan) SetAttributes(attributes map[string]interface{}) {
}

func (s *nilSpan) AddEvent(name string, attributes map[string]interface{}) {
}

func (s *nilSpan) SetStatus(code Code, info string) {
}

// SetError sets the error
func (s *nilSpan) SetError(err error) {
}

// End ends the span
func (s *nilSpan) End() {
}

func (s *nilSpan) SpanContext() trace.SpanContext {
	return trace.SpanContext{}
}

func (s *nilSpan) Context() context.Context {
	return context.Background()
}

// nilTracer
type nilTracer struct {
	provider *nilTracerProvider
}

// Start starts a new span
func (t *nilTracer) Start(name string) Span {
	return &nilSpan{}
}

// StartWithParent starts a new span with a parent
func (t *nilTracer) StartWithParent(parent trace.SpanContext, name string) Span {
	return &nilSpan{}
}

func (t *nilTracer) StartWithParentFromContext(ctx context.Context, name string) Span {
	return &nilSpan{}
}

func (t *nilTracer) GetTraceProvider() TracerProvider {
	return t.provider
}

// nilTracerProvider
type nilTracerProvider struct {
}

// NewTracer creates a new tracer
func (p *nilTracerProvider) NewTracer(namespace string) Tracer {
	return &nilTracer{
		provider: p,
	}
}

// Shutdown shuts down the tracer provider
func (p *nilTracerProvider) Shutdown(ctx context.Context) error {
	return nil
}

// NewNilTracerProvider creates a new trace provider
func NewNilTracerProvider(ctx context.Context) TracerProvider {
	return &nilTracerProvider{}
}
