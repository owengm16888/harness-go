# Go GMP 调度模型在 Harness Engineering 中的深度应用

## 概述

本文档深入分析 Go GMP 调度模型的原理，并展示如何在 Harness Engineering 框架中应用这些概念。

## 一、GMP 核心概念

### 1.1 三个核心角色

**G（Goroutine）**
- 初始栈大小 2KB（可动态增长到 1GB）
- 包含栈、程序计数器、状态等信息
- 创建和切换成本极低

**M（Machine）**
- OS 线程，由操作系统管理和调度
- 默认最大数量 10000
- 必须绑定 P 才能执行 G

**P（Processor）**
- 逻辑处理器，是 G 和 M 之间的桥梁
- 持有本地运行队列（最多 256 个 G）
- 数量由 GOMAXPROCS 决定，默认等于 CPU 核心数

### 1.2 调度流程

```
┌─────────────────────────────────────────────────┐
│            全局运行队列 (GRQ)                      │
│            [G5] [G6] [G7] ...                    │
│            (有锁)                                 │
└────────────┬──────────────────┬──────────────────┘
             │                  │
    ┌────────▼────────┐  ┌──────▼────────┐
    │       P0        │  │       P1      │
    │   本地队列(LRQ)  │  │   本地队列    │
    │   [G1][G2][G3]  │  │     [G4]     │
    │   ┌─────────┐   │  │   ┌───────┐  │
    │   │   M0    │   │  │   │  M1   │  │
    │   └────┬────┘   │  │   └───┬───┘  │
    │        │        │  │       │      │
    │    正在执行 G0   │  │   正在执行 G8 │
    └─────────────────┘  └──────────────┘
```

## 二、Harness 中的 GMP 应用

### 2.1 任务调度器设计

**设计思路**：
- **G** → 任务（Task）
- **M** → 工作线程（Worker）
- **P** → 任务队列（Queue）

```go
// pkg/scheduler/scheduler.go

// G - 任务
type Task struct {
    ID          string
    Name        string
    Handler     TaskHandler
    Status      TaskStatus
    CreatedAt   time.Time
    StartedAt   time.Time
    CompletedAt time.Time
}

// M - 工作线程
type Worker struct {
    id       int
    taskChan chan *Task
    quit     chan struct{}
}

// P - 任务队列
type TaskQueue struct {
    mu       sync.RWMutex
    tasks    []*Task
    maxSize  int
}

// 调度器 - 类似 GMP 调度器
type Scheduler struct {
    mu          sync.RWMutex
    workers     []*Worker        // M - 工作线程池
    globalQueue *TaskQueue       // 全局队列
    localQueues map[int]*TaskQueue  // 每个 worker 的本地队列
    taskChan    chan *Task        // 任务提交通道
    stopChan    chan struct{}
    maxWorkers  int
}
```

### 2.2 调度循环实现

```go
// pkg/scheduler/scheduler.go

// 调度循环 - 类似 M 获取 G 的流程
func (s *Scheduler) schedule(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-s.stopChan:
            return
        default:
            // 按优先级获取任务
            task := s.getNextTask()
            if task == nil {
                // 没有任务，休眠
                time.Sleep(10 * time.Millisecond)
                continue
            }
            
            // 分配给空闲 worker
            s.assignTask(task)
        }
    }
}

// 获取下一个任务 - 类似调度优先级
func (s *Scheduler) getNextTask() *Task {
    // 1. 每 61 次调度，先检查全局队列（防止饥饿）
    if s.scheduleCount%61 == 0 {
        if task := s.globalQueue.Dequeue(); task != nil {
            return task
        }
    }
    
    // 2. 从本地队列取
    for _, queue := range s.localQueues {
        if task := queue.Dequeue(); task != nil {
            return task
        }
    }
    
    // 3. 本地队列为空，从全局队列取一批
    if task := s.globalQueue.Dequeue(); task != nil {
        return task
    }
    
    // 4. 全局队列也为空，从其他 worker 偷取（工作窃取）
    return s.stealTask()
}
```

### 2.3 工作窃取（Work Stealing）

```go
// pkg/scheduler/scheduler.go

// 工作窃取 - 类似 P 从其他 P 偷取 G
func (s *Scheduler) stealTask() *Task {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    // 随机选择一个 worker
    victimIndex := rand.Intn(len(s.workers))
    victimQueue := s.localQueues[victimIndex]
    
    // 偷取一半的任务
    victimQueue.mu.Lock()
    stealCount := victimQueue.Len() / 2
    if stealCount == 0 {
        victimQueue.mu.Unlock()
        return nil
    }
    
    tasks := victimQueue.tasks[:stealCount]
    victimQueue.tasks = victimQueue.tasks[stealCount:]
    victimQueue.mu.Unlock()
    
    // 返回第一个任务，其余放入自己的队列
    if len(tasks) > 0 {
        for _, task := range tasks[1:] {
            s.localQueues[s.currentWorker].Enqueue(task)
        }
        return tasks[0]
    }
    
    return nil
}
```

