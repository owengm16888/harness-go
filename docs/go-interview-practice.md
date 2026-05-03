# Go 面试知识点在 Harness Engineering 中的实战应用

## 概述

本文档将 Go 面试中的 40 个精选题目与 Harness Engineering 框架的实际代码相结合，展示每个知识点在真实项目中的应用。

## 一、语法基础

### 1. := 与 var 的区别

**面试要点**：`:=` 只能在函数内部使用，同时声明和初始化；`var` 可以在包级别使用，可以声明不初始化。

**Harness 应用**：

```go
// pkg/config/config.go
var (  // 包级别使用 var
    defaultConfig *Config
    configOnce    sync.Once
)

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)  // 函数内部使用 :=
    if err != nil {
        return nil, err  // 提前返回
    }
    
    config := &Config{}  // 函数内部使用 :=
    if err := yaml.Unmarshal(data, config); err != nil {
        return nil, err
    }
    
    return config, nil
}
```

### 2. new 与 make 的区别

**面试要点**：`new(T)` 返回零值指针；`make` 仅用于 slice、map、channel，返回初始化后的值。

**Harness 应用**：

```go
// pkg/cache/cache.go
func NewMemoryCache(maxSize int) *MemoryCache {
    return &MemoryCache{
        items: make(map[string]*cacheItem),  // make 初始化 map
        // 不是 new(map[string]*cacheItem)
    }
}

// pkg/event/event.go
func NewEventBus() *EventBus {
    return &EventBus{
        handlers: make(map[EventType][]handlerEntry),  // make 初始化 map
        // channel 也用 make
        queue: make(chan Event, 100),
    }
}
```

## 二、数据结构

### 3. 切片共享底层数组陷阱

**面试要点**：子切片与原切片共享底层数组，修改会相互影响。

**Harness 应用**：

```go
// pkg/knowledge/knowledge_base.go
func (kb *KnowledgeBase) Search(query string) []*KnowledgeEntry {
    results := kb.searchInternal(query)
    
    // 不好 - 返回的切片可能被外部修改影响内部数据
    // return results
    
    // 好 - 复制一份返回
    safeResults := make([]*KnowledgeEntry, len(results))
    copy(safeResults, results)
    return safeResults
}
```

### 4. map 并发安全

**面试要点**：map 不是并发安全的，需要使用 sync.Mutex 或 sync.Map。

**Harness 应用**：

```go
// pkg/cache/cache.go
type MemoryCache struct {
    mu    sync.RWMutex  // 使用读写锁保护 map
    items map[string]*cacheItem
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
    c.mu.RLock()  // 读锁
    defer c.mu.RUnlock()
    
    item, exists := c.items[key]
    return item.value, exists
}

func (c *MemoryCache) Set(key string, value interface{}) {
    c.mu.Lock()  // 写锁
    defer c.mu.Unlock()
    
    c.items[key] = &cacheItem{value: value}
}
```

## 三、接口与类型系统

### 5. 接口的底层实现

**面试要点**：空接口 `interface{}` 底层是 `eface`；非空接口底层是 `iface`。

**Harness 应用**：

```go
// pkg/errors/errors.go
// 使用接口实现多态
type Error interface {
    error
    Code() ErrorCode
    HTTPStatus() int
}

// 具体实现
type AppError struct {
    code       ErrorCode
    message    string
    httpStatus int
}

func (e *AppError) Error() string { return e.message }
func (e *AppError) Code() ErrorCode { return e.code }
func (e *AppError) HTTPStatus() int { return e.httpStatus }
```

### 6. nil 接口陷阱

**面试要点**：接口只有当类型和值都为 nil 时才等于 nil。

**Harness 应用**：

```go
// internal/core/engine.go
func (e *Engine) GetAdapter(name string) (Adapter, error) {
    adapter, exists := e.adapters[name]
    if !exists {
        return nil, errors.WrapAdapterNotFound(name)
        // 不要返回 (*ClaudeCodeAdapter)(nil)，否则接口 != nil
    }
    return adapter, nil
}
```

## 四、并发编程

### 7. GMP 调度模型

**面试要点**：G（Goroutine）、M（OS 线程）、P（逻辑处理器）。

**Harness 应用**：

```go
// pkg/scheduler/scheduler.go
type Scheduler struct {
    tasks   map[string]*Task  // G - 任务
    workers int               // M - 工作线程
    queue   chan *Task         // P 的本地队列
}

func (s *Scheduler) Start(ctx context.Context) {
    // 启动多个 worker - 类似 M
    for i := 0; i < s.workers; i++ {
        go s.worker(ctx)
    }
}

func (s *Scheduler) worker(ctx context.Context) {
    for task := range s.queue {  // 从队列取任务 - 类似调度循环
        s.executeTask(ctx, task)
    }
}
```

### 8. channel 底层结构

**面试要点**：hchan 结构体，包含环形缓冲、等待队列、互斥锁。

**Harness 应用**：

