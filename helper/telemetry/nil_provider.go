package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// nilSpan
type nilSpan struct {
	ctx context.Context
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

func (s *nilSpan) RecordError(err error) {
}

// End ends the span
func (s *nilSpan) End() {
}

func (s *nilSpan) SpanContext() trace.SpanContext {
	return trace.SpanContext{}
}

func (s *nilSpan) Context() context.Context {
	return s.ctx
}

// nilTracer
type nilTracer struct {
	provider *nilTracerProvider
	ctx      context.Context
}

// Start starts a new span
func (t *nilTracer) Start(name string) Span {
	return &nilSpan{
		ctx: t.ctx,
	}
}

// StartWithParent starts a new span with a parent
func (t *nilTracer) StartWithParent(parent trace.SpanContext, name string) Span {
	return &nilSpan{}
}

func (t *nilTracer) StartWithContext(ctx context.Context, name string) Span {
	return &nilSpan{}
}

func (t *nilTracer) GetTraceProvider() TracerProvider {
	return t.provider
}

// nilTracerProvider
type nilTracerProvider struct {
	ctx context.Context
}

// NewTracer creates a new tracer
func (p *nilTracerProvider) NewTracer(namespace string) Tracer {
	return &nilTracer{
		provider: p,
		ctx:      p.ctx,
	}
}

// Shutdown shuts down the tracer provider
func (p *nilTracerProvider) Shutdown(ctx context.Context) error {
	return nil
}

// NewNilTracerProvider creates a new trace provider
func NewNilTracerProvider(ctx context.Context) TracerProvider {
	return &nilTracerProvider{
		ctx,
	}
}
