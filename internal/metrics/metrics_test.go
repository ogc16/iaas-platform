package metrics

import (
	"bytes"
	"strings"
	"testing"
)

func TestCounterIncrements(t *testing.T) {
	reg := NewRegistry()
	c := reg.NewCounter("iaas_test_total", "test counter", []string{"method"})
	c.WithLabelValues("GET").Add(1)
	c.WithLabelValues("GET").Add(2)
	c.WithLabelValues("POST").Add(1)

	var buf bytes.Buffer
	if err := reg.Write(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`iaas_test_total{method="GET"} 3`,
		`iaas_test_total{method="POST"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestGaugeSet(t *testing.T) {
	reg := NewRegistry()
	g := reg.NewGauge("iaas_test_gauge", "test gauge", nil)
	g.WithLabelValues().Set(42)
	g.WithLabelValues().Set(-1)

	var buf bytes.Buffer
	_ = reg.Write(&buf)
	if got := buf.String(); !strings.Contains(got, "iaas_test_gauge -1") {
		t.Errorf("expected updated gauge value, got:\n%s", got)
	}
}

func TestHistogramBuckets(t *testing.T) {
	reg := NewRegistry()
	h := reg.NewHistogram("iaas_test_duration_seconds", "test duration", nil, []float64{0.1, 0.5, 1})
	h.WithLabelValues().Observe(0.05)
	h.WithLabelValues().Observe(0.3)
	h.WithLabelValues().Observe(2.0)

	var buf bytes.Buffer
	_ = reg.Write(&buf)
	out := buf.String()
	for _, want := range []string{
		"iaas_test_duration_seconds_bucket{le=\"0.1\"} 1",
		"iaas_test_duration_seconds_bucket{le=\"0.5\"} 2",
		"iaas_test_duration_seconds_bucket{le=\"1\"} 2",
		"iaas_test_duration_seconds_bucket{le=\"+Inf\"} 3",
		"iaas_test_duration_seconds_sum 2.35",
		"iaas_test_duration_seconds_count 3",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

func TestDuplicateNamePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate metric name")
		}
	}()
	reg := NewRegistry()
	reg.NewCounter("iaas_test_dup_total", "one", nil)
	reg.NewCounter("iaas_test_dup_total", "two", nil)
}

func TestLabelEscape(t *testing.T) {
	reg := NewRegistry()
	c := reg.NewCounter("iaas_test_escape_total", "escape", []string{"route"})
	c.WithLabelValues(`/a"b\c`).Add(1)

	var buf bytes.Buffer
	_ = reg.Write(&buf)
	if got := buf.String(); !strings.Contains(got, `route="/a\"b\\c"`) {
		t.Errorf("expected escaped label, got:\n%s", got)
	}
}
