package telemetry

import (
	"context"
)

const (
	NilContextName contextValue = "nil"
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

// SetError sets the error
func (s *nilSpan) SetError(err error) {
}

// End ends the span
func (s *nilSpan) End() {
}

// context returns the span context
func (s *nilSpan) context() context.Context {
	return s.ctx
}

// nilTracer
type nilTracer struct {
	// context
	context context.Context
}

// Start starts a new span
func (t *nilTracer) Start(name string) Span {
	return &nilSpan{
		ctx: t.context,
	}
}

// StartWithParent starts a new span with a parent
func (t *nilTracer) StartWithParent(parent Span, name string) Span {
	return &nilSpan{
		ctx: t.context,
	}
}

// nilTracerProvider
type nilTracerProvider struct {
	// context
	context context.Context
}

// NewTracer creates a new tracer
func (p *nilTracerProvider) NewTracer(namespace string) Tracer {
	return &nilTracer{
		context: p.context,
	}
}

// Shutdown shuts down the tracer provider
func (p *nilTracerProvider) Shutdown(ctx context.Context) error {
	return nil
}

// NewNilTracerProvider creates a new trace provider
func NewNilTracerProvider(ctx context.Context) TracerProvider {
	return &nilTracerProvider{
		context: context.WithValue(ctx, ContextNamespace, NilContextName),
	}
}
