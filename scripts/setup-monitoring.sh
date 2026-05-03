#!/bin/bash

# Harness Engineering 监控设置脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    print_info "检查依赖..."
    
    if ! command -v docker &> /dev/null; then
        print_error "docker 未安装"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        print_error "docker-compose 未安装"
        exit 1
    fi
    
    print_info "依赖检查通过"
}

# 创建监控目录
create_monitoring_dirs() {
    print_info "创建监控目录..."
    
    mkdir -p monitoring/prometheus
    mkdir -p monitoring/grafana/provisioning/datasources
    mkdir -p monitoring/grafana/provisioning/dashboards
    mkdir -p monitoring/grafana/dashboards
    mkdir -p monitoring/alertmanager
    
    print_info "监控目录创建完成"
}

# 生成 Prometheus 配置
generate_prometheus_config() {
    print_info "生成 Prometheus 配置..."
    
    cat > monitoring/prometheus/prometheus.yml << 'EOF'
global:
  scrape_interval: 15s
  evaluation_interval: 15s

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093

rule_files:
  - "alerts.yml"

scrape_configs:
  - job_name: 'harness'
    static_configs:
      - targets: ['harness:9090']
    metrics_path: '/metrics'
    
  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
EOF
    
    print_info "Prometheus 配置生成完成"
}

# 生成告警规则
generate_alert_rules() {
    print_info "生成告警规则..."
    
    cat > monitoring/prometheus/alerts.yml << 'EOF'
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
      description: "Task error rate is {{ $value }}%"
      
  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(harness_task_duration_seconds_bucket[5m])) > 10
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High task latency"
      description: "95th percentile latency is {{ $value }}s"
      
  - alert: HighMemoryUsage
    expr: process_resident_memory_bytes / 1024 / 1024 > 500
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High memory usage"
      description: "Memory usage is {{ $value }}MB"
      
  - alert: HighGoroutines
    expr: go_goroutines > 1000
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High goroutine count"
      description: "Goroutine count is {{ $value }}"
      
  - alert: ServiceDown
    expr: up{job="harness"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Harness service is down"
      description: "Harness service has been down for more than 1 minute"
EOF
    
    print_info "告警规则生成完成"
}

# 生成 Grafana 数据源配置
generate_grafana_datasource() {
    print_info "生成 Grafana 数据源配置..."
    
    cat > monitoring/grafana/provisioning/datasources/prometheus.yml << 'EOF'
apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
    editable: false
EOF
    
    print_info "Grafana 数据源配置生成完成"
}

# 生成 Grafana 仪表板配置
generate_grafana_dashboard_config() {
    print_info "生成 Grafana 仪表板配置..."
    
    cat > monitoring/grafana/provisioning/dashboards/dashboards.yml << 'EOF'
apiVersion: 1

providers:
  - name: 'default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    options:
      path: /var/lib/grafana/dashboards
      foldersFromFilesStructure: true
EOF
    
    print_info "Grafana 仪表板配置生成完成"
}

