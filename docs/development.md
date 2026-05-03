# Harness Engineering 开发指南

## 概述

本指南介绍如何开发和扩展 Harness Engineering 框架。

## 项目结构

```
harness/
├── cmd/                    # 命令行工具
│   ├── harness/           # 主服务器
│   └── harness-cli/       # CLI 工具
├── config/                # 配置管理
├── docs/                  # 文档
├── examples/              # 示例代码
├── internal/              # 内部实现
│   ├── adapters/          # 环境适配器
│   ├── admin/             # 管理界面
│   ├── api/               # RESTful API
│   ├── core/              # 核心组件
│   ├── learning/          # 学习系统
│   ├── patterns/          # 模式系统
│   └── storage/           # 存储层
├── k8s/                   # Kubernetes 部署
├── models/                # 数据模型
├── pkg/                   # 公共包
│   ├── cache/             # 缓存
│   ├── distributed/       # 分布式支持
│   ├── errors/            # 错误处理
│   ├── event/             # 事件系统
│   ├── logger/            # 日志系统
│   ├── metrics/           # 指标收集
│   ├── middleware/         # 中间件
│   ├── plugin/            # 插件系统
│   ├── pool/              # 连接池
│   ├── scheduler/         # 任务调度
│   ├── utils/             # 工具函数
│   ├── validator/         # 配置验证
│   └── webhook/           # Webhook 系统
├── scripts/               # 脚本
├── .gitignore
├── Dockerfile
├── docker-compose.yaml
├── go.mod
├── harness.yaml           # 配置文件
├── Makefile
└── README.md
```

## 核心组件

### 1. Engine（核心引擎）

核心引擎是框架的中心协调器，负责：
- 管理所有组件的生命周期
- 协调适配器、任务管理器、状态管理器等
- 处理任务执行和反馈

```go
// 创建引擎
engine, err := core.NewEngine(cfg, store)

// 注册适配器
engine.RegisterAdapter("claude-code", adapters.NewClaudeCodeAdapter())

// 初始化引擎
engine.Initialize(ctx)

// 执行任务
result, err := engine.ExecuteTask(ctx, "claude-code", task)
```

### 2. TaskManager（任务管理器）

任务管理器负责：
- 创建、执行、取消任务
- 任务状态跟踪
- 优先级队列管理

```go
// 创建任务
task := models.Task{
    ID:          "task-1",
    Type:        "implement",
    Description: "实现用户认证功能",
}

state, err := engine.TaskManager().CreateTask(ctx, task)

// 执行任务
result, err := engine.TaskManager().ExecuteTask(ctx, "task-1")

// 获取任务状态
taskState, err := engine.TaskManager().GetTask(ctx, "task-1")
```

### 3. StateManager（状态管理器）

状态管理器负责：
- 会话管理
- 状态更新和历史
- 事件通知

```go
// 创建会话
session, err := engine.StateManager().CreateSession(ctx, "production")

// 更新状态
update := models.StateUpdate{
    Type: "add_task",
    Data: json.RawMessage(`{"id": "task-1"}`),
    Reason: "添加新任务",
}
err := engine.StateManager().UpdateState(ctx, session.ID, update)

// 获取状态
state, err := engine.StateManager().GetState(ctx, session.ID)
```

### 4. FeedbackLoop（反馈循环）

反馈循环负责：
- 验证任务结果
- 自动修复问题
- 记录指标

```go
// 添加验证器
engine.FeedbackLoop().AddValidator(&SecurityValidator{})

// 添加修复器
engine.FeedbackLoop().AddFixer(&AutoFixer{})

// 处理反馈
feedback, err := engine.ProcessFeedback(ctx, result)
```

### 5. KnowledgeBase（知识库）

知识库负责：
- 知识条目管理
- 全文搜索
- 索引支持

```go
// 添加知识
entry := models.KnowledgeEntry{
    ID:      "knowledge-1",
    Type:    "pattern",
    Title:   "Go 错误处理最佳实践",
    Content: "使用 errors.Wrap 包装错误...",
}
err := engine.Knowledge().AddEntry(ctx, entry)

// 搜索知识
results, err := engine.Knowledge().Search(ctx, "错误处理", 10)
```

### 6. PatternEngine（模式引擎）

模式引擎负责：
- 模式匹配
- 学习器集成
- 成功率统计

```go
// 添加模式
pattern := models.Pattern{
    ID:      "pattern-1",
    Name:    "用户认证模式",
    Trigger: "认证|authentication",
}
err := engine.Pattern().AddPattern(ctx, pattern)

// 匹配模式
matched, err := engine.Pattern().Match(ctx, task)
```

## 环境适配器

### 创建自定义适配器

```go
type MyAdapter struct {
    config config.AdapterConfig
}

func NewMyAdapter() *MyAdapter {
    return &MyAdapter{}
}

func (a *MyAdapter) Name() string {
    return "my-adapter"
}

func (a *MyAdapter) Initialize(ctx context.Context, cfg config.AdapterConfig) error {
    a.config = cfg
    return nil
}

func (a *MyAdapter) ExecuteTask(ctx context.Context, task models.Task) (models.Result, error) {
    // 实现任务执行逻辑
    return models.Result{
        TaskID: task.ID,
        Status: models.TaskStatusCompleted,
        Output: "任务完成",
    }, nil
}

func (a *MyAdapter) GetState(ctx context.Context) (models.State, error) {
    // 实现状态获取逻辑
    return models.State{}, nil
}

func (a *MyAdapter) Cleanup(ctx context.Context) error {
    // 实现清理逻辑
    return nil
}
```

