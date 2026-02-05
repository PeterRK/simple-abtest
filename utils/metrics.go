package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func RecordLatencyUs(s prometheus.Observer, begin time.Time) {
	s.Observe(float64(time.Since(begin)) / float64(time.Microsecond))
}

var summaryObjectives = map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001}

func NewSummaryVec(name string, labelNames ...string) *prometheus.SummaryVec {
	return prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       name,
			Objectives: summaryObjectives,
		}, labelNames)
}

func NewCounterVec(name string, labelNames ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: name,
		}, labelNames)
}

func NewGaugeVec(name string, labelNames ...string) *prometheus.GaugeVec {
	return prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: name,
		}, labelNames)
}

func NewSummary(name string) prometheus.Summary {
	return prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name:       name,
			Objectives: summaryObjectives,
		})
}

func NewCounter(name string) prometheus.Counter {
	return prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: name,
		})
}

func NewGauge(name string) prometheus.Gauge {
	return prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: name,
		})
}

type ActionMetrics struct {
	Count   prometheus.Counter
	Failure prometheus.Counter
	Latency prometheus.Summary
}

func (m *ActionMetrics) Init(prefix string) {
	m.Count = NewCounter(prefix + "count")
	m.Failure = NewCounter(prefix + "failure")
	m.Latency = NewSummary(prefix + "latency_us")
}

func (m *ActionMetrics) MustRegister(registry *prometheus.Registry) {
	registry.MustRegister(m.Count)
	registry.MustRegister(m.Failure)
	registry.MustRegister(m.Latency)
}

func (m *ActionMetrics) Register(registry *prometheus.Registry) {
	registry.Register(m.Count)
	registry.Register(m.Failure)
	registry.Register(m.Latency)
}

var (
	ptnMetrisName        = regexp.MustCompile("^[a-z][a-z0-9_]*$")
	errIllegalMetrisName = errors.New("illegal metrics name")
)

func MaybeMetrisName(name string) bool {
	return ptnMetrisName.MatchString(name)
}

func TryToConvertMetrisName(name string) (string, error) {
	name = strings.ToLower(strings.ReplaceAll(name, "-", "_"))
	if MaybeMetrisName(name) {
		return name, nil
	}
	return "", errIllegalMetrisName
}

func TryRegisterMetris(r *prometheus.Registry, m prometheus.Collector) {
	if err := r.Register(m); err != nil {
		fmt.Printf("fail to register metrics: %v\n", err)
	}
}
