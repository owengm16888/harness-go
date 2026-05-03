# Question 4: HTTP 优雅关停 (Graceful Shutdown)

## 题目

如何在 Go 中实现 HTTP 服务的优雅关停？请详细说明各个组件的作用。

## 核心要点

### 什么是优雅关停？

优雅关停（Graceful Shutdown）是指服务在接收到关停信号后：
1. **立即停止接受新请求**
2. **等待正在处理的请求完成**
3. **释放所有资源**（数据库连接、文件句柄等）
4. **超时强制退出**，防止无限等待

### `srv.Shutdown()` 的行为

```
Shutdown 不会中断正在处理的连接（active connections）。
它会：
1. 关闭所有监听器（Listener），停止 Accept 新连接
2. 关闭所有空闲连接（idle connections）
3. 等待所有活跃连接（active connections）处理完毕后关闭
4. 如果传入的 ctx 超时，返回 context.DeadlineExceeded
```

**关键点：Shutdown 内部已经处理了 Listener 的关闭，不需要手动关闭。**

## 完整实现

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	// ============================================
	// 1. 创建可取消的 context，用于传播取消信号
	// ============================================
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ============================================
	// 2. 创建 WaitGroup，用于等待后台 goroutine 退出
	// ============================================
	var wg sync.WaitGroup

	// ============================================
	// 3. 初始化依赖（如数据库连接）
	// ============================================
	// store := NewStore(ctx)
	// defer store.Close()  // 确保资源被释放

	// ============================================
	// 4. 创建 HTTP Server
	// ============================================
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 将 request context 与 server context 结合
		// 这样当 server 关停时，handler 可以感知到
		ctx := r.Context()
		select {
		case <-ctx.Done():
			// 连接被取消（Shutdown 会关闭 idle connections）
			return
		default:
		}
		w.Write([]byte("Hello, World!"))
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// ============================================
	// 5. 监听系统信号 (SIGINT, SIGTERM)
	// ============================================

	// 【关键】buffer size 为 1，防止信号丢失
	// 如果 channel 没有 buffer，当信号发送时如果没有接收者，
	// 信号会被丢弃（runtime 不会阻塞等待信号被接收）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// ============================================
	// 6. 启动 HTTP Server（在独立 goroutine 中）
	// ============================================
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Println("HTTP server starting on", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// ============================================
	// 7. 阻塞等待关停信号
	// ============================================
	sig := <-quit
	log.Printf("Received signal: %v, shutting down...", sig)

	// ============================================
	// 8. 取消 context，通知所有使用该 context 的 goroutine
	// ============================================
	cancel()

	// ============================================
	// 9. 设置关停超时（例如 30 秒）
	// ============================================
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer shutdownCancel()

	// ============================================
	// 10. 执行优雅关停
	// ============================================
	// Shutdown 会：
	//   - 关闭 Listener（停止 Accept 新连接）
	//   - 关闭空闲连接
	//   - 等待活跃连接处理完成
	//   - 超时返回 context.DeadlineExceeded
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// ============================================
	// 11. 等待所有后台 goroutine 退出
	// ============================================
	wg.Wait()

	// ============================================
	// 12. 释放资源
	// ============================================
	// store.Close()

	log.Println("Server exited gracefully")
}
```

## 关键点详解

### 1. `context.WithCancel` 传播取消信号

```go
ctx, cancel := context.WithCancel(context.Background())
```

- 创建一个可取消的 context，作为整个应用的根 context
- 当调用 `cancel()` 时，所有从该 ctx 派生的 context 都会被取消
- 传递给数据库连接等依赖，使其感知到关停信号
- **区别于 `shutdownCtx`**：`shutdownCtx` 是给 `srv.Shutdown` 用的超时 context

### 2. `signal.Notify` 信号监听

```go
quit := make(chan os.Signal, 1)  // buffer size 为 1！
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
```

**为什么 buffer size 必须为 1？**

```
signal.Notify 会将信号发送到 channel。
Go 的 signal delivery 机制：

  信号到达 -> runtime.signal_recv() -> 尝试发送到 channel

如果 channel 没有 buffer：
  - 如果此时没有 goroutine 在接收，信号会被"丢弃"
  - runtime 不会阻塞等待 channel 被消费

如果 buffer size >= 1：
  - 信号可以暂存在 channel 中
  - 即使程序还没有读取，信号也不会丢失

通常 buffer size 为 1 就够了（一次只处理一个关停信号）。
```

### 3. `srv.Shutdown` 行为详解

```go
if err := srv.Shutdown(shutdownCtx); err != nil {
    // 如果超时，err == context.DeadlineExceeded
    log.Printf("shutdown error: %v", err)
}
```

**Shutdown 的内部流程：**

```
1. 设置 Server.inShutdown 为 1（原子操作）
2. 关闭所有 Listener
   - 调用 ln.Close()
   - ListenAndServe() 会返回 http.ErrServerClosed
3. 关闭所有空闲连接
   - 从 idle connections 链表中取出并关闭
4. 等待活跃连接完成
   - 对于 HTTP/1.x: 等待当前请求处理完毕
   - 对于 HTTP/2: 发送 GOAWAY 帧
