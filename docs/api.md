# Harness Engineering API 文档

## 概述

Harness Engineering 提供 RESTful API 用于管理任务、会话、知识和模式。

## 基础信息

- **Base URL**: `http://localhost:8080`
- **Content-Type**: `application/json`
- **认证**: Bearer Token（可选）

## 错误处理

所有错误响应格式：

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "错误描述"
  }
}
```

### 错误代码

| 代码 | HTTP 状态码 | 描述 |
|------|-------------|------|
| `INTERNAL_ERROR` | 500 | 内部服务器错误 |
| `INVALID_INPUT` | 400 | 输入无效 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 资源冲突 |
| `UNAUTHORIZED` | 401 | 未授权 |
| `FORBIDDEN` | 403 | 禁止访问 |
| `TIMEOUT` | 408 | 请求超时 |

## API 端点

### 任务管理

#### 创建任务

```http
POST /api/tasks
```

**请求体**:

```json
{
  "id": "task-1",
  "type": "implement",
  "description": "实现用户认证功能",
  "context": {
    "environment": "production",
    "language": "go"
  },
  "constraints": [
    {
      "type": "security",
      "rule": "no-hardcoded-secrets",
      "severity": "error",
      "message": "不能使用硬编码的密钥"
    }
  ],
  "priority": 1
}
```

**响应**:

```json
{
  "task": {
    "id": "task-1",
    "type": "implement",
    "description": "实现用户认证功能"
  },
  "status": "pending",
  "created_at": "2026-05-02T12:00:00Z",
  "updated_at": "2026-05-02T12:00:00Z"
}
```

#### 列出任务

```http
GET /api/tasks
```

**查询参数**:

- `status`: 任务状态（pending, in_progress, completed, failed, cancelled）
- `type`: 任务类型

**响应**:

```json
[
  {
    "task": {
      "id": "task-1",
      "type": "implement",
      "description": "实现用户认证功能"
    },
    "status": "completed",
    "created_at": "2026-05-02T12:00:00Z",
    "updated_at": "2026-05-02T12:05:00Z"
  }
]
```

#### 获取任务

```http
GET /api/tasks/{id}
```

**响应**:

```json
{
  "task": {
    "id": "task-1",
    "type": "implement",
    "description": "实现用户认证功能"
  },
  "status": "completed",
  "result": {
    "task_id": "task-1",
    "status": "completed",
    "output": "任务完成",
    "metrics": {
      "duration": 5000000000,
      "token_count": 1000,
      "tool_uses": 10
    }
  },
  "created_at": "2026-05-02T12:00:00Z",
  "updated_at": "2026-05-02T12:05:00Z"
}
```

#### 执行任务

```http
POST /api/tasks/{id}/execute
```

**响应**:

```json
{
  "task_id": "task-1",
  "status": "completed",
  "output": "任务完成",
  "metrics": {
    "duration": 5000000000,
    "token_count": 1000,
    "tool_uses": 10
  }
}
```

#### 取消任务

```http
POST /api/tasks/{id}/cancel
```

**响应**: 204 No Content

### 会话管理

#### 创建会话

```http
POST /api/sessions
```

**请求体**:

```json
{
  "environment": "production"
}
```

**响应**:

```json
{
  "id": "session-1",
  "environment": "production",
  "state": {
    "session_id": "session-1",
    "environment": "production",
    "tasks": [],
    "context": {},
    "timestamp": "2026-05-02T12:00:00Z"
  },
  "created_at": "2026-05-02T12:00:00Z",
  "updated_at": "2026-05-02T12:00:00Z"
}
```

#### 列出会话

```http
GET /api/sessions
```

**响应**:

```json
[
  {
    "id": "session-1",
    "environment": "production",
    "created_at": "2026-05-02T12:00:00Z",
    "updated_at": "2026-05-02T12:00:00Z"
  }
]
```

#### 获取会话

```http
GET /api/sessions/{id}
```

**响应**:

```json
{
  "id": "session-1",
  "environment": "production",
  "state": {
    "session_id": "session-1",
    "environment": "production",
    "tasks": [],
    "context": {},
    "timestamp": "2026-05-02T12:00:00Z"
  },
  "history": [],
  "created_at": "2026-05-02T12:00:00Z",
  "updated_at": "2026-05-02T12:00:00Z"
}
```

#### 获取状态

```http
GET /api/sessions/{id}/state
```

**响应**:

```json
{
  "session_id": "session-1",
  "environment": "production",
  "tasks": [],
  "context": {},
  "timestamp": "2026-05-02T12:00:00Z"
}
```

#### 获取历史

```http
GET /api/sessions/{id}/history
```

**响应**:

```json
[
  {
    "state": {
      "session_id": "session-1",
      "environment": "production",
      "tasks": [],
      "context": {},
      "timestamp": "2026-05-02T12:00:00Z"
    },
    "timestamp": "2026-05-02T12:00:00Z",
    "reason": "initial state"
  }
]
```

### 知识管理

#### 添加知识

```http
POST /api/knowledge
```

**请求体**:

```json
{
  "id": "knowledge-1",
  "type": "pattern",
  "title": "Go 错误处理最佳实践",
  "content": "在 Go 中，应该使用 errors.Wrap 来包装错误...",
  "tags": ["go", "error-handling", "best-practices"],
  "metadata": {
    "language": "go",
    "category": "error-handling"
  }
}
```

**响应**: 201 Created

#### 列出知识

```http
GET /api/knowledge
```

**响应**:

```json
[
  {
    "id": "knowledge-1",
    "type": "pattern",
    "title": "Go 错误处理最佳实践",
    "content": "在 Go 中，应该使用 errors.Wrap 来包装错误...",
    "tags": ["go", "error-handling", "best-practices"],
    "metadata": {
      "language": "go",
      "category": "error-handling"
    },
    "created_at": "2026-05-02T12:00:00Z",
    "updated_at": "2026-05-02T12:00:00Z",
    "access_count": 0
  }
]
```

#### 搜索知识

```http
GET /api/knowledge/search?q={query}&limit={limit}
```

**查询参数**:

- `q`: 搜索查询
- `limit`: 结果数量限制（默认 10）

**响应**:

```json
[
  {
    "id": "knowledge-1",
    "type": "pattern",
    "title": "Go 错误处理最佳实践",
    "content": "在 Go 中，应该使用 errors.Wrap 来包装错误...",
    "tags": ["go", "error-handling", "best-practices"],
    "metadata": {
      "language": "go",
      "category": "error-handling"
    },
    "created_at": "2026-05-02T12:00:00Z",
    "updated_at": "2026-05-02T12:00:00Z",
    "access_count": 1
  }
]
```

#### 获取知识

```http
GET /api/knowledge/{id}
```

**响应**:

```json
{
  "id": "knowledge-1",
  "type": "pattern",
  "title": "Go 错误处理最佳实践",
  "content": "在 Go 中，应该使用 errors.Wrap 来包装错误...",
  "tags": ["go", "error-handling", "best-practices"],
  "metadata": {
    "language": "go",
    "category": "error-handling"
  },
  "created_at": "2026-05-02T12:00:00Z",
  "updated_at": "2026-05-02T12:00:00Z",
  "access_count": 1
}
```

#### 更新知识

```http
PUT /api/knowledge/{id}
```

**请求体**:

```json
{
  "title": "更新后的标题",
  "content": "更新后的内容",
  "tags": ["go", "error-handling"]
}
```

**响应**: 204 No Content

#### 删除知识

```http
DELETE /api/knowledge/{id}
```

**响应**: 204 No Content

### 模式管理

#### 添加模式

```http
POST /api/patterns
```

**请求体**:

```json
{
  "id": "pattern-1",
  "name": "用户认证模式",
  "description": "实现用户认证功能的通用模式",
  "trigger": "认证|authentication|auth",
  "actions": [
    {
      "type": "implement",
      "parameters": {
        "framework": "jwt",
        "storage": "database"
      },
      "timeout": 300000000000,
      "retryable": true
    }
  ],
  "metadata": {
    "task_type": "implement",
    "context": {
      "language": "go"
    }
  }
}
```

**响应**: 201 Created

#### 列出模式

```http
GET /api/patterns
```

**响应**:

```json
[
  {
    "id": "pattern-1",
    "name": "用户认证模式",
    "description": "实现用户认证功能的通用模式",
    "trigger": "认证|authentication|auth",
    "actions": [
      {
        "type": "implement",
        "parameters": {
          "framework": "jwt",
          "storage": "database"
        },
        "timeout": 300000000000,
        "retryable": true
      }
    ],
    "metadata": {
      "task_type": "implement",
      "context": {
        "language": "go"
      }
    },
    "success_rate": 0,
    "usage_count": 0,
    "last_used": "2026-05-02T12:00:00Z"
  }
]
```

#### 获取模式

```http
GET /api/patterns/{id}
```

**响应**:

```json
{
  "id": "pattern-1",
  "name": "用户认证模式",
  "description": "实现用户认证功能的通用模式",
  "trigger": "认证|authentication|auth",
  "actions": [
    {
      "type": "implement",
      "parameters": {
        "framework": "jwt",
        "storage": "database"
      },
      "timeout": 300000000000,
      "retryable": true
    }
  ],
  "metadata": {
    "task_type": "implement",
    "context": {
      "language": "go"
    }
  },
  "success_rate": 0,
  "usage_count": 0,
  "last_used": "2026-05-02T12:00:00Z"
}
```

#### 更新模式

```http
PUT /api/patterns/{id}
```

**请求体**:

```json
{
  "name": "更新后的模式名称",
  "description": "更新后的描述",
  "trigger": "更新后的触发器"
}
```

**响应**: 204 No Content

#### 删除模式

```http
DELETE /api/patterns/{id}
```

**响应**: 204 No Content

#### 匹配模式

```http
POST /api/patterns/match
```

**请求体**:

```json
{
  "type": "implement",
  "description": "实现用户认证功能",
  "context": {
    "language": "go"
  }
}
```

**响应**:

```json
[
  {
    "id": "pattern-1",
    "name": "用户认证模式",
    "description": "实现用户认证功能的通用模式",
    "trigger": "认证|authentication|auth",
    "success_rate": 0.85,
    "usage_count": 10
  }
]
```

### 反馈管理

#### 处理反馈

```http
POST /api/feedback
```

**请求体**:

```json
{
  "task_id": "task-1",
  "status": "completed",
  "output": "任务完成",
  "evidence": [
    {
      "type": "test",
      "content": "所有测试通过",
      "source": "test-runner",
      "timestamp": "2026-05-02T12:00:00Z",
      "verified": true
    }
  ],
  "metrics": {
    "duration": 5000000000,
    "token_count": 1000,
    "tool_uses": 10
  }
}
```

**响应**:

```json
{
  "task_id": "task-1",
  "status": "passed",
  "violations": [],
  "fixes": [],
  "timestamp": "2026-05-02T12:00:00Z"
}
```

#### 获取反馈

```http
GET /api/feedback/{task_id}
```

**响应**:

```json
[
  {
    "task_id": "task-1",
    "status": "passed",
    "violations": [],
    "fixes": [],
    "timestamp": "2026-05-02T12:00:00Z"
  }
]
```

### 监控

#### 获取指标

```http
GET /api/monitor/metrics
```

**响应**:

```json
{
  "total_tasks": 10,
  "success_tasks": 8,
  "failed_tasks": 2,
  "total_feedback": 10,
  "passed_feedback": 9,
  "fixed_feedback": 1,
  "average_duration": 5000000000
}
```

#### 健康检查

```http
GET /api/monitor/health
```

**响应**:

```json
{
  "status": "ok",
  "timestamp": "2026-05-02T12:00:00Z",
  "version": "1.0.0"
}
```

## 使用示例

### 使用 curl

#### 创建任务

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "id": "task-1",
    "type": "implement",
    "description": "实现用户认证功能"
  }'
```

#### 执行任务

```bash
curl -X POST http://localhost:8080/api/tasks/task-1/execute
```

#### 搜索知识

```bash
curl "http://localhost:8080/api/knowledge/search?q=认证&limit=10"
```

#### 匹配模式

```bash
curl -X POST http://localhost:8080/api/patterns/match \
  -H "Content-Type: application/json" \
  -d '{
    "type": "implement",
    "description": "实现用户认证功能"
  }'
```

### 使用 Go 客户端

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

func main() {
    // 创建任务
    task := map[string]interface{}{
        "id":          "task-1",
        "type":        "implement",
        "description": "实现用户认证功能",
    }
    
    body, _ := json.Marshal(task)
    resp, err := http.Post(
        "http://localhost:8080/api/tasks",
        "application/json",
        bytes.NewBuffer(body),
    )
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    
    fmt.Println("Status:", resp.Status)
}
```

## 限制

- 最大请求体大小: 10MB
- 最大并发连接数: 100
- 请求超时: 30 秒
- 速率限制: 100 请求/分钟（可配置）
