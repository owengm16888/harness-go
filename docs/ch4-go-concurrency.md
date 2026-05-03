# 第4章 Go语言并发编程

> 来源: https://chat.deepseek.com/share/xr1cmv7tzd929ms95c
> 深度补充: 结合 Harness 工程实践

## 目录

- 4.1 什么是并发问题
- 4.2 CSP并发模型
  - 4.2.1 基于管道-协程的CSP模型
  - 4.2.2 管道与select关键字
  - 4.2.3 如何实现无限缓存管道
- 4.3 基于锁的协程同步
  - 4.3.1 乐观锁
  - 4.3.2 悲观锁
- 4.4 如何并发操作map
  - 4.4.1 map的并发问题
  - 4.4.2 并发散列表sync.Map
- 4.5 并发控制sync.WaitGroup
- 4.6 并发对象池sync.Pool
- 4.7 并发限流与singleflight
- 4.8 Harness 并发实践

---

## 4.1 什么是并发问题

### 并发 vs 并行

```
并发 (Concurrency):              并行 (Parallelism):
  ┌─A─┐ ┌─B─┐ ┌─A─┐               ┌─A──────┐
  └───┘ └───┘ └───┘               ├────────┤
     时间片轮转                     ┌─B──────┐
                                   ├────────┤
                                  两个核同时执行
```

- **并发**: 结构设计。多个任务在时间上重叠推进，不一定同时运行。
- **并行**: 执行方式。多个任务在同一时刻真正同时运行。

> Go 的口号是 "Write concurrent, not parallel" —— 你写并发结构，runtime 决定是否并行。

### 并发带来的三类问题

| 问题 | 症状 | 示例 |
|------|------|------|
| **竞态条件 (Race Condition)** | 结果不确定 | 两个 goroutine 同时写 `count++` |
| **死锁 (Deadlock)** | 所有 goroutine 永久阻塞 | A 等 B，B 等 A |
| **活锁 (Livelock)** | 不断执行但无进展 | 两个人在走廊互相让路 |

### 竞态条件演示

```go
// BUG: 有竞态条件
func unsafeCounter() int {
    count := 0
    var wg sync.WaitGroup
    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            count++ // 非原子操作: 读 -> 加1 -> 写
        }()
    }
    wg.Wait()
    return count // 结果不确定，可能是 987、953 等
}
```

用 `go run -race` 可检测竞态:

```bash
go run -race main.go
# DATA RACE: Write at 0x... by goroutine 7
# Previous read at 0x... by goroutine 6
```

---

## 4.2 CSP并发模型

### 4.2.1 基于管道-协程的CSP模型

**CSP (Communicating Sequential Processes)** 是 Tony Hoare 于 1978年提出的并发模型。

核心思想:

> Don't communicate by sharing memory; share memory by communicating.
> (不要通过共享内存来通信，而要通过通信来共享内存。)

Go 的 CSP 实现由三个要素组成:

```
┌──────────┐    channel    ┌──────────┐
│ Goroutine│──────────────→│ Goroutine│
│    A     │←──────────────│    B     │
└──────────┘   (管道)      └──────────┘
       ↕                        ↕
    ┌──────────────────────────────┐
    │     Go Runtime Scheduler     │
    │   G (Goroutine)              │
    │   M (OS Thread)              │
    │   P (Processor)              │
    └──────────────────────────────┘
```

#### Channel 基本操作

```go
// 创建 channel
ch := make(chan int)        // 无缓冲（同步）
ch := make(chan int, 100)   // 有缓冲（异步，容量100）

// 发送
ch <- 42

// 接收
val := <-ch
val, ok := <-ch  // ok 为 false 表示 channel 已关闭

// 关闭
close(ch)

// 单向 channel（用于函数签名约束）
func producer(ch chan<- int) { ch <- 1 } // 只写
func consumer(ch <-chan int) { <-ch }    // 只读
```

#### 生产者-消费者模式

