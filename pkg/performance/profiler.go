package performance

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// Profiler 性能分析器
type Profiler struct {
	mu          sync.RWMutex
	snapshots   []Snapshot
	maxSnapshots int
	interval    time.Duration
	stopCh      chan struct{}
	running     bool
}

// ProfilerConfig 分析器配置
type ProfilerConfig struct {
	MaxSnapshots int           // 最大快照数
	Interval     time.Duration // 采集间隔
}

// NewProfiler 创建分析器
func NewProfiler(cfg ProfilerConfig) *Profiler {
	if cfg.MaxSnapshots <= 0 {
		cfg.MaxSnapshots = 100
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}

	return &Profiler{
		snapshots:    make([]Snapshot, 0, cfg.MaxSnapshots),
		maxSnapshots: cfg.MaxSnapshots,
		interval:     cfg.Interval,
		stopCh:       make(chan struct{}),
	}
}

// Snapshot 快照
type Snapshot struct {
	Timestamp    time.Time      `json:"timestamp"`
	MemStats     MemStats       `json:"mem_stats"`
	Goroutines   int            `json:"goroutines"`
	GCStats      GCStats        `json:"gc_stats"`
}

// MemStats 内存统计
type MemStats struct {
	Alloc        uint64 `json:"alloc"`         // 当前分配的内存
	TotalAlloc   uint64 `json:"total_alloc"`   // 累计分配的内存
	Sys          uint64 `json:"sys"`           // 系统内存
	HeapAlloc    uint64 `json:"heap_alloc"`    // 堆内存分配
	HeapSys      uint64 `json:"heap_sys"`      // 堆系统内存
	HeapIdle     uint64 `json:"heap_idle"`     // 堆空闲内存
	HeapInuse    uint64 `json:"heap_inuse"`    // 堆使用中内存
	HeapReleased uint64 `json:"heap_released"` // 释放的堆内存
}

// GCStats GC 统计
type GCStats struct {
	NumGC        uint32        `json:"num_gc"`         // GC 次数
	PauseTotal   time.Duration `json:"pause_total"`    // GC 总暂停时间
	LastGC       time.Time     `json:"last_gc"`        // 上次 GC 时间
	Pause        time.Duration `json:"pause"`          // 最近一次 GC 暂停时间
}

// Start 启动分析器
func (p *Profiler) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	go p.collect(ctx)
}

// Stop 停止分析器
func (p *Profiler) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	close(p.stopCh)
	p.running = false
}

func (p *Profiler) collect(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.takeSnapshot()
		}
	}
}

func (p *Profiler) takeSnapshot() {
	p.mu.Lock()
	defer p.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	snapshot := Snapshot{
		Timestamp:  time.Now(),
		Goroutines: runtime.NumGoroutine(),
		MemStats: MemStats{
			Alloc:        m.Alloc,
			TotalAlloc:   m.TotalAlloc,
			Sys:          m.Sys,
			HeapAlloc:    m.HeapAlloc,
			HeapSys:      m.HeapSys,
			HeapIdle:     m.HeapIdle,
			HeapInuse:    m.HeapInuse,
			HeapReleased: m.HeapReleased,
		},
		GCStats: GCStats{
			NumGC:      m.NumGC,
			PauseTotal: time.Duration(m.PauseTotalNs),
			LastGC:     time.Unix(0, int64(m.LastGC)),
			Pause:      time.Duration(m.PauseNs[(m.NumGC+255)%256]),
		},
	}

	// 限制快照数量
	if len(p.snapshots) >= p.maxSnapshots {
		p.snapshots = p.snapshots[1:]
	}

	p.snapshots = append(p.snapshots, snapshot)
}

// GetSnapshots 获取所有快照
func (p *Profiler) GetSnapshots() []Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]Snapshot, len(p.snapshots))
	copy(result, p.snapshots)
	return result
}

// GetLatestSnapshot 获取最新快照
func (p *Profiler) GetLatestSnapshot() *Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.snapshots) == 0 {
		return nil
	}

	snapshot := p.snapshots[len(p.snapshots)-1]
	return &snapshot
}

// GetStats 获取统计信息
func (p *Profiler) GetStats() ProfilerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.snapshots) == 0 {
		return ProfilerStats{}
	}

	stats := ProfilerStats{
		SnapshotCount: len(p.snapshots),
		StartTime:     p.snapshots[0].Timestamp,
		EndTime:       p.snapshots[len(p.snapshots)-1].Timestamp,
	}

	// 计算内存统计
	var totalAlloc, totalHeapAlloc, totalHeapInuse uint64
	var maxGoroutines int
	var totalGCPause time.Duration

	for _, s := range p.snapshots {
		totalAlloc += s.MemStats.Alloc
		totalHeapAlloc += s.MemStats.HeapAlloc
		totalHeapInuse += s.MemStats.HeapInuse
		totalGCPause += s.GCStats.Pause

		if s.Goroutines > maxGoroutines {
			maxGoroutines = s.Goroutines
		}
	}

	n := uint64(len(p.snapshots))
	stats.AvgAlloc = totalAlloc / n
	stats.AvgHeapAlloc = totalHeapAlloc / n
	stats.AvgHeapInuse = totalHeapInuse / n
	stats.MaxGoroutines = maxGoroutines
	stats.TotalGCPause = totalGCPause

	// 最新快照
	latest := p.snapshots[len(p.snapshots)-1]
	stats.CurrentGoroutines = latest.Goroutines
	stats.CurrentAlloc = latest.MemStats.Alloc
	stats.CurrentHeapAlloc = latest.MemStats.HeapAlloc

	return stats
}

// ProfilerStats 分析器统计
type ProfilerStats struct {
	SnapshotCount      int           `json:"snapshot_count"`
	StartTime          time.Time     `json:"start_time"`
	EndTime            time.Time     `json:"end_time"`
	AvgAlloc           uint64        `json:"avg_alloc"`
	AvgHeapAlloc       uint64        `json:"avg_heap_alloc"`
	AvgHeapInuse       uint64        `json:"avg_heap_inuse"`
	MaxGoroutines      int           `json:"max_goroutines"`
	TotalGCPause       time.Duration `json:"total_gc_pause"`
	CurrentGoroutines  int           `json:"current_goroutines"`
	CurrentAlloc       uint64        `json:"current_alloc"`
	CurrentHeapAlloc   uint64        `json:"current_heap_alloc"`
}

// ForceGC 强制 GC
func ForceGC() {
	runtime.GC()
}

// GetMemStats 获取内存统计
func GetMemStats() MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemStats{
		Alloc:        m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		Sys:          m.Sys,
		HeapAlloc:    m.HeapAlloc,
		HeapSys:      m.HeapSys,
		HeapIdle:     m.HeapIdle,
		HeapInuse:    m.HeapInuse,
		HeapReleased: m.HeapReleased,
	}
}

// GetGoroutines 获取 goroutine 数量
func GetGoroutines() int {
	return runtime.NumGoroutine()
}