### 2.4 Hand Off 机制

```go
// pkg/scheduler/scheduler.go

// Hand Off - M 阻塞时，P 与 M 解绑
func (s *Scheduler) handleBlockingTask(task *Task) {
    // 当任务可能阻塞时（如系统调用）
    if task.MayBlock {
        // 1. 将任务放入全局队列
        s.globalQueue.Enqueue(task)
        
        // 2. 当前 worker 继续处理其他任务
        // （类似 P 与阻塞的 M 解绑，绑定到其他 M）
        return
    }
    
    // 正常执行
    s.executeTask(task)
}

// 系统调用封装
func (s *Scheduler) syscall(task *Task) {
    // 进入系统调用前
    s.enterSyscall(task)
    
    // 执行系统调用
    result := task.ExecuteSyscall()
    
    // 退出系统调用后
    s.exitSyscall(task, result)
}

func (s *Scheduler) enterSyscall(task *Task) {
    // 通知调度器 M 被阻塞
    // 类似 runtime.entersyscall
    s.mu.Lock()
    task.Status = TaskStatusBlocked
    s.mu.Unlock()
    
    // 将 P 与 M 解绑
    s.detachPFromM(task.WorkerID)
}

func (s *Scheduler) exitSyscall(task *Task, result interface{}) {
    // 尝试重新获取 P
    p := s.findIdleP()
    if p == nil {
        // 没有空闲 P，任务放入全局队列
        s.globalQueue.Enqueue(task)
        return
    }
    
    // 绑定 P，继续执行
    s.attachPToM(task.WorkerID, p)
    task.Status = TaskStatusRunning
}
```

### 2.5 抢占式调度

```go
// pkg/scheduler/scheduler.go

// sysmon 监控线程 - 类似 Go 的 sysmon
func (s *Scheduler) sysmon(ctx context.Context) {
    ticker := time.NewTicker(10 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.checkLongRunningTasks()
            s.checkBlockedWorkers()
            s.triggerGC()
        }
    }
}

// 检查长时间运行的任务 - 类似抢占式调度
func (s *Scheduler) checkLongRunningTasks() {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    now := time.Now()
    for _, task := range s.runningTasks {
        // 运行超过 10ms 的任务
        if now.Sub(task.StartedAt) > 10*time.Millisecond {
            // 发送抢占信号 - 类似 SIGURG
            s.preemptTask(task)
        }
    }
}

// 抢占任务
func (s *Scheduler) preemptTask(task *Task) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    // 将任务状态改为可抢占
    task.Preempted = true
    
    // 通知 worker 让出 CPU
    task.Worker.preemptChan <- struct{}{}
    
    // 将任务放回队列
    task.Status = TaskStatusRunnable
    s.globalQueue.Enqueue(task)
}
```

## 三、GMP 在分布式系统中的应用

### 3.1 集群调度

```go
// pkg/distributed/distributed.go

// 集群 - 类似多核 CPU
type Cluster struct {
    mu       sync.RWMutex
    nodes    map[string]*Node    // 类似多个 P
    leader   string
    scheduler *TaskDistribution
}

// 节点 - 类似 P
type Node struct {
    ID       string
    Address  string
    Port     int
    Status   NodeStatus
    Load     int
    Tasks    []*Task
}

// 任务分配 - 类似 GMP 调度
func (c *Cluster) scheduleTask(task *Task) error {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    // 1. 选择负载最低的节点（类似选择空闲的 P）
    targetNode := c.selectLeastLoadedNode()
    
    // 2. 如果所有节点都忙，放入全局队列
    if targetNode == nil {
        c.globalQueue.Enqueue(task)
        return nil
    }
    
    // 3. 分配任务
    return c.assignTaskToNode(task, targetNode)
}

// 工作窃取 - 跨节点
func (c *Cluster) stealTasks(fromNode, toNode *Node) {
    fromNode.mu.Lock()
    stealCount := len(fromNode.Tasks) / 2
    tasks := fromNode.Tasks[:stealCount]
    fromNode.Tasks = fromNode.Tasks[stealCount:]
    fromNode.mu.Unlock()
    
    toNode.mu.Lock()
    toNode.Tasks = append(toNode.Tasks, tasks...)
    toNode.mu.Unlock()
}
```

### 3.2 负载均衡