```go
func producer(ch chan<- int) {
    for i := 0; i < 10; i++ {
        ch <- i
    }
    close(ch)
}

func consumer(ch <-chan int, id int) {
    for val := range ch { // channel 关闭后自动退出
        fmt.Printf("consumer %d: %d\n", id, val)
    }
}

func main() {
    ch := make(chan int, 5)
    go producer(ch)

    var wg sync.WaitGroup
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            consumer(ch, id)
        }(i)
    }
    wg.Wait()
}
```

> **面试要点**: `range ch` 在 channel 关闭后自动退出循环。如果 channel 不关闭，`range` 永远阻塞。

### 4.2.2 管道与select关键字

#### select 多路复用

`select` 让一个 goroutine 同时等待多个 channel 操作:

```go
select {
case msg := <-ch1:
    fmt.Println("from ch1:", msg)
case ch2 <- 42:
    fmt.Println("sent to ch2")
case <-time.After(3 * time.Second):
    fmt.Println("timeout")
default:
    fmt.Println("no channel ready") // 非阻塞
}
```

#### select 行为规则

| 场景 | 行为 |
|------|------|
| 多个 case 就绪 | **随机选一个**执行 |
| 没有 case 就绪，有 default | 执行 default |
| 没有 case 就绪，无 default | **阻塞**直到某个就绪 |
| 所有 channel 都已关闭 | 选中但收到零值 |

#### 超时控制

```go
func fetchWithTimeout(url string, timeout time.Duration) (string, error) {
    resultCh := make(chan string, 1)
    errCh := make(chan error, 1)

    go func() {
        resp, err := http.Get(url)
        if err != nil {
            errCh <- err
            return
        }
        defer resp.Body.Close()
        body, _ := io.ReadAll(resp.Body)
        resultCh <- string(body)
    }()

    select {
    case result := <-resultCh:
        return result, nil
    case err := <-errCh:
        return "", err
    case <-time.After(timeout):
        return "", fmt.Errorf("timeout after %v", timeout)
    }
}
```

#### 优雅退出（done channel）

```go
func worker(done <-chan struct{}, jobs <-chan int, results chan<- int) {
    for {
        select {
        case <-done:
            fmt.Println("worker shutting down")
            return
        case job := <-jobs:
            results <- job * 2
        }
    }
}
```

> **面试要点**: `done` channel 通常用 `chan struct{}` 而非 `chan bool`，因为 `struct{}` 不占内存（零字节）。

### 4.2.3 如何实现无限缓存管道

Go 的 buffered channel 有固定容量。如果需要"无限缓冲"，需要自己实现。

#### 方案一: 链表 channel

```go
// UnboundedChan 无限缓冲 channel
type UnboundedChan[T any] struct {
    In     chan<- T       // 写入端
    Out    <-chan T       // 读取端
    buffer []T
}

func NewUnboundedChan[T any](initCap int) *UnboundedChan[T] {
    in := make(chan T, 1)
    out := make(chan T, 1)
    ub := &UnboundedChan[T]{
        In:     in,
        Out:    out,
        buffer: make([]T, 0, initCap),
    }
    go ub.run()
    return ub
}

func (ub *UnboundedChan[T]) run() {
    defer close(ub.Out)

    for {
        // 当 buffer 为空时，从 in 读取；否则尝试写入 out
        if len(ub.buffer) == 0 {
            val, ok := <-ub.In
            if !ok {
                return
            }
            ub.buffer = append(ub.buffer, val)
        }

        // 尝试发送 buffer[0] 到 out，同时也在接收 in
        select {
        case ub.Out <- ub.buffer[0]:
            ub.buffer = ub.buffer[1:]
            // 缩容: 当 buffer 只用了 1/4 时释放
            if cap(ub.buffer) > 256 && len(ub.buffer) < cap(ub.buffer)/4 {
                ub.buffer = append([]T(nil), ub.buffer...)
            }
        case val, ok := <-ub.In:
            if !ok {
                // in 关闭了，把剩余 buffer 发完
                for _, v := range ub.buffer {
                    ub.Out <- v
                }
                return
            }
            ub.buffer = append(ub.buffer, val)
        }
    }
}
```

