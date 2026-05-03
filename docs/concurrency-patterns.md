# Go 并发编程在 Harness Engineering 中的实战应用

## 概述

本文档深入分析 Go 并发编程的核心概念，并展示如何在 Harness Engineering 框架中应用这些模式。

## 一、CSP 模型

### 1.1 核心思想

**CSP（Communicating Sequential Processes）**：
- "不要通过共享内存来通信；通过通信来共享内存。"
- 进程之间通过通道提交消息，各自顺序执行，不直接共享状态。

### 1.2 Harness 中的 CSP 应用

```go
// pkg/event/event.go

// 事件总线 - 基于 CSP 模型
type EventBus struct {
    handlers map[EventType][]EventHandler
    eventCh  chan Event      // 通信通道
    doneCh   chan struct{}   // 完成信号
}

// 发布事件 - 通过通道通信
func (eb *EventBus) Publish(event Event) {
    eb.eventCh <- event
}

// 订阅事件 - 独立的 goroutine 处理
func (eb *EventBus) Subscribe(handler EventHandler) {
    go func() {
        for event := range eb.eventCh {
            handler.Handle(event)
        }
    }()
}

// 使用示例
func main() {
    bus := NewEventBus()
    
    // 订阅者 - 独立的 goroutine
    bus.Subscribe(func(event Event) {
        fmt.Printf("Received: %s\n", event.Type)
    })
    
    // 发布者 - 通过通道通信
    bus.Publish(Event{Type: "task.created"})
}
```

## 二、Channel 高级用法

### 2.1 Channel 方向限制

```go
// pkg/event/event.go

// 只写通道
type EventProducer struct {
    ch chan<- Event
}

// 只读通道
type EventConsumer struct {
    ch <-chan Event
}

// 使用方向限制防止误用
func NewEventSystem() (*EventProducer, *EventConsumer) {
    ch := make(chan Event, 100)
    return &EventProducer{ch: ch}, &EventConsumer{ch: ch}
}
```

### 2.2 Fan-out / Fan-in 模式

```go
// pkg/event/event.go

// Fan-out: 一个事件分发给多个处理器
func (eb *EventBus) fanOut(event Event) {
    for _, handler := range eb.handlers[event.Type] {
        go handler.Handle(event)  // 并发处理
    }
}

// Fan-in: 多个事件汇聚到一个通道
func (eb *EventBus) fanIn(events ...<-chan Event) <-chan Event {
    merged := make(chan Event)
    var wg sync.WaitGroup
    
    for _, ch := range events {
        wg.Add(1)
        go func(c <-chan Event) {
            defer wg.Done()
            for event := range c {
                merged <- event
            }
        }(ch)
    }
    
    go func() {
        wg.Wait()
        close(merged)
    }()
    
    return merged
}
```

### 2.3 Or-Done 模式

```go
// pkg/event/event.go

// Or-Done: 同时监听数据通道和完成通道
func (eb *EventBus) listenWithCancel(ctx context.Context, eventCh <-chan Event) {
    for {
        select {
        case <-ctx.Done():
            return  // 上下文取消
        case event, ok := <-eventCh:
            if !ok {
                return  // 通道关闭
            }
            eb.processEvent(event)
        }
    }
}
```

### 2.4 信号量模式

```go
// pkg/scheduler/scheduler.go

// 信号量: 使用带缓冲 channel 控制并发数
type Semaphore struct {
    ch chan struct{}
}

func NewSemaphore(maxConcurrency int) *Semaphore {
    return &Semaphore{
        ch: make(chan struct{}, maxConcurrency),
    }
}

func (s *Semaphore) Acquire() {
    s.ch <- struct{}{}
}

func (s *Semaphore) Release() {
    <-s.ch
}

// 使用示例
func (s *Scheduler) executeWithSemaphore(task *Task) {
    s.semaphore.Acquire()
    defer s.semaphore.Release()
    
    task.Execute()
}
```

## 三、sync 包同步原语

### 3.1 sync.Mutex 互斥锁

```go
// pkg/cache/cache.go

type MemoryCache struct {
    mu    sync.Mutex
    items map[string]*cacheItem
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    item, exists := c.items[key]
    return item.value, exists
}

func (c *MemoryCache) Set(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.items[key] = &cacheItem{value: value}
}
```

