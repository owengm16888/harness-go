# Go 面试知识库 — 快速 Review 指南

> 更新: 2026-05-03
> 总题数: 51 题
> 存档: FULL-ARCHIVE.md (28KB)

---

## 题目索引

### 基础 (1-6)
| # | 题目 | 难度 |
|---|------|------|
| 1 | make vs new 区别 | 基础 |
| 2 | 闭包捕获循环变量 | 中等 |
| 3 | defer 执行顺序 | 基础 |
| 4 | nil interface 陷阱 | 中等 |
| 5 | HTTP 优雅关停 | 高级 |
| 6 | 锁粒度设计 | 高级 |

### 并发 (7-9)
| # | 题目 | 难度 |
|---|------|------|
| 7 | goroutine vs 线程 | 基础 |
| 8 | channel 底层结构 | 中等 |
| 9 | 避免 goroutine 泄漏 | 中等 |

### 内存 (10-12)
| # | 题目 | 难度 |
|---|------|------|
| 10 | Go 垃圾回收机制 | 中等 |
| 11 | 逃逸分析 | 中等 |
| 12 | slice vs 数组 + append 扩容 | 基础 |

### 类型系统 (13-16)
| # | 题目 | 难度 |
|---|------|------|
| 13 | 泛型使用场景和限制 | 中等 |
| 14 | 值接收者 vs 指针接收者 | 基础 |
| 15 | sync.Map vs 加锁 map | 中等 |
| 16 | pprof 性能剖析 | 高级 |

### 高级调度 (17-18)
| # | 题目 | 难度 |
|---|------|------|
| 17 | GMP 调度模型详解 | 高级 |
| 18 | Go 1.14 异步抢占 | 高级 |

### 高级内存 (19-22)
| # | 题目 | 难度 |
|---|------|------|
| 19 | happens-before 规则 | 高级 |
| 20 | atomic vs mutex | 高级 |
| 21 | 三色标记与写屏障 | 高级 |
| 22 | sync.Pool 实现原理 | 高级 |

### 高级 channel/context (23-24)
| # | 题目 | 难度 |
|---|------|------|
| 23 | select 底层与随机选择 | 高级 |
| 24 | context 传播原理 | 高级 |

### 反射/泛型 (25-26)
| # | 题目 | 难度 |
|---|------|------|
| 25 | reflect 性能开销 | 高级 |
| 26 | 泛型类型约束与 ~T | 高级 |

### 并发模式 (27-28)
| # | 题目 | 难度 |
|---|------|------|
| 27 | errgroup/semaphore 并发原语 | 高级 |
| 28 | 高性能 worker pool | 高级 |

### 编译/链接 (29-30)
| # | 题目 | 难度 |
|---|------|------|
| 29 | 编译器指令 | 高级 |
| 30 | CGO 工作机制 | 高级 |

### 工程架构 (31-32)
| # | 题目 | 难度 |
|---|------|------|
| 31 | 微服务链路追踪与可观测性 | 高级 |
| 32 | functional options vs builder | 高级 |

### 基础进阶 (33-51)
| # | 题目 | 难度 |
|---|------|------|
| 33 | Go 语言优势 | 基础 |
| 34 | 协程 | 基础 |
| 35 | 协程/线程/进程区别 | 基础 |
| 36 | for range 地址变化 | 中等 |
| 37 | 高效拼接字符串 | 中等 |
| 38 | rune 类型 | 基础 |
| 39 | struct tag 作用 | 基础 |
| 40 | %v %+v %#v 区别 | 基础 |
| 41 | 空 struct{} 占空间 | 中等 |
| 42 | 空 struct{} 用途 | 中等 |
| 43 | init() 执行时机 | 基础 |
| 44 | 两个 interface 比较 | 中等 |
| 45 | 两个 nil 不相等 | 中等 |
| 46 | 函数传参 | 基础 |
| 47 | 栈分配 vs 堆分配 | 中等 |
| 48 | 多返回值实现 | 中等 |
| 49 | _ 的作用 | 基础 |
| 50 | 普通指针 vs unsafe.Pointer | 高级 |
| 51 | unsafe.Pointer vs uintptr | 高级 |

---

## 必考清单 (面试前 5 分钟)

```
🔴 必考:
  [x] GMP 调度模型 (#17)
  [x] channel 底层原理 (#8)
  [x] interface 内部结构 (#4, #44, #45)
  [x] context 机制 (#24)
  [x] error 处理哲学 (#4)
  [x] defer 执行顺序 (#3)

🟡 高频:
  [x] Mutex/RWMutex (#6, #20)
  [x] slice/map 底层 (#12)
  [x] GC 三色标记 (#10, #21)
  [x] 逃逸分析 (#11, #47)
  [x] graceful shutdown (#5)

🟢 基础:
  [x] make vs new (#1)
  [x] goroutine vs 线程 (#7, #35)
  [x] rune 类型 (#38)
  [x] struct tag (#39)
  [x] init() (#43)
  [x] _ 的作用 (#49)
```

---

## 快速 Review 命令

```bash
# 查看完整存档
cat docs/interview/FULL-ARCHIVE.md

# API 搜索
curl http://localhost:8088/api/knowledge/search?q=interface
curl http://localhost:8088/api/knowledge/search?q=mutex

# 面试前速览
cat docs/interview/00-experience-summary.md

# 按主题查
cat docs/interview/03-nil-interface-trap.md
cat docs/interview/01-lock-granularity.md
cat docs/interview/04-graceful-shutdown.md
```

---

## 文件结构

```
docs/interview/
├── README.md                    本文件（快速 Review）
├── FULL-ARCHIVE.md              完整存档（51 题）
├── 00-experience-summary.md     面试经验总结
├── 01-lock-granularity.md       锁粒度详解
├── 03-nil-interface-trap.md     nil interface 陷阱
├── 04-graceful-shutdown.md      HTTP 优雅关停
└── 面试题 与 答案.html          Claude 对话导出
```

---

> 生成模型: mimo-v2-pro (custom)
> 存档时间: 2026-05-03


## 模拟面试记录

| 日期 | 模式 | 文件 | 平均分 |
|------|------|------|--------|
| 2026-05-04 | Routine + LLM (Claude) | [routine-llm-session-20260504.md](routine-llm-session-20260504.md) | 22.3/100 |
