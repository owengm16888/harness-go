package metrics

import (
	"testing"
)

func TestCounter_Inc(t *testing.T) {
	counter := NewCounter("test_counter", "Test counter")

	counter.Inc()
	if counter.Get() != 1 {
		t.Errorf("Expected 1, got %f", counter.Get())
	}

	counter.Inc()
	if counter.Get() != 2 {
		t.Errorf("Expected 2, got %f", counter.Get())
	}
}

func TestCounter_Add(t *testing.T) {
	counter := NewCounter("test_counter", "Test counter")

	counter.Add(5)
	if counter.Get() != 5 {
		t.Errorf("Expected 5, got %f", counter.Get())
	}

	counter.Add(3)
	if counter.Get() != 8 {
		t.Errorf("Expected 8, got %f", counter.Get())
	}
}

func TestGauge_Set(t *testing.T) {
	gauge := NewGauge("test_gauge", "Test gauge")

	gauge.Set(10)
	if gauge.Get() != 10 {
		t.Errorf("Expected 10, got %f", gauge.Get())
	}

	gauge.Set(20)
	if gauge.Get() != 20 {
		t.Errorf("Expected 20, got %f", gauge.Get())
	}
}

func TestGauge_IncDec(t *testing.T) {
	gauge := NewGauge("test_gauge", "Test gauge")

	gauge.Set(10)

	gauge.Inc()
	if gauge.Get() != 11 {
		t.Errorf("Expected 11, got %f", gauge.Get())
	}

	gauge.Dec()
	if gauge.Get() != 10 {
		t.Errorf("Expected 10, got %f", gauge.Get())
	}
}

func TestGauge_AddSub(t *testing.T) {
	gauge := NewGauge("test_gauge", "Test gauge")

	gauge.Set(10)

	gauge.Add(5)
	if gauge.Get() != 15 {
		t.Errorf("Expected 15, got %f", gauge.Get())
	}

	gauge.Sub(3)
	if gauge.Get() != 12 {
		t.Errorf("Expected 12, got %f", gauge.Get())
	}
}

func TestHistogram_Observe(t *testing.T) {
	histogram := NewHistogram("test_histogram", "Test histogram", []float64{1, 5, 10})

	histogram.Observe(0.5)
	histogram.Observe(3)
	histogram.Observe(7)
	histogram.Observe(15)

	data := histogram.Get()

	if data.Count != 4 {
		t.Errorf("Expected 4 observations, got %d", data.Count)
	}

	if data.Sum != 25.5 {
		t.Errorf("Expected sum 25.5, got %f", data.Sum)
	}
}

func TestRegistry_RegisterCounter(t *testing.T) {
	registry := NewRegistry()

	counter := NewCounter("test_counter", "Test counter")
	registry.RegisterCounter(counter)

	retrieved, exists := registry.GetCounter("test_counter")
	if !exists {
		t.Error("Expected counter to exist")
	}

	if retrieved != counter {
		t.Error("Expected same counter instance")
	}
}

func TestRegistry_RegisterGauge(t *testing.T) {
	registry := NewRegistry()

	gauge := NewGauge("test_gauge", "Test gauge")
	registry.RegisterGauge(gauge)

	retrieved, exists := registry.GetGauge("test_gauge")
	if !exists {
		t.Error("Expected gauge to exist")
	}

	if retrieved != gauge {
		t.Error("Expected same gauge instance")
	}
}

func TestRegistry_GetAll(t *testing.T) {
	registry := NewRegistry()

	counter := NewCounter("test_counter", "Test counter")
	gauge := NewGauge("test_gauge", "Test gauge")

	registry.RegisterCounter(counter)
	registry.RegisterGauge(gauge)

	metrics := registry.GetAll()

	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics, got %d", len(metrics))
	}
}

func TestRegistry_Reset(t *testing.T) {
	registry := NewRegistry()

	counter := NewCounter("test_counter", "Test counter")
	gauge := NewGauge("test_gauge", "Test gauge")

	registry.RegisterCounter(counter)
	registry.RegisterGauge(gauge)

	registry.Reset()

	metrics := registry.GetAll()
	if len(metrics) != 0 {
		t.Errorf("Expected 0 metrics after reset, got %d", len(metrics))
	}
}

func TestDefaultRegistry(t *testing.T) {
	registry := GetDefaultRegistry()

	if registry == nil {
		t.Error("Expected default registry to exist")
	}

	// 检查全局指标是否已注册
	_, exists := registry.GetCounter("harness_tasks_total")
	if !exists {
		t.Error("Expected harness_tasks_total counter to exist")
	}

	_, exists = registry.GetGauge("harness_active_tasks")
	if !exists {
		t.Error("Expected harness_active_tasks gauge to exist")
	}
}
