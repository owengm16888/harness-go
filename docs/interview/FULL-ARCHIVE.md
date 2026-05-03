# Go 面试知识库 — 完整存档

> 日期: 2026-05-02 ~ 2026-05-03
> 模型: mimo-v2-pro (custom)
> 来源: Harness 框架模拟面试 + 深度问答

---

## 目录

- [第一部分: Harness API 操作](#第一部分-harness-api-操作)
- [第二部分: 模拟面试](#第二部分-模拟面试)
- [第三部分: 面试题库（15 题）](#第三部分-面试题库15-题)
- [第四部分: 高级主题（16 题）](#第四部分-高级主题16-题)
- [第五部分: 基础进阶（19 题）](#第五部分-基础进阶19-题)
- [第六部分: 经验总结](#第六部分-经验总结)

---

# 第一部分: Harness API 操作

## 完成的 API 调用

```
POST /api/tasks              ✅ 创建任务 "Go面试"
GET  /api/tasks              ✅ 列出任务
GET  /api/tasks/{id}         ✅ 获取任务详情
POST /api/tasks/{id}/execute ✅ 执行任务 (interview 类型)
POST /api/tasks/{id}/cancel  ⬜ 未测试
```

## 代码改动

文件: `internal/adapters/claude_code.go`

1. `execute()` switch 新增 `case "interview"`
2. 新增 `executeInterview()` — 优先调 claude CLI，失败回退内置题库
3. 新增 `generateBuiltinInterview()` — 内置 Go 面试题生成
4. `buildPrompt()` 新增 `case "interview"` 的 prompt 构建

## 关键发现

- `POST /api/tasks` 要求客户端自生成 `id` 字段
- 适配器 `execute()` 只支持 implement/review/test，新增了 interview
- claude CLI 不可用时回退到内置题库

---

# 第二部分: 模拟面试

## 评分

| # | 题目 | 得分 | 难度 |
|---|------|------|------|
| 1 | make vs new 区别 | 8/10 | 基础 |
| 2 | 闭包捕获循环变量 | 9/10 | 中等 |
| 3 | nil interface 陷阱 | 10/10 | 中等 |
| 4 | HTTP 优雅关停 | 8/10 | 高级 |
| 5 | 锁粒度设计 | 9/10 | 高级 |

总分: 44/50 (88%) 评级: A

## 扣分分析

- 基础题: 回答准确但偏简洁，应主动展开代码示例
- 设计题: 思路正确但缺少边界情况讨论

## 面试技巧

回答公式: 结论 → 原理 → 代码示例 → 边界情况 → 项目实际应用

---

# 第三部分: 面试题库（15 题）

## 1. make vs new 区别

**结论:** new 负责"分配"，make 负责"分配 + 初始化内部结构"。

```go
// new: 分配零值内存，返回指针，任何类型
p := new(int)       // *int, 值为 0
s := new([]int)     // *[]int, 值为 nil

// make: 分配 + 初始化内部结构，返回值，仅 slice/map/channel
s := make([]int, 3, 5)      // len=3, cap=5
m := make(map[string]int)   // 已初始化 hash 表
ch := make(chan int, 10)    // 已初始化环形缓冲区
```

记忆: new 分配指针，make 初始化引用类型。new 什么类型都行，make 只能 slice map channel。

---

## 2. 闭包捕获循环变量

```go
for i := 0; i < 5; i++ {
    go func() {
        fmt.Println(i)  // 全部输出 5
    }()
}
```

原因: 闭包捕获变量引用而非值拷贝，goroutine 启动有延迟，循环结束时 i=5。

修复:
```go
go func(n int) { fmt.Println(n) }(i)  // 参数传递
```

Go 1.22+ 已修复，每次迭代创建新变量。

---

## 3. defer 执行顺序

**结论:** defer 按 LIFO（后进先出）执行，参数在声明时求值。

```go
// LIFO 顺序
func f1() {
    defer fmt.Println("A")
    defer fmt.Println("B")
    defer fmt.Println("C")
}
// 输出: C B A

// 参数在声明时求值
func f2() {
    x := 1
    defer fmt.Println(x)  // x 在此处求值为 1
    x = 2
}
// 输出: 1（不是 2）

// 修改命名返回值
func f3() (result int) {
    defer func() { result++ }()
    return 0
}
// 返回 1（return 0 赋值 → defer 改为 1）
```

return 与 defer 执行顺序: 先赋值返回值 → 再执行 defer。defer 可以修改命名返回值，但不能修改非命名返回值。

---

## 4. nil interface 陷阱

```go
var err2 *os.PathError = nil
var err3 error = err2
fmt.Println(err3 == nil)  // false
```

interface 内部由 `tab`(类型信息) + `data`(值) 组成。==nil 要求两者都为 nil。赋值后 tab 记录了 `*os.PathError` 类型，不为 nil。

正确做法: 函数返回 error 时直接 `return nil`，不要返回值为 nil 的具体类型变量。

---

## 5. HTTP 优雅关停

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

quit := make(chan os.Signal, 1)  // 带缓冲防信号丢失
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
defer shutdownCancel()
srv.Shutdown(shutdownCtx)  // 停止 Accept，等待存量请求

cancel()
wg.Wait()      // 等待 goroutine 退出
store.Close()  // 清理资源
```

Shutdown 内部流程: 停止 Accept → 关闭 idle 连接 → 等待 active 连接 → 超时返回 DeadlineExceeded。

---

## 6. 锁粒度设计

**核心原则:** 锁只保护共享状态的读写，不保护业务逻辑。

```go
// 正确做法
tm.mu.Lock()
state.Status = TaskStatusInProgress  // 状态机守卫
tm.mu.Unlock()

result := tm.executor.Execute(ctx, state.Task)  // 耗时操作，不持锁

tm.mu.Lock()
state.Status = TaskStatusCompleted
tm.mu.Unlock()
```

持锁问题: 吞吐量暴跌、死锁风险、锁饥饿。

---

## 7. goroutine vs 线程

```
                    goroutine              线程
创建者              Go runtime             OS 内核
调度者              Go scheduler (GMP)     OS 调度器
初始栈大小          2KB（可动态增长）       1-8MB（固定）
上下文切换成本      ~100ns（用户态）        ~1-10μs（内核态）
最大并发数          百万级                  千级
```

---

## 8. channel 底层结构

```go
type hchan struct {
    qcount   uint           // 当前元素数
    dataqsiz uint           // 缓冲区大小
    buf      unsafe.Pointer // 环形缓冲区
    sendx    uint           // 发送索引
    recvx    uint           // 接收索引
    recvq    waitq          // 接收等待队列
    sendq    waitq          // 发送等待队列
    lock     mutex          // 互斥锁
}
```

发送流程: 找接收者 → 看缓冲区 → 挂起等待。
接收流程: 找发送者 → 看缓冲区 → 挂起等待。

---

## 9. 避免 goroutine 泄漏

四种根源: channel 永远阻塞、select 无退出路径、死锁、无限循环。

```go
// 通用防护: 所有 goroutine 都必须有退出路径
func worker(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case job := <-jobCh:
            doJob(job)
        }
    }
}
```

---

## 10. Go 垃圾回收机制

三色标记清除 + 写屏障的并发 GC，目标停顿时间 < 1ms。

```
白色: 未扫描，可能是垃圾
灰色: 已扫描，子对象未扫描
黑色: 已扫描，子对象已扫描
```

GC 四阶段:
```
1. 标记准备 (STW)    — 开启写屏障，扫描栈，~0.1ms
2. 并发标记          — 与用户代码并发，最耗时
3. 标记终止 (STW)    — 关闭写屏障，~0.1ms
4. 并发清除          — 回收白色对象，与用户并发
```

Go 1.8+ 使用混合写屏障（插入 + 删除），无需 STW 重新扫描栈。
GOGC=100（默认）: 堆增长 100% 触发 GC。GOMEMLIMIT 可设置内存上限。

---

## 11. 逃逸分析

编译器决定变量分配在栈还是堆上。

```go
func f() *int {
    x := 42
    return &x  // x 逃逸到堆
}

func g() {
    x := 42
    _ = x  // 不逃逸，栈上
}
```

查看: `go build -gcflags="-m" ./...`

---

## 12. slice vs 数组 + append 扩容

数组是值类型，slice 是引用类型（指针 + len + cap）。

```
Go 1.21+ 扩容策略:
cap < 256:  newCap = oldCap * 2（翻倍）
cap >= 256: newCap = oldCap + oldCap/4 + 192（约 1.25 倍 + 平滑因子）
结果还要内存对齐到 size class
```

子切片 append 可能污染原数组（共享底层数组）。用 copy 创建独立副本避免。

---

## 13. 泛型使用场景和限制

```go
func Max[T constraints.Ordered](a, b T) T {
    if a > b { return a }
    return b
}
```

限制: 方法不能有额外类型参数、不支持特化、不支持元编程。
实现: GC Shape Stenciling，相同 GC shape 共享机器码。

---

## 14. 值接收者 vs 指针接收者

```
必须用指针: 修改状态、大结构体、含锁字段
建议用指针: 一致性（有一个用指针则全部用指针）
建议用值:   小型不可变结构体
```

指针接收者实现的接口，只有 `*T` 满足接口。值接收者实现的接口，`T` 和 `*T` 都满足。

---

## 15. sync.Map vs 加锁 map

```
sync.Map: 读多写少、key 稳定、无锁读
RWMutex+map: 读写均衡、频繁增删、需要 Len()
```

sync.Map 内部: read (原子) + dirty (加锁) + misses 计数。

---

## 16. pprof 性能剖析

```
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
go tool pprof http://localhost:6060/debug/pprof/heap
```

五种 Profile: CPU、Heap、Goroutine、Mutex、Block。

---

# 第四部分: 高级主题（16 题）

## 17. GMP 调度模型详解

G (Goroutine): 用户态执行体，2KB 栈
M (Machine): OS 线程，执行 G 的载体
P (Processor): 逻辑处理器，持有本地运行队列

取 G 顺序: P.runnext → P.runq → 全局队列 → 偷其他 P → netpoller

G 阻塞时: 系统调用 → M 阻塞，P 转移给新 M；channel 阻塞 → G 挂起，M 继续执行其他 G。

---

## 18. Go 1.14 异步抢占

解决了"没有函数调用的死循环无法被抢占"的问题。

协作式抢占: 函数调用时检查 preempt 标志
信号式抢占: 发送 SIGURG 强制中断

Go 1.14+ 混合使用: 大多数用协作式，死循环等场景用信号式。

---

## 19. Go 内存模型 happens-before

```
go 语句          happens-before  goroutine 执行
ch <- v          happens-before  <-ch 完成
close(ch)        happens-before  <-ch 返回零值
mu.Unlock()      happens-before  mu.Lock() (下一次)
wg.Done()        happens-before  wg.Wait() 返回
atomic.Store     happens-before  atomic.Load
```

---

## 20. sync/atomic vs mutex

atomic: 单变量无锁操作，CAS 指令，~1ns
mutex: 多变量互斥锁，自旋+futex，~20-100ns

atomic 限制: 只能操作单变量、64 位需要对齐、不能用于复杂操作。

---

## 21. 三色标记与写屏障

三种写屏障:
- 插入写屏障: 新引用目标标灰
- 删除写屏障: 旧引用目标标灰
- 混合写屏障: Go 1.8+，两者结合，消除 STW 栈重扫描

---

## 22. sync.Pool 实现原理

per-P 本地存储: private (无锁) + shared (无锁 CAS)
Get 流程: private → shared → 偷其他 P → victim → New()
GC 会清空 Pool，对象最多存活 2 轮 GC。

---

## 23. select 底层与随机选择

编译时转换为 switch-case，运行时调用 selectgo。
随机起始位置轮转遍历，保证每个 case 概率均等。
全部阻塞时挂起 goroutine，注册到所有 channel 等待队列。

---

## 24. context 传播原理

树形结构，parent 取消自动取消所有 child。
WithCancel: 手动取消，close(done channel)
WithTimeout: 定时器触发取消
Done() 懒初始化，用到才创建 channel。

---

## 25. reflect 性能开销

四大开销: 类型信息查找、间接调用、逃逸分析失效、内联失败。
优化: 泛型替代、类型断言、预编译字段索引、code generation。

---

## 26. 泛型类型约束与 ~T

~T 匹配底层类型为 T 的所有类型（包括自定义类型）。
T 只匹配精确类型。
comparable 可比较，any 是全集。
约束可以混合类型集合和方法约束。

---

## 27. errgroup/semaphore 并发原语

errgroup: WaitGroup + context，任一出错 cancel 全部。
semaphore: 令牌桶，Acquire 阻塞等待，Release 唤醒。
singleflight: 合并相同 key 的并发请求。
worker pool: 预分配 worker + channel 任务队列。

---

## 28. 高性能 worker pool

核心: 预分配 worker、无锁队列、动态伸缩、优雅关停。
优先级: 嵌套 select 实现高/中/低优先级。
监控: 队列长度、活跃数、错误率、平均耗时。

---

## 29. 逃逸分析/内联/编译器指令

```
//go:noescape   禁止逃逸分析
//go:nosplit    禁止栈增长检查
//go:noinline   禁止内联
//go:linkname   链接私有函数
```

查看: `go build -gcflags="-m -m" ./...`

---

## 30. CGO 工作机制

开销来源: 栈切换、信号屏蔽、内存管理、goroutine 绑定。
CString 必须手动 free，Go 指针不能传给 C。
C 代码阻塞 M，需要限制并发 CGO 调用数。

---

## 31. 微服务链路追踪与可观测性

三支柱: Metrics + Logging + Tracing
OpenTelemetry 统一采集，OTLP 协议传输。
TraceID 贯穿全链路，SpanID 标记每一步。
HTTP/gRPC/DB/Redis 自动追踪中间件。

---

## 32. functional options vs builder

functional options: Go 惯用法，函数闭包配对象，类型安全可扩展。
builder: 链式调用，Build 时统一校验，适合复杂构建过程。

Go 库/框架推荐 functional options。

---

# 第五部分: 基础进阶（19 题）

## 33. Go 语言优势

**结论:** Go 的核心优势是简洁 + 高效并发 + 快速编译 + 部署简单。

```
vs Java/C++:   编译快（秒级），单二进制部署，无 JVM/运行时依赖
vs Python/Ruby: 原生并发（goroutine），编译型高性能，类型安全
vs Rust:        学习曲线低，GC 自动内存管理，开发效率高
```

独特卖点: goroutine 轻量并发（百万级）、交叉编译简单、go fmt 统一风格、go module 依赖管理。

记忆: Go = C 的性能 + Python 的简洁 + 原生并发。

---

## 34. 协程 (goroutine)

**结论:** goroutine 是 Go runtime 管理的用户态轻量级线程，初始栈仅 2KB。

```go
go func() {
    fmt.Println("hello") // 在新 goroutine 中执行
}()

// 启动成本极低，可轻松创建百万级
for i := 0; i < 1000000; i++ {
    go worker(i)
}
```

vs 操作系统线程: goroutine 由 Go scheduler 调度（用户态），栈可动态增长；OS 线程由内核调度，栈固定 1-8MB。

记忆: goroutine = 用户态轻量线程，2KB 起步，Go runtime 调度。

---

## 35. 协程/线程/进程区别

```
                  进程              线程              协程 (goroutine)
调度者            OS 内核           OS 内核           Go runtime
内存共享          不共享(独立地址)   共享(同进程内)     共享(同进程内)
创建开销          大(fork)          中(clone)         小(go func())
切换开销          ~ms(内核态)       ~1-10μs(内核态)   ~100ns(用户态)
栈大小            独立地址空间       1-8MB 固定        2KB 动态增长
数量级            百级              千级              百万级
```

记忆: 进程重(独立空间) → 线程中(共享内存+内核调度) → 协程轻(用户态+动态栈)。

---

## 36. for range 地址变化

**结论:** for range 中 v 的地址始终不变，每次迭代只更新其值。取 &v 拿到的是同一个地址。

```go
s := []int{1, 2, 3}
var addrs []*int
for _, v := range s {
    addrs = append(addrs, &v) // &v 始终是同一个地址！
}
fmt.Println(*addrs[0], *addrs[1], *addrs[2]) // 3 3 3（全是最后一个值）

// 修复: 在循环内拷贝
for _, v := range s {
    v := v // 新变量拷贝
    addrs = append(addrs, &v)
}
```

Go 1.22+ 已修复，每次迭代创建新变量，&v 地址不同。

记忆: Go < 1.22 的 range v 是同一个地址；需要拷贝或用索引。

---

## 37. 高效拼接字符串

**结论:** 字符串拼接优先用 strings.Builder，少量用 +，避免循环中用 +。

```go
// ❌ 最慢: 循环中 + 创建大量临时字符串
s := ""
for _, w := range words { s += w }

// ✅ 最优: strings.Builder（内部用 []byte，Grow 预分配）
var b strings.Builder
b.Grow(totalLen) // 预估总长度
for _, w := range words { b.WriteString(w) }
s := b.String()  // 零拷贝

// 也行: strings.Join（内部也是 Builder）
s := strings.Join(words, "")

// 也行: fmt.Sprintf（格式化场景）
s := fmt.Sprintf("%s-%d-%v", name, age, tags)
```

性能: Builder ≈ Join > bytes.Buffer > fmt.Sprintf > +

记忆: 循环拼接必须用 Builder，一次性的用 Join。

---

## 38. rune 类型

**结论:** rune 是 int32 的别名，表示一个 Unicode 码点（一个字符）。

```go
s := "Hello, 世界"
fmt.Println(len(s))         // 13（字节数，不是字符数）
fmt.Println(utf8.RuneCountInString(s)) // 9（字符数）

for i, r := range s {
    fmt.Printf("index=%d, rune=%c, codepoint=%U\n", i, r, r)
}
// 'H' → U+0048, '世' → U+4E16（占 3 字节）

// 字符串操作
runes := []rune(s)   // 转 rune 切片，按字符索引
runes[7] = '中'       // 替换第 8 个字符
s = string(runes)    // 转回字符串
```

记忆: byte=ASCII(1字节), rune=Unicode(4字节,int32)。中文用 rune。

---

## 39. struct tag 作用

**结论:** tag 是附加在 struct 字段上的元数据，供反射读取，最常见于 JSON/DB 序列化。

```go
type User struct {
    ID        int    `json:"id" db:"user_id" validate:"required"`
    Name      string `json:"name,omitempty" db:"user_name"`
    Password  string `json:"-"` // JSON 序列化时忽略
}

// JSON 编码器通过反射读取 tag:
// - "id" 作为 JSON key
// - "omitempty" 零值时省略
// - "-" 忽略该字段

// 自定义读取 tag
field := reflect.TypeOf(User{}).Field(0)
fmt.Println(field.Tag.Get("json")) // "id"
fmt.Println(field.Tag.Get("db"))   // "user_id"
```

记忆: tag = struct 字段的元数据，通过反射读取，JSON/ORM 核心机制。

---

## 40. %v %+v %#v 格式化区别

**结论:** %v 默认格式，%+v 带字段名，%#v Go 语法表示。

```go
type User struct {
    Name string
    Age  int
}
u := User{"Alice", 30}

fmt.Printf("%v\n",  u)  // {Alice 30}
fmt.Printf("%+v\n", u)  // {Name:Alice Age:30}
fmt.Printf("%#v\n", u)  // main.User{Name:"Alice", Age:30}

// 其他常用
fmt.Printf("%T\n", u)   // main.User (类型名)
fmt.Printf("%t\n", true) // true (布尔)
fmt.Printf("%d\n", 42)   // 42 (十进制)
fmt.Printf("%x\n", 255)  // ff (十六进制)
fmt.Printf("%s\n", "hi") // hi (字符串)
```

记忆: %v=值, %+v=字段:值, %#v=完整Go语法, %T=类型。

---

## 41. 空 struct{} 占空间吗

**结论:** 单个空 struct{} 占 0 字节，但作为数组元素时特殊。

```go
var s struct{}
fmt.Println(unsafe.Sizeof(s))  // 0

// 作为 map value: 零内存分配，等同于 set
m := make(map[string]struct{})
m["key"] = struct{}{}

// 特殊: [N]struct{} 数组占 0 字节，但 len 仍为 N
var a [1000000]struct{}
fmt.Println(unsafe.Sizeof(a)) // 0
```

记忆: struct{} = 零大小类型，Go runtime 专门优化，不占内存。

---

## 42. 空 struct{} 用途

**结论:** 主要用于信号传递和集合，零内存开销。

```go
// 1. set 实现（只关心 key 是否存在）
set := make(map[string]struct{})
set["apple"] = struct{}{}
if _, ok := set["apple"]; ok { /* 存在 */ }

// 2. channel 信号（不传数据，只传事件）
done := make(chan struct{})
go func() {
    // do work...
    close(done) // 发送完成信号
}()
<-done // 等待完成

// 3. 方法接收者（无状态方法集）
type Parser struct{}
func (Parser) Parse(s string) error { ... }
```

记忆: struct{} 三个用途: 集合、信号通道、无状态方法。

---

## 43. init() 执行时机

**结论:** init() 在 main() 之前自动执行，顺序: 依赖包 → 当前包 → main()。

```go
// 执行顺序:
// 1. 包级变量初始化（按声明顺序，依赖优先）
// 2. init() 函数执行（每个包可有多个 init）
// 3. main.main()

var db = initDB() // 先执行

func init() { fmt.Println("first init") }  // 再执行
func init() { fmt.Println("second init") } // 再执行
func main() { fmt.Println("main") }        // 最后执行

// 包依赖顺序:
// import 的包先初始化 → 当前包再初始化
// 同一包内: 变量声明 → init() 按源码顺序
```

记忆: init() = 包加载时自动调用，main() 之前，依赖先于被依赖。

---

## 44. 两个 interface 如何比较

**结论:** interface 比较 = (类型相同 AND 值相同)，动态类型不可比较会 panic。

```go
var a interface{} = 1
var b interface{} = 1
fmt.Println(a == b) // true (类型都是 int，值都是 1)

var c interface{} = []int{1}
var d interface{} = []int{1}
// fmt.Println(c == d) // panic: comparing uncomparable type []int

// 安全比较: reflect.DeepEqual
fmt.Println(reflect.DeepEqual(c, d)) // true

// nil interface 特殊:
var e error
var f interface{} = e
fmt.Println(f == nil) // true（error 是 interface）
```

记忆: interface == 比较类型+值，slice/map/func 不可比较会 panic。

---

## 45. 两个 nil 不相等

**结论:** interface nil 判等要求 (type=nil, value=nil)，值为 nil 但类型非 nil 时 != nil。

```go
var p *int = nil
var err error = (*os.PathError)(nil)

fmt.Println(p == nil)    // true（具体类型指针）
fmt.Println(err == nil)  // false！interface 的 tab 非 nil

// interface 内部结构:
// type interfaceValue struct { tab *itab; data unsafe.Pointer }
// err = (*os.PathError)(nil)  →  tab = PathError 的 itab, data = nil
// err == nil  →  要求 tab==nil && data==nil  →  false

// 正确做法: 返回 error 时直接 return nil
func f() error {
    var p *MyError = nil
    if bad { return p }  // ❌ 返回了带类型的 nil
    return nil           // ✅ 返回无类型的 nil
}
```

记忆: interface = 类型 + 值，只有两者都 nil 才 == nil。陷阱坑死人。

---

## 46. Go 函数传参方式

**结论:** Go 只有值传递（pass by value），但 slice/map/channel/指针传的是"引用的值拷贝"。

```go
func modify(s []int, m map[string]int, p *int) {
    s[0] = 999       // ✅ 修改底层数组
    m["a"] = 999     // ✅ 修改原 map
    *p = 999         // ✅ 修改指向的值

    s = append(s, 4) // ❌ 不影响外部（新 slice header）
}

// slice 本质: 传的是 {ptr, len, cap} 的拷贝
// 拷贝的 ptr 指向同一底层数组 → 修改元素生效
// append 改的是拷贝的 len/cap → 不影响外部

// 要修改 slice 本身（如 append 后长度变化）:
func modify(s *[]int) {
    *s = append(*s, 4) // 通过指针修改
}
```

记忆: Go = 值传递。slice/map 是"引用类型的值拷贝"，修改元素生效，修改结构（append）不生效。

---

## 47. 栈分配 vs 堆分配（逃逸分析）

**结论:** 编译器通过逃逸分析决定分配位置。栈上分配无 GC 压力，性能更好。

```go
// 堆分配（逃逸）
func f() *int {
    x := 42
    return &x  // x 逃逸到堆（生命周期超出函数）
}

// 栈分配（不逃逸）
func g() int {
    x := 42
    return x  // x 在栈上
}

// 常见逃逸场景:
// 1. 返回局部变量指针
// 2. 赋值给 interface{} (any)
// 3. 闭包引用局部变量
// 4. slice/map 超出编译器分析能力

// 查看逃逸分析结果:
// go build -gcflags="-m" ./...
```

记忆: 函数内变量默认栈分配，返回指针/赋给 interface 会逃逸到堆。

---

## 48. Go 多返回值的底层实现

**结论:** 多返回值通过栈上分配多个返回值空间实现，调用者传入返回值地址。

```go
func divide(a, b int) (int, error) {
    if b == 0 { return 0, errors.New("division by zero") }
    return a / b, nil
}

// 底层等价于:
func divide(a, b int, ret0 *int, ret1 *error) {
    if b != 0 {
        *ret0 = a / b
        *ret1 = nil
        return
    }
    *ret0 = 0
    *ret1 = errors.New("division by zero")
}

// 命名返回值: 函数内创建局部变量，return 时自动赋值
func divide(a, b int) (result int, err error) {
    // result 和 err 是函数内的局部变量
    if b == 0 { return 0, errors.New("division by zero") }
    return a / b, nil  // 显式赋值
    // 或: result = a/b; return  // 隐式赋值
}
```

记忆: 多返回值 = 编译器在栈上分配返回空间，命名返回值 = 局部变量 + 自动 return。

---

## 49. _ 的作用

**结论:** _ 是空白标识符，用于忽略不需要的值、导入包副作用。

```go
// 1. 忽略多返回值
val, _ := strconv.Atoi("42")  // 忽略 error
_, ok := m["key"]              // 只要 ok，忽略值

// 2. 忽略 for range 索引或值
for _, v := range slice { /* 只要值 */ }
for i, _ := range slice { /* 只要索引 */ }
for range slice { /* Go 1.22+ 都不要 */ }

// 3. 导入包（仅执行 init）
import _ "github.com/go-sql-driver/mysql" // 注册 driver

// 4. 编译期接口检查
var _ io.Reader = (*MyType)(nil) // 确保 *MyType 实现 io.Reader

// 5. 忽略函数参数
func callback(_ context.Context, msg string) { ... }
```

记忆: _ = "我不需要这个值"，导入副作用包用 _ "pkg"。

---

## 50. 普通指针 vs unsafe.Pointer

**结论:** 普通指针类型安全，unsafe.Pointer 可转换任意指针但绕过类型检查。

```go
// 普通指针: 类型安全
var x int = 42
var p *int = &x
// var q *string = &x  // 编译错误：类型不匹配

// unsafe.Pointer: 可转换任意指针
var p *int = &x
var up unsafe.Pointer = unsafe.Pointer(p) // *int → unsafe.Pointer
var q *float64 = (*float64)(up)           // unsafe.Pointer → *float64

// 实际用途: 结构体内存布局操作
type S struct { a int32; b int64 }
s := S{1, 2}
p := unsafe.Pointer(&s)
offset := unsafe.Offsetof(s.b)
bPtr := (*int64)(unsafe.Pointer(uintptr(p) + offset))
fmt.Println(*bPtr) // 2
```

unsafe.Pointer 允许: 指针类型转换、指针算术（需配合 uintptr）、reflect 和 syscall 桥接。

记忆: 普通指针 = 类型安全；unsafe.Pointer = 绕过类型系统，性能利器但危险。

---

## 51. unsafe.Pointer vs uintptr

**结论:** unsafe.Pointer 是指针（GC 追踪），uintptr 是整数（GC 不追踪）。

```go
// unsafe.Pointer: 可以转换为任意指针类型，GC 知道它指向的对象
var p *int = &x
var up unsafe.Pointer = p // GC 追踪，x 不会被回收

// uintptr: 无符号整数，存储地址值，GC 不追踪
var u uintptr = uintptr(up) // GC 不知道 u 指向 x
// 如果 x 只有 u 引用，GC 可能回收 x！

// 危险模式:
u := uintptr(unsafe.Pointer(p))
u += 8  // 指针算术
q := (*int)(unsafe.Pointer(u)) // 可能已经悬空！

// 正确模式: 指针算术必须在一个表达式内完成
q := (*int)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 8))
// 不要拆成多行，中间不能有 GC 点
```

关键区别:
```
unsafe.Pointer  →  指针语义，GC 追踪，可转换为任意指针
uintptr         →  整数语义，GC 不追踪，存储地址的数字
```

记忆: Pointer 是指针（GC 友好），uintptr 是数字（GC 盲区）。指针算术必须一行完成。

---

# 第六部分: 经验总结

## 回答公式

```
结论 → 原理 → 代码示例 → 边界情况 → 项目实际应用
```

## 知识库文件

```
docs/interview/
├── README.md                   快速 Review 指南
├── 00-experience-summary.md    面试总览
├── 01-lock-granularity.md      锁粒度详解
├── 03-nil-interface-trap.md    nil interface 陷阱
├── 04-graceful-shutdown.md     HTTP 优雅关停
└── FULL-ARCHIVE.md             本文件（完整存档）
```

## 快速 Review

```bash
# 查看索引
cat docs/interview/README.md

# API 搜索
curl http://localhost:8088/api/knowledge/search?q=interface
curl http://localhost:8088/api/knowledge/search?q=mutex

# 面试前速览
cat docs/interview/00-experience-summary.md
```

## 知识点清单

### 必考（几乎每次面试都有）
- [x] goroutine 调度模型 GMP
- [x] channel 语义和底层结构
- [x] interface 内部结构和 nil 陷阱
- [x] context 传播和取消
- [x] error 处理链（wrap/unwrap/Is/As）

### 高频
- [x] sync.Mutex/RWMutex 使用和陷阱
- [x] defer 执行顺序和求值时机
- [x] slice/map 底层实现
- [x] 内存逃逸和 GC
- [x] HTTP 优雅关停

### 项目相关
- [x] 锁粒度设计（最小化临界区）
- [x] 状态机模式（状态转换守卫）
- [x] 并发控制（semaphore、WaitGroup）
- [x] 观察者模式（EventNotifier）
- [x] 适配器模式（Adapter 接口）

---

> 生成模型: mimo-v2-pro (custom)
> 存档时间: 2026-05-03