#### 方案二: 分段 channel（Ring Buffer 思路）

```go
// RingChan 基于 ring buffer 的无限 channel
type RingChan[T any] struct {
    chunks [][]T
    chunkSize int
    in chan T
    out chan T
}

func NewRingChan[T any](chunkSize int) *RingChan[T] {
    rc := &RingChan[T]{
        chunkSize: chunkSize,
        in:        make(chan T, 1),
        out:       make(chan T, 1),
        chunks:    [][]T{make([]T, 0, chunkSize)},
    }
    go rc.run()
    return rc
}

func (rc *RingChan[T]) run() {
    defer close(rc.out)

    for val := range rc.in {
        // 追加到当前 chunk
        current := rc.chunks[len(rc.chunks)-1]
        if len(current) >= rc.chunkSize {
            // 当前 chunk 满了，创建新的
            newChunk := make([]T, 0, rc.chunkSize)
            rc.chunks = append(rc.chunks, newChunk)
            current = newChunk
        }
        rc.chunks[len(rc.chunks)-1] = append(current, val)

        // 尝试发送第一个元素
        select {
        case rc.out <- rc.chunks[0][0]:
            rc.chunks[0] = rc.chunks[0][1:]
            if len(rc.chunks[0]) == 0 && len(rc.chunks) > 1 {
                rc.chunks = rc.chunks[1:]
            }
        default:
            // out 已满，等待下次循环
        }
    }
}
```

> **面试要点**: 无限缓冲 channel 的核心思想是用一个中间 goroutine 做"中转站"，在 in 和 out 之间维护一个可增长的缓冲区。

---

## 4.3 基于锁的协程同步

### 4.3.1 乐观锁

乐观锁假设冲突不发生，只在提交时检查是否有其他 goroutine 修改了数据。

#### CAS (Compare-And-Swap)

```go
// atomic.CompareAndSwap* 系列函数
func casCounter() int64 {
    var count int64 = 0
    var wg sync.WaitGroup

    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for {
                old := atomic.LoadInt64(&count)
                new := old + 1
                if atomic.CompareAndSwapInt64(&count, old, new) {
                    return // 成功则退出
                }
                // 失败则重试（乐观锁的核心）
            }
        }()
    }

    wg.Wait()
    return atomic.LoadInt64(&count) // 永远是 1000
}
```

#### atomic 包常用函数

```go
// 加法
atomic.AddInt64(&count, 1)

// 读取
val := atomic.LoadInt64(&count)

// 写入
atomic.StoreInt64(&count, 42)

// 交换
old := atomic.SwapInt64(&count, 42)

// CAS
atomic.CompareAndSwapInt64(&count, old, new)
```

#### atomic.Value — 任意类型的原子操作

```go
var config atomic.Value

// 存储
config.Store(map[string]string{"env": "prod"})

// 读取
cfg := config.Load().(map[string]string)
```

> **面试要点**: `atomic` 操作比 `sync.Mutex` 轻量，适合简单的计数器、标志位等场景。但不适合复杂的临界区。

### 4.3.2 悲观锁

悲观锁假设冲突一定会发生，先加锁再操作。

#### sync.Mutex — 互斥锁

```go
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func (c *SafeCounter) Increment() {
    c.mu.Lock()
    defer c.mu.Unlock() // 必须配对，用 defer 保证
    c.count++
}

func (c *SafeCounter) Get() int {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.count
}
```

#### sync.RWMutex — 读写锁

```go
type ConfigStore struct {
    mu     sync.RWMutex
    config map[string]string
}

func (s *ConfigStore) Get(key string) string {
    s.mu.RLock()         // 读锁：多个读可并发
    defer s.mu.RUnlock()
    return s.config[key]
}

func (s *ConfigStore) Set(key, value string) {
    s.mu.Lock()          // 写锁：独占
    defer s.mu.Unlock()
    s.config[key] = value
}
```

#### Mutex vs RWMutex