```go
// pkg/distributed/distributed.go

// 负载均衡策略
type LoadBalancer interface {
    SelectNode(nodes []*Node, task *Task) *Node
}

// 轮询 - 类似 Round Robin
type RoundRobinBalancer struct {
    current int
}

func (b *RoundRobinBalancer) SelectNode(nodes []*Node, task *Task) *Node {
    node := nodes[b.current%len(nodes)]
    b.current++
    return node
}

// 最少连接 - 类似选择空闲的 P
type LeastConnectionsBalancer struct{}

func (b *LeastConnectionsBalancer) SelectNode(nodes []*Node, task *Task) *Node {
    var selected *Node
    minLoad := int(^uint(0) >> 1)
    
    for _, node := range nodes {
        if node.Status == NodeStatusActive && node.Load < minLoad {
            minLoad = node.Load
            selected = node
        }
    }
    
    return selected
}

// 一致性哈希 - 类似 G 与 P 的绑定
type ConsistentHashBalancer struct {
    ring *consistenthash.Map
}

func (b *ConsistentHashBalancer) SelectNode(nodes []*Node, task *Task) *Node {
    // 根据 task ID 计算哈希，选择固定的节点
    nodeID := b.ring.Get(task.ID)
    for _, node := range nodes {
        if node.ID == nodeID {
            return node
        }
    }
    return nil
}
```

## 四、GMP 在并发控制中的应用

### 4.1 Goroutine 池

```go
// pkg/pool/pool.go

// Goroutine 池 - 类似 M 的数量限制
type GoroutinePool struct {
    mu          sync.RWMutex
    workers     int
    taskQueue   chan Task
    workerPool  chan struct{}
    wg          sync.WaitGroup
    stopChan    chan struct{}
}

func NewGoroutinePool(workers int) *GoroutinePool {
    return &GoroutinePool{
        workers:    workers,
        taskQueue:  make(chan Task, workers*10),
        workerPool: make(chan struct{}, workers),
        stopChan:   make(chan struct{}),
    }
}

func (p *GoroutinePool) Start(ctx context.Context) {
    // 启动固定数量的 worker - 类似 M
    for i := 0; i < p.workers; i++ {
        go p.worker(ctx, i)
    }
}

func (p *GoroutinePool) worker(ctx context.Context, id int) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-p.stopChan:
            return
        case task := <-p.taskQueue:
            // 获取信号量 - 类似 M 绑定 P
            p.workerPool <- struct{}{}
            
            // 执行任务
            p.executeTask(ctx, task, id)
            
            // 释放信号量
            <-p.workerPool
        }
    }
}
```

### 4.2 并发限制

```go
// pkg/scheduler/scheduler.go

// 并发限制 - 类似 GOMAXPROCS
type ConcurrencyLimiter struct {
    mu          sync.RWMutex
    maxConcurrent int
    current       int
    queue         chan struct{}
}

func NewConcurrencyLimiter(max int) *ConcurrencyLimiter {
    return &ConcurrencyLimiter{
        maxConcurrent: max,
        queue:         make(chan struct{}, max),
    }
}

func (l *ConcurrencyLimiter) Acquire() {
    l.queue <- struct{}{}  // 阻塞直到有空位
}

func (l *ConcurrencyLimiter) Release() {
    <-l.queue
}

// 使用
func (s *Scheduler) executeWithLimit(task *Task) {
    s.limiter.Acquire()
    defer s.limiter.Release()
    
    task.Execute()
}
```

## 五、GMP 在内存管理中的应用

### 5.1 对象池

```go
// pkg/cache/cache.go

// 对象池 - 类似 P 的 mcache
type ObjectPool struct {
    mu       sync.RWMutex
    pool     sync.Pool
    size     int
    count    int
}

func NewObjectPool(size int) *ObjectPool {
    return &ObjectPool{
        size: size,
        pool: sync.Pool{
            New: func() interface{} {
                return make([]byte, 1024)
            },
        },
    }
}

func (p *ObjectPool) Get() []byte {
    return p.pool.Get().([]byte)
}

func (p *ObjectPool) Put(buf []byte) {
    p.pool.Put(buf)
}
```

### 5.2 内存分配策略

```go
// pkg/utils/utils.go

// 三级分配 - 类似 Go 的内存分配
type MemoryAllocator struct {
    mu       sync.RWMutex
    tiny     *TinyAllocator   // < 16B
    small    *SmallAllocator  // <= 32KB
    large    *LargeAllocator  // > 32KB
}

func (a *MemoryAllocator) Allocate(size int) []byte {
    switch {
    case size < 16:
        return a.tiny.Allocate(size)
    case size <= 32*1024:
        return a.small.Allocate(size)
    default:
        return a.large.Allocate(size)
    }
}
```

## 六、GMP 在事件系统中的应用

### 6.1 事件调度

