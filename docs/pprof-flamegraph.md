# pprof 火焰图：生成与阅读

本文说明如何在本库中开启 pprof、生成 CPU / 内存火焰图，以及如何解读结果。便于本地排查性能与内存问题。

---

## 1. 开启 pprof

本库通过 `ServeMux.Pprof()` 挂载标准库 `net/http/pprof`：

```go
mux := requests.NewServeMux(
    requests.URL("0.0.0.0:8080"),
)
mux.Pprof() // 注册 /debug/pprof/*

_ = mux.ListenAndServe()
```

挂载后可访问：

| 路径 | 用途 |
|------|------|
| `/debug/pprof/` | 索引页 |
| `/debug/pprof/profile` | CPU 采样（默认约 30s，会阻塞） |
| `/debug/pprof/heap` | 堆内存快照 |
| `/debug/pprof/allocs` | 累计分配采样 |
| `/debug/pprof/goroutine` | goroutine 栈 |
| `/debug/pprof/cmdline` / `symbol` / `trace` | 命令行、符号、执行追踪 |

需要记录 **GC** 时：

```bash
# 每次 GC 一行日志（不改代码）
GODEBUG=gctrace=1 ./your-app 2> gc.log

# 或看 GC 与请求时间线（需已 Pprof）
curl -o trace.out "http://localhost:8080/debug/pprof/trace?seconds=5"
go tool trace trace.out
```

**注意：** 生产环境慎开；`block` / `mutex` profile 需自行调用 `runtime.SetBlockProfileRate` / `runtime.SetMutexProfileFraction`。

---

## 2. 生成火焰图

依赖：已安装 Go toolchain。交互 UI 用 `go tool pprof -http=...` 即可，**火焰图不依赖 graphviz**。

采样期间请对服务加压（压测或真实流量），否则 CPU 图几乎为空。

### 2.1 CPU

```bash
# 采样 30 秒并打开浏览器（自动选端口）
go tool pprof -http=:0 "http://localhost:8080/debug/pprof/profile?seconds=30"

# 先落盘再打开（便于对比、归档）
curl -o cpu.pb.gz "http://localhost:8080/debug/pprof/profile?seconds=30"
go tool pprof -http=:0 cpu.pb.gz
```

浏览器中选择 **Flame Graph**（或 View → Flame Graph）。

### 2.2 内存（堆）

```bash
# 当前仍占用：找泄漏 / 长期持有（默认多为 inuse）
go tool pprof -http=:0 http://localhost:8080/debug/pprof/heap

# 累计分配：找分配热点 / GC 压力来源
go tool pprof -http=:0 "http://localhost:8080/debug/pprof/heap?sample=alloc_space"
# 或
go tool pprof -http=:0 http://localhost:8080/debug/pprof/allocs

# 落盘
curl -o heap.pb.gz "http://localhost:8080/debug/pprof/heap"
go tool pprof -http=:0 heap.pb.gz
```

在 pprof UI 顶部可切换样本类型：

| 类型 | 含义 |
|------|------|
| `inuse_space` / `inuse_objects` | 当前仍占用的字节 / 对象数 |
| `alloc_space` / `alloc_objects` | 累计分配的字节 / 对象数 |

### 2.3 无 HTTP 服务时（测试 / benchmark）

```bash
go test -bench=. -cpuprofile=cpu.out ./...
go tool pprof -http=:0 cpu.out

go test -bench=. -memprofile=mem.out ./...
go tool pprof -http=:0 mem.out
```

可按包缩小范围，例如只跑某个包的 benchmark：

```bash
go test -bench=BenchmarkXXX -cpuprofile=cpu.out ./path/to/pkg
```

---

## 3. 怎么读火焰图

### 3.1 共同约定

1. **横轴**：该层函数（及子调用）占采样的比例。越宽越值得先看。横轴**不是**时间顺序。
2. **纵轴**：调用栈深度。底部是入口（如 `main` / `runtime`），向上是被调用方。
3. **颜色**：多为按名字哈希上色，方便区分；一般**不表示**冷热。
4. **交互**：点击某框可 zoom 到该栈；搜索函数名可高亮。

口诀：**先找最宽的几块，再顺着栈落到自己的业务代码。**

### 3.2 CPU 火焰图

回答的问题：这段时间里 **CPU 在干什么**。

| 现象 | 可能含义 |
|------|----------|
| 顶部宽平台 | 叶子函数自己在算，或卡在底层库 / runtime |
| 中部某路径突然变宽 | 该调用链很热，继续下钻到业务函数 |
| 大量细碎尖峰 | 调用分散，或采样噪声 |
| `runtime.mallocgc` / `runtime.gcBgMark*` 很宽 | 分配 / GC 压力大 → 再看堆火焰图 |
| `runtime.lock*`、channel 相关很宽 | 锁或通信争用 |
| 业务函数在顶层且很宽 | 算法或循环本身贵 |

### 3.3 内存火焰图

| 样本 | 用途 |
|------|------|
| `alloc_*` | 谁**一共**分配得多 → 减临时对象、缓解 GC |
| `inuse_*` | 谁**还占着** → 查泄漏或只增不减的缓存 |

| 现象 | 可能含义 |
|------|----------|
| 业务路径上很宽 | 该路径分配或持有量大 |
| 仅 `alloc` 宽、`inuse` 不宽 | 分配多但很快释放 → 增加 GC，未必泄漏 |
| `inuse` 随时间单调变宽 | 更像泄漏或缓存只增不放 |
| `make` / `append` / `bytes.Buffer` / JSON 编解码很宽 | 典型临时对象热点 |

**与 CPU 对照：**

- CPU 上 GC 很宽 + heap `alloc` 很宽 → 优先减少分配。
- heap `inuse` 某路径很宽但 CPU 不热 → 内存被持有，未必在持续计算。

### 3.4 推荐排查顺序

1. 按宽度记下 top 3～5。
2. 跳过暂时改不了的纯标准库深处，落到本包 / 业务函数。
3. CPU：改算法、减锁、减系统调用；内存：减分配、复用 buffer、查是否只增不放。
4. 改完再采一版，对比同一函数是否变窄。

**大致健康 vs 可疑：**

- 健康：宽栈分散，业务宽度合理，runtime/GC 不是压倒性第一。
- 可疑：单一业务函数或单一分配路径独占大部分宽度；或 `inuse` 随时间单调上涨。

---

## 4. 常用命令速查

```bash
# 索引
curl -s http://localhost:8080/debug/pprof/ | head

# CPU 30s → 浏览器火焰图
go tool pprof -http=:0 "http://localhost:8080/debug/pprof/profile?seconds=30"

# 堆 inuse
go tool pprof -http=:0 http://localhost:8080/debug/pprof/heap

# 堆累计分配
go tool pprof -http=:0 http://localhost:8080/debug/pprof/allocs
```

---

## 5. 注意点

| 点 | 说明 |
|----|------|
| 加压 | CPU profile 采样窗口内必须有负载 |
| 阻塞 | `/debug/pprof/profile` 默认会阻塞数秒，自动化/健康检查勿随便打 |
| 端口 | `-http=:0` 自动选端口并尝试打开浏览器 |
| 生产 | 暴露进程与符号信息，建议鉴权或只绑内网 |
| 对比 | 优化前后用落盘的 `.pb.gz` / `.out` 对比更可靠 |

---

## 相关代码

- [`debug.go`](../debug.go)：`Pprof()` 实现