| 特性 | Mutex | RWMutex |
|------|-------|---------|
| 读-读 | 互斥 | **并行** |
| 读-写 | 互斥 | 互斥 |
| 写-写 | 互斥 | 互斥 |
| 适用场景 | 写多读少 | **读多写少** |
| 性能 | 简单高效 | 读多时更优 |

#### 常见陷阱

```go
// 陷阱1: 忘记 Unlock
func bad() {
    mu.Lock()
    if err := doSomething(); err != nil {
        return // BUG: mu 没有 Unlock
    }
    mu.Unlock()
}

// 修复: 用 defer
func good() {
    mu.Lock()
    defer mu.Unlock()
    if err := doSomething(); err != nil {
        return // defer 会自动 Unlock
    }
}

// 陷阱2: 复制含锁的结构体
type Counter struct {
    mu    sync.Mutex
    count int
}

func bad(c Counter) { // 值传递，复制了锁！
    c.mu.Lock()
    c.count++
    c.mu.Unlock()
}

// 修复: 用指针
func good(c *Counter) {
    c.mu.Lock()
    c.count++
    c.mu.Unlock()
}

// 陷阱3: 在持有锁时调用外部函数（可能死锁）
func (s *Service) Process() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.otherService.DoSomething() // 危险: 可能内部也要加锁
}
```

> **面试要点**: `sync.Mutex` 和 `sync.RWMutex` 都**不可复制**。`go vet` 的 `copylocks` 检查器会检测。

---

## 4.4 如何并发操作map

### 4.4.1 map的并发问题

Go 的 `map` **不是并发安全的**:

```go
// FATAL: concurrent map writes
func unsafeMapAccess() {
    m := make(map[int]int)
    var wg sync.WaitGroup

    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func(i int) {
            defer wg.Done()
            m[i] = i * 2 // 并发写 -> panic!
        }(i)
    }

    wg.Wait()
}
```

运行时会直接 panic: `fatal error: concurrent map writes`

#### 解决方案对比

| 方案 | 读性能 | 写性能 | 内存 | 适用场景 |
|------|--------|--------|------|----------|
| `sync.Mutex` + `map` | 中 | 中 | 低 | 通用 |
| `sync.RWMutex` + `map` | **高** | 中 | 低 | 读多写少 |
| `sync.Map` | **高** | 低 | 高 | key 稳定、读远多于写 |

### 4.4.2 并发散列表 sync.Map

`sync.Map` 是 Go 1.9 引入的并发安全 map，内部使用了两个 map（read + dirty）和原子操作优化读取。

```go
var m sync.Map

// 写入
m.Store("key", "value")

// 读取
val, ok := m.Load("key")

// 读取或写入（原子操作）
val, loaded := m.LoadOrStore("key", "default")
// loaded = true 表示 key 已存在，返回旧值
// loaded = false 表示 key 不存在，存入并返回新值

// 条件写入（Go 1.20+）
actual, loaded := m.LoadAndDelete("key")

// 遍历
m.Range(func(key, value any) bool {
    fmt.Printf("%v: %v\n", key, value)
    return true // 返回 false 停止遍历
})

// 删除
m.Delete("key")
```

#### sync.Map 内部原理

```
sync.Map 内部结构:

  ┌──────────────┐     ┌──────────────┐
  │   read (只读) │     │  dirty (读写) │
  │  atomic 操作  │     │   加锁访问    │
  │  无锁读取     │     │  miss 计数    │
  └──────┬───────┘     └──────┬───────┘
         │                    │
         └────  misses > len(dirty) ────┘
                    ↓
              read = dirty
              dirty = nil
              misses = 0

读取流程:
1. 先从 read 查（无锁，原子操作）
2. miss → 从 dirty 查（加锁）
3. misses 累积超过 len(dirty) → 提升 dirty 为 read
```

#### sync.Map 适用场景

