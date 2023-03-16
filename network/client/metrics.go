package client

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type methodName string

// Metrics represents the grpc client metrics
type Metrics interface {
	// rpcMethodCallBegin is the duration of a grpc client call
	rpcMethodCallBegin(method methodName) time.Time

	// rpcMethodCallEnd is the duration of a grpc client call
	rpcMethodCallEnd(method methodName, begin time.Time)

	// GrpcClientCallCount is the count of a grpc client call
	rpcMethodCallCountInc(method methodName)

	// GrpcClientCallErrorCount is the count of a grpc client call error
	rpcMethodCallErrorCountInc(method methodName)
}

type metrics struct {
	// GrpcClientCallDurations is the duration of a grpc client call
	rpcMethodCallDurationVec *prometheus.HistogramVec

	// GrpcClientCallCount is the count of a grpc client call
	rpcMethodCallCount *prometheus.CounterVec

	// GrpcClientCallErrorCount is the count of a grpc client call error
	rpcMethodCallErrorCount *prometheus.CounterVec
}

func NewMetrics() Metrics {
	m := &metrics{
		rpcMethodCallDurationVec: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "grpc_client_call_durations_seconds",
				Help:    "The grpc client call latencies in seconds.",
				Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10),
			},
			[]string{"method"},
		),
		rpcMethodCallCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "grpc_client_call_count",
				Help: "The grpc client call count.",
			},
			[]string{"method"},
		),
		rpcMethodCallErrorCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "grpc_client_call_error_count",
				Help: "The grpc client call error count.",
			},
			[]string{"method"},
		),
	}

	prometheus.MustRegister(
		m.rpcMethodCallDurationVec,
		m.rpcMethodCallCount,
		m.rpcMethodCallErrorCount,
	)

	return m
}

func (m *metrics) rpcMethodCallBegin(method methodName) time.Time {
	return time.Now()
}

func (m *metrics) rpcMethodCallEnd(method methodName, begin time.Time) {
	if m.rpcMethodCallDurationVec != nil {
		m.rpcMethodCallDurationVec.WithLabelValues(string(method)).Observe(time.Since(begin).Seconds())
	}
}

func (m *metrics) rpcMethodCallCountInc(method methodName) {
	if m.rpcMethodCallCount != nil {
		m.rpcMethodCallCount.WithLabelValues(string(method)).Inc()
	}
}

func (m *metrics) rpcMethodCallErrorCountInc(method methodName) {
	if m.rpcMethodCallErrorCount != nil {
		m.rpcMethodCallErrorCount.WithLabelValues(string(method)).Inc()
	}
}

func NilMetrics() Metrics {
	return &metrics{}
}
