package telemetry

import (
	"os"

	"github.com/dogechain-lab/dogechain/helper/common"
	"github.com/dogechain-lab/dogechain/versioning"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// NewTraceProvider creates a new trace provider
func NewJaegerProvider(url string, service string) (*tracesdk.TracerProvider, error) {
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
	)

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)

	return tp, nil
}
