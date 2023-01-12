package metrics

import "github.com/prometheus/client_golang/prometheus"

// helper function

func ParseLables(labelsWithValues ...string) prometheus.Labels {
	constLabels := map[string]string{}

	if len(labelsWithValues)%2 == 0 {
		for i := 1; i < len(labelsWithValues); i += 2 {
			constLabels[labelsWithValues[i-1]] = labelsWithValues[i]
		}
	} else {
		panic("invalid labels")
	}

	return constLabels
}

func CounterInc(counter prometheus.Counter) {
	if counter == nil {
		return
	}

	counter.Inc()
}

func SetGauge(gauge prometheus.Gauge, v float64) {
	if gauge == nil {
		return
	}

	gauge.Set(v)
}

func HistogramObserve(histogram prometheus.Histogram, v float64) {
	if histogram == nil {
		return
	}

	histogram.Observe(v)
}
