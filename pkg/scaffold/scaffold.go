package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ProjectConfig 项目配置
type ProjectConfig struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Module      string `json:"module"` // Go module path
	Type        string `json:"type"`   // cli, api, library, microservice
	Features    []string `json:"features"`
}

// Scaffold 脚手架生成器
type Scaffold struct {
	config    ProjectConfig
	templates map[string]string
}

// NewScaffold 创建脚手架生成器
func NewScaffold(config ProjectConfig) *Scaffold {
	s := &Scaffold{
		config:    config,
		templates: make(map[string]string),
	}
	s.loadTemplates()
	return s
}

// loadTemplates 加载模板
func (s *Scaffold) loadTemplates() {
	// go.mod 模板
	s.templates["go.mod"] = `module {{.Module}}

go 1.21

require (
	github.com/gorilla/mux v1.8.1
	github.com/mattn/go-sqlite3 v1.14.22
	github.com/spf13/cobra v1.8.0
	gopkg.in/yaml.v3 v3.0.1
)
`

	// main.go 模板
	s.templates["cmd/{{.Name}}/main.go"] = `package main

import (
	"fmt"
	"os"

	"{{.Module}}/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
`

	// app.go 模板
	s.templates["internal/app/app.go"] = `package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// Run 运行应用
func Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 处理信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// 初始化应用
	app, err := NewApp()
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	// 运行应用
	return app.Run(ctx)
}
`

	// config.go 模板
	s.templates["internal/config/config.go"] = `package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 配置
type Config struct {
	Server  ServerConfig  ` + "`yaml:\"server\"`" + `
	Storage StorageConfig ` + "`yaml:\"storage\"`" + `
	Log     LogConfig     ` + "`yaml:\"log\"`" + `
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Addr    string ` + "`yaml:\"addr\"`" + `
	Timeout string ` + "`yaml:\"timeout\"`" + `
}

// StorageConfig 存储配置
type StorageConfig struct {
	Type string ` + "`yaml:\"type\"`" + `
	Path string ` + "`yaml:\"path\"`" + `
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string ` + "`yaml:\"level\"`" + `
	Format string ` + "`yaml:\"format\"`" + `
}

// Load 加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Addr:    ":8080",
			Timeout: "30s",
		},
		Storage: StorageConfig{
			Type: "sqlite",
			Path: "./data/app.db",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "text",
		},
	}
}
`

	// Makefile 模板
	s.templates["Makefile"] = `.PHONY: build run test clean

APP_NAME := {{.Name}}
BUILD_DIR := ./build

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) cmd/{{.Name}}/main.go

run:
	go run cmd/{{.Name}}/main.go

test:
	go test ./... -v

clean:
	rm -rf $(BUILD_DIR)

lint:
	golangci-lint run ./...

docker-build:
	docker build -t $(APP_NAME) .

docker-run:
	docker run -p 8080:8080 $(APP_NAME)
`

	// Dockerfile 模板
	s.templates["Dockerfile"] = `FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/{{.Name}} cmd/{{.Name}}/main.go

FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite
WORKDIR /app

COPY --from=builder /app/{{.Name}} .
COPY config.yaml .

EXPOSE 8080

CMD ["./{{.Name}}"]
`

	// docker-compose.yaml 模板
	s.templates["docker-compose.yaml"] = `version: '3.8'

services:
  app:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./config.yaml:/app/config.yaml
    environment:
      - APP_ENV=production
    restart: unless-stopped
`

	// config.yaml 模板
	s.templates["config.yaml"] = `server:
  addr: ":8080"
  timeout: "30s"

storage:
  type: "sqlite"
  path: "./data/{{.Name}}.db"

log:
  level: "info"
  format: "text"
`

	// .gitignore 模板
	s.templates[".gitignore"] = `# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
build/

# Test binary
*.test

# Output of the go coverage tool
*.out

# Dependency directories
vendor/

# IDE
.idea/
.vscode/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Data
data/
*.db
`

	// README.md 模板
	s.templates["README.md"] = `# {{.Name}}

{{.Description}}

## 快速开始

### 安装

` + "```bash" + `
go mod download
` + "```" + `

### 运行

` + "```bash" + `
make run
` + "```" + `

### 测试

` + "```bash" + `
make test
` + "```" + `

### 构建

` + "```bash" + `
make build
` + "```" + `

## 项目结构

` + "```" + `
{{.Name}}/
├── cmd/
│   └── {{.Name}}/
│       └── main.go
├── internal/
│   ├── app/
│   │   └── app.go
│   └── config/
│       └── config.go
├── config.yaml
├── Dockerfile
├── docker-compose.yaml
├── Makefile
└── README.md
` + "```" + `

## 配置

编辑 ` + "`config.yaml`" + ` 文件进行配置。

## API

### 健康检查

` + "```bash" + `
curl http://localhost:8080/health
` + "```" + `

## License

MIT
`

	// .golangci.yml 模板
	s.templates[".golangci.yml"] = `linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gocritic
    - gofmt
    - goimports
    - revive

linters-settings:
  errcheck:
    check-type-assertions: true
  gocritic:
    enabled-tags:
      - diagnostic
      - experimental
      - opinionated
      - performance
      - style

run:
  timeout: 5m
  modules-download-mode: readonly
`

	// health.go 模板
	s.templates["internal/api/health.go"] = `package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string    ` + "`json:\"status\"`" + `
	Timestamp time.Time ` + "`json:\"timestamp\"`" + `
	Version   string    ` + "`json:\"version\"`" + `
}

// HealthHandler 健康检查处理器
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
`

	// router.go 模板
	s.templates["internal/api/router.go"] = `package api

import (
	"github.com/gorilla/mux"
)

// NewRouter 创建路由器
func NewRouter() *mux.Router {
	router := mux.NewRouter()

	// 健康检查
	router.HandleFunc("/health", HealthHandler).Methods("GET")

	// API v1
	v1 := router.PathPrefix("/api/v1").Subrouter()
	_ = v1 // 添加路由

	return router
}
`

	// server.go 模板
	s.templates["internal/api/server.go"] = `package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"{{.Module}}/internal/config"
)

// Server HTTP 服务器
type Server struct {
	config *config.Config
	server *http.Server
}

// NewServer 创建服务器
func NewServer(cfg *config.Config) *Server {
	router := NewRouter()

	server := &Server{
		config: cfg,
		server: &http.Server{
			Addr:         cfg.Server.Addr,
			Handler:      router,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
	}

	return server
}

// Start 启动服务器
func (s *Server) Start() error {
	fmt.Printf("Server starting on %s\n", s.config.Server.Addr)
	return s.server.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
`

	// logger.go 模板
	s.templates["pkg/logger/logger.go"] = `package logger

import (
	"fmt"
	"os"
	"time"
)

// Level 日志级别
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

// Logger 日志器
type Logger struct {
	level  Level
	format string
}

// New 创建日志器
func New(level string, format string) *Logger {
	l := &Logger{
		format: format,
	}

	switch level {
	case "debug":
		l.level = DEBUG
	case "info":
		l.level = INFO
	case "warn":
		l.level = WARN
	case "error":
		l.level = ERROR
	default:
		l.level = INFO
	}

	return l
}

// Debug 调试日志
func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.level <= DEBUG {
		l.log("DEBUG", msg, args...)
	}
}

// Info 信息日志
func (l *Logger) Info(msg string, args ...interface{}) {
	if l.level <= INFO {
		l.log("INFO", msg, args...)
	}
}

// Warn 警告日志
func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.level <= WARN {
		l.log("WARN", msg, args...)
	}
}

// Error 错误日志
func (l *Logger) Error(msg string, args ...interface{}) {
	if l.level <= ERROR {
		l.log("ERROR", msg, args...)
	}
}

func (l *Logger) log(level string, msg string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(msg, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", timestamp, level, message)
}
`

	// errors.go 模板
	s.templates["pkg/errors/errors.go"] = `package errors

import (
	"fmt"
)

// AppError 应用错误
type AppError struct {
	Code    string ` + "`json:\"code\"`" + `
	Message string ` + "`json:\"message\"`" + `
	Err     error  ` + "`json:\"-\"`" + `
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 解包错误
func (e *AppError) Unwrap() error {
	return e.Err
}

// New 创建应用错误
func New(code string, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap 包装错误
func Wrap(err error, code string, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// 常用错误
var (
	ErrNotFound     = New("NOT_FOUND", "resource not found")
	ErrUnauthorized = New("UNAUTHORIZED", "unauthorized")
	ErrForbidden    = New("FORBIDDEN", "forbidden")
	ErrInternal     = New("INTERNAL", "internal error")
)
`
}