### 3.2 sync.RWMutex 读写锁

```go
// pkg/cache/cache.go

type MemoryCache struct {
    mu    sync.RWMutex  // 读写锁
    items map[string]*cacheItem
}

// 读操作 - 使用 RLock
func (c *MemoryCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    item, exists := c.items[key]
    return item.value, exists
}

// 写操作 - 使用 Lock
func (c *MemoryCache) Set(key string, value interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    c.items[key] = &cacheItem{value: value}
}
```

### 3.3 sync.WaitGroup

```go
// pkg/webhook/webhook.go

func (m *WebhookManager) FireAll(ctx context.Context, event string) error {
    var wg sync.WaitGroup
    
    for _, webhook := range m.webhooks {
        if !webhook.Enabled {
            continue
        }
        
        wg.Add(1)  // 在 goroutine 启动前调用
        go func(wh *Webhook) {
            defer wg.Done()
            m.executeWebhook(ctx, wh, event)
        }(webhook)  // 传递参数，避免闭包捕获
    }
    
    wg.Wait()  // 等待所有 goroutine 完成
    return nil
}
```

### 3.4 sync.Once

```go
// pkg/config/config.go

var (
    config     *Config
    configOnce sync.Once
)

func GetConfig() *Config {
    configOnce.Do(func() {
        config = loadConfig()  // 只执行一次
    })
    return config
}
```

### 3.5 sync.Pool

```go
// pkg/utils/utils.go

var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

func FormatJSON(v interface{}) ([]byte, error) {
    buf := bufferPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        bufferPool.Put(buf)
    }()
    
    encoder := json.NewEncoder(buf)
    if err := encoder.Encode(v); err != nil {
        return nil, err
    }
    
    return buf.Bytes(), nil
}
```

### 3.6 sync.Map

```go
// pkg/cache/cache.go

type ConcurrentMap struct {
    m sync.Map
}

func (m *ConcurrentMap) Get(key string) (interface{}, bool) {
    return m.m.Load(key)
}

func (m *ConcurrentMap) Set(key string, value interface{}) {
    m.m.Store(key, value)
}

func (m *ConcurrentMap) Delete(key string) {
    m.m.Delete(key)
}

func (m *ConcurrentMap) Range(f func(key, value interface{}) bool) {
    m.m.Range(f)
}
```

## 四、并发模式

### 4.1 Worker Pool 模式

```go
// pkg/pool/pool.go

type WorkerPool struct {
    workers   int
    taskQueue chan Task
    wg        sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
    return &WorkerPool{
        workers:   workers,
        taskQueue: make(chan Task, workers*10),
    }
}

func (p *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go p.worker(ctx, i)
    }
}

func (p *WorkerPool) worker(ctx context.Context, id int) {
    defer p.wg.Done()
    
    for {
        select {
        case <-ctx.Done():
            return
        case task, ok := <-p.taskQueue:
            if !ok {
                return
            }
            p.executeTask(ctx, task, id)
        }
    }
}

func (p *WorkerPool) Submit(task Task) {
    p.taskQueue <- task
}

func (p *WorkerPool) Stop() {
    close(p.taskQueue)
    p.wg.Wait()
}
```

### 4.2 Pipeline 模式

```go
// pkg/pipeline/pipeline.go

type Stage func(ctx context.Context, in <-chan interface{}) <-chan interface{}

type Pipeline struct {
    stages []Stage
}

func NewPipeline(stages ...Stage) *Pipeline {
    return &Pipeline{stages: stages}
}

func (p *Pipeline) Execute(ctx context.Context, input <-chan interface{}) <-chan interface{} {
    current := input
    
    for _, stage := range p.stages {
        current = stage(ctx, current)
    }
    
    return current
}

// 使用示例
func main() {
    pipeline := NewPipeline(
        // Stage 1: 过滤
        func(ctx context.Context, in <-chan interface{}) <-chan interface{} {
            out := make(chan interface{})
            go func() {
                defer close(out)
                for v := range in {
                    if v.(int)%2 == 0 {
                        out <- v
                    }
                }
            }()
            return out
        },
        // Stage 2: 转换
        func(ctx context.Context, in <-chan interface{}) <-chan interface{} {
            out := make(chan interface{})
            go func() {
                defer close(out)
                for v := range in {
                    out <- v.(int) * 2
                }
            }()
            return out
        },
    )
    
    input := make(chan interface{})
    go func() {
        for i := 0; i < 10; i++ {
            input <- i
        }
        close(input)
    }()
    
    output := pipeline.Execute(context.Background(), input)
    for v := range output {
        fmt.Println(v)
    }
}
```

