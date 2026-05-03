# Harness Engineering 架构文档

## 概述

Harness Engineering 是一个用于 AI Agent（特别是 Coding Agent）的"驾驭系统工程"框架。它通过约束、反馈回路、工作流控制、工具接口与持续改进，让 Agent **稳定、可靠、可控** 地完成复杂任务。

## 核心概念

### Agent = Model + Harness

- **Model（模型）** 提供智能（推理、生成、理解能力）
- **Harness（驾驭系统）** 提供约束、工具、反馈和执行环境

### Human Steer, Agent Execute

人类掌舵，智能体执行。AI 模型就像一匹强壮但可能"任性"的马，Harness 就是那套让它安全奔跑的装备。

## 系统架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Harness Core (Go)                         │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   Adapter   │  │   Adapter   │  │   Adapter   │         │
│  │ Claude Code │  │   Hermes    │  │  Codex CLI  │         │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘         │
│         │                │                │                 │
│  ┌──────┴────────────────┴────────────────┴──────┐         │
│  │              Core Engine                       │         │
│  ├───────────────────────────────────────────────┤         │
│  │  Task Manager  │  State Manager  │  Feedback  │         │
│  ├───────────────────────────────────────────────┤         │
│  │  Knowledge Base │  Pattern Engine │  Monitor  │         │
│  └───────────────────────────────────────────────┘         │
│                           │                                 │
│  ┌────────────────────────┴────────────────────────┐       │
│  │              Storage Layer                       │       │
│  ├─────────────────────────────────────────────────┤       │
│  │  SQLite  │  File System  │  Memory Cache        │       │
│  └─────────────────────────────────────────────────┘       │
└─────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. Engine（核心引擎）

**职责**：
- 协调所有组件
- 管理适配器生命周期
- 处理任务执行和反馈

**接口**：
```go
type Engine struct {
    adapters     map[string]Adapter
    taskManager  *TaskManager
    stateManager *StateManager
    feedbackLoop *FeedbackLoop
    knowledge    *KnowledgeBase
    pattern      *PatternEngine
    monitor      *Monitor
    storage      storage.Storage
}
```

### 2. TaskManager（任务管理器）

**职责**：
- 创建、执行、取消任务
- 任务状态跟踪
- 优先级队列

**接口**：
```go
type TaskManager interface {
    CreateTask(ctx context.Context, task Task) (*TaskState, error)
    ExecuteTask(ctx context.Context, taskID string) (*Result, error)
    GetTask(ctx context.Context, taskID string) (*TaskState, error)
    ListTasks(ctx context.Context, filter TaskFilter) ([]*TaskState, error)
    CancelTask(ctx context.Context, taskID string) error
}
```

### 3. StateManager（状态管理器）

**职责**：
- 会话管理
- 状态更新和历史
- 事件通知

**接口**：
```go
type StateManager interface {
    CreateSession(ctx context.Context, env string) (*Session, error)
    UpdateState(ctx context.Context, sessionID string, update StateUpdate) error
    GetState(ctx context.Context, sessionID string) (*State, error)
    GetHistory(ctx context.Context, sessionID string) ([]StateSnapshot, error)
    ListSessions(ctx context.Context) ([]*Session, error)
    DeleteSession(ctx context.Context, sessionID string) error
}
```

### 4. FeedbackLoop（反馈循环）

**职责**：
- 验证器和修复器
- 自动修复支持
- 指标记录

**接口**：
```go
type FeedbackLoop interface {
    AddValidator(v Validator)
    AddFixer(f Fixer)
    Process(ctx context.Context, result Result) (*FeedbackResult, error)
}
```

### 5. KnowledgeBase（知识库）

**职责**：
- 知识条目管理
- 全文搜索
- 索引支持

**接口**：
```go
type KnowledgeBase interface {
    AddEntry(ctx context.Context, entry KnowledgeEntry) error
    Search(ctx context.Context, query string, limit int) ([]*KnowledgeEntry, error)
    GetEntry(ctx context.Context, id string) (*KnowledgeEntry, error)
    UpdateEntry(ctx context.Context, id string, update KnowledgeUpdate) error
    DeleteEntry(ctx context.Context, id string) error
    ListEntries(ctx context.Context, offset, limit int) ([]*KnowledgeEntry, error)
}
```

### 6. PatternEngine（模式引擎）

**职责**：
- 模式匹配
- 学习器集成
- 成功率统计

**接口**：
```go
type PatternEngine interface {
    Match(ctx context.Context, task Task) ([]*Pattern, error)
    Learn(ctx context.Context, observation Observation) error
    AddPattern(ctx context.Context, pattern Pattern) error
    UpdatePattern(ctx context.Context, id string, update PatternUpdate) error
    DeletePattern(ctx context.Context, id string) error
    ListPatterns(ctx context.Context) ([]*Pattern, error)
    GetPattern(ctx context.Context, id string) (*Pattern, error)
}
```

### 7. Monitor（监控器）

**职责**：
- 任务指标
- 反馈指标
- 健康检查

**接口**：
```go
type Monitor interface {
    RecordTask(result Result)
    RecordFeedback(result *FeedbackResult)
    GetMetrics() *Metrics
    GetFeedback(taskID string) []*FeedbackResult
}
```

## 环境适配器

### Claude Code 适配器