```go
// pkg/event/event.go
type EventBus struct {
    handlers map[EventType][]handlerEntry
    queue    chan Event  // 带缓冲的 channel
}

func NewEventBus(bufferSize int) *EventBus {
    return &EventBus{
        handlers: make(map[EventType][]handlerEntry),
        queue:    make(chan Event, bufferSize),  // 环形缓冲
    }
}
```

### 9. 优雅关闭 channel

**面试要点**：由发送方关闭，不由接收方关闭。

**Harness 应用**：

```go
// pkg/scheduler/scheduler.go
func (s *Scheduler) Stop() {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if !s.running {
        return
    }
    
    close(s.stopChan)  // 由管理方关闭
    s.running = false
}

// worker 中使用 select 监听
func (s *Scheduler) worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-s.stopChan:  // 监听关闭信号
            return
        case task := <-s.queue:
            s.executeTask(ctx, task)
        }
    }
}
```

### 10. select 多路复用

**面试要点**：多个 case 就绪时随机选择一个执行。

**Harness 应用**：

```go
// pkg/scheduler/scheduler.go
func (s *Scheduler) run(ctx context.Context) {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-s.stopChan:
            return
        case <-ticker.C:
            s.checkAndRun(ctx)
        }
    }
}
```

### 11. context 的作用

**面试要点**：控制 goroutine 生命周期：取消、超时、传值。

**Harness 应用**：

```go
// internal/core/engine.go
func (e *Engine) ExecuteTask(ctx context.Context, adapterName string, task Task) (*Result, error) {
    // 创建带超时的 context
    ctx, cancel := context.WithTimeout(ctx, e.config.TaskTimeout)
    defer cancel()
    
    // 执行任务
    result, err := adapter.ExecuteTask(ctx, task)
    if err != nil {
        if ctx.Err() == context.DeadlineExceeded {
            return nil, errors.New(ErrCodeTimeout, "任务执行超时")
        }
        return nil, err
    }
    
    return &result, nil
}
```

### 12. WaitGroup 的坑

**面试要点**：Add 必须在 goroutine 启动前调用；不能复制 WaitGroup。

**Harness 应用**：

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
    
    wg.Wait()
    return nil
}
```

## 五、内存管理与 GC

### 13. 逃逸分析

**面试要点**：编译器决定变量分配在栈还是堆。

**Harness 应用**：

```go
// 不好 - 返回指针导致逃逸
func createTask() *Task {
    task := Task{ID: "test"}
    return &task  // task 逃逸到堆
}

// 好 - 值类型在栈上
func processTask(task Task) Task {
    task.Status = "processed"
    return task  // 在栈上
}

// 使用 go build -gcflags="-m" 查看逃逸分析
```

### 14. 内存对齐

**面试要点**：struct 字段顺序影响内存占用。

**Harness 应用**：

```go
// pkg/models/types.go

// 不好 - 24 字节
type BadTaskState struct {
    Task      Task       // 大结构
    Status    bool       // 1 byte + 7 padding
    CreatedAt time.Time  // 24 bytes
}

// 好 - 按大小排序
type GoodTaskState struct {
    CreatedAt time.Time  // 24 bytes
    Task      Task       // 大结构
    Status    TaskStatus // 16 bytes (string)
}
```

### 15. sync.Pool 对象复用

**面试要点**：复用临时对象，减少 GC 压力。

**Harness 应用**：

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

## 六、关键字与语法细节

### 16. defer 的执行顺序

**面试要点**：LIFO（后进先出）；参数在 defer 语句时求值。

**Harness 应用**：

```go
// internal/storage/sqlite.go
func (s *SQLiteStorage) SaveTask(ctx context.Context, state *TaskState) error {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    
    defer tx.Rollback()  // 如果 Commit 成功，Rollback 会报错但不影响
    
    if err := s.saveTaskTx(ctx, tx, state); err != nil {
        return err
    }
    
    return tx.Commit()  // Commit 成功后，Rollback 不会执行实际操作
}
```

### 17. panic 和 recover

**面试要点**：panic 沿调用栈传播；recover 只在 defer 中有效。

**Harness 应用**：

```go
// pkg/middleware/middleware.go
func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            defer func() {
                if err := recover(); err != nil {
                    // 在 defer 中 recover
                    stack := debug.Stack()
                    
                    log.WithFields(map[string]any{
                        "error": err,
                        "stack": string(stack),
                    }).Error("Panic recovered")
                    
                    http.Error(w, "Internal Server Error", 
                        http.StatusInternalServerError)
                }
            }()
            
            next.ServeHTTP(w, r)
        })
    }
}
```

## 七、标准库与实战

### 18. HTTP 中间件链

**面试要点**：函数组合，形成处理链。

**Harness 应用**：

```go
// pkg/middleware/middleware.go
func Chain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        handler = middlewares[i](handler)
    }
    return handler
}