### 4.3 Fan-out / Fan-in 模式

```go
// pkg/fanout/fanout.go

// Fan-out: 将任务分发给多个 worker
func FanOut(ctx context.Context, tasks []Task, workers int) <-chan Result {
    results := make(chan Result)
    taskCh := make(chan Task)
    
    // 启动多个 worker
    var wg sync.WaitGroup
    for i := 0; i < workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for task := range taskCh {
                result := task.Execute(ctx)
                results <- result
            }
        }()
    }
    
    // 分发任务
    go func() {
        for _, task := range tasks {
            taskCh <- task
        }
        close(taskCh)
    }()
    
    // 等待完成
    go func() {
        wg.Wait()
        close(results)
    }()
    
    return results
}

// Fan-in: 将多个结果汇聚到一个通道
func FanIn(results ...<-chan Result) <-chan Result {
    merged := make(chan Result)
    var wg sync.WaitGroup
    
    for _, ch := range results {
        wg.Add(1)
        go func(c <-chan Result) {
            defer wg.Done()
            for result := range c {
                merged <- result
            }
        }(ch)
    }
    
    go func() {
        wg.Wait()
        close(merged)
    }()
    
    return merged
}
```

### 4.4 限流模式

```go
// pkg/ratelimit/ratelimit.go

// 令牌桶限流
type TokenBucket struct {
    mu         sync.Mutex
    tokens     int
    maxTokens  int
    refillRate time.Duration
    lastRefill time.Time
}

func NewTokenBucket(maxTokens int, refillRate time.Duration) *TokenBucket {
    return &TokenBucket{
        tokens:     maxTokens,
        maxTokens:  maxTokens,
        refillRate: refillRate,
        lastRefill: time.Now(),
    }
}

func (b *TokenBucket) Allow() bool {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    // 补充令牌
    now := time.Now()
    elapsed := now.Sub(b.lastRefill)
    refillCount := int(elapsed / b.refillRate)
    if refillCount > 0 {
        b.tokens = min(b.maxTokens, b.tokens+refillCount)
        b.lastRefill = now
    }
    
    // 检查是否有可用令牌
    if b.tokens > 0 {
        b.tokens--
        return true
    }
    
    return false
}
```

## 五、Context 应用

### 5.1 控制 goroutine 生命周期

```go
// pkg/scheduler/scheduler.go

func (s *Scheduler) Start(ctx context.Context) error {
    // 创建可取消的 context
    ctx, cancel := context.WithCancel(ctx)
    s.cancel = cancel
    
    // 启动 worker
    for i := 0; i < s.workers; i++ {
        go s.worker(ctx, i)
    }
    
    return nil
}

func (s *Scheduler) Stop() {
    s.cancel()  // 取消 context，通知所有 worker 退出
}

func (s *Scheduler) worker(ctx context.Context, id int) {
    for {
        select {
        case <-ctx.Done():
            return  // 收到取消信号，退出
        case task := <-s.taskQueue:
            s.executeTask(ctx, task)
        }
    }
}
```

### 5.2 超时控制

```go
// internal/core/engine.go

func (e *Engine) ExecuteTask(ctx context.Context, task Task) (*Result, error) {
    // 创建带超时的 context
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // 执行任务
    resultCh := make(chan Result, 1)
    errCh := make(chan error, 1)
    
    go func() {
        result, err := e.executeTask(ctx, task)
        if err != nil {
            errCh <- err
            return
        }
        resultCh <- *result
    }()
    
    // 等待结果或超时
    select {
    case result := <-resultCh:
        return &result, nil
    case err := <-errCh:
        return nil, err
    case <-ctx.Done():
        return nil, errors.New(ErrCodeTimeout, "任务执行超时")
    }
}
```

### 5.3 传递值