**职责**：
- 支持 Hook 系统（PreToolUse/PostToolUse）
- 支持 Plans.md 状态管理
- 支持约束检查

**配置**：
```yaml
adapters:
  claude_code:
    enabled: true
    root_dir: "."
    hooks_path: ".claude-plugin/hooks.json"
    plans_path: "Plans.md"
```

### Hermes 适配器

**职责**：
- 支持 HTTP API 调用
- 支持会话管理
- 支持任务执行

**配置**：
```yaml
adapters:
  hermes:
    enabled: true
    url: "http://localhost:3000"
    api_key: "${HERMES_API_KEY}"
```

### Codex CLI 适配器

**职责**：
- 支持 Codex CLI 命令
- 支持 AGENTS.md 解析
- 支持约束传递

**配置**：
```yaml
adapters:
  codex_cli:
    enabled: true
    root_dir: "."
    agents_path: "AGENTS.md"
```

## 数据流

### 任务执行流程

```
1. 用户创建任务
   ↓
2. TaskManager 创建任务状态
   ↓
3. 适配器执行任务
   ↓
4. FeedbackLoop 处理反馈
   ↓
5. PatternEngine 学习模式
   ↓
6. Monitor 记录指标
   ↓
7. 返回任务结果
```

### 状态管理流程

```
1. 用户创建会话
   ↓
2. StateManager 创建会话状态
   ↓
3. 用户更新状态
   ↓
4. StateManager 保存历史
   ↓
5. 事件通知
```

### 知识管理流程

```
1. 用户添加知识
   ↓
2. KnowledgeBase 索引知识
   ↓
3. 用户搜索知识
   ↓
4. KnowledgeBase 返回结果
```

### 模式匹配流程

```
1. 用户提交任务
   ↓
2. PatternEngine 匹配模式
   ↓
3. 返回匹配的模式
   ↓
4. 用户执行模式
   ↓
5. PatternEngine 学习结果
```

## 存储层

### SQLite 存储

**职责**：
- 持久化存储
- 事务支持
- 并发控制

**表结构**：
- `tasks`: 任务表
- `sessions`: 会话表
- `knowledge`: 知识表
- `patterns`: 模式表

### 内存存储

**职责**：
- 高速缓存
- 临时数据
- 会话状态

## 中间件

### 请求处理链

```
Request
  ↓
RequestID
  ↓
Logging
  ↓
Recovery
  ↓
CORS
  ↓
RateLimit
  ↓
Auth
  ↓
ErrorHandler
  ↓
Handler
  ↓
Response
```

## 错误处理

### 错误类型

- **ValidationError**: 验证错误
- **NotFoundError**: 资源不存在
- **ConflictError**: 资源冲突
- **InternalError**: 内部错误

### 错误响应格式

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "错误描述",
    "details": {}
  }
}
```

## 配置管理

### 配置文件

```yaml
server:
  addr: ":8080"
  timeout: 30s

engine:
  max_concurrent_tasks: 10
  task_timeout: 5m
  retry_count: 3

adapters:
  claude_code:
    enabled: true
    root_dir: "."
    hooks_path: ".claude-plugin/hooks.json"
    plans_path: "Plans.md"
  
  hermes:
    enabled: true
    url: "http://localhost:3000"
    api_key: "${HERMES_API_KEY}"
  
  codex_cli:
    enabled: true
    root_dir: "."
    agents_path: "AGENTS.md"

storage:
  type: "sqlite"
  path: "./data/harness.db"
```

### 环境变量

- `HERMES_API_KEY`: Hermes API 密钥
- `HARNESS_CONFIG`: 配置文件路径
- `HARNESS_LOG_LEVEL`: 日志级别

## 部署架构

### 单机部署

```
┌─────────────────┐
│   Harness       │
│   Engine        │
│   + SQLite      │
└─────────────────┘
```

### Docker 部署

```
┌─────────────────┐
│   Docker        │
│   Container     │
│   + Harness     │
│   + SQLite      │
└─────────────────┘
```

### Kubernetes 部署

```
┌─────────────────┐
│   Kubernetes    │
│   Cluster       │
│   + Pods        │
│   + Services    │
│   + PVC         │
└─────────────────┘
```

## 性能优化

### 连接池

- 数据库连接池
- HTTP 客户端连接池

### 缓存

- 内存缓存
- Redis 缓存（可选）

### 并发

- Goroutine 池
- 任务队列

## 安全性

### 认证

- API Key 认证
- Bearer Token 认证

### 授权

- 基于角色的访问控制
- 资源级权限

### 数据保护

- 输入验证
- SQL 注入防护
- XSS 防护

## 监控与告警

### 指标

- 任务成功率
- 平均响应时间
- 错误率
- 资源使用率

### 日志

- 结构化日志
- 日志级别
- 日志轮转

### 告警

- 错误告警
- 性能告警
- 资源告警

## 扩展性

### 插件系统

- 适配器插件
- 验证器插件
- 修复器插件

### API 扩展

- RESTful API
- gRPC API（计划中）
- WebSocket API（计划中）

## 未来规划

### 短期（1-3 个月）

- 完善测试覆盖
- 优化性能
- 添加更多适配器

### 中期（3-6 个月）

- 支持多 Agent 协作
- 添加可视化界面
- 支持分布式部署

### 长期（6-12 个月）

- 支持更多 AI 模型
- 建立生态系统
- 开源社区建设
