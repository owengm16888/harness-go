# Go 设计模式在 Harness Engineering 中的应用

## 概述

本文档深入分析 Go 语言中的设计模式，并展示如何在 Harness Engineering 框架中应用这些模式。

## 一、创建型模式

### 1.1 函数式选项模式（Functional Options）

**面试要点**：使用闭包实现灵活的配置。

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

func WithTimeout(d time.Duration) Option {
    return func(s *Scheduler) {
        s.timeout = d
    }
}

func NewScheduler(opts ...Option) *Scheduler {
    s := &Scheduler{
        maxConcurrent: 10,
        checkInterval: 1 * time.Second,
        timeout:       30 * time.Second,
    }
    
    for _, opt := range opts {
        opt(s)
    }
    
    return s
}

// 使用
scheduler := NewScheduler(
    WithMaxConcurrent(20),
    WithCheckInterval(5 * time.Second),
    WithTimeout(time.Minute),
)
```

### 1.2 建造者模式（Builder）

```go
// pkg/models/builder.go

type TaskBuilder struct {
    task *Task
}

func NewTaskBuilder() *TaskBuilder {
    return &TaskBuilder{
        task: &Task{
            Context: make(map[string]any),
        },
    }
}

func (b *TaskBuilder) WithID(id string) *TaskBuilder {
    b.task.ID = id
    return b
}

func (b *TaskBuilder) WithType(taskType string) *TaskBuilder {
    b.task.Type = taskType
    return b
}

func (b *TaskBuilder) WithDescription(desc string) *TaskBuilder {
    b.task.Description = desc
    return b
}

func (b *TaskBuilder) WithConstraint(constraint Constraint) *TaskBuilder {
    b.task.Constraints = append(b.task.Constraints, constraint)
    return b
}

func (b *TaskBuilder) Build() (*Task, error) {
    if b.task.ID == "" {
        return nil, errors.New("task ID is required")
    }
    if b.task.Type == "" {
        return nil, errors.New("task type is required")
    }
    return b.task, nil
}

// 使用
task, err := NewTaskBuilder().
    WithID("task-1").
    WithType("implement").
    WithDescription("实现用户认证功能").
    WithConstraint(Constraint{
        Type:     "security",
        Rule:     "no-hardcoded-secrets",
        Severity: SeverityError,
    }).
    Build()
```

### 1.3 单例模式（Singleton）

```go
// pkg/config/config.go

var (
    instance *Config
    once     sync.Once
)

func GetConfig() *Config {
    once.Do(func() {
        instance = loadConfig()
    })
    return instance
}

// 使用
config := GetConfig()
```

## 二、结构型模式

### 2.1 适配器模式（Adapter）

**面试要点**：将一个接口转换成另一个接口。

```go
// internal/adapters/adapter.go

// 目标接口
type Adapter interface {
    Name() string
    ExecuteTask(ctx context.Context, task Task) (Result, error)
}

// Claude Code 适配器
type ClaudeCodeAdapter struct {
    config Config
}

func (a *ClaudeCodeAdapter) Name() string {
    return "claude-code"
}

func (a *ClaudeCodeAdapter) ExecuteTask(ctx context.Context, task Task) (Result, error) {
    // 将通用任务转换为 Claude Code 特定格式
    claudeTask := a.convertTask(task)
    
    // 执行任务
    result, err := a.executeClaudeTask(ctx, claudeTask)
    if err != nil {
        return Result{}, err
    }
    
    // 将结果转换回通用格式
    return a.convertResult(result), nil
}

// Hermes 适配器
type HermesAdapter struct {
    client *http.Client
    url    string
}

func (a *HermesAdapter) Name() string {
    return "hermes"
}

func (a *HermesAdapter) ExecuteTask(ctx context.Context, task Task) (Result, error) {
    // 将通用任务转换为 HTTP 请求
    req := a.createRequest(task)
    
    // 发送请求
    resp, err := a.client.Do(req)
    if err != nil {
        return Result{}, err
    }
    
    // 将响应转换回通用格式
    return a.parseResponse(resp), nil
}
```

### 2.2 装饰器模式（Decorator）

```go
// pkg/middleware/middleware.go

// HTTP 中间件 - 装饰器模式
type Middleware func(http.Handler) http.Handler