```go
// ✅ 适合: key 集合稳定，读远多于写
var cache sync.Map

// 缓存配置（读多写少）
cache.Store("config", loadConfig())

// 读取
if v, ok := cache.Load("config"); ok {
    cfg := v.(*Config)
    // 使用配置...
}

// ✅ 适合: 不同 goroutine 操作不同的 key
// 每个 goroutine "拥有"各自的 key，不会冲突
var counters sync.Map

for i := 0; i < 100; i++ {
    go func(id int) {
        counters.Store(id, 0)           // 写自己的 key
        val, _ := counters.Load(id)     // 读自己的 key
        counters.Store(id, val.(int)+1)
    }(i)
}
```

#### sync.Map 不适合的场景

```go
 ❌ 不适合: 频繁写入同一 key
var m sync.Map
for i := 0; i < 1000000; i++ {
    m.Store("counter", i) // 每次写都要加锁提升 dirty -> read
}

// ❌ 不适合: 需要 Len() 方法
// sync.Map 没有 Len() 方法，遍历计数很慢
```

#### RWMutex + map vs sync.Map 性能对比

```go
// 读多写少 (99% 读): sync.Map 胜出
// 读写均衡 (50% 读):  RWMutex + map 胜出
// 写多读少 (1% 读):   Mutex + map 胜出

// 基准测试
func BenchmarkRWMutexMap(b *testing.B) {
    m := make(map[string]int)
    var mu sync.RWMutex

    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            if i%100 == 0 {
                mu.Lock()
                m["key"] = i
                mu.Unlock()
            } else {
                mu.RLock()
                _ = m["key"]
                mu.RUnlock()
            }
            i++
        }
    })
}

func BenchmarkSyncMap(b *testing.B) {
    var m sync.Map

    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            if i%100 == 0 {
                m.Store("key", i)
            } else {
                m.Load("key")
            }
            i++
        }
    })
}
```

> **面试要点**: 问 "sync.Map 和 RWMutex+map 怎么选？" 答: 如果 key 稳定且读远多于写，用 sync.Map；否则用 RWMutex+map 更可控。

---

## 4.5 并发控制 sync.WaitGroup

`WaitGroup` 用于等待一组 goroutine 完成。

### 基本用法

```go
var wg sync.WaitGroup

for i := 0; i < 5; i++ {
    wg.Add(1) // 在启动 goroutine 之前 Add
    go func(id int) {
        defer wg.Done() // 完成时 Done
        fmt.Printf("worker %d done\n", id)
    }(i)
}

wg.Wait() // 阻塞直到计数器归零
```

### 三个方法

| 方法 | 作用 | 注意 |
|------|------|------|
| `Add(n)` | 计数器加 n | **必须在 goroutine 外**调用 |
| `Done()` | 计数器减 1 | 等价于 `Add(-1)` |
| `Wait()` | 阻塞直到计数器为 0 | 不能在 Wait 之后再 Add |

### 常见陷阱

```go
// 陷阱1: 在 goroutine 内 Add
for i := 0; i < 5; i++ {
    go func(id int) {
        wg.Add(1) // BUG: 可能在 Wait 之后才 Add
        defer wg.Done()
        doWork(id)
    }(i)
}
wg.Wait() // 可能提前返回

// 修复: 在 go 之前 Add
for i := 0; i < 5; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        doWork(id)
    }(i)
}
wg.Wait()

// 陷阱2: 复制 WaitGroup
func bad(wg sync.WaitGroup) { // 值传递!
    defer wg.Done()
    doWork()
}

// 修复: 用指针
func good(wg *sync.WaitGroup) {
    defer wg.Done()
    doWork()
}
```

### WaitGroup + Error Group (Go 扩展)

标准库没有自带 errgroup，需要 `golang.org/x/sync/errgroup`:

```go
import "golang.org/x/sync/errgroup"

g, ctx := errgroup.WithContext(context.Background())

// 限制并发数
g.SetLimit(10)

for _, url := range urls {
    url := url // 捕获变量
    g.Go(func() error {
        return fetch(ctx, url)
    })
}

if err := g.Wait(); err != nil {
    // 返回第一个错误
    log.Fatal(err)
}
```

### 手写 WaitGroup（面试题）