# 生成 Grafana 仪表板
generate_grafana_dashboard() {
    print_info "生成 Grafana 仪表板..."
    
    cat > monitoring/grafana/dashboards/harness.json << 'EOF'
{
  "annotations": {
    "list": []
  },
  "editable": true,
  "gnetId": null,
  "graphTooltip": 0,
  "id": null,
  "links": [],
  "panels": [
    {
      "aliasColors": {},
      "bars": false,
      "dashLength": 10,
      "dashes": false,
      "datasource": "Prometheus",
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "palette-classic"
          },
          "custom": {
            "axisLabel": "",
            "axisPlacement": "auto",
            "barAlignment": 0,
            "drawStyle": "line",
            "fillOpacity": 10,
            "gradientMode": "none",
            "hideFrom": {
              "legend": false,
              "tooltip": false,
              "viz": false
            },
            "lineInterpolation": "linear",
            "lineWidth": 1,
            "pointSize": 5,
            "scaleDistribution": {
              "type": "linear"
            },
            "showPoints": "auto",
            "spanNulls": false,
            "stacking": {
              "group": "A",
              "mode": "none"
            },
            "thresholdsStyle": {
              "mode": "off"
            }
          },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "green",
                "value": null
              }
            ]
          }
        },
        "overrides": []
      },
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 0
      },
      "id": 1,
      "options": {
        "legend": {
          "calcs": [],
          "displayMode": "list",
          "placement": "bottom"
        },
        "tooltip": {
          "mode": "single"
        }
      },
      "title": "Task Rate",
      "type": "timeseries",
      "targets": [
        {
          "datasource": "Prometheus",
          "expr": "rate(harness_tasks_total[5m])",
          "legendFormat": "{{status}}",
          "refId": "A"
        }
      ]
    },
    {
      "aliasColors": {},
      "bars": false,
      "dashLength": 10,
      "dashes": false,
      "datasource": "Prometheus",
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "palette-classic"
          },
          "custom": {
            "axisLabel": "",
            "axisPlacement": "auto",
            "barAlignment": 0,
            "drawStyle": "line",
            "fillOpacity": 10,
            "gradientMode": "none",
            "hideFrom": {
              "legend": false,
              "tooltip": false,
              "viz": false
            },
            "lineInterpolation": "linear",
            "lineWidth": 1,
            "pointSize": 5,
            "scaleDistribution": {
              "type": "linear"
            },
            "showPoints": "auto",
            "spanNulls": false,
            "stacking": {
              "group": "A",
              "mode": "none"
            },
            "thresholdsStyle": {
              "mode": "off"
            }
          },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "green",
                "value": null
              }
            ]
          }
        },
        "overrides": []
      },
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 0
      },
      "id": 2,
      "options": {
        "legend": {
          "calcs": [],
          "displayMode": "list",
          "placement": "bottom"
        },
        "tooltip": {
          "mode": "single"
        }
      },
      "title": "Task Latency",
      "type": "timeseries",
      "targets": [
        {
          "datasource": "Prometheus",
          "expr": "histogram_quantile(0.95, rate(harness_task_duration_seconds_bucket[5m]))",
          "legendFormat": "95th percentile",
          "refId": "A"
        },
        {
          "datasource": "Prometheus",
          "expr": "histogram_quantile(0.5, rate(harness_task_duration_seconds_bucket[5m]))",
          "legendFormat": "50th percentile",
          "refId": "B"
        }
      ]
    },
    {
      "aliasColors": {},
      "bars": false,
      "dashLength": 10,
      "dashes": false,
      "datasource": "Prometheus",
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "palette-classic"
          },
          "custom": {
            "axisLabel": "",
            "axisPlacement": "auto",
            "barAlignment": 0,
            "drawStyle": "line",
            "fillOpacity": 10,
            "gradientMode": "none",
            "hideFrom": {
              "legend": false,
              "tooltip": false,
              "viz": false
            },
            "lineInterpolation": "linear",
            "lineWidth": 1,
            "pointSize": 5,
            "scaleDistribution": {
              "type": "linear"
            },
            "showPoints": "auto",
            "spanNulls": false,
            "stacking": {
              "group": "A",
              "mode": "none"
            },
            "thresholdsStyle": {
              "mode": "off"
            }
          },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "green",
                "value": null
              }
            ]
          }
        },
        "overrides": []
      },
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 0,
        "y": 8
      },
      "id": 3,
      "options": {
        "legend": {
          "calcs": [],
          "displayMode": "list",
          "placement": "bottom"
        },
        "tooltip": {
          "mode": "single"
        }
      },
      "title": "Memory Usage",
      "type": "timeseries",
      "targets": [
        {
          "datasource": "Prometheus",
          "expr": "process_resident_memory_bytes / 1024 / 1024",
          "legendFormat": "Memory (MB)",
          "refId": "A"
        }
      ]
    },
    {
      "aliasColors": {},
      "bars": false,
      "dashLength": 10,
      "dashes": false,
      "datasource": "Prometheus",
      "fieldConfig": {
        "defaults": {
          "color": {
            "mode": "palette-classic"
          },
          "custom": {
            "axisLabel": "",
            "axisPlacement": "auto",
            "barAlignment": 0,
            "drawStyle": "line",
            "fillOpacity": 10,
            "gradientMode": "none",
            "hideFrom": {
              "legend": false,
              "tooltip": false,
              "viz": false
            },
            "lineInterpolation": "linear",
            "lineWidth": 1,
            "pointSize": 5,
            "scaleDistribution": {
              "type": "linear"
            },
            "showPoints": "auto",
            "spanNulls": false,
            "stacking": {
              "group": "A",
              "mode": "none"
            },
            "thresholdsStyle": {
              "mode": "off"
            }
          },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {
                "color": "green",
                "value": null
              }
            ]
          }
        },
        "overrides": []
      },
      "gridPos": {
        "h": 8,
        "w": 12,
        "x": 12,
        "y": 8
      },
      "id": 4,
      "options": {
        "legend": {
          "calcs": [],
          "displayMode": "list",
          "placement": "bottom"
        },
        "tooltip": {
          "mode": "single"
        }
      },
      "title": "Goroutines",
      "type": "timeseries",
      "targets": [
        {
          "datasource": "Prometheus",
          "expr": "go_goroutines",
          "legendFormat": "Goroutines",
          "refId": "A"
        }
      ]
    }
  ],
  "refresh": "10s",
  "schemaVersion": 30,
  "style": "dark",
  "tags": ["harness"],
  "templating": {
    "list": []
  },
  "time": {
    "from": "now-1h",
    "to": "now"
  },
  "timepicker": {},
  "timezone": "",
  "title": "Harness Engineering",
  "uid": "harness",
  "version": 1
}
EOF
    
    print_info "Grafana 仪表板生成完成"
}