// 使用
handler := Chain(
    businessHandler,
    Logging(logger),
    Recovery(logger),
    CORS([]string{"*"}),
    RateLimit(100),
    Auth(apiKey),
)
```

### 19. 优雅关停

**面试要点**：监听信号 → 停止接收新请求 → 等待现有请求完成 → 关闭资源。

**Harness 应用**：

```go
// cmd/harness/main.go
func main() {
    engine, _ := initializeEngine()
    
    server := &http.Server{
        Addr:    ":8080",
        Handler: engine.Router(),
    }
    
    // 启动服务器
    go func() {
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            log.Fatalf("Server failed: %v", err)
        }
    }()
    
    // 监听信号
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    // 优雅关停
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("Server shutdown failed: %v", err)
    }
    
    // 清理资源
    engine.Cleanup(ctx)
}
```

### 20. singleflight 防止缓存击穿

**面试要点**：相同 key 的并发请求只执行一次。

**Harness 应用**：

```go
// pkg/cache/cache.go
import "golang.org/x/sync/singleflight"

type Cache struct {
    mu    sync.RWMutex
    items map[string]*cacheItem
    group singleflight.Group
}

func (c *Cache) GetOrLoad(key string, loader func() (interface{}, error)) (interface{}, error) {
    // 先查缓存
    c.mu.RLock()
    if item, exists := c.items[key]; exists {
        c.mu.RUnlock()
        return item.value, nil
    }
    c.mu.RUnlock()
    
    // 使用 singleflight 防止并发加载
    v, err, _ := c.group.Do(key, func() (interface{}, error) {
        // 再次检查缓存（可能其他 goroutine 已经加载）
        c.mu.RLock()
        if item, exists := c.items[key]; exists {
            c.mu.RUnlock()
            return item.value, nil
        }
        c.mu.RUnlock()
        
        // 加载数据
        return loader()
    })
    
    if err != nil {
        return nil, err
    }
    
    // 写入缓存
    c.mu.Lock()
    c.items[key] = &cacheItem{value: v}
    c.mu.Unlock()
    
    return v, nil
}
```

## 八、高端编程题

### 21. 协程池实现

**面试要点**：固定数量的 worker 从任务通道取任务执行。

**Harness 应用**：

```go
// pkg/pool/pool.go
type WorkerPool struct {
    workers    int
    taskQueue  chan Task
    workerPool chan struct{}
    wg         sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
    return &WorkerPool{
        workers:    workers,
        taskQueue:  make(chan Task, workers*10),
        workerPool: make(chan struct{}, workers),
    }
}

func (p *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < p.workers; i++ {
        go p.worker(ctx)
    }
}

func (p *WorkerPool) worker(ctx context.Context) {
    for task := range p.taskQueue {
        p.workerPool <- struct{}{}  // 获取信号量
        p.executeTask(ctx, task)
        <-p.workerPool  // 释放信号量
    }
}

func (p *WorkerPool) Submit(task Task) {
    p.taskQueue <- task
}
```

## 九、设计模式应用

### 22. 函数式选项模式

**面试要点**：使用闭包实现灵活的配置。

**Harness 应用**：

```go
// pkg/scheduler/scheduler.go
type Option func(*Scheduler)

func WithMaxConcurrent(n int) Option {
    return func(s *Scheduler) {
        s.maxConcurrent = n
    }
}

func WithCheckInterval(d time.Duration) Option {
    return func(s *Scheduler) {
        s.checkInterval = d
    }
}

func NewScheduler(opts ...Option) *Scheduler {
    s := &Scheduler{
        maxConcurrent: 10,
        checkInterval: 1 * time.Second,
    }
    
    for _, opt := range opts {
        opt(s)
    }
    
    return s
}
```

### 23. 依赖注入

**面试要点**：通过接口注入依赖，方便测试。

**Harness 应用**：

```go
// internal/core/engine.go
type Engine struct {
    taskManager  TaskManager
    stateManager StateManager
    store        Storage
}

type TaskManager interface {
    CreateTask(ctx context.Context, task Task) (*TaskState, error)
    ExecuteTask(ctx context.Context, id string) (*Result, error)
}

func NewEngine(tm TaskManager, sm StateManager, store Storage) *Engine {
    return &Engine{
        taskManager:  tm,
        stateManager: sm,
        store:        store,
    }
}

// 测试时注入 mock
func TestEngine(t *testing.T) {
    mockTM := &MockTaskManager{}
    mockSM := &MockStateManager{}
    mockStore := &MockStorage{}
    
    engine := NewEngine(mockTM, mockSM, mockStore)
    // 测试...
}
```

## 十、总结

通过将 Go 面试知识点与 Harness Engineering 框架结合，我们可以看到：

1. **GMP 模型** → 任务调度器设计
2. **Channel 方向限制** → 事件系统接口
3. **sync 包** → 并发安全保证
4. **错误包装链** → 错误处理体系
5. **内存对齐** → 结构体优化
6. **sync.Pool** → 对象复用
7. **singleflight** → 防止缓存击穿
8. **函数式选项** → 灵活配置
9. **依赖注入** → 可测试架构
10. **优雅关停** → 服务生命周期管理

这些知识点不仅是面试必备，更是构建高质量 Go 项目的基石。
