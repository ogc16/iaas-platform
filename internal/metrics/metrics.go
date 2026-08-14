// Package metrics implements a small, dependency-free Prometheus-compatible
// registry. It exposes counters, gauges, and histograms (with optional label
// sets) rendered in the Prometheus text exposition format, so operators can
// scrape the control plane without pulling in the full client_golang
// dependency.
package metrics

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// DefaultBuckets are the histogram bucket boundaries (in seconds) used for
// request latency unless overridden.
var DefaultBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type metricKind int

const (
	kindCounter metricKind = iota
	kindGauge
	kindHistogram
)

// Registry owns the metric families and serializes them in a stable order.
type Registry struct {
	mu        sync.Mutex
	metrics   []*Metric
	seenNames map[string]bool
}

func NewRegistry() *Registry {
	return &Registry{seenNames: map[string]bool{}}
}

// NewCounter registers a counter metric family. The labelNames are the label
// keys; each unique combination of label values becomes its own time series.
func (r *Registry) NewCounter(name, help string, labelNames []string) *Metric {
	return r.register(name, help, kindCounter, labelNames, nil)
}

// NewGauge registers a gauge metric family (a value that can go up and down).
func (r *Registry) NewGauge(name, help string, labelNames []string) *Metric {
	return r.register(name, help, kindGauge, labelNames, nil)
}

// NewHistogram registers a histogram metric family with the given bucket
// boundaries. A nil boundaries slice selects DefaultBuckets.
func (r *Registry) NewHistogram(name, help string, labelNames []string, buckets []float64) *Metric {
	if len(buckets) == 0 {
		buckets = DefaultBuckets
	}
	return r.register(name, help, kindHistogram, labelNames, buckets)
}

func (r *Registry) register(name, help string, kind metricKind, labelNames []string, buckets []float64) *Metric {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seenNames[name] {
		panic(fmt.Sprintf("metrics: metric %q already registered", name))
	}
	r.seenNames[name] = true
	m := &Metric{
		registry:   r,
		name:       name,
		help:       help,
		kind:       kind,
		labelNames: labelNames,
		buckets:    buckets,
		series:     map[string]*sample{},
	}
	r.metrics = append(r.metrics, m)
	return m
}

// Metric is a registered metric family. Use WithLabelValues to select (or
// lazily create) a time series, then Add/Set/Observe on the returned sample.
type Metric struct {
	registry   *Registry
	name       string
	help       string
	kind       metricKind
	labelNames []string
	buckets    []float64

	mu     sync.Mutex
	series map[string]*sample
}

// Sample is a single labelled time series of a metric family.
type Sample struct {
	m   *Metric
	key string
}

// WithLabelValues returns the sample for the given label values, creating it
// if it does not exist yet. The number of values must match the metric's
// label names.
func (m *Metric) WithLabelValues(values ...string) *Sample {
	if len(values) != len(m.labelNames) {
		panic(fmt.Sprintf("metrics: metric %q expects %d label values, got %d", m.name, len(m.labelNames), len(values)))
	}
	kv := make([]string, len(values))
	keyParts := make([]string, len(values))
	for i, v := range values {
		kv[i] = fmt.Sprintf("%s=\"%s\"", m.labelNames[i], escapeLabel(v))
		keyParts[i] = m.labelNames[i] + "=" + v
	}
	key := strings.Join(keyParts, ",")

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.series[key]; !ok {
		s := &sample{labelKV: kv}
		if m.kind == kindHistogram {
			s.bucketCounts = make([]uint64, len(m.buckets))
		}
		m.series[key] = s
	}
	return &Sample{m: m, key: key}
}

// Add increments a counter or gauge sample by delta.
func (s *Sample) Add(delta float64) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	s.m.series[s.key].value += delta
}

// Set records a gauge sample's current value.
func (s *Sample) Set(value float64) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	s.m.series[s.key].value = value
}

// Observe records one observation into a histogram sample.
func (s *Sample) Observe(value float64) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	se := s.m.series[s.key]
	se.sum += value
	se.count++
	for i, b := range s.m.buckets {
		if value <= b {
			se.bucketCounts[i]++
		}
	}
}

type sample struct {
	labelKV      []string
	value        float64
	sum          float64
	count        uint64
	bucketCounts []uint64
}

// Write renders every metric family in the registry to w in the Prometheus
// text exposition format (version 0.0.4).
func (r *Registry) Write(w io.Writer) error {
	r.mu.Lock()
	metrics := append([]*Metric(nil), r.metrics...)
	r.mu.Unlock()

	for _, m := range metrics {
		if err := m.write(w); err != nil {
			return err
		}
	}
	return nil
}

func (m *Metric) write(w io.Writer) error {
	m.mu.Lock()
	samples := make([]*sample, 0, len(m.series))
	keys := make([]string, 0, len(m.series))
	for k, s := range m.series {
		samples = append(samples, s)
		keys = append(keys, k)
	}
	m.mu.Unlock()

	sort.Slice(samples, func(i, j int) bool { return keys[i] < keys[j] })

	fmt.Fprintf(w, "# HELP %s %s\n", m.name, m.help)
	if m.kind == kindHistogram {
		fmt.Fprintf(w, "# TYPE %s histogram\n", m.name)
		for _, s := range samples {
			for i, b := range m.buckets {
				fmt.Fprintf(w, "%s_bucket{%sle=\"%s\"} %d\n", m.name, m.labelString(s), formatFloat(b), s.bucketCounts[i])
			}
			fmt.Fprintf(w, "%s_bucket{%sle=\"+Inf\"} %d\n", m.name, m.labelString(s), s.count)
			fmt.Fprintf(w, "%s_sum%s %s\n", m.name, m.labelString(s), formatFloat(s.sum))
			fmt.Fprintf(w, "%s_count%s %d\n", m.name, m.labelString(s), s.count)
		}
		return nil
	}

	typ := "gauge"
	if m.kind == kindCounter {
		typ = "counter"
	}
	fmt.Fprintf(w, "# TYPE %s %s\n", m.name, typ)
	for _, s := range samples {
		fmt.Fprintf(w, "%s%s %s\n", m.name, m.labelString(s), formatFloat(s.value))
	}
	return nil
}

func (m *Metric) labelString(s *sample) string {
	if len(s.labelKV) == 0 {
		return ""
	}
	return "{" + strings.Join(s.labelKV, ",") + "}"
}

func formatFloat(v float64) string {
	return fmt.Sprintf("%g", v)
}

// escapeLabel escapes a Prometheus label value per the exposition format:
// backslashes, double quotes, and newlines.
func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return v
}
