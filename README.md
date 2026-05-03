# Harness Engineering - Golang 后端实现

## 概述

Harness Engineering 是一个用于 AI Agent（特别是 Coding Agent）的"驾驭系统工程"框架。它通过约束、反馈回路、工作流控制、工具接口与持续改进，让 Agent **稳定、可靠、可控** 地完成复杂任务。

## 核心概念

### Agent = Model + Harness

- **Model（模型）** 提供智能（推理、生成、理解能力）
- **Harness（驾驭系统）** 提供约束、工具、反馈和执行环境

### Human Steer, Agent Execute

人类掌舵，智能体执行。AI 模型就像一匹强壮但可能"任性"的马，Harness 就是那套让它安全奔跑的装备。

## 架构设计

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
- 协调所有组件
- 管理适配器生命周期
- 处理任务执行和反馈

### 2. TaskManager（任务管理器）
- 创建、执行、取消任务
- 任务状态跟踪
- 优先级队列

### 3. StateManager（状态管理器）
- 会话管理
- 状态更新和历史
- 事件通知

### 4. FeedbackLoop（反馈循环）
- 验证器和修复器
- 自动修复支持
- 指标记录

### 5. KnowledgeBase（知识库）
- 知识条目管理
- 全文搜索
- 索引支持

### 6. PatternEngine（模式引擎）
- 模式匹配
- 学习器集成
- 成功率统计

### 7. Monitor（监控器）
- 任务指标
- 反馈指标
- 健康检查

## 环境适配器

### Claude Code 适配器
- 支持 Hook 系统（PreToolUse/PostToolUse）
- 支持 Plans.md 状态管理
- 支持约束检查

### Hermes 适配器
- 支持 HTTP API 调用
- 支持会话管理
- 支持任务执行

### Codex CLI 适配器
- 支持 Codex CLI 命令
- 支持 AGENTS.md 解析
- 支持约束传递

## 快速开始

### 1. 安装依赖

```bash
go mod download
```

### 2. 配置

编辑 `harness.yaml` 文件：

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

### 3. 运行

```bash
# 使用 Makefile
make run

# 或直接运行
go run cmd/harness/main.go
```

### 4. 使用 CLI

```bash
# 构建 CLI
go build -o harness-cli cmd/harness-cli/main.go

# 查看帮助
./harness-cli --help

# 创建任务
./harness-cli task create --id task-1 --type implement --description "实现用户认证功能"

# 执行任务
./harness-cli task execute task-1

# 列出任务
./harness-cli task list

# 搜索知识
./harness-cli knowledge search "认证"

# 匹配模式
./harness-cli pattern match --type implement --description "实现用户认证功能"
```

### 5. 使用 Docker

```bash
# 构建镜像
docker build -t harness .

# 运行容器
docker run -p 8080:8080 -p 9090:9090 -v ./data:/app/data harness
```

### 6. 使用 Kubernetes

```bash
# 部署
kubectl apply -f k8s/deployment.yaml

# 查看状态
kubectl get pods -n harness
```

## API 接口

### 任务 API

- `POST /api/tasks` - 创建任务
- `GET /api/tasks` - 列出任务
- `GET /api/tasks/{id}` - 获取任务
- `POST /api/tasks/{id}/execute` - 执行任务
- `POST /api/tasks/{id}/cancel` - 取消任务

### 状态 API

- `POST /api/sessions` - 创建会话
- `GET /api/sessions` - 列出会话
- `GET /api/sessions/{id}` - 获取会话
- `GET /api/sessions/{id}/state` - 获取状态
- `GET /api/sessions/{id}/history` - 获取历史

### 知识 API

- `POST /api/knowledge` - 添加知识
- `GET /api/knowledge` - 列出知识
- `GET /api/knowledge/search` - 搜索知识
- `GET /api/knowledge/{id}` - 获取知识
- `PUT /api/knowledge/{id}` - 更新知识
- `DELETE /api/knowledge/{id}` - 删除知识

### 模式 API

- `POST /api/patterns` - 添加模式
- `GET /api/patterns` - 列出模式
- `GET /api/patterns/{id}` - 获取模式
- `PUT /api/patterns/{id}` - 更新模式
- `DELETE /api/patterns/{id}` - 删除模式
- `POST /api/patterns/match` - 匹配模式

### 反馈 API

- `POST /api/feedback` - 处理反馈
- `GET /api/feedback/{task_id}` - 获取反馈

### 监控 API

- `GET /api/monitor/metrics` - 获取指标
- `GET /api/monitor/health` - 健康检查

## 使用示例

### 创建任务

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "task-1",
    "type": "implement",
    "description": "实现用户认证功能",
    "constraints": [
      {
        "type": "security",
        "rule": "no-hardcoded-secrets",
        "severity": "error",
        "message": "不能使用硬编码的密钥"
      }
    ]
  }'
