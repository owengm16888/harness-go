# Go 面试知识：锁粒度设计模式

> 来源：模拟面试第 5 题 — TaskManager.ExecuteTask 为什么要 Unlock 后执行任务？

## 核心原则

**锁只保护共享状态的读写，不保护业务逻辑。**

## 错误做法：持锁执行任务

```go
func (tm *TaskManager) ExecuteTask(task Task) {
    tm.mu.Lock()
    defer tm.mu.Unlock()

    result := task.Run()   // ← 耗时操作在锁内执行！
    tm.results[task.ID] = result
}
```

### 问题

| 问题 | 说明 |
|------|------|
| 吞吐量暴跌 | 所有 goroutine 串行等锁，并发形同虚设 |
| 死锁风险 | task.Run() 内部若也需要获取同一把锁 → 死锁 |
| 锁饥饿 | 长任务持锁期间，其他读操作（查状态）全部阻塞 |

## 正确做法：最小化锁的持有范围

```go
func (tm *TaskManager) ExecuteTask(task Task) {
    // 1. 加锁，做状态变更（极短）
    tm.mu.Lock()
    tm.status[task.ID] = Running
    tm.mu.Unlock()          // ← 释放锁

    // 2. 裸跑任务（耗时，不持锁）
    result := task.Run()

    // 3. 加锁，写回结果（极短）
    tm.mu.Lock()
    tm.results[task.ID] = result
    tm.status[task.ID] = Done
    tm.mu.Unlock()
}
```

## 面试答题要点

1. **任务执行是耗时操作**（网络、IO、计算），持锁会阻塞整个 TaskManager
2. **锁的职责边界**：锁保护的是共享数据结构（results/status），不是业务逻辑
3. **并发度**：Unlock 后多个 Task 可以真正并行执行，发挥 goroutine 的价值
4. **死锁预防**：task 内部逻辑不可控，持锁调用外部代码是危险的

## 进阶：状态机守卫

```go
// 原子地将状态改为 in_progress，防止同一任务被并发执行
tm.mu.Lock()
if state.Status != TaskStatusPending {
    tm.mu.Unlock()
    return ErrTaskNotCancellable
}
state.Status = TaskStatusInProgress
tm.mu.Unlock()
```

第二个 goroutine 检查 status 不是 pending 就拒绝，无需全程持锁。

## 参考

- 项目文件：`internal/core/task_manager.go` — ExecuteTask 方法
- 项目文件：`internal/api/server.go` — recoveryMiddleware + loggingMiddleware