// 日志装饰器
func Logging(log *logger.Logger) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            next.ServeHTTP(w, r)
            
            log.WithFields(map[string]any{
                "method":   r.Method,
                "path":     r.URL.Path,
                "duration": time.Since(start),
            }).Info("Request completed")
        })
    }
}

// 认证装饰器
func Auth(apiKey string) Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.Header.Get("Authorization") != "Bearer "+apiKey {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}

// 限流装饰器
func RateLimit(limit int) Middleware {
    return func(next http.Handler) http.Handler {
        limiter := NewRateLimiter(limit)
        
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if !limiter.Allow() {
                http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}

// 链式装饰器
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
    for i := len(middlewares) - 1; i >= 0; i-- {
        handler = middlewares[i](handler)
    }
    return handler
}

// 使用
handler := Chain(
    businessHandler,
    Logging(logger),
    Auth(apiKey),
    RateLimit(100),
)
```

### 2.3 代理模式（Proxy）

```go
// pkg/cache/cache.go

// 缓存代理
type CacheProxy struct {
    cache   Cache
    loader  func(key string) (interface{}, error)
}

func (p *CacheProxy) Get(key string) (interface{}, error) {
    // 先查缓存
    if value, exists := p.cache.Get(key); exists {
        return value, nil
    }
    
    // 缓存未命中，加载数据
    value, err := p.loader(key)
    if err != nil {
        return nil, err
    }
    
    // 写入缓存
    p.cache.Set(key, value, 10*time.Minute)
    
    return value, nil
}

// 使用
proxy := &CacheProxy{
    cache: NewMemoryCache(),
    loader: func(key string) (interface{}, error) {
        return loadFromDB(key)
    },
}

value, err := proxy.Get("user:123")
```

### 2.4 外观模式（Facade）

```go
// internal/core/engine.go

// 引擎外观 - 简化复杂子系统
type Engine struct {
    taskManager  *TaskManager
    stateManager *StateManager
    knowledge    *KnowledgeBase
    pattern      *PatternEngine
    monitor      *Monitor
}

// 简化的接口
func (e *Engine) ExecuteTask(ctx context.Context, task Task) (*Result, error) {
    // 1. 验证任务
    if err := e.validateTask(task); err != nil {
        return nil, err
    }
    
    // 2. 创建任务状态
    state, err := e.taskManager.CreateTask(ctx, task)
    if err != nil {
        return nil, err
    }
    
    // 3. 执行任务
    result, err := e.executeTask(ctx, state)
    if err != nil {
        return nil, err
    }
    
    // 4. 处理反馈
    if err := e.processFeedback(ctx, result); err != nil {
        return nil, err
    }
    
    // 5. 记录指标
    e.monitor.RecordTask(result)
    
    return result, nil
}
```

## 三、行为型模式

### 3.1 观察者模式（Observer）

**面试要点**：定义对象间的一对多依赖关系。

```go
// pkg/event/event.go

// 观察者接口
type Observer interface {
    OnEvent(event Event)
}

// 主题
type EventBus struct {
    observers map[EventType][]Observer
    mu        sync.RWMutex
}

func (eb *EventBus) Subscribe(eventType EventType, observer Observer) {
    eb.mu.Lock()
    defer eb.mu.Unlock()
    
    eb.observers[eventType] = append(eb.observers[eventType], observer)
}

func (eb *EventBus) Unsubscribe(eventType EventType, observer Observer) {
    eb.mu.Lock()
    defer eb.mu.Unlock()
    
    observers := eb.observers[eventType]
    for i, o := range observers {
        if o == observer {
            eb.observers[eventType] = append(observers[:i], observers[i+1:]...)
            break
        }
    }
}

func (eb *EventBus) Publish(event Event) {
    eb.mu.RLock()
    defer eb.mu.RUnlock()
    
    for _, observer := range eb.observers[event.Type] {
        go observer.OnEvent(event)  // 异步通知
    }
}

// 具体观察者
type TaskLogger struct{}

func (l *TaskLogger) OnEvent(event Event) {
    fmt.Printf("Task event: %s\n", event.Type)
}

type TaskMetrics struct{}

func (m *TaskMetrics) OnEvent(event Event) {
    metrics.TaskCounter.Inc()
}

// 使用
bus := NewEventBus()
bus.Subscribe(EventTaskCreated, &TaskLogger{})
bus.Subscribe(EventTaskCreated, &TaskMetrics{})

bus.Publish(Event{Type: EventTaskCreated, Data: map[string]any{"task_id": "1"}})
```

### 3.2 策略模式（Strategy）

```go
// pkg/distributed/distributed.go

// 策略接口
type DistributionStrategy interface {
    SelectNode(nodes []*Node, task *Task) *Node
}

// 轮询策略
type RoundRobinStrategy struct {
    current int
}

func (s *RoundRobinStrategy) SelectNode(nodes []*Node, task *Task) *Node {
    node := nodes[s.current%len(nodes)]
    s.current++
    return node
}

// 最少负载策略
type LeastLoadStrategy struct{}

func (s *LeastLoadStrategy) SelectNode(nodes []*Node, task *Task) *Node {
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

// 随机策略
type RandomStrategy struct{}

func (s *RandomStrategy) SelectNode(nodes []*Node, task *Task) *Node {
    return nodes[rand.Intn(len(nodes))]
}

// 上下文
type TaskDistribution struct {
    strategy DistributionStrategy
    nodes    []*Node
}

func NewTaskDistribution(strategy DistributionStrategy) *TaskDistribution {
    return &TaskDistribution{
        strategy: strategy,
    }
}

func (d *TaskDistribution) Distribute(task *Task) *Node {
    return d.strategy.SelectNode(d.nodes, task)
}

// 使用
dist := NewTaskDistribution(&LeastLoadStrategy{})
node := dist.Distribute(task)
```

### 3.3 命令模式（Command）

```go
// pkg/command/command.go

// 命令接口
type Command interface {
    Execute(ctx context.Context) error
    Undo(ctx context.Context) error
}

// 创建任务命令
type CreateTaskCommand struct {
    manager *TaskManager
    task    Task
    state   *TaskState
}

func (c *CreateTaskCommand) Execute(ctx context.Context) error {
    state, err := c.manager.CreateTask(ctx, c.task)
    if err != nil {
        return err
    }
    c.state = state
    return nil
}

func (c *CreateTaskCommand) Undo(ctx context.Context) error {
    if c.state != nil {
        return c.manager.DeleteTask(ctx, c.state.Task.ID)
    }
    return nil
}

// 执行任务命令
type ExecuteTaskCommand struct {
    manager *TaskManager
    taskID  string
    result  *Result
}

func (c *ExecuteTaskCommand) Execute(ctx context.Context) error {
    result, err := c.manager.ExecuteTask(ctx, c.taskID)
    if err != nil {
        return err
    }
    c.result = result
    return nil
}

func (c *ExecuteTaskCommand) Undo(ctx context.Context) error {
    // 无法撤销任务执行
    return nil
}

// 命令历史
type CommandHistory struct {
    commands []Command
    current  int
}

func (h *CommandHistory) Execute(ctx context.Context, cmd Command) error {
    if err := cmd.Execute(ctx); err != nil {
        return err
    }
    
    h.commands = append(h.commands[:h.current], cmd)
    h.current++
    
    return nil
}

func (h *CommandHistory) Undo(ctx context.Context) error {
    if h.current <= 0 {
        return errors.New("nothing to undo")
    }
    
    h.current--
    return h.commands[h.current].Undo(ctx)
}
```

### 3.4 状态模式（State）

```go
// pkg/task/state.go

// 状态接口
type TaskState interface {
    Execute(ctx context.Context, task *Task) error
    Cancel(ctx context.Context, task *Task) error
    Complete(ctx context.Context, task *Task) error
}

// 待执行状态
type PendingState struct{}

func (s *PendingState) Execute(ctx context.Context, task *Task) error {
    task.Status = TaskStatusRunning
    task.state = &RunningState{}
    return nil
}

func (s *PendingState) Cancel(ctx context.Context, task *Task) error {
    task.Status = TaskStatusCancelled
    task.state = &CancelledState{}
    return nil
}

func (s *PendingState) Complete(ctx context.Context, task *Task) error {
    return errors.New("cannot complete pending task")
}

// 运行状态
type RunningState struct{}

func (s *RunningState) Execute(ctx context.Context, task *Task) error {
    return errors.New("task is already running")
}

func (s *RunningState) Cancel(ctx context.Context, task *Task) error {
    task.Status = TaskStatusCancelled
    task.state = &CancelledState{}
    return nil
}

func (s *RunningState) Complete(ctx context.Context, task *Task) error {
    task.Status = TaskStatusCompleted
    task.state = &CompletedState{}
    return nil
}

// 任务
type Task struct {
    ID     string
    Status TaskStatus
    state  TaskState
}

func (t *Task) Execute(ctx context.Context) error {
    return t.state.Execute(ctx, t)
}

func (t *Task) Cancel(ctx context.Context) error {
    return t.state.Cancel(ctx, t)
}

func (t *Task) Complete(ctx context.Context) error {
    return t.state.Complete(ctx, t)
}
```

### 3.5 模板方法模式（Template Method）

```go
// internal/core/engine.go

// 模板方法
type TaskProcessor interface {
    validate(task Task) error
    prepare(ctx context.Context, task Task) error
    execute(ctx context.Context, task Task) (*Result, error)
    cleanup(ctx context.Context, task Task) error
}

// 基类
type BaseProcessor struct {
    logger *logger.Logger
}

func (p *BaseProcessor) Process(ctx context.Context, task Task) (*Result, error) {
    // 模板方法 - 定义算法骨架
    if err := p.validate(task); err != nil {
        return nil, err
    }
    
    if err := p.prepare(ctx, task); err != nil {
        return nil, err
    }
    
    result, err := p.execute(ctx, task)
    if err != nil {
        return nil, err
    }
    
    if err := p.cleanup(ctx, task); err != nil {
        p.logger.Warn("Cleanup failed", "error", err)
    }
    
    return result, nil
}

// 具体实现
type ImplementProcessor struct {
    BaseProcessor
}

func (p *ImplementProcessor) validate(task Task) error {
    if task.Type != "implement" {
        return errors.New("invalid task type")
    }
    return nil
}

func (p *ImplementProcessor) prepare(ctx context.Context, task Task) error {
    // 准备实现环境
    return nil
}

func (p *ImplementProcessor) execute(ctx context.Context, task Task) (*Result, error) {
    // 执行实现
    return &Result{Status: TaskStatusCompleted}, nil
}

func (p *ImplementProcessor) cleanup(ctx context.Context, task Task) error {
    // 清理资源
    return nil
}
```

## 四、并发模式

### 4.1 生产者-消费者模式

```go
// pkg/scheduler/scheduler.go

type ProducerConsumer struct {
    queue    chan Task
    workers  int
    wg       sync.WaitGroup
}

func NewProducerConsumer(workers int, queueSize int) *ProducerConsumer {
    return &ProducerConsumer{
        queue:   make(chan Task, queueSize),
        workers: workers,
    }
}

// 生产者
func (pc *ProducerConsumer) Produce(tasks []Task) {
    for _, task := range tasks {
        pc.queue <- task
    }
    close(pc.queue)
}

// 消费者
func (pc *ProducerConsumer) Consume(ctx context.Context) {
    for i := 0; i < pc.workers; i++ {
        pc.wg.Add(1)
        go func() {
            defer pc.wg.Done()
            
            for task := range pc.queue {
                pc.processTask(ctx, task)
            }
        }()
    }
    
    pc.wg.Wait()
}
```

### 4.2 扇出-扇入模式

```go
// pkg/pipeline/pipeline.go

// 扇出：将任务分发给多个 worker
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

// 扇入：将多个结果汇聚到一个通道
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

## 五、总结

Go 设计模式在 Harness Engineering 中的应用：

| 模式 | 应用场景 |
|------|---------|
| 函数式选项 | 配置管理 |
| 建造者 | 复杂对象创建 |
| 单例 | 全局配置、连接池 |
| 适配器 | 多环境适配 |
| 装饰器 | HTTP 中间件 |
| 代理 | 缓存代理 |
| 外观 | 简化复杂系统 |
| 观察者 | 事件系统 |
| 策略 | 负载均衡 |
| 命令 | 操作历史 |
| 状态 | 任务状态管理 |
| 模板方法 | 处理流程 |
| 生产者-消费者 | 任务调度 |
| 扇出-扇入 | 并行处理 |

通过合理使用这些设计模式，可以构建出灵活、可扩展、易维护的系统。
