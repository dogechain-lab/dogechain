package telemetry

import (
	"context"
	"os"

	"github.com/dogechain-lab/dogechain/helper/common"
	"github.com/dogechain-lab/dogechain/versioning"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

const (
	JaegerContextName contextValue = "jaeger"
)

// newJaegerProvider creates a new jaeger provider
func newJaegerProvider(url string, service string) (*tracesdk.TracerProvider, error) {
	hostname, err := os.Hostname()
	if err != nil {
		// get ip address
		ip, err := common.GetOutboundIP()
		if err != nil {
			hostname = "unknown"
		} else {
			hostname = ip.String()
		}
	}

	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}

	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
			attribute.String("hostname", hostname),
			attribute.String("version", versioning.Version),
			attribute.String("commit", common.Substr(versioning.Commit, 0, 8)),
			attribute.String("buildTime", versioning.BuildTime),
		)),
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
	)

	return tp, nil
}

// jaegerSpan
type jaegerSpan struct {
	// span
	span trace.Span

	// context
	ctx context.Context
}

// SetAttribute sets an attribute
func (s *jaegerSpan) SetAttribute(key string, value interface{}) {
	s.span.SetAttributes(attribute.KeyValue{
		Key:   attribute.Key(key),
		Value: convertTypeToAttribute(value),
	})
}

// SetAttributes sets attributes
func (s *jaegerSpan) SetAttributes(attributes map[string]interface{}) {
	kvs := make([]attribute.KeyValue, 0, len(attributes))
	for key, value := range attributes {
		kvs = append(kvs, attribute.KeyValue{
			Key:   attribute.Key(key),
			Value: convertTypeToAttribute(value),
		})
	}

	s.span.SetAttributes(kvs...)
}

func (s *jaegerSpan) AddEvent(name string, attributes map[string]interface{}) {
	kvs := make([]attribute.KeyValue, 0, len(attributes))
	for key, value := range attributes {
		kvs = append(kvs, attribute.KeyValue{
			Key:   attribute.Key(key),
			Value: convertTypeToAttribute(value),
		})
	}

	s.span.AddEvent(name, trace.WithAttributes(kvs...))
}

func (s *jaegerSpan) SetStatus(code Code, info string) {
	s.span.SetStatus(codes.Code(code), info)
}

func (s *jaegerSpan) RecordError(err error) {
	s.span.RecordError(err)
}

func (s *jaegerSpan) End() {
	s.span.End()
}

// SpanContext returns the span context
func (s *jaegerSpan) SpanContext() trace.SpanContext {
	return s.span.SpanContext()
}

// Context returns the span context
func (s *jaegerSpan) Context() context.Context {
	ctx := trace.ContextWithSpanContext(s.ctx, s.SpanContext())

	return ctx
}

// jaegerTracer
type jaegerTracer struct {
	// context
	context context.Context

	// tracer
	tracer trace.Tracer

	// provider
	provider *jaegerTracerProvider
}

// Start starts a new span
func (t *jaegerTracer) Start(name string) Span {
	ctx, span := t.tracer.Start(t.context, name)

	return &jaegerSpan{
		span: span,
		ctx:  ctx,
	}
}

// StartWithParent starts a new span with a parent
func (t *jaegerTracer) StartWithParent(parent trace.SpanContext, name string) Span {
	// create a new span context config
	spanContextConfig := trace.SpanContextConfig{
		TraceID:    parent.TraceID(),
		SpanID:     parent.SpanID(),
		TraceFlags: parent.TraceFlags(),
		Remote:     parent.IsRemote(),
	}
	spanContext := trace.NewSpanContext(spanContextConfig)

	// create a new context with the parent span context
	ctx := trace.ContextWithSpanContext(t.context, spanContext)

	// create a new span
	childContext, span := t.tracer.Start(ctx, name)

	return &jaegerSpan{
		span: span,
		ctx:  childContext,
	}
}

// StartWithContext starts a new span with a parent from the context
func (t *jaegerTracer) StartWithContext(ctx context.Context, name string) Span {
	childContext, span := t.tracer.Start(ctx, name)

	return &jaegerSpan{
		span: span,
		ctx:  childContext,
	}
}

// GetTracerProvider returns the tracer provider
func (t *jaegerTracer) GetTraceProvider() TracerProvider {
	return t.provider
}

// jaegerTracerProvider
type jaegerTracerProvider struct {
	// context
	context context.Context

	// provider
	provider *tracesdk.TracerProvider
}

// NewTracer creates a new tracer
func (p *jaegerTracerProvider) NewTracer(namespace string) Tracer {
	return &jaegerTracer{
		context:  p.context,
		tracer:   p.provider.Tracer(namespace),
		provider: p,
	}
}

// Shutdown shuts down the tracer provider
func (p *jaegerTracerProvider) Shutdown(ctx context.Context) error {
	return p.provider.Shutdown(ctx)
}

// NewTracerProvider creates a new trace provider
func NewTracerProvider(ctx context.Context, url string, service string) (TracerProvider, error) {
	tp, err := newJaegerProvider(url, service)
	if err != nil {
		return nil, err
	}

	// Register TracerProvider
	otel.SetTracerProvider(tp)

	return &jaegerTracerProvider{
		context:  context.WithValue(ctx, JaegerContextName, JaegerContextName),
		provider: tp,
	}, nil
}