// Generate 生成项目
func (s *Scaffold) Generate(outputDir string) error {
	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 生成文件
	files := []string{
		"go.mod",
		"cmd/{{.Name}}/main.go",
		"internal/app/app.go",
		"internal/config/config.go",
		"internal/api/health.go",
		"internal/api/router.go",
		"internal/api/server.go",
		"pkg/logger/logger.go",
		"pkg/errors/errors.go",
		"Makefile",
		"Dockerfile",
		"docker-compose.yaml",
		"config.yaml",
		".gitignore",
		".golangci.yml",
		"README.md",
	}

	for _, fileTmpl := range files {
		// 解析文件路径
		t, err := template.New("path").Parse(fileTmpl)
		if err != nil {
			return fmt.Errorf("failed to parse path template: %w", err)
		}

		var pathBuf strings.Builder
		if err := t.Execute(&pathBuf, s.config); err != nil {
			return fmt.Errorf("failed to render path: %w", err)
		}

		path := pathBuf.String()
		fullPath := filepath.Join(outputDir, path)

		// 创建目录
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// 生成文件内容
		content, exists := s.templates[fileTmpl]
		if !exists {
			continue
		}

		t, err = template.New("content").Parse(content)
		if err != nil {
			return fmt.Errorf("failed to parse content template: %w", err)
		}

		var contentBuf strings.Builder
		if err := t.Execute(&contentBuf, s.config); err != nil {
			return fmt.Errorf("failed to render content: %w", err)
		}

		// 写入文件
		if err := os.WriteFile(fullPath, []byte(contentBuf.String()), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		fmt.Printf("Generated: %s\n", path)
	}

	return nil
}

// GenerateWithFeatures 带特性生成项目
func (s *Scaffold) GenerateWithFeatures(outputDir string) error {
	// 生成基础项目
	if err := s.Generate(outputDir); err != nil {
		return err
	}

	// 根据特性生成额外文件
	for _, feature := range s.config.Features {
		switch feature {
		case "database":
			if err := s.generateDatabase(outputDir); err != nil {
				return err
			}
		case "auth":
			if err := s.generateAuth(outputDir); err != nil {
				return err
			}
		case "websocket":
			if err := s.generateWebSocket(outputDir); err != nil {
				return err
			}
		case "grpc":
			if err := s.generateGRPC(outputDir); err != nil {
				return err
			}
		}
	}

	return nil
}

// generateDatabase 生成数据库支持
func (s *Scaffold) generateDatabase(outputDir string) error {
	// 生成数据库相关文件
	dbTemplate := `package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// DB 数据库
type DB struct {
	conn *sql.DB
}

// New 创建数据库
func New(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 测试连接
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close 关闭数据库
func (db *DB) Close() error {
	return db.conn.Close()
}

// Conn 获取连接
func (db *DB) Conn() *sql.DB {
	return db.conn
}
`

	path := filepath.Join(outputDir, "internal/database/database.go")
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(dbTemplate), 0644)
}

