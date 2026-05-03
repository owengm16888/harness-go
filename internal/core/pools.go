package core

import (
	"sync"
	"time"

	"github.com/harness-engineering/harness/models"
)

// 对象池 — 减少 GC 压力
var (
	// ResultPool Result 对象池
	ResultPool = sync.Pool{
		New: func() any {
			return &models.Result{}
		},
	}

	// TaskStatePool TaskState 对象池
	TaskStatePool = sync.Pool{
		New: func() any {
			return &models.TaskState{}
		},
	}

	// ObservationPool Observation 对象池
	ObservationPool = sync.Pool{
		New: func() any {
			return &models.Observation{}
		},
	}

	// TimeBufferPool 时间格式化缓冲区池
	TimeBufferPool = sync.Pool{
		New: func() any {
			buf := make([]byte, 0, 64)
			return &buf
		},
	}
)

// GetResult 从池中获取 Result
func GetResult() *models.Result {
	return ResultPool.Get().(*models.Result)
}

// PutResult 归还 Result 到池
func PutResult(r *models.Result) {
	// 清空数据避免内存泄漏
	r.TaskID = ""
	r.Status = ""
	r.Output = nil
	r.Evidence = r.Evidence[:0]
	r.Errors = r.Errors[:0]
	r.Metrics = models.Metrics{}
	ResultPool.Put(r)
}

// GetTaskState 从池中获取 TaskState
func GetTaskState() *models.TaskState {
	return TaskStatePool.Get().(*models.TaskState)
}

// PutTaskState 归还 TaskState 到池
func PutTaskState(ts *models.TaskState) {
	ts.Task = models.Task{}
	ts.Status = ""
	ts.Result = nil
	ts.History = ts.History[:0]
	ts.CreatedAt = time.Time{}
	ts.UpdatedAt = time.Time{}
	TaskStatePool.Put(ts)
}

// GetObservation 从池中获取 Observation
func GetObservation() *models.Observation {
	return ObservationPool.Get().(*models.Observation)
}

// PutObservation 归还 Observation 到池
func PutObservation(o *models.Observation) {
	o.Task = models.Task{}
	o.Result = models.Result{}
	o.Pattern = ""
	o.Success = false
	ObservationPool.Put(o)
}
