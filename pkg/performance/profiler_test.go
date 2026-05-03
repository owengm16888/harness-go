package performance

import (
	"context"
	"testing"
	"time"
)

func TestProfiler_StartStop(t *testing.T) {
	profiler := NewProfiler(ProfilerConfig{
		MaxSnapshots: 10,
		Interval:     100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动
	profiler.Start(ctx)

	// 等待采集
	time.Sleep(250 * time.Millisecond)

	// 停止
	profiler.Stop()

	// 检查快照
	snapshots := profiler.GetSnapshots()
	if len(snapshots) == 0 {
		t.Error("Expected at least 1 snapshot")
	}

	if len(snapshots) > 10 {
		t.Errorf("Expected max 10 snapshots, got %d", len(snapshots))
	}
}

func TestProfiler_GetLatestSnapshot(t *testing.T) {
	profiler := NewProfiler(ProfilerConfig{
		MaxSnapshots: 10,
		Interval:     100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	profiler.Start(ctx)
	time.Sleep(250 * time.Millisecond)
	profiler.Stop()

	snapshot := profiler.GetLatestSnapshot()
	if snapshot == nil {
		t.Fatal("Expected snapshot, got nil")
	}

	if snapshot.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	if snapshot.Goroutines <= 0 {
		t.Errorf("Expected positive goroutines, got %d", snapshot.Goroutines)
	}
}

func TestProfiler_GetStats(t *testing.T) {
	profiler := NewProfiler(ProfilerConfig{
		MaxSnapshots: 10,
		Interval:     100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	profiler.Start(ctx)
	time.Sleep(350 * time.Millisecond)
	profiler.Stop()

	stats := profiler.GetStats()

	if stats.SnapshotCount == 0 {
		t.Error("Expected snapshot count > 0")
	}

	if stats.StartTime.IsZero() {
		t.Error("Expected non-zero start time")
	}

	if stats.EndTime.IsZero() {
		t.Error("Expected non-zero end time")
	}

	if stats.CurrentGoroutines <= 0 {
		t.Errorf("Expected positive current goroutines, got %d", stats.CurrentGoroutines)
	}
}

func TestProfiler_MaxSnapshots(t *testing.T) {
	profiler := NewProfiler(ProfilerConfig{
		MaxSnapshots: 5,
		Interval:     50 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	profiler.Start(ctx)
	time.Sleep(500 * time.Millisecond)
	profiler.Stop()

	snapshots := profiler.GetSnapshots()
	if len(snapshots) > 5 {
		t.Errorf("Expected max 5 snapshots, got %d", len(snapshots))
	}
}

func TestProfiler_ContextCancel(t *testing.T) {
	profiler := NewProfiler(ProfilerConfig{
		MaxSnapshots: 10,
		Interval:     100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	profiler.Start(ctx)
	time.Sleep(150 * time.Millisecond)

	// 取消 context
	cancel()
	time.Sleep(150 * time.Millisecond)

	// 应该已经停止采集
	snapshots1 := profiler.GetSnapshots()
	time.Sleep(200 * time.Millisecond)
	snapshots2 := profiler.GetSnapshots()

	if len(snapshots2) > len(snapshots1)+1 {
		t.Error("Expected profiler to stop after context cancel")
	}
}

func TestGetMemStats(t *testing.T) {
	stats := GetMemStats()

	if stats.Alloc == 0 {
		t.Error("Expected non-zero alloc")
	}

	if stats.Sys == 0 {
		t.Error("Expected non-zero sys")
	}
}

func TestGetGoroutines(t *testing.T) {
	n := GetGoroutines()
	if n <= 0 {
		t.Errorf("Expected positive goroutines, got %d", n)
	}
}

func TestForceGC(t *testing.T) {
	// 不应该 panic
	ForceGC()
}