```go
// pkg/event/event.go

// 事件调度 - 类似 GMP 调度
type EventScheduler struct {
    mu          sync.RWMutex
    handlers    map[EventType][]EventHandler
    eventQueue  chan Event
    workers     int
    stopChan    chan struct{}
}

func (s *EventScheduler) Start(ctx context.Context) {
    // 启动事件处理 worker - 类似 M
    for i := 0; i < s.workers; i++ {
        go s.eventWorker(ctx, i)
    }
    
    // 启动事件分发 - 类似调度循环
    go s.dispatch(ctx)
}

func (s *EventScheduler) dispatch(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case event := <-s.eventQueue:
            // 分发事件到对应的 handler
            s.dispatchEvent(event)
        }
    }
}

func (s *EventScheduler) eventWorker(ctx context.Context, id int) {
    for event := range s.eventQueue {
        // 获取事件处理权 - 类似 M 获取 G
        s.processEvent(ctx, event, id)
    }
}
```

## 七、性能优化

### 7.1 减少锁竞争

```go
// pkg/cache/cache.go

// 使用分片减少锁竞争 - 类似 P 的本地队列
type ShardedCache struct {
    shards    []*CacheShard
    shardCount int
}

type CacheShard struct {
    mu    sync.RWMutex
    items map[string]*cacheItem
}

func (c *ShardedCache) getShard(key string) *CacheShard {
    hash := fnv.New32a()
    hash.Write([]byte(key))
    return c.shards[hash.Sum32()%uint32(c.shardCount)]
}

func (c *ShardedCache) Get(key string) (interface{}, bool) {
    shard := c.getShard(key)
    shard.mu.RLock()
    defer shard.mu.RUnlock()
    
    item, exists := shard.items[key]
    return item.value, exists
}
```

### 7.2 无锁队列

```go
// pkg/scheduler/scheduler.go

// 无锁队列 - 类似 P 的本地队列
type LockFreeQueue struct {
    head unsafe.Pointer
    tail unsafe.Pointer
}

type node struct {
    value interface{}
    next  unsafe.Pointer
}

func (q *LockFreeQueue) Enqueue(value interface{}) {
    newNode := &node{value: value}
    
    for {
        tail := atomic.LoadPointer(&q.tail)
        next := atomic.LoadPointer(&(*node)(tail).next)
        
        if tail == atomic.LoadPointer(&q.tail) {
            if next == nil {
                if atomic.CompareAndSwapPointer(&(*node)(tail).next, nil, unsafe.Pointer(newNode)) {
                    atomic.CompareAndSwapPointer(&q.tail, tail, unsafe.Pointer(newNode))
                    return
                }
            } else {
                atomic.CompareAndSwapPointer(&q.tail, tail, next)
            }
        }
    }
}
```

## 八、监控与调试

### 8.1 调度器指标

```go
// pkg/metrics/metrics.go

// 调度器指标 - 类似 runtime 指标
type SchedulerMetrics struct {
    TaskCount      *Counter
    WorkerCount    *Gauge
    QueueSize      *Gauge
    StealCount     *Counter
    PreemptCount   *Counter
    ScheduleLatency *Histogram
}

func (m *SchedulerMetrics) RecordSchedule(duration time.Duration) {
    m.ScheduleLatency.Observe(duration.Seconds())
}

func (m *SchedulerMetrics) RecordSteal() {
    m.StealCount.Inc()
}
```

### 8.2 追踪调度行为

```go
// pkg/logger/logger.go

// 调度追踪 - 类似 go tool trace
type SchedulerTracer struct {
    logger *Logger
    mu     sync.RWMutex
    events []TraceEvent
}

type TraceEvent struct {
    Timestamp time.Time
    Type      string
    TaskID    string
    WorkerID  int
    Details   map[string]any
}

func (t *SchedulerTracer) Trace(event TraceEvent) {
    t.mu.Lock()
    defer t.mu.Unlock()
    
    t.events = append(t.events, event)
    t.logger.WithFields(map[string]any{
        "type":     event.Type,
        "task_id":  event.TaskID,
        "worker_id": event.WorkerID,
    }).Debug("Scheduler event")
}
```

## 九、总结

GMP 调度模型在 Harness Engineering 中的应用：

| GMP 概念 | Harness 应用 |
|---------|-------------|
| G (Goroutine) | Task (任务) |
| M (Machine) | Worker (工作线程) |
| P (Processor) | Queue (任务队列) |
| 全局队列 | GlobalTaskQueue |
| 本地队列 | WorkerLocalQueue |
| 工作窃取 | TaskStealing |
| Hand Off | TaskReassignment |
| 抢占式调度 | TaskPreemption |
| sysmon | SchedulerMonitor |
| GOMAXPROCS | MaxConcurrency |

通过理解 GMP 模型，我们可以设计出高效、可扩展的任务调度系统。