```go
type MyWaitGroup struct {
    ch chan struct{}
}

func NewMyWaitGroup() *MyWaitGroup {
    return &MyWaitGroup{ch: make(chan struct{})}
}

func (wg *MyWaitGroup) Add(delta int) {
    for i := 0; i < delta; i++ {
        wg.ch <- struct{}{}
    }
}

func (wg *MyWaitGroup) Done() {
    <-wg.ch
}

func (wg *MyWaitGroup) Wait() {
    for len(wg.ch) > 0 {
        runtime.Gosched()
    }
}
```

> **面试要点**: WaitGroup 的核心就是一个原子计数器 + 信号量。`Add` 增加计数，`Done` 减少计数，`Wait` 阻塞直到计数归零。

---

## 4.6 并发对象池 sync.Pool

`sync.Pool` 是一个临时对象缓存池，用于减少 GC 压力。

### 基本用法

```go
var bufPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

// 从池中获取
buf := bufPool.Get().(*bytes.Buffer)
buf.Reset() // 使用前必须 Reset

// 使用
buf.WriteString("hello")
fmt.Println(buf.String())

// 归还到池
bufPool.Put(buf)
```

### 内部原理

```
sync.Pool 三层结构:

  ┌─────────────────────────────────────────┐
  │             sync.Pool                    │
  │  ┌─────────┐  ┌─────────┐  ┌─────────┐ │
  │  │  local   │  │  local   │  │  local   │ │
  │  │  (P0)    │  │  (P1)    │  │  (P2)    │ │
  │  │ private  │  │ private  │  │ private  │ │
  │  │ shared   │  │ shared   │  │ shared   │ │
  │  └─────────┘  └─────────┘  └─────────┘ │
  │       每个 P 有自己的 local，无锁访问     │
  └─────────────────────────────────────────┘

Get() 流程:
1. 先从当前 P 的 private 取（无锁）
2. 再从当前 P 的 shared 取（无锁）
3. 最后从其他 P 的 shared 偷（加锁）
4. 都没有 → 调用 New 创建

Put() 流程:
1. 先放到当前 P 的 private（无锁）
2. private 已满 → 放到 shared 头部（无锁）
```

### 重要特性: GC 会清空 Pool

```go
var pool = sync.Pool{
    New: func() any { return new(bytes.Buffer) },
}

// 放入对象
buf := pool.Get().(*bytes.Buffer)
buf.WriteString("data")
pool.Put(buf)

runtime.GC() // GC 后，Pool 被清空!

buf = pool.Get().(*bytes.Buffer)
fmt.Println(buf.Len()) // 0 — 被 GC 了，New 创建了新的
```

### 最佳实践

```go
// ✅ 场景1: 高频创建临时对象
var jsonBufPool = sync.Pool{
    New: func() any {
        return new(bytes.Buffer)
    },
}

func marshalJSON(v any) ([]byte, error) {
    buf := jsonBufPool.Get().(*bytes.Buffer)
    defer func() {
        buf.Reset()
        jsonBufPool.Put(buf)
    }()

    enc := json.NewEncoder(buf)
    if err := enc.Encode(v); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

// ✅ 场景2: 复用大对象
var largeBufPool = sync.Pool{
    New: func() any {
        b := make([]byte, 64*1024) // 64KB buffer
        return &b
    },
}
```

### sync.Pool vs 手动缓存

| 特性 | sync.Pool | 手动缓存 |
|------|-----------|----------|
| 线程安全 | 内置 | 需自行加锁 |
| GC 行为 | 自动清空 | 持久化 |
| 复杂度 | 低 | 高 |
| 适用 | 临时对象 | 长期缓存 |

> **面试要点**: sync.Pool 不是连接池！它用于减少 GC 压力，对象在 GC 时会被清空。连接池需要自己实现或用 `database/sql` 内置的。

---

## 4.7 并发限流与singleflight

### 限流器

#### time.Ticker 固定速率

```go
ticker := time.NewTicker(100 * time.Millisecond) // 每 100ms 一个
defer ticker.Stop()

for req := range requests {
    <-ticker.C // 等待下一个 tick
    go process(req)
}
```