```

### 执行任务

```bash
curl -X POST http://localhost:8080/api/tasks/task-1/execute
```

### 搜索知识

```bash
curl "http://localhost:8080/api/knowledge/search?q=authentication"
```

### 匹配模式

```bash
curl -X POST http://localhost:8080/api/patterns/match \
  -H "Content-Type: application/json" \
  -d '{
    "id": "task-2",
    "type": "implement",
    "description": "实现用户认证功能"
  }'
```

## 学习与模式系统

### 学习器

学习器通过观察任务执行结果来学习模式：

```go
learner := learning.NewLearner(5, 0.7)

// 观察任务执行
learner.Observe(ctx, models.Observation{
    Task:    task,
    Result:  result,
    Success: true,
})

// 预测任务结果
prediction, err := learner.Predict(ctx, task)
```

### 模式匹配器

模式匹配器通过规则来匹配任务和模式：

```go
matcher := patterns.NewMatcher()

// 匹配模式
matched := matcher.Match(ctx, task, pattern)
```

## 监控与指标

### 任务指标

- 总任务数
- 成功任务数
- 失败任务数
- 平均执行时间

### 反馈指标

- 总反馈数
- 通过反馈数
- 修复反馈数

### 健康检查

```bash
curl http://localhost:8080/api/monitor/health
```

响应：

```json
{
  "status": "ok",
  "timestamp": "2026-05-02T12:00:00Z",
  "version": "1.0.0"
}
```

## 最佳实践

### 1. 约束设计

- 使用明确的约束类型和规则
- 提供清晰的错误消息
- 支持自动修复

### 2. 反馈循环

- 实现验证和修复
- 记录所有反馈
- 支持手动干预

### 3. 知识积累

- 记录成功和失败的经验
- 定期清理过期知识
- 支持知识搜索

### 4. 模式学习

- 收集足够的观察数据
- 定期更新模式
- 验证模式有效性

## 项目结构

```
harness/
├── cmd/
│   ├── harness/              # 主程序入口
│   └── harness-cli/          # CLI 工具
├── config/
│   └── config.go             # 配置管理
├── docs/
│   ├── api.md                # API 文档
│   └── architecture.md       # 架构文档
├── examples/
│   └── main.go               # 示例代码
├── internal/
│   ├── adapters/             # 环境适配器
│   │   ├── adapter.go        # 适配器接口
│   │   ├── claude_code.go    # Claude Code 适配器
│   │   ├── hermes.go         # Hermes 适配器
│   │   └── codex_cli.go      # Codex CLI 适配器
│   ├── api/                  # RESTful API
│   │   └── server.go         # API 服务器
│   ├── core/                 # 核心组件
│   │   ├── engine.go         # 核心引擎
│   │   ├── task_manager.go   # 任务管理器
│   │   ├── state_manager.go  # 状态管理器
│   │   ├── feedback_loop.go  # 反馈循环
│   │   ├── knowledge_base.go # 知识库
│   │   └── pattern_engine.go # 模式引擎
│   ├── learning/             # 学习系统
│   │   └── learner.go        # 学习器
│   ├── patterns/             # 模式系统
│   │   └── matcher.go        # 模式匹配器
│   └── storage/              # 存储层
│       ├── storage.go        # 存储接口
│       └── sqlite.go         # SQLite 实现
├── k8s/
│   └── deployment.yaml       # Kubernetes 部署
├── models/
│   └── types.go              # 数据模型
├── pkg/
│   ├── cache/                # 缓存
│   ├── errors/               # 错误处理
│   ├── logger/               # 日志系统
│   ├── middleware/            # 中间件
│   ├── pool/                 # 连接池
│   ├── utils/                # 工具函数
│   └── validator/            # 配置验证
├── scripts/
│   ├── setup.sh              # 环境设置
│   └── test.sh               # 测试脚本
├── .gitignore
├── Dockerfile
├── go.mod
├── harness.yaml              # 配置文件
├── Makefile
└── README.md
```

## 测试

### 运行测试

```bash
# 运行所有测试
make test

# 运行单元测试
./scripts/test.sh unit

# 运行 API 测试
./scripts/test.sh api

# 运行存储测试
./scripts/test.sh storage

# 运行适配器测试
./scripts/test.sh adapters

# 生成覆盖率报告
./scripts/test.sh coverage
```

### 测试覆盖率

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 性能优化

### 1. 连接池

使用连接池管理数据库连接和 HTTP 客户端连接。

### 2. 缓存

使用内存缓存存储频繁访问的数据。

### 3. 并发

使用 Goroutine 池处理并发任务。

### 4. 索引

为常用查询字段创建索引。

## 故障排除

### 1. 任务执行失败

检查：
- 约束是否满足
- 适配器是否正常
- 网络连接是否正常

### 2. 状态同步问题

检查：
- 会话是否有效
- 存储是否正常
- 并发冲突

### 3. 知识搜索不准确

检查：
- 索引是否更新
- 查询词是否正确
- 知识条目是否完整

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 许可证

MIT License

## 联系方式

- GitHub: https://github.com/harness-engineering/harness
- Email: harness@example.com
