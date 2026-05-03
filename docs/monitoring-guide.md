# Harness Engineering 监控指南

## 概述

本指南介绍如何设置和使用 Harness Engineering 的监控系统，包括 Prometheus、Grafana 和 Alertmanager。

## 快速开始

### 1. 设置监控

```bash
# 运行监控设置脚本
./scripts/setup-monitoring.sh
```

### 2. 启动监控栈

```bash
cd monitoring
./start.sh
```

### 3. 访问监控界面

- **Harness API**: http://localhost:8080
- **Harness Metrics**: http://localhost:9090
- **Prometheus**: http://localhost:9091
- **Alertmanager**: http://localhost:9093
- **Grafana**: http://localhost:3000 (默认用户名/密码: admin/admin)

## 监控组件

### Prometheus

Prometheus 是一个开源的监控和告警工具包，用于收集和查询时间序列数据。

**配置文件**: `monitoring/prometheus/prometheus.yml`

**主要功能**:
- 从 Harness 收集指标
- 执行告警规则
- 提供 PromQL 查询语言

**常用查询**:

```promql
# 任务成功率
rate(harness_tasks_total{status="success"}[5m]) / rate(harness_tasks_total[5m])

# 任务平均延迟
rate(harness_task_duration_seconds_sum[5m]) / rate(harness_task_duration_seconds_count[5m])

# 内存使用量
process_resident_memory_bytes / 1024 / 1024

# Goroutine 数量
go_goroutines
```

### Grafana

Grafana 是一个开源的可视化和分析平台。

**配置文件**: `monitoring/grafana/`

**预配置仪表板**:
- Harness Engineering: 显示任务速率、延迟、内存使用和 Goroutine 数量