#### 令牌桶限流 (golang.org/x/time/rate)

```go
import "golang.org/x/time/rate"

// 每秒 10 个请求，突发最多 20 个
limiter := rate.NewLimiter(10, 20)

if limiter.Allow() {
    // 处理请求
} else {
    // 限流
}

// 阻塞等待
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
if err := limiter.Wait(ctx); err != nil {
    // 超时
}
```

### singleflight — 防止缓存击穿

`singleflight` 确保同一个 key 的并发请求只执行一次，其他请求等待并共享结果。

```go
import "golang.org/x/sync/singleflight"

var g singleflight.Group
var cache = make(map[string]string)

func GetFromCache(key string) (string, error) {
    // 先查缓存
    if val, ok := cache[key]; ok {
        return val, nil
    }

    // 缓存未命中，用 singleflight 防止并发穿透
    v, err, shared := g.Do(key, func() (any, error) {
        // 只有一个 goroutine 会执行这里
        val, err := loadFromDB(key)
        if err == nil {
            cache[key] = val
        }
        return val, err
    })
    _ = shared

    if err != nil {
        return "", err
    }
    return v.(string), nil
}
```

#### singleflight 原理

```
请求 A ──→ g.Do("user:1", fn) ──→ 执行 fn
请求 B ──→ g.Do("user:1", fn) ──→ 等待 A 的结果
请求 C ──→ g.Do("user:1", fn) ──→ 等待 A 的结果
                ↓
请求 A 完成 ──→ B、C 也获得相同结果
```

> **面试要点**: singleflight 和缓存一起用时，可以有效防止**缓存击穿**（大量并发请求同时穿透到数据库）。

---

## 4.8 Harness 并发实践

### Harness 中的并发模式

#### 1. 事件总线 — Channel + Goroutine

```go
// pkg/event/event.go 核心实现
type EventBus struct {
    handlers map[EventType][]Handler
    mu       sync.RWMutex
    async    bool
    buffer   chan Event
}

func (eb *EventBus) Publish(ctx context.Context, event Event) error {
    eb.mu.RLock()
    handlers := eb.handlers[event.Type]
    eb.mu.RUnlock()

    if eb.async {
        for _, h := range handlers {
            go h(ctx, event) // 异步分发
        }
    } else {
        for _, h := range handlers {
            if err := h(ctx, event); err != nil {
                return err
            }
        }
    }
    return nil
}
```

**面试知识点**: Channel 做事件队列，RWMutex 保护 handler map（读多写少），async 模式用 goroutine 异步分发。

#### 2. 并发任务执行 — Semaphore Pattern

```go
// internal/core/engine.go
type Engine struct {
    semaphore chan struct{} // 并发信号量
}

func NewEngine(cfg *config.Config) *Engine {
    return &Engine{
        semaphore: make(chan struct{}, cfg.Engine.MaxConcurrentTasks),
    }
}

func (e *Engine) ExecuteTask(ctx context.Context, task Task) (*Result, error) {
    // 获取信号量
    select {
    case e.semaphore <- struct{}{}:
        defer func() { <-e.semaphore }()
    case <-ctx.Done():
        return nil, ctx.Err()
    }

    // 执行任务...
}
```

**面试知识点**: 用 buffered channel 做信号量限制并发数，比 Mutex 更 Go 惯用。`select` + `ctx.Done()` 实现取消。

#### 3. 弹性系统 — Mutex + Atomic

```go
// pkg/resilience/circuit_breaker.go
type CircuitBreaker struct {
    mu               sync.RWMutex // 保护状态
    state            CircuitState
    failureCount     int32  // 用 atomic 操作
    successCount     int32
}

func (cb *CircuitBreaker) AllowRequest() bool {
    cb.mu.RLock()
    defer cb.mu.RUnlock()

    switch cb.state {
    case CircuitStateClosed:
        return true
    case CircuitStateOpen:
        if time.Since(cb.lastFailureTime) > cb.timeout {
            cb.mu.RUnlock()
            cb.mu.Lock()
            cb.setState(CircuitStateHalfOpen)
            cb.mu.Unlock()
            cb.mu.RLock()
            return true
        }
        return false
    }
    return false
}
```

