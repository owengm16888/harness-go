package metrics

import (
	"fmt"
	"sync"
	"time"
)

// MetricType 指标类型
type MetricType string

const (
	MetricCounter   MetricType = "counter"
	MetricGauge     MetricType = "gauge"
	MetricHistogram MetricType = "histogram"
	MetricSummary   MetricType = "summary"
)

// Metric 指标
type Metric struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Help      string            `json:"help"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// Counter 计数器
type Counter struct {
	mu      sync.Mutex
	name    string
	help    string
	value   float64
	labels  map[string]string
}

// NewCounter 创建计数器
func NewCounter(name, help string) *Counter {
	return &Counter{
		name:   name,
		help:   help,
		labels: make(map[string]string),
	}
}

// Inc 增加
func (c *Counter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

// Add 增加指定值
func (c *Counter) Add(value float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value += value
}

// Get 获取值
func (c *Counter) Get() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// WithLabels 添加标签
func (c *Counter) WithLabels(labels map[string]string) *Counter {
	return &Counter{
		name:   c.name,
		help:   c.help,
		value:  c.value,
		labels: labels,
	}
}

// Gauge 仪表
type Gauge struct {
	mu      sync.Mutex
	name    string
	help    string
	value   float64
	labels  map[string]string
}

// NewGauge 创建仪表
func NewGauge(name, help string) *Gauge {
	return &Gauge{
		name:   name,
		help:   help,
		labels: make(map[string]string),
	}
}

// Set 设置值
func (g *Gauge) Set(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value = value
}

// Inc 增加
func (g *Gauge) Inc() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value++
}

// Dec 减少
func (g *Gauge) Dec() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value--
}

// Add 增加指定值
func (g *Gauge) Add(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value += value
}

// Sub 减少指定值
func (g *Gauge) Sub(value float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.value -= value
}

// Get 获取值
func (g *Gauge) Get() float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.value
}

// WithLabels 添加标签
func (g *Gauge) WithLabels(labels map[string]string) *Gauge {
	return &Gauge{
		name:   g.name,
		help:   g.help,
		value:  g.value,
		labels: labels,
	}
}

// Histogram 直方图
type Histogram struct {
	mu      sync.Mutex
	name    string
	help    string
	buckets []float64
	counts  []uint64
	sum     float64
	count   uint64
	labels  map[string]string
}

// NewHistogram 创建直方图
func NewHistogram(name, help string, buckets []float64) *Histogram {
	if len(buckets) == 0 {
		buckets = []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10}
	}

	return &Histogram{
		name:    name,
		help:    help,
		buckets: buckets,
		counts:  make([]uint64, len(buckets)+1),
		labels:  make(map[string]string),
	}
}

// Observe 观察
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += value
	h.count++

	for i, bucket := range h.buckets {
		if value <= bucket {
			h.counts[i]++
		}
	}
	h.counts[len(h.buckets)]++
}

// Get 获取统计
func (h *Histogram) Get() *HistogramData {
	h.mu.Lock()
	defer h.mu.Unlock()

	return &HistogramData{
		Name:    h.name,
		Help:    h.help,
		Buckets: h.buckets,
		Counts:  h.counts,
		Sum:     h.sum,
		Count:   h.count,
	}
}

// HistogramData 直方图数据
type HistogramData struct {
	Name    string    `json:"name"`
	Help    string    `json:"help"`
	Buckets []float64 `json:"buckets"`
	Counts  []uint64  `json:"counts"`
	Sum     float64   `json:"sum"`
	Count   uint64    `json:"count"`
}

// WithLabels 添加标签
func (h *Histogram) WithLabels(labels map[string]string) *Histogram {
	return &Histogram{
		name:    h.name,
		help:    h.help,
		buckets: h.buckets,
		counts:  h.counts,
		sum:     h.sum,
		count:   h.count,
		labels:  labels,
	}
}

// Registry 指标注册表
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*Counter
	gauges   map[string]*Gauge
	histograms map[string]*Histogram
	summaries  map[string]*Summary
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
		summaries:  make(map[string]*Summary),
	}
}

// RegisterCounter 注册计数器
func (r *Registry) RegisterCounter(counter *Counter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[counter.name] = counter
}

// RegisterGauge 注册仪表
func (r *Registry) RegisterGauge(gauge *Gauge) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.gauges[gauge.name] = gauge
}

// RegisterHistogram 注册直方图
func (r *Registry) RegisterHistogram(histogram *Histogram) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.histograms[histogram.name] = histogram
}

// RegisterSummary 注册摘要
func (r *Registry) RegisterSummary(summary *Summary) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.summaries[summary.name] = summary
}

// GetCounter 获取计数器
func (r *Registry) GetCounter(name string) (*Counter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.counters[name]
	return c, ok
}

// GetGauge 获取仪表
func (r *Registry) GetGauge(name string) (*Gauge, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.gauges[name]
	return g, ok
}

// GetHistogram 获取直方图
func (r *Registry) GetHistogram(name string) (*Histogram, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	h, ok := r.histograms[name]
	return h, ok
}

// GetSummary 获取摘要
func (r *Registry) GetSummary(name string) (*Summary, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.summaries[name]
	return s, ok
}

// GetAll 获取所有指标
func (r *Registry) GetAll() []Metric {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var metrics []Metric
	now := time.Now()

	for _, c := range r.counters {
		metrics = append(metrics, Metric{
			Name:      c.name,
			Type:      MetricCounter,
			Help:      c.help,
			Value:     c.value,
			Labels:    c.labels,
			Timestamp: now,
		})
	}

	for _, g := range r.gauges {
		metrics = append(metrics, Metric{
			Name:      g.name,
			Type:      MetricGauge,
			Help:      g.help,
			Value:     g.value,
			Labels:    g.labels,
			Timestamp: now,
		})
	}

	return metrics
}

// Collect 收集所有指标
func (r *Registry) Collect() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := map[string]any{
		"counters":   r.counters,
		"gauges":     r.gauges,
		"histograms": r.histograms,
		"summaries":  r.summaries,
	}

	return result
}

// Reset 重置所有指标
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.counters = make(map[string]*Counter)
	r.gauges = make(map[string]*Gauge)
	r.histograms = make(map[string]*Histogram)
	r.summaries = make(map[string]*Summary)
}

// Summary 摘要
type Summary struct {
	mu      sync.Mutex
	name    string
	help    string
	values  []float64
	sum     float64
	count   uint64
	labels  map[string]string
}

// NewSummary 创建摘要
func NewSummary(name, help string) *Summary {
	return &Summary{
		name:   name,
		help:   help,
		labels: make(map[string]string),
	}
}

// Observe 观察
func (s *Summary) Observe(value float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.values = append(s.values, value)
	s.sum += value
	s.count++
}

// Get 获取统计
func (s *Summary) Get() *SummaryData {
	s.mu.Lock()
	defer s.mu.Unlock()

	return &SummaryData{
		Name:  s.name,
		Help:  s.help,
		Sum:   s.sum,
		Count: s.count,
	}
}

// SummaryData 摘要数据
type SummaryData struct {
	Name  string  `json:"name"`
	Help  string  `json:"help"`
	Sum   float64 `json:"sum"`
	Count uint64  `json:"count"`
}

// WithLabels 添加标签
func (s *Summary) WithLabels(labels map[string]string) *Summary {
	return &Summary{
		name:   s.name,
		help:   s.help,
		sum:    s.sum,
		count:  s.count,
		labels: labels,
	}
}

// 指标收集器
var defaultRegistry = NewRegistry()

// GetDefaultRegistry 获取默认注册表
func GetDefaultRegistry() *Registry {
	return defaultRegistry
}

// 全局计数器
var (
	TaskCounter     = NewCounter("harness_tasks_total", "Total number of tasks")
	SuccessCounter  = NewCounter("harness_tasks_success_total", "Total number of successful tasks")
	FailureCounter  = NewCounter("harness_tasks_failure_total", "Total number of failed tasks")
	SessionCounter  = NewCounter("harness_sessions_total", "Total number of sessions")
	RequestCounter  = NewCounter("harness_requests_total", "Total number of requests")
	ErrorCounter    = NewCounter("harness_errors_total", "Total number of errors")
)

// 全局仪表
var (
	ActiveTasksGauge    = NewGauge("harness_active_tasks", "Number of active tasks")
	ActiveSessionsGauge = NewGauge("harness_active_sessions", "Number of active sessions")
	QueueSizeGauge      = NewGauge("harness_queue_size", "Size of task queue")
	MemoryUsageGauge    = NewGauge("harness_memory_usage_bytes", "Memory usage in bytes")
)

// 全局直方图
var (
	TaskDurationHistogram = NewHistogram("harness_task_duration_seconds", "Task duration in seconds",
		[]float64{0.1, 0.5, 1, 5, 10, 30, 60, 300})
	RequestDurationHistogram = NewHistogram("harness_request_duration_seconds", "Request duration in seconds",
		[]float64{0.01, 0.05, 0.1, 0.5, 1, 5})
)

// 全局摘要
var (
	TaskSizeSummary = NewSummary("harness_task_size_bytes", "Task size in bytes")
)

func init() {
	// 注册全局指标
	defaultRegistry.RegisterCounter(TaskCounter)
	defaultRegistry.RegisterCounter(SuccessCounter)
	defaultRegistry.RegisterCounter(FailureCounter)
	defaultRegistry.RegisterCounter(SessionCounter)
	defaultRegistry.RegisterCounter(RequestCounter)
	defaultRegistry.RegisterCounter(ErrorCounter)

	defaultRegistry.RegisterGauge(ActiveTasksGauge)
	defaultRegistry.RegisterGauge(ActiveSessionsGauge)
	defaultRegistry.RegisterGauge(QueueSizeGauge)
	defaultRegistry.RegisterGauge(MemoryUsageGauge)

	defaultRegistry.RegisterHistogram(TaskDurationHistogram)
	defaultRegistry.RegisterHistogram(RequestDurationHistogram)

	defaultRegistry.RegisterSummary(TaskSizeSummary)
}

// FormatMetrics 格式化指标
func FormatMetrics(metrics []Metric) string {
	result := ""
	for _, m := range metrics {
		result += fmt.Sprintf("# HELP %s %s\n", m.Name, m.Help)
		result += fmt.Sprintf("# TYPE %s %s\n", m.Name, m.Type)
		result += fmt.Sprintf("%s %f\n", m.Name, m.Value)
	}
	return result
}