5. 如果 ctx 超时，返回 context.DeadlineExceeded
```

### 4. WaitGroup 等待 goroutine 清理

```go
var wg sync.WaitGroup

// 启动 goroutine 前 Add
wg.Add(1)
go func() {
    defer wg.Done()
    // ...
}()

// Shutdown 完成后 Wait
wg.Wait()  // 确保所有 goroutine 退出
```

**使用场景：**
- 后台任务 goroutine（如消息消费者、定时任务）
- 每个需要优雅退出的 goroutine 都应该加入 WaitGroup
- goroutine 内部应监听 `ctx.Done()` 来感知取消信号

### 5. 资源清理顺序

```go
// 正确的清理顺序：
// 1. 停止接受新请求      <- srv.Shutdown
// 2. 取消 context         <- cancel()
// 3. 等待 goroutine 退出   <- wg.Wait()
// 4. 关闭资源             <- store.Close()
// 5. defer 按 LIFO 顺序执行

defer cancel()           // 最后执行（LIFO）
defer shutdownCancel()
defer store.Close()      // 倒数第二
```

### 6. `http.ErrServerClosed` 的处理

```go
if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    log.Fatalf("HTTP server error: %v", err)
}
```

- `ListenAndServe` 在 `Shutdown` 被调用后会返回 `http.ErrServerClosed`
- 这不是真正的错误，不应该 fatal
- 用 `errors.Is(err, http.ErrServerClosed)` 过滤

## 常见面试追问

### Q: 为什么不用 `srv.Close()` 而用 `srv.Shutdown()`？

```
srv.Close()   -> 立即关闭所有连接，包括正在处理的请求（粗暴）
srv.Shutdown() -> 优雅关停，等待活跃请求处理完毕（推荐）
```

### Q: 如果有长时间运行的 WebSocket 连接怎么办？

```go
// WebSocket 连接不会自动被 Shutdown 中断
// 需要手动监听 context 并关闭连接
func handleWebSocket(ctx context.Context, w http.ResponseWriter, r *http.Request) {
    conn, _ := upgrader.Upgrade(w, r, nil)
    defer conn.Close()

    // 创建可取消的 context
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // 当 ctx 被取消时，关闭 WebSocket 连接
    go func() {
        <-ctx.Done()
        conn.Close()
    }()

    for {
        _, msg, err := conn.ReadMessage()
        if err != nil {
            return
        }
        // 处理消息...
    }
}
```

### Q: 如何在 Kubernetes 中实现零停机部署？

```
1. K8s 发送 SIGTERM 给 Pod
2. 同时从 Service 的 Endpoints 中移除该 Pod
3. 但已有连接可能还会到达（短暂窗口期）
4. 需要设置 terminationGracePeriodSeconds > shutdown timeout
5. 在 preStop hook 中添加 sleep，让 K8s 有时间更新路由
```

```yaml
# K8s 配置示例
spec:
  terminationGracePeriodSeconds: 45  # 比代码中的 30s 超时更长
  containers:
  - name: app
    lifecycle:
      preStop:
        exec:
          command: ["sleep", "5"]  # 等待 K8s 更新路由
```

### Q: `signal.Notify` 的 channel 会不会满？

```
不会（buffer size 为 1 的情况下）：
- SIGINT 和 SIGTERM 都会导致程序关停
- 收到第一个信号后就开始处理，channel 被消费
- 即使第二个信号到达，buffer 也有空间

但如果程序在收到信号后处理太慢（还没来得及读 channel），
而用户连续发了 3 个信号，第 3 个会丢失。
这在实际场景中不是问题，因为我们只需要第一个信号。
```

### Q: 与 context 的关系？

```
ctx (root)
├── -> 传递给 store/DB，让依赖感知关停
├── -> 传递给后台 goroutine，配合 WaitGroup 退出
│
shutdownCtx (独立)
├── -> 只给 srv.Shutdown() 使用
├── -> 30s 超时后强制关闭
└── -> 与 root ctx 无关，即使 root ctx 已取消
```

## 项目参考

完整实现参见: `cmd/harness/main.go`

该文件展示了生产级别的优雅关停实现，包括：
- 配置加载
- 数据库连接初始化与清理
- HTTP Server 启动与关停
- 后台 goroutine 管理
- 完整的错误处理

## 流程图

```
main() 启动
    │
    ├── ctx, cancel = context.WithCancel(...)
    ├── wg sync.WaitGroup
    ├── store = NewStore(ctx)
    ├── srv = &http.Server{...}
    ├── quit = make(chan os.Signal, 1)
    ├── signal.Notify(quit, SIGINT, SIGTERM)
    │
    ├── go srv.ListenAndServe()     ──→ [运行中]
    │
    ├── <-quit                       ──→ 收到 SIGTERM
    │
    ├── cancel()                     ──→ ctx.Done() 被触发
    │                                   ├── store 内部感知
    │                                   └── 后台 goroutine 感知
    │
    ├── srv.Shutdown(30s ctx)        ──→ 停止 Accept
    │                                   ├── 关闭 idle connections
    │                                   └── 等待 active connections
    │
    ├── wg.Wait()                    ──→ 等待所有 goroutine 退出
    │
    ├── store.Close()                ──→ 关闭 DB 连接
    │
    └── log("exited gracefully")     ──→ 进程退出
```