**自定义仪表板**:
1. 登录 Grafana (http://localhost:3000)
2. 点击 "+" -> "Create Dashboard"
3. 添加面板并使用 PromQL 查询

### Alertmanager

Alertmanager 处理 Prometheus 发送的告警。

**配置文件**: `monitoring/alertmanager/alertmanager.yml`

**默认告警规则**:

| 告警名称 | 条件 | 严重程度 |
|---------|------|---------|
| HighErrorRate | 任务失败率 > 10% | warning |
| HighLatency | 95% 延迟 > 10s | warning |
| HighMemoryUsage | 内存使用 > 500MB | warning |
| HighGoroutines | Goroutine 数量 > 1000 | warning |
| ServiceDown | 服务不可用 | critical |

## 指标参考

### 任务指标

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `harness_tasks_total` | counter | 任务总数（按状态分类） |
| `harness_task_duration_seconds` | histogram | 任务执行时间 |
| `harness_task_queue_size` | gauge | 任务队列大小 |

### 系统指标

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `process_resident_memory_bytes` | gauge | 内存使用量 |
| `go_goroutines` | gauge | Goroutine 数量 |
| `go_gc_duration_seconds` | summary | GC 持续时间 |

### API 指标

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `harness_http_requests_total` | counter | HTTP 请求总数 |
| `harness_http_request_duration_seconds` | histogram | HTTP 请求时间 |

## 告警配置

### 添加自定义告警

编辑 `monitoring/prometheus/alerts.yml`:

```yaml
groups:
- name: custom
  rules:
  - alert: CustomAlert
    expr: your_metric > threshold
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "Custom alert summary"
      description: "Custom alert description"
```

### 配置通知渠道

编辑 `monitoring/alertmanager/alertmanager.yml`:

```yaml
receivers:
- name: 'email'
  email_configs:
  - to: 'admin@example.com'
    from: 'alertmanager@example.com'
    smarthost: 'smtp.example.com:587'
    subject: 'Alert: {{ .GroupLabels.alertname }}'
    body: |
      {{ range .Alerts }}
      Alert: {{ .Annotations.summary }}
      Description: {{ .Annotations.description }}
      {{ end }}

- name: 'slack'
  slack_configs:
  - api_url: 'https://hooks.slack.com/services/...'
    channel: '#alerts'
    title: 'Alert: {{ .GroupLabels.alertname }}'
    text: '{{ .Annotations.summary }}'

- name: 'webhook'
  webhook_configs:
  - url: 'http://your-webhook-endpoint/alert'
    send_resolved: true
```

## 运维操作

### 查看监控状态

```bash
# 查看 Prometheus 目标状态
curl http://localhost:9091/api/v1/targets

# 查看告警规则
curl http://localhost:9091/api/v1/rules

# 查看当前告警
curl http://localhost:9093/api/v1/alerts
```

### 重启监控服务

```bash
cd monitoring

# 重启所有服务
docker-compose restart

# 重启单个服务
docker-compose restart prometheus
docker-compose restart grafana
docker-compose restart alertmanager
```

### 查看日志

```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f prometheus
docker-compose logs -f grafana
docker-compose logs -f alertmanager
```

### 备份监控数据

```bash
# 备份 Prometheus 数据
docker run --rm -v monitoring_prometheus_data:/data -v $(pwd):/backup alpine tar czf /backup/prometheus-backup.tar.gz /data

# 备份 Grafana 数据
docker run --rm -v monitoring_grafana_data:/data -v $(pwd):/backup alpine tar czf /backup/grafana-backup.tar.gz /data
```

### 恢复监控数据

```bash
# 恢复 Prometheus 数据
docker run --rm -v monitoring_prometheus_data:/data -v $(pwd):/backup alpine sh -c "cd /data && tar xzf /backup/prometheus-backup.tar.gz --strip-components=1"

# 恢复 Grafana 数据
docker run --rm -v monitoring_grafana_data:/data -v $(pwd):/backup alpine sh -c "cd /data && tar xzf /backup/grafana-backup.tar.gz --strip-components=1"
```

## 故障排查

### Prometheus 无法采集指标

1. 检查 Harness 是否正常运行:
   ```bash
   curl http://localhost:9090/metrics
   ```

2. 检查 Prometheus 目标状态:
   ```bash
   curl http://localhost:9091/api/v1/targets
   ```

3. 检查网络连接:
   ```bash
   docker exec -it monitoring_prometheus_1 wget -qO- http://harness:9090/metrics
   ```

### Grafana 无法显示数据

1. 检查数据源配置:
   - 登录 Grafana
   - 进入 Configuration -> Data Sources
   - 验证 Prometheus 数据源 URL

2. 检查查询语法:
   - 在 Grafana Explore 中测试 PromQL 查询
   - 确保查询语法正确

### Alertmanager 无法发送通知

1. 检查 Alertmanager 配置:
   ```bash
   docker exec -it monitoring_alertmanager_1 amtool config show
   ```

2. 测试通知:
   ```bash
   docker exec -it monitoring_alertmanager_1 amtool alert add alertname=test severity=warning
   ```

3. 查看通知日志:
   ```bash
   docker-compose logs alertmanager
   ```

## 最佳实践

### 1. 监控关键指标

- **任务成功率**: 监控任务成功/失败比例
- **任务延迟**: 监控任务执行时间
- **系统资源**: 监控 CPU、内存、磁盘使用
- **API 性能**: 监控请求速率和响应时间

### 2. 设置合理的告警阈值

- 根据历史数据设置阈值
- 避免过于敏感的告警
- 设置告警抑制规则

### 3. 定期审查监控配置

- 定期检查告警规则
- 更新 Grafana 仪表板
- 清理过期数据

### 4. 文档化监控流程

- 记录告警处理流程
- 维护监控配置文档
- 培训运维人员

## 扩展阅读

- [Prometheus 官方文档](https://prometheus.io/docs/)
- [Grafana 官方文档](https://grafana.com/docs/)
- [Alertmanager 官方文档](https://prometheus.io/docs/alerting/latest/alertmanager/)
- [Harness Engineering 部署指南](./deployment-operations.md)