# 生成 Alertmanager 配置
generate_alertmanager_config() {
    print_info "生成 Alertmanager 配置..."
    
    cat > monitoring/alertmanager/alertmanager.yml << 'EOF'
global:
  smtp_smarthost: 'localhost:587'
  smtp_from: 'alertmanager@example.com'
  smtp_auth_username: 'alertmanager@example.com'
  smtp_auth_password: 'password'

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'

receivers:
- name: 'web.hook'
  webhook_configs:
  - url: 'http://webhook:5001/'
    send_resolved: true
  
  # 邮件配置（可选）
  # email_configs:
  # - to: 'admin@example.com'
  #   from: 'alertmanager@example.com'
  #   smarthost: 'localhost:587'
  #   subject: 'Alert: {{ .GroupLabels.alertname }}'
  #   body: |
  #     {{ range .Alerts }}
  #     Alert: {{ .Annotations.summary }}
  #     Description: {{ .Annotations.description }}
  #     {{ end }}

inhibit_rules:
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'dev', 'instance']
EOF
    
    print_info "Alertmanager 配置生成完成"
}

# 生成 Docker Compose 配置
generate_docker_compose() {
    print_info "生成 Docker Compose 配置..."
    
    cat > monitoring/docker-compose.yml << 'EOF'
version: '3.8'

services:
  harness:
    build: ..
    ports:
      - "8080:8080"
      - "9090:9090"
    volumes:
      - ../data:/app/data
      - ../config:/app/config
    environment:
      - HARNESS_ENV=production
      - HARNESS_LOG_LEVEL=info
    restart: unless-stopped
    networks:
      - monitoring

  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
      - ./prometheus/alerts.yml:/etc/prometheus/alerts.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=200h'
      - '--web.enable-lifecycle'
    restart: unless-stopped
    networks:
      - monitoring

  alertmanager:
    image: prom/alertmanager:latest
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager/alertmanager.yml:/etc/alertmanager/alertmanager.yml
      - alertmanager_data:/alertmanager
    command:
      - '--config.file=/etc/alertmanager/alertmanager.yml'
      - '--storage.path=/alertmanager'
    restart: unless-stopped
    networks:
      - monitoring

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    volumes:
      - ./grafana/provisioning:/etc/grafana/provisioning
      - ./grafana/dashboards:/var/lib/grafana/dashboards
      - grafana_data:/var/lib/grafana
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_USERS_ALLOW_SIGN_UP=false
    restart: unless-stopped
    networks:
      - monitoring

volumes:
  prometheus_data:
  alertmanager_data:
  grafana_data:

networks:
  monitoring:
    driver: bridge
EOF
    
    print_info "Docker Compose 配置生成完成"
}

# 生成启动脚本
generate_start_script() {
    print_info "生成启动脚本..."
    
    cat > monitoring/start.sh << 'EOF'
#!/bin/bash

# 启动监控栈

set -e

echo "启动监控栈..."

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo "错误: Docker 未运行"
    exit 1
fi

# 启动服务
docker-compose up -d

echo "监控栈启动完成!"
echo ""
echo "访问地址:"
echo "  - Harness API: http://localhost:8080"
echo "  - Harness Metrics: http://localhost:9090"
echo "  - Prometheus: http://localhost:9091"
echo "  - Alertmanager: http://localhost:9093"
echo "  - Grafana: http://localhost:3000 (admin/admin)"
echo ""
echo "查看日志: docker-compose logs -f"
echo "停止服务: docker-compose down"
EOF
    
    chmod +x monitoring/start.sh
    
    print_info "启动脚本生成完成"
}

# 生成停止脚本
generate_stop_script() {
    print_info "生成停止脚本..."
    
    cat > monitoring/stop.sh << 'EOF'
#!/bin/bash

# 停止监控栈

set -e

echo "停止监控栈..."

docker-compose down

echo "监控栈已停止"
EOF
    
    chmod +x monitoring/stop.sh
    
    print_info "停止脚本生成完成"
}

# 主函数
main() {
    echo "=========================================="
    echo "Harness Engineering 监控设置"
    echo "=========================================="
    echo ""
    
    check_dependencies
    create_monitoring_dirs
    generate_prometheus_config
    generate_alert_rules
    generate_grafana_datasource
    generate_grafana_dashboard_config
    generate_grafana_dashboard
    generate_alertmanager_config
    generate_docker_compose
    generate_start_script
    generate_stop_script
    
    echo ""
    echo "=========================================="
    echo "监控设置完成!"
    echo "=========================================="
    echo ""
    echo "下一步:"
    echo "1. 进入监控目录: cd monitoring"
    echo "2. 启动监控栈: ./start.sh"
    echo "3. 访问 Grafana: http://localhost:3000"
    echo ""
}

# 执行主函数
main
