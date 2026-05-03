# 部署和运维指南

## 目录

1. [部署选项](#部署选项)
2. [Docker 部署](#docker-部署)
3. [Kubernetes 部署](#kubernetes-部署)
4. [配置管理](#配置管理)
5. [监控和告警](#监控和告警)
6. [日志管理](#日志管理)
7. [备份和恢复](#备份和恢复)
8. [性能调优](#性能调优)
9. [故障排查](#故障排查)

## 部署选项

### 1. 单机部署
适合开发和测试环境。

```bash
# 直接运行
go run cmd/harness/main.go

# 或编译后运行
go build -o harness cmd/harness/main.go
./harness
```

### 2. Docker 部署
适合生产环境，提供隔离和一致性。

```bash
# 构建镜像
docker build -t harness-engineering .

# 运行容器
docker run -d \
  --name harness \
  -p 8080:8080 \
  -p 9090:9090 \
  -v ./data:/app/data \
  -v ./config:/app/config \
  harness-engineering
```

### 3. Kubernetes 部署
适合大规模生产环境，提供高可用和自动扩缩容。

```bash
# 部署
kubectl apply -f k8s/

# 查看状态
kubectl get pods -l app=harness
```

## Docker 部署

### Dockerfile 说明

```dockerfile
# 多阶段构建
FROM golang:1.21-alpine AS builder
# ... 构建阶段

FROM alpine:latest
# ... 运行阶段
```

### Docker Compose

```yaml
version: '3.8'
services:
  harness:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
      - ./data:/app/data
      - ./config:/app/config
    environment:
      - HARNESS_ENV=production
    restart: unless-stopped
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `HARNESS_ENV` | 运行环境 | `development` |
| `HARNESS_LOG_LEVEL` | 日志级别 | `info` |
| `HARNESS_STORAGE_PATH` | 存储路径 | `./data` |
| `HARNESS_SERVER_ADDR` | 服务地址 | `:8080` |
| `HARNESS_METRICS_ADDR` | 指标地址 | `:9090` |

## Kubernetes 部署

### 部署清单

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: harness
spec:
  replicas: 3
  selector:
    matchLabels:
      app: harness
  template:
    metadata:
      labels:
        app: harness
    spec:
      containers:
      - name: harness
        image: harness-engineering:latest
        ports:
        - containerPort: 8080
        - containerPort: 9090
        env:
        - name: HARNESS_ENV
          value: "production"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### 服务暴露

```yaml
apiVersion: v1
kind: Service
metadata:
  name: harness-service
spec:
  selector:
    app: harness
  ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
  type: LoadBalancer
```

### 水平扩缩容

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: harness-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: harness
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## 配置管理

### 配置文件

```yaml
# harness.yaml
server:
  addr: ":8080"
  read_timeout: 30s
  write_timeout: 30s

engine:
  max_concurrent_tasks: 100
  retry_count: 3
  retry_delay: 1s
  task_timeout: 5m

storage:
  type: "sqlite"
  path: "./data/harness.db"

monitor:
  metrics_port: 9090
  log_level: "info"
```

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: harness-config
data:
  harness.yaml: |
    server:
      addr: ":8080"
    engine:
      max_concurrent_tasks: 100
```

### Secrets

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: harness-secrets
type: Opaque
stringData:
  api-key: "your-api-key-here"
```

## 监控和告警

### Prometheus 指标

Harness 暴露以下 Prometheus 指标：

```yaml
# 任务指标
harness_tasks_total{status="success|failed|pending"} counter
harness_task_duration_seconds histogram

# 系统指标
harness_goroutines gauge
harness_memory_bytes gauge

# API 指标
harness_http_requests_total{method, path, status} counter
harness_http_request_duration_seconds histogram
```

### Grafana 仪表板

导入 `monitoring/grafana-dashboard.json` 获取预配置的仪表板。

### 告警规则

```yaml
# prometheus-alerts.yml
groups:
- name: harness
  rules:
  - alert: HighErrorRate
    expr: rate(harness_tasks_total{status="failed"}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High task error rate"
      
  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(harness_task_duration_seconds_bucket[5m])) > 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High task latency"
```

## 日志管理

### 日志格式

```json
{
  "time": "2024-01-01T00:00:00Z",
  "level": "info",
  "msg": "Task completed",
  "task_id": "task-123",
  "duration": "1.5s"
}
```

### 日志级别

- `debug`: 详细的调试信息
- `info`: 一般信息
- `warn`: 警告信息
- `error`: 错误信息
- `fatal`: 致命错误

### 日志收集

```yaml
# fluentd-config.yml
<source>
  @type tail
  path /var/log/harness/*.log
  tag harness
  format json
</source>

<match harness>
  @type elasticsearch
  host elasticsearch.logging.svc.cluster.local
  port 9200
  index_name harness
</match>
```

## 备份和恢复

### 数据备份

```bash
# SQLite 备份
sqlite3 data/harness.db ".backup data/backup.db"

# 自动备份脚本
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
sqlite3 data/harness.db ".backup data/backup_$DATE.db"
find data/backup_*.db -mtime +7 -delete
```

### 数据恢复

```bash
# 恢复备份
cp data/backup.db data/harness.db
```

### Kubernetes 备份

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: harness-backup
spec:
  schedule: "0 2 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: backup
            image: alpine:latest
            command:
            - /bin/sh
            - -c
            - |
              apk add sqlite
              sqlite3 /data/harness.db ".backup /backup/harness_$(date +%Y%m%d).db"
            volumeMounts:
            - name: data
              mountPath: /data
            - name: backup
              mountPath: /backup
          volumes:
          - name: data
            persistentVolumeClaim:
              claimName: harness-data
          - name: backup
            persistentVolumeClaim:
              claimName: harness-backup
```

## 性能调优

### 系统参数

```bash
# 增加文件描述符限制
ulimit -n 65536

# 调整内核参数
sysctl -w net.core.somaxconn=65535
sysctl -w net.ipv4.tcp_max_syn_backlog=65535
```

### 应用配置

```yaml
# 高性能配置
engine:
  max_concurrent_tasks: 1000
  worker_pool_size: 100
  
storage:
  type: "sqlite"
  path: "./data/harness.db"
  journal_mode: "WAL"
  synchronous: "NORMAL"
  cache_size: -64000  # 64MB
```

### 资源限制

```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "500m"
  limits:
    memory: "1Gi"
    cpu: "1000m"
```

## 故障排查

### 常见问题

#### 1. 服务无法启动

```bash
# 检查日志
docker logs harness

# 检查端口占用
lsof -i :8080

# 检查配置
cat config/harness.yaml
```

#### 2. 任务执行失败

```bash
# 检查任务状态
curl http://localhost:8080/api/tasks/{task_id}

# 查看任务日志
grep "task_id" /var/log/harness/*.log
```

#### 3. 内存使用过高

```bash
# 查看内存统计
curl http://localhost:9090/metrics | grep memory

# 强制 GC
curl -X POST http://localhost:9090/debug/pprof/gc
```

#### 4. 数据库锁定

```bash
# 检查数据库状态
sqlite3 data/harness.db "PRAGMA journal_mode;"
sqlite3 data/harness.db "PRAGMA busy_timeout;"

# 设置超时
sqlite3 data/harness.db "PRAGMA busy_timeout = 5000;"
```

### 健康检查

```bash
# 健康检查
curl http://localhost:8080/health

# 就绪检查
curl http://localhost:8080/ready

# 指标检查
curl http://localhost:9090/metrics
```

### 调试工具

```bash
# pprof 性能分析
go tool pprof http://localhost:6060/debug/pprof/profile

# 追踪分析
go tool trace trace.out
```

## 扩展阅读

- [API 文档](./api.md)
- [架构设计](./architecture.md)
- [开发指南](./development.md)
- [Go 面试知识点](./go-interview-integration.md)