### 注册适配器

```go
engine.RegisterAdapter("my-adapter", NewMyAdapter())
```

## 存储层

### 创建自定义存储

```go
type MyStorage struct {
    // 实现存储接口
}

func (s *MyStorage) SaveTask(ctx context.Context, state *models.TaskState) error {
    // 实现保存任务逻辑
    return nil
}

func (s *MyStorage) GetTask(ctx context.Context, id string) (*models.TaskState, error) {
    // 实现获取任务逻辑
    return nil, nil
}

// ... 其他接口方法
```

## 事件系统

### 订阅事件

```go
bus := event.NewEventBus(event.EventBusConfig{})

// 订阅任务创建事件
bus.Subscribe(event.EventTaskCreated, func(ctx context.Context, event event.Event) error {
    fmt.Printf("任务创建: %v\n", event.Data)
    return nil
})

// 发布事件
bus.Publish(ctx, event.Event{
    Type: event.EventTaskCreated,
    Data: map[string]any{"task_id": "task-1"},
})
```

## 任务调度

### 创建定时任务

```go
scheduler := scheduler.NewScheduler(scheduler.SchedulerConfig{})

task := &scheduler.Task{
    ID:       "cleanup",
    Name:     "清理过期数据",
    Type:     scheduler.ScheduleDaily,
    Schedule: "02:00",
    Handler: func(ctx context.Context, task *scheduler.Task) error {
        // 执行清理逻辑
        return nil
    },
    Enabled: true,
}

scheduler.AddTask(task)
scheduler.Start(ctx)
```

## Webhook 系统

### 注册 Webhook

```go
mgr := webhook.NewWebhookManager(webhook.WebhookManagerConfig{})

webhook := &webhook.Webhook{
    ID:      "notify",
    Name:    "通知 Webhook",
    URL:     "https://example.com/webhook",
    Events:  []string{"task.completed", "task.failed"},
    Enabled: true,
}

mgr.AddWebhook(webhook)

// 触发 Webhook
mgr.Fire(ctx, "task.completed", map[string]any{"task_id": "task-1"})
```

## 指标收集

### 使用全局指标

```go
// 增加计数器
metrics.TaskCounter.Inc()
metrics.SuccessCounter.Add(1)

// 设置仪表
metrics.ActiveTasksGauge.Set(10)

// 记录直方图
metrics.TaskDurationHistogram.Observe(5.0)

// 获取指标
registry := metrics.GetDefaultRegistry()
allMetrics := registry.GetAll()
```

## 插件系统

### 创建插件

```go
type MyPlugin struct {
    name string
}

func (p *MyPlugin) Name() string {
    return p.name
}

func (p *MyPlugin) Version() string {
    return "1.0.0"
}

func (p *MyPlugin) Description() string {
    return "我的插件"
}

func (p *MyPlugin) Initialize(ctx context.Context, config map[string]any) error {
    return nil
}

func (p *MyPlugin) Start(ctx context.Context) error {
    return nil
}

func (p *MyPlugin) Stop(ctx context.Context) error {
    return nil
}

func (p *MyPlugin) Health(ctx context.Context) error {
    return nil
}
```

## 分布式支持

### 创建集群

```go
cfg := distributed.ClusterConfig{
    NodeID:  "node-1",
    Address: "localhost",
    Port:    8080,
}

cluster := distributed.NewCluster(cfg)
cluster.Start(ctx)

// 添加节点
node := &distributed.Node{
    ID:       "node-2",
    Address:  "localhost",
    Port:     8081,
    Status:   distributed.NodeStatusActive,
}
cluster.AddNode(node)

// 任务分配
dist := distributed.NewTaskDistribution(cluster, distributed.StrategyRoundRobin)
targetNode, err := dist.Distribute("task-1")
```

## 测试

### 运行测试

```bash
# 运行所有测试
make test

# 运行特定包的测试
go test ./pkg/cache/...
go test ./pkg/event/...
go test ./internal/core/...

# 运行基准测试
make bench
```

### 编写测试

```go
func TestMyFunction(t *testing.T) {
    // 准备
    expected := "expected value"
    
    // 执行
    result := MyFunction()
    
    // 验证
    if result != expected {
        t.Errorf("Expected %s, got %s", expected, result)
    }
}
```

## 最佳实践

### 1. 错误处理

```go
if err != nil {
    return errors.Wrap(err, errors.ErrCodeInternal, "操作失败")
}
```

### 2. 日志记录

```go
logger.WithFields(map[string]any{
    "task_id": task.ID,
    "status":  task.Status,
}).Info("任务执行完成")
```

### 3. 指标收集

```go
start := time.Now()
// 执行操作
duration := time.Since(start)
metrics.TaskDurationHistogram.Observe(duration.Seconds())
```

### 4. 配置验证

```go
validator := validator.New()
result := validator.Validate(cfg)
if !result.Valid {
    for _, err := range result.Errors {
        fmt.Printf("配置错误: %s - %s\n", err.Field, err.Message)
    }
}
```

## 故障排除

### 常见问题

1. **编译错误**
   - 检查 Go 版本（需要 1.21+）
   - 运行 `go mod tidy` 更新依赖

2. **测试失败**
   - 检查测试环境
   - 查看错误日志

3. **性能问题**
   - 使用基准测试定位瓶颈
   - 检查并发设置

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 编写测试
4. 提交更改
5. 创建 Pull Request

## 许可证

MIT License