```go
// pkg/middleware/middleware.go

// 使用 context 传递请求 ID
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = generateRequestID()
        }
        
        // 将 request ID 存入 context
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        w.Header().Set("X-Request-ID", requestID)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// 从 context 获取 request ID
func GetRequestID(ctx context.Context) string {
    if id, ok := ctx.Value("request_id").(string); ok {
        return id
    }
    return ""
}
```

## 六、并发安全最佳实践

### 6.1 避免数据竞争

```go
// 使用 -race 检测数据竞争
// go run -race main.go

// 不好 - 存在数据竞争
var counter int

func increment() {
    counter++  // 多个 goroutine 同时访问
}

// 好 - 使用互斥锁
var (
    counter int
    mu      sync.Mutex
)

func increment() {
    mu.Lock()
    defer mu.Unlock()
    counter++
}

// 好 - 使用原子操作
var counter int64

func increment() {
    atomic.AddInt64(&counter, 1)
}
```

### 6.2 避免 goroutine 泄漏

```go
// 不好 - goroutine 泄漏
func leak() {
    ch := make(chan int)
    go func() {
        ch <- 1  // 如果没有接收者，goroutine 永远阻塞
    }()
}

// 好 - 使用 context 控制
func noLeak(ctx context.Context) {
    ch := make(chan int, 1)
    go func() {
        select {
        case ch <- 1:
        case <-ctx.Done():
            return  // context 取消时退出
        }
    }()
}
```

### 6.3 避免死锁

```go
// 不好 - 可能死锁
func deadlock() {
    var mu1, mu2 sync.Mutex
    
    go func() {
        mu1.Lock()
        mu2.Lock()
        // ...
        mu2.Unlock()
        mu1.Unlock()
    }()
    
    go func() {
        mu2.Lock()
        mu1.Lock()  // 与上面顺序不同，可能死锁
        // ...
        mu1.Unlock()
        mu2.Unlock()
    }()
}

// 好 - 固定加锁顺序
func noDeadlock() {
    var mu1, mu2 sync.Mutex
    
    lock := func() {
        mu1.Lock()
        mu2.Lock()
    }
    
    unlock := func() {
        mu2.Unlock()
        mu1.Unlock()
    }
    
    // 所有 goroutine 都使用相同的顺序
}
```

## 七、性能优化

### 7.1 减少锁竞争

```go
// 使用分片减少锁竞争
type ShardedMap struct {
    shards    []*Shard
    shardCount int
}

type Shard struct {
    mu    sync.RWMutex
    items map[string]interface{}
}

func (m *ShardedMap) getShard(key string) *Shard {
    hash := fnv.New32a()
    hash.Write([]byte(key))
    return m.shards[hash.Sum32()%uint32(m.shardCount)]
}

func (m *ShardedMap) Get(key string) (interface{}, bool) {
    shard := m.getShard(key)
    shard.mu.RLock()
    defer shard.mu.RUnlock()
    
    item, exists := shard.items[key]
    return item, exists
}
```

### 7.2 使用原子操作

```go
// 使用原子操作替代互斥锁
type AtomicCounter struct {
    value int64
}

func (c *AtomicCounter) Inc() {
    atomic.AddInt64(&c.value, 1)
}

func (c *AtomicCounter) Dec() {
    atomic.AddInt64(&c.value, -1)
}

func (c *AtomicCounter) Get() int64 {
    return atomic.LoadInt64(&c.value)
}
```

### 7.3 预分配容量

```go
// 预分配切片容量，避免频繁扩容
func processItems(items []Item) []Result {
    results := make([]Result, 0, len(items))  // 预分配容量
    
    for _, item := range items {
        results = append(results, process(item))
    }
    
    return results
}
```

## 八、总结

Go 并发编程在 Harness Engineering 中的应用：

| 模式 | 应用场景 |
|------|---------|
| CSP | 事件系统、任务调度 |
| Fan-out/Fan-in | 并行处理、结果汇聚 |
| Pipeline | 数据处理流水线 |
| Worker Pool | 任务并发执行 |
| 限流 | API 限流、资源控制 |
| Context | 生命周期管理、超时控制 |

通过合理使用这些并发模式，可以构建高效、可扩展的系统。