// generateAuth 生成认证支持
func (s *Scaffold) generateAuth(outputDir string) error {
	authTemplate := `package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT Claims
type Claims struct {
	UserID   string ` + "`json:\"user_id\"`" + `
	Username string ` + "`json:\"username\"`" + `
	jwt.RegisteredClaims
}

// JWTManager JWT 管理器
type JWTManager struct {
	secret     []byte
	expiration time.Duration
}

// NewJWTManager 创建 JWT 管理器
func NewJWTManager(secret string, expiration time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

// Generate 生成 JWT
func (m *JWTManager) Generate(userID, username string) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Verify 验证 JWT
func (m *JWTManager) Verify(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ContextKey 上下文键
type ContextKey string

const UserIDKey ContextKey = "user_id"

// GetUserID 从上下文获取用户 ID
func GetUserID(ctx context.Context) string {
	userID, _ := ctx.Value(UserIDKey).(string)
	return userID
}
`

	path := filepath.Join(outputDir, "internal/auth/auth.go")
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(authTemplate), 0644)
}

// generateWebSocket 生成 WebSocket 支持
func (s *Scaffold) generateWebSocket(outputDir string) error {
	wsTemplate := `package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub WebSocket Hub
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// Client WebSocket 客户端
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Message 消息
type Message struct {
	Type    string      ` + "`json:\"type\"`" + `
	Payload interface{} ` + "`json:\"payload\"`" + `
}

// NewHub 创建 Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run 运行 Hub
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast 广播消息
func (h *Hub) Broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}
	h.broadcast <- data
}

// HandleWebSocket 处理 WebSocket 连接
func HandleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade: %v", err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 256),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.hub.broadcast <- message
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
`

	path := filepath.Join(outputDir, "internal/websocket/websocket.go")
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(wsTemplate), 0644)
}

// generateGRPC 生成 gRPC 支持
func (s *Scaffold) generateGRPC(outputDir string) error {
	// 生成 proto 文件
	protoTemplate := `syntax = "proto3";

package {{.Name}};

option go_package = "{{.Module}}/proto";

service {{.Name}}Service {
  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);
}

message HealthCheckRequest {}

message HealthCheckResponse {
  string status = 1;
  string version = 2;
}
`

	path := filepath.Join(outputDir, "proto/service.proto")
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	t, err := template.New("proto").Parse(protoTemplate)
	if err != nil {
		return err
	}

	var buf strings.Builder
	if err := t.Execute(&buf, s.config); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(buf.String()), 0644)
}
