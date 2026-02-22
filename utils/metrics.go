package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// RecordLatencyUs observes the elapsed time since begin in microseconds.
func RecordLatencyUs(s prometheus.Observer, begin time.Time) {
	s.Observe(float64(time.Since(begin)) / float64(time.Microsecond))
}

var summaryObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}

// NewSummaryVec creates a SummaryVec with preset objectives.
func NewSummaryVec(name string, labelNames ...string) *prometheus.SummaryVec {
	return prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       name,
			Objectives: summaryObjectives,
		}, labelNames)
}

// NewCounterVec creates a CounterVec with the given metric name.
func NewCounterVec(name string, labelNames ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
		}, labelNames)
}

// NewGaugeVec creates a GaugeVec with the given metric name.
func NewGaugeVec(name string, labelNames ...string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
		}, labelNames)
}

// NewSummary creates a Summary with preset objectives.
func NewSummary(name string) prometheus.Summary {
	return prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name:       name,
			Objectives: summaryObjectives,
		})
}

// NewCounter creates a Counter with the given metric name.
func NewCounter(name string) prometheus.Counter {
	return prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: name,
		})
}

// NewGauge creates a Gauge with the given metric name.
func NewGauge(name string) prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: name,
		})
}

// ActionMetrics bundles common metrics for counting, failures and latency.
type ActionMetrics struct {
	Count   prometheus.Counter
	Failure prometheus.Counter
	Latency prometheus.Summary
}

// Init initializes metrics with the given name prefix.
func (m *ActionMetrics) Init(prefix string) {
	m.Count = NewCounter(prefix + "count")
	m.Failure = NewCounter(prefix + "failure")
	m.Latency = NewSummary(prefix + "latency_us")
}

// MustRegister registers metrics in the registry and panics on failure.
func (m *ActionMetrics) MustRegister(registry *prometheus.Registry) {
	registry.MustRegister(m.Count)
	registry.MustRegister(m.Failure)
	registry.MustRegister(m.Latency)
}

// Register registers metrics in the registry and ignores registration errors.
func (m *ActionMetrics) Register(registry *prometheus.Registry) {
	registry.Register(m.Count)
	registry.Register(m.Failure)
	registry.Register(m.Latency)
}

var (
	ptnMetrisName        = regexp.MustCompile("^[a-z][a-z0-9_]*$")
	errIllegalMetrisName = errors.New("illegal metrics name")
)

// MaybeMetrisName reports whether name is a valid Prometheus metric name.
func MaybeMetrisName(name string) bool {
	return ptnMetrisName.MatchString(name)
}

// TryToConvertMetrisName converts a string into a Prometheus metric name if possible.
func TryToConvertMetrisName(name string) (string, error) {
	name = strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	if MaybeMetrisName(name) {
		return name, nil
	}
	return "", errIllegalMetrisName
}

// TryRegisterMetris registers a collector and logs the error instead of failing.
func TryRegisterMetris(r *prometheus.Registry, m prometheus.Collector) {
	if err := r.Register(m); err != nil {
		fmt.Printf("fail to register metrics: %v\n", err)
	}
}