**面试知识点**: 读写锁保护状态机，状态转换时需要"升级"为写锁。`RUnlock` → `Lock` → 修改 → `Unlock` → `RLock` 是标准的锁升级模式。

#### 4. 连接池 — sync.Pool 思想

```go
// pkg/pool/pool.go
type GenericPool struct {
    mu       sync.Mutex
    pool     chan interface{} // 用 channel 做对象池
    factory  func(ctx context.Context) (interface{}, error)
    close    func(conn interface{}) error
    validate func(conn interface{}) bool
}

func (p *GenericPool) Get(ctx context.Context) (interface{}, error) {
    select {
    case conn := <-p.pool:
        // 验证连接
        if p.validate(conn) {
            return conn, nil
        }
        p.close(conn)
        // 验证失败，创建新的
        return p.factory(ctx)
    default:
        // 池空了，创建新的
        return p.factory(ctx)
    }
}

func (p *GenericPool) Put(ctx context.Context, conn interface{}) error {
    select {
    case p.pool <- conn:
        return nil
    default:
        // 池满了，关闭连接
        return p.close(conn)
    }
}
```

**面试知识点**: 用 buffered channel 实现对象池，比 Mutex + slice 更高效。`select` + `default` 实现非阻塞操作。

### Harness 并发知识点总结

| Harness 组件 | Go 并发原语 | 面试考点 |
|-------------|------------|----------|
| 事件总线 | Channel + RWMutex | Channel 通信、读写锁 |
| 任务执行 | Semaphore Channel | 信号量模式、select |
| 熔断器 | RWMutex + 状态机 | 锁升级、状态机 |
| 连接池 | Channel 做对象池 | 非阻塞 select |
| 缓存 | sync.Map / MemoryCache | 并发 map 选型 |
| 配置热更新 | atomic.Value | 原子操作 |
| 指标收集 | atomic.AddInt64 | 无锁计数器 |
| 任务等待 | sync.WaitGroup | 等待组模式 |

---

## 本章小结

### 面试高频问题

1. **Goroutine 和线程的区别？**
   - Goroutine 是用户态协程（~2KB 栈），线程是内核态（~1MB）
   - Goroutine 由 Go runtime 调度，线程由 OS 调度
   - Goroutine 切换成本低（~100ns vs ~1μs）

2. **Channel 和 Mutex 怎么选？**
   - 传递数据所有权 → Channel
   - 保护共享状态 → Mutex
   - 协调 goroutine 生命周期 → Channel (done channel)
   - 简单计数器 → atomic

3. **Go 的内存模型是什么？**
   - happens-before 关系
   - Channel 通信建立 happens-before
   - sync 包建立 happens-before

4. **什么是 goroutine 泄漏？怎么防止？**
   - Goroutine 永远阻塞，无法退出
   - 原因: channel 未关闭、缺少 cancel 机制
   - 防止: done channel、context cancel、超时控制

5. **sync.Map 的原理？**
   - read map (原子访问) + dirty map (加锁访问)
   - 读多写少时性能好
   - miss 累积后提升 dirty 为 read

### 代码速查表

```go
// 并发安全计数器
var count int64
atomic.AddInt64(&count, 1)

// 并发安全 map (读多写少)
var m sync.Map
m.Store(k, v)
v, ok := m.Load(k)

// 等待一组 goroutine
var wg sync.WaitGroup
wg.Add(1)
go func() { defer wg.Done(); ... }()
wg.Wait()

// 限制并发数
sem := make(chan struct{}, 10)
sem <- struct{}{}
defer func() { <-sem }()

// 防止缓存击穿
g.Do(key, func() (any, error) { ... })

// 对象复用
pool := sync.Pool{New: func() any { return new(T) }}
obj := pool.Get().(*T)
defer pool.Put(obj)
```
