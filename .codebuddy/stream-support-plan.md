# requests 库流式（Stream）支持改造计划

> 目标：让 `requests` 丝滑支持流式响应，并保持与标准库 `net/http` **完全相同的使用体验**，唯一差异是**不需要 `defer resp.Body.Close()`**。
>
> 本文档是改造工作的唯一依据。**每一项由用户逐条下达指令后才执行，未指定的项一律不动。**
>
> 基线信息：
> - 分析时间：2026-09-08（2026-09-09 补充 A5 / C6 / C7 / S9 / D2）
> - 分支：`dev`
> - `go.mod`：`go 1.22.1`（→ `context.AfterFunc` 可用；`runtime.AddCleanup`(1.24)、`iter.Seq`(1.23) 不可用）
> - 核对依据：本地 Go 1.27.1 标准库源码 + 本仓库全链路源码 + 探针实测（探针跑完即删，未留在仓库）

---

## 0. 目标状态

```go
resp, err := sess.DoRequest(ctx, requests.URL(u), requests.Stream())
if err != nil { return err }

// resp.Body 是「活」的流，可直接 Read / bufio.Scanner / json.Decoder
// 无需 defer resp.Body.Close()
sc := bufio.NewScanner(resp.Body)
for sc.Scan() { ... }
```

即 **pull 模型（拉）+ 兜底自动关闭**。当前库是 **push 模型（回调）+ 强制全量缓存**，两者在语义上互斥。

> 注意：「支持流式」必须**客户端能收、服务端能推**两边都通。客户端侧的阻断项是 A2/A3，服务端侧是 **A5**。

---

## 1. 进度总表

| 编号 | 级别 | 简述 | 优先级 | 状态 |
|---|---|---|---|---|
| A1 | 阻断 | `DoRequest` 无条件吞掉整个 Body | P0 | ☑ 已完成 |
| A2 | 阻断 | 客户端默认 `Timeout: 30s` 砍断长流 | P0 | ☐ 未开始（用户暂缓） |
| A3 | 阻断 | 请求级 `Timeout()` 完全无效（既有 bug，已实测） | P0 | ☐ 未开始（用户暂缓） |
| A4 | 阻断 | `Stream()` 是 push 模型，与标准库相反 | P0 | ☑ 已完成 |
| **A5** | **阻断** | **服务端 `WriteTimeout` 砍断自己的 SSE 推流（已实测）** | **P0** | ☐ 未开始（用户暂缓） |
| B1 | 正确性 | `streamRoundTrip` 错误路径泄漏 Body/连接 | P0 | ☐ 未开始 |
| B2 | 正确性 | `Logf` 会吞掉流（隐蔽杀手） | P0 | ☑ 已完成 |
| B3 | 正确性 | `ContentLength` 语义被污染 | P2 | ☐ 未开始 |
| B4 | 正确性 | `ReadBytes('\n')` 无上界，可 OOM | P2 | ☐ 未开始 |
| B5 | 正确性 | `streamRoundTrip` 丢弃自建 Response 的统计 | P2 | ☐ 未开始 |
| C1 | 体验 | `DisableCompression: true` 偏离标准库 | P2 | ☐ 未开始 |
| C2 | 体验 | 缺首字节超时 / 流空闲超时（客户端） | P2 | ☐ 未开始（用户暂缓） |
| C3 | 体验 | 请求侧流式上传缺 `ContentLength` / `GetBody` 控制 | P2 | ☐ 未开始 |
| C4 | 体验 | 缺客户端 SSE 解析器；现有解析不符规范 | P1 | ☐ 未开始 |
| C5 | 体验 | `isStreaming` 靠 Content-Type 猜，不可靠 | P1 | ☑ 已完成 |
| **C6** | **体验** | **服务端未设 `ReadHeaderTimeout`/`IdleTimeout`（放开 Timeout 后成敞口）** | **P1** | ☐ 未开始（用户暂缓） |
| **C7** | **体验** | **`Timeout(0)` 三方语义不一致；`server_test.go` 有名不副实的用例** | **P2** | ☐ 未开始（用户暂缓） |
| **C8** | **体验** | **`resp.Body` 编译歧义，无法像标准库那样使用（改造中新发现）** | **P0** | ☑ 已完成 |
| S1 | 方案 | `Stream()` 改造为开关（向后兼容） | P0 | ☑ 已完成 |
| S2 | 方案 | 流式标记穿透中间件（context 传递） | P0 | ☑ 已完成 |
| S5 | 方案 | 自动关闭三层保险（实现「免 defer close」） | P1 | ☑ 已完成 |
| S7 | 方案 | 客户端 SSE `EventScanner` | P1 | ☐ 未开始 |
| S8 | 方案 | 请求侧 `ContentLength()` / `BodyFunc()` Option | P2 | ☐ 未开始 |
| **S9** | **方案** | **拆分客户端/服务端 Timeout Option（只增不改）** | **P1** | ☐ 未开始（用户暂缓） |
| D1 | 决策 | 客户端 Timeout 重做走方案 X / Y / Y′ / Z | — | ⏳ 暂缓 |
| **D2** | **决策** | **是否拆分 Timeout Option（S9）** | — | ⏳ 暂缓 |

状态图例：☐ 未开始 / ◐ 进行中 / ☑ 已完成 / ✗ 已放弃

---

## 2. 阻断级问题（不改则流式根本做不到）

### A1 `DoRequest` 无条件吞掉整个 Body

**位置**：`session.go:239-247`

```go
// 自动读取并缓存响应内容，然后安全关闭 Body
defer resp.Response.Body.Close()
_, resp.Err = resp.Content.ReadFrom(resp.Response.Body)

// 重新包装 Body，使其仍然可读
resp.Response.Body = io.NopCloser(bytes.NewReader(resp.Content.Bytes()))
return resp, resp.Err
```

**问题**：`Content.ReadFrom` 会阻塞到流结束。SSE / LLM 流式接口下这里死等到服务端关连接，流式完全不可用。

**根因**：`DoRequest` 的「自动缓存 + 自动关闭」卖点与流式语义互斥，必须有开关分叉。

**修复方向**：
```go
if options.Stream {
    resp.Response.Body = newAutoCloseBody(ctx, resp.Response.Body)
    return resp, nil   // 不读 Body，Content 保持空
}
```
并在 `Response` 上加 `IsStream bool` 标记。

**附带**：`Response.String()` / `Response.JSON()` 在 stream 模式下会拿到空 buffer，用户一头雾水。建议 `String()` 在 stream 模式返回明确提示文案而非空串。

---

### A2 客户端默认 `Timeout: 30s` 会砍断长流 —— 最隐蔽的致命项

**位置**：`options.go:117-128`（默认值） + `session.go:73-79`（固化进 Client）

```go
// options.go
opt := Options{
    URL:         "http://127.0.0.1:80",
    ...
    Timeout:     30 * time.Second,
```

```go
// session.go
func New(opts ...Option) *Session {
    options := newOptions(opts)
    transport := newTransport(opts...)
    client := &http.Client{Timeout: options.Timeout, Transport: transport}
```

**问题**：标准库 `http.Client.Timeout` 的计时**包含 Body 读取全过程**（这是它与 `Transport` 各种细粒度 timeout 的关键区别）。因此任何超过 30s 的流，`resp.Body.Read` 会在第 30 秒返回 `context deadline exceeded`。

标准库 `http.Client{}` 默认 `Timeout: 0`（不限）。**这是与「完全相同体验」最大的偏离，且用户几乎不可能自行排查出来。**

**修复方向**：见 §6 方案 4 / 决策项 D1。另见 §3 客户端与服务端 Timeout 语义对照，以及对称问题 A5。

---

### A3 请求级 `Timeout()` 完全无效（既有 bug，导致 A2 无解）

**位置**：`session.go:266-283`

```go
func (s *Session) RoundTripper(opts ...Option) http.RoundTripper {
    return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
        options := newOptions(s.opts, opts...)
        if options.Transport == nil {
            options.Transport = RoundTripperFunc(s.client.Do)
        }
```

**问题**：`s.client` 在 `New()` 时已固化 `Timeout`。请求级传 `requests.Timeout(0)` 只改了 `options.Timeout`，**没有任何地方消费它**。用户**没有办法**在单次请求上关掉那 30s。

**实测**（服务端固定 sleep 300ms，探针已删）：

| 用例 | 期望 | 实际 |
|---|---|---|
| 会话默认 30s ＋ 请求级 `Timeout(50ms)` | 50ms 超时 | ❌ `err=nil`，耗时 **316ms** |
| 会话级 `Timeout(50ms)`（对照组） | 50ms 超时 | ✅ 53ms 超时 |
| 会话级 50ms ＋ 请求级 `Timeout(5s)` | 成功 | ❌ 仍在 **51ms** 被砍 |
| `sess.HTTPClient(Timeout(50ms))` | `.Timeout=50ms` | ❌ `.Timeout=30s` |

→ 请求级 `Timeout()` **双向失效**：收紧无效、放宽也无效。

**根因链**（4 步）：
1. `Timeout()` 只写 `Options.Timeout`（`options.go:557`），本身无问题；
2. `New()` 把它**快照**进 `s.client`（`session.go:76`），此后永久冻结；
3. `RoundTripper()` 里**确实**重新合并出了带请求级覆盖的 `options`，但闭包内**没有任何一行引用 `options.Timeout`**，最终执行的仍是冻结的 `s.client`；
4. `HTTPClient()`（`session.go:94`）同样直接读 `s.client.Timeout`，而非合并后的 `options`。

**全库确认**：客户端 HTTP 路径上 `options.Timeout` 的消费点**只有 `session.go:76` 一处**（`socket.go:104` 是裸 TCP 的 `Socket()`，`server.go:736` 是服务端，均与客户端请求无关）。即 `Options.Timeout` 对客户端而言是「只在 `New()` 时被读一次的死字段」。

**文档与实现直接矛盾**（不只是「限制」）：注释在两处明确承诺请求级可覆盖 —— `options.go:40-45`、`options.go:552-556`。对比之下 `ConnTimeout`（`options.go:575`）和 `URL(unix://)`（`transport.go:108-117`）都老实交代了「仅会话级生效」。只有 `Timeout` 是**明确承诺、静默失效**。

**与 A2 的耦合（关键）**：`s.client.Do` 位于中间件链的**最内层**，30s 硬上限就在那里 ——

```
用户中间件 → ... → RoundTripperFunc(s.client.Do) → s.transport
                    ↑ 超时在此，外层中间件无法绕过
```

于是流式形成死局：请求级放宽被 A3 挡住；套中间件绕不过（超时在最内层）；唯一出路是新建 `Session`，但连接池也会跟着重建。**所以 A3 必须先于 A2 修。**

**附带设计问题**：`RoundTripperFunc(s.client.Do)` 把 `Client.Do` 当 `RoundTripper` 用，违反契约（`RoundTripper` 不应处理重定向/认证/Cookie）。四个后果：
1. 中间件看不到重定向的每一跳（内层已跟完跳转），与 `Trace` 基于 `httptrace` 的单次 round-trip 定位自相矛盾；
2. **Client 套 Client**：`HTTPClient()` 返回的 Client 内层又是 `s.client.Do`；用户写 `&http.Client{Transport: sess}`（`Session` 自身实现了 `RoundTrip`，`session.go:122`）同样双层——两套重定向策略、两个 Timeout 叠加；
3. 错误被多包一层 `*url.Error`（实测输出 `Get "http://...": context deadline exceeded` 即此格式），违反 `RoundTrip` 应返回原始错误的契约；
4. `s.client` 未设 `Jar`/`CheckRedirect`，实际是「默认重定向 + 无 Cookie」，与 `Session` 名字暗示的会话语义有落差。

**修复方向**：拆成两个风险级别不同的层次。

**A3-a（浅层，低风险）—— 让请求级 `Timeout` 真正生效**：

```go
if options.Transport == nil {
    if options.Timeout == s.client.Timeout {
        options.Transport = RoundTripperFunc(s.client.Do)      // 未覆盖，复用，行为不变
    } else {
        options.Transport = RoundTripperFunc((&http.Client{
            Timeout:       options.Timeout,
            Transport:     s.transport,          // 共享连接池，无额外开销
            CheckRedirect: s.client.CheckRedirect,
            Jar:           s.client.Jar,
        }).Do)
    }
}
```

两个**必须注意**的点：
- **`CheckRedirect` 和 `Jar` 一定要一起带过去**，否则会出现「一旦请求级设了 Timeout，重定向策略和 Cookie 就悄悄丢了」这种更隐蔽的 bug；
- `http.Client` 是轻量结构体，连接池挂在 `Transport` 上（共享 `s.transport`），每次请求 new 一个不影响性能。

同时 `HTTPClient()` 改为读 `newOptions(s.opts, opts...).Timeout`。

**A3-b（深层，高风险）—— 修正 RoundTripper 契约**：不再用 `Client.Do` 当 `RoundTripper`，改为直连 `s.transport`，把重定向/Cookie/超时统一上移到 `Do`/`DoRequest` 层。会改变中间件可见范围（能看到每一跳），属行为变更，建议独立评估。**会影响 B2/C5 中间件改造的落点，宜早定。**

---

### A4 `Stream()` 是 push 模型，与标准库相反

**位置**：`setup.go:84-109`、`options.go:664-668`、`response.go:174-214`

```go
// setup.go:102-105
// 使用 http.NoBody 表示 Body 已被流式处理，没有可读内容
resp.Response.Body = http.NoBody
```

**问题**：用户拿不到 `resp.Body`，只能写 `func(int64, []byte) error` 回调。要做到「和标准库相同体验」，`Body` 必须保持是活的可读流。

**修复方向**：见 §5 方案 1（`Stream()` 改造为开关，同时向后兼容旧回调签名）。

---

### A5 服务端 `WriteTimeout` 砍断自己的 SSE 推流

**位置**：`server.go:734-736`

```go
// 设置超时
// Set timeouts
s.server.ReadTimeout, s.server.WriteTimeout = s.options.Timeout, s.options.Timeout
```

**问题**：标准库对 `WriteTimeout` 的定义（`net/http/server.go:3080-3085`）——

> *WriteTimeout is the maximum duration before timing out writes of the response. **It is reset whenever a new request's header is read.***

SSE 是**一个请求推 N 个事件**，整个流期间没有新请求头，因此 **`WriteTimeout` 从不重置**，退化为「整个 SSE 连接的寿命上限」。

**实测**（服务端 `Timeout(800ms)`，每 100ms 推一条，共 20 条；客户端用裸标准库排除干扰。探针已删）：

```
服务端配置: Timeout(800ms) => ReadTimeout=800ms WriteTimeout=800ms
  [客户端] 收到第 1 条 (t=0s): data:event-0
  ...
  [客户端] 收到第 8 条 (t=710ms): data:event-7
  [客户端] 读取中断 t=910ms: unexpected EOF
  结论: 期望收到 20 条(约2s)，实际收到 8 条
  [服务端] 第 10 次推送失败: write tcp 127.0.0.1:58231->127.0.0.1:58301: i/o timeout
```

**影响**：本库自带 `requests.SSE()` 中间件（`use.go:179`）主打流式推送，而 `NewServer` 默认 `Timeout` 为 30s（`options.go:121`），于是——

> **用 `requests.SSE()` 搭的服务端，任何超过 30 秒的事件流都会被自己的 `WriteTimeout` 砍断，客户端收到 `unexpected EOF`。**

比客户端 A2 更隐蔽的三个原因：
1. **无法在 handler 内绕过** —— 标准库明说 *"it does not let Handlers make decisions on a per-request basis"*；
2. 错误只出现在服务端 `Write` 的返回值里，而 `SSE()` 中间件的 `defer sse.End()`（`use.go:183`）忽略了 `Send` 的返回值，**用户很可能完全看不到错误**；
3. 客户端侧表现为 `unexpected EOF`，像网络抖动，极难定位到是自己服务端的配置。

**与 A2 完全对称**：

| | 客户端 | 服务端 |
|---|---|---|
| 字段 | `Client.Timeout` | `Server.WriteTimeout` |
| 默认值 | 30s（`options.go:121`） | 30s（**同一个字段**） |
| 后果 | 读流 30s 后 `context deadline exceeded` | 推流 30s 后 `i/o timeout` + 断连 |
| 编号 | A2 | **A5** |

**修复方向**：
- 流式路由需 `WriteTimeout = 0`，并用 `ReadHeaderTimeout` 保底（见 C6）；
- ⚠️ `WriteTimeout` **无法按 handler 区分**。若同一 Server 上既有普通接口又有 SSE，只能整体放开 `WriteTimeout`，再用 `http.ResponseController.SetWriteDeadline`（Go 1.20+，`go.mod` 1.22 可用）在**非流式 handler 内**逐个收紧。这是标准库目前唯一干净的解法；
- 配套：`SSE()` 中间件应处理 `Send` 的返回值，让写失败可见。

---

## 3. 客户端 vs 服务端 Timeout 语义对照

本库 `requests.Timeout(d)` 只写一个字段 `Options.Timeout`，却被**两个语义完全不同的地方**消费：

```go
// session.go:76  —— 客户端
client := &http.Client{Timeout: options.Timeout, Transport: transport}
```
```go
// server.go:736  —— 服务端
s.server.ReadTimeout, s.server.WriteTimeout = s.options.Timeout, s.options.Timeout
```

同一个 `Timeout(30s)`，在客户端是「一次请求的总时长」，在服务端是「一条连接读/写各自的 deadline」——**计时起点、覆盖范围、超时后果全都不同**。

| 维度 | 客户端 `Client.Timeout` | 服务端 `ReadTimeout` / `WriteTimeout` |
|---|---|---|
| 计时对象 | **一次逻辑请求**（含所有重定向跳转） | **一条 TCP 连接上的单次读/写阶段** |
| 计时起点 | `Client.Do` 调用瞬间 | 每读到一个新请求头就**重置** |
| 覆盖范围 | DNS + 拨号 + TLS + 写请求 + 等响应 + **读完 Body** | Read：读完整请求（含 body）<br>Write：写完整响应 |
| 实现机制 | 内部 `context.WithDeadline` | `conn.SetReadDeadline` / `SetWriteDeadline`（OS 层 socket deadline） |
| 超时表现 | `resp.Body.Read` 返回 `context deadline exceeded` | 读/写系统调用返回 `i/o timeout`，连接被断 |
| 是否可分阶段 | ❌ 一刀切 | ✅ 可细分 `ReadHeaderTimeout` / `IdleTimeout` |
| `0` 的含义 | 不限 | 不限（标准库：*A zero or negative value means there will be no timeout*） |

一句话：
- **客户端 Timeout = 一次请求的总预算**，跨越整个 round trip 生命周期；
- **服务端 Timeout = 一条连接上单个 I/O 阶段的墙钟 deadline**，每个新请求重新计时。

**关键共性：两者都包含 body 传输阶段，所以两者都会砍断流式** —— 这正是 A2（客户端）与 A5（服务端）的共同成因。Timeout 语义混用是本次流式改造的**共同根因**。

---

## 4. 正确性级问题（真 bug）

### B1 `streamRoundTrip` 错误路径泄漏 Body/连接

**位置**：`setup.go:92-96`

```go
if resp.Response.ContentLength, resp.Err = streamRead(r.Context(), resp.Response.Body, fn); resp.Err != nil {
    return resp.Response, resp.Err
}
```

**问题**：ctx 取消或回调返错时直接 return，`Body` 从未 `Close()`。标准库 `transfer.go` 的 `body.Close()` 是归还/丢弃 `persistConn` 的唯一入口，不调用则该连接被 `readLoop` 永久占用 → 连接池慢性耗尽。

**修复方向**：改为 `defer resp.Response.Body.Close()`（`Close` 幂等，安全），之后再替换为 `http.NoBody`。

---

### B2 `Logf` 会吞掉流（隐蔽杀手）

**位置**：`setup.go:32-41`（`printRoundTripper`）→ `response.go:140-142`（`Stat()`）→ `stat.go:204-211`（`responseLoad`）

```go
// stat.go:204-211
if resp.Response != nil {
    var err error
    if resp.Content == nil || resp.Content.Len() == 0 {
        if resp.Content, resp.Response.Body, err = CopyBody(resp.Response.Body); err != nil {
```

**问题**：只要开了 `Logf`，**所有响应体都被全量读入内存再重放**。流式场景下：
1. 阻塞到流结束（等价于流式失效）；
2. 内存无上界。

`trace.go` 已有的 `isStreaming` 保护**没有覆盖 `Logf` 路径**：

```go
// trace.go:304-307 —— 只有 trace 做了保护
if isStreaming(resp) {
    Log("< [Streaming Response - Body not traced to preserve real-time streaming]")
    return resp, nil
}
```

**修复方向**：`printRoundTripper` 与 `responseLoad` 都必须先判断「是否流式」再决定是否 `CopyBody`；判断以显式标记为主（见方案 2），`isStreaming` 仅作补充（见 C5）。

---

### B3 `ContentLength` 语义被污染

**位置**：`setup.go:94`

把 `streamRead` 返回的「实际读到字节数」写入 `resp.Response.ContentLength`。该字段语义是「服务端声明的长度」，`-1` 表示未知。覆盖后下游全部失真，包括：

```go
// stat.go:219-221
if stat.Response.ContentLength == -1 && resp.Content.Len() != 0 {
    stat.Response.ContentLength = int64(resp.Content.Len())
}
```

**修复方向**：不覆盖 `ContentLength`，实际字节数放到独立字段（如 `Response.StreamBytes`）。

---

### B4 `streamRead` 的 `ReadBytes('\n')` 无上界，可 OOM

**位置**：`response.go:189`

```go
raw, err1 := r.ReadBytes(10)
```

**问题**：`bufio.Reader.ReadBytes` 会持续 append 直到遇到 `\n`。无换行的流（二进制文件、恶意/异常流）会导致内存 unbounded 增长。

**修复方向**：换 `bufio.Scanner` + 显式 `scanner.Buffer(buf, max)`，或给 `streamRead` 增加 `maxLineSize` 上界参数并在超限时返回错误。

---

### B5 `streamRoundTrip` 丢弃自建 Response 的统计

**位置**：`setup.go:86-106`

`newResponse(r)` 造出的 `StartAt` / `Cost` / `Err` 全部作废（函数返回的是内层 `resp.Response`，不是 `*requests.Response`）。

**修复方向**：要么保留并回传统计，要么直接删掉无用的 `newResponse` 调用，避免误导。

---

## 5. 体验级差异

### C1 `DisableCompression: true` 偏离标准库

**位置**：`transport.go:138`

标准库 `Transport` 默认会自动加 `Accept-Encoding: gzip` 并**透明解压**。此处关闭后，服务端返回 gzip 时用户拿到的是压缩字节。流式场景尤其容易踩（很多 SSE 网关会 gzip）。

**注意**：这可能是刻意设计（注释写「由应用层控制」）。改动前需确认是否为有意行为。

---

### C2 缺首字节超时 / 流空闲超时

**位置**：`transport.go:98-146`

流式需要的不是「总超时」，而是：
- `ResponseHeaderTimeout`：首字节（响应头）超时；
- **流空闲超时**：两次成功 `Read` 之间的最大间隔。

两者当前都没有。

**修复方向**：新增 `ResponseHeaderTimeout(d)` Option；`StreamIdleTimeout(d)` 用 Body wrapper + timer 实现。

---

### C3 请求侧流式上传缺 `ContentLength` / `GetBody` 控制

**位置**：`request.go:46-73`（`makeBody`）

```go
case io.Reader, io.ReadSeeker, *bytes.Reader, *strings.Reader:
    return body.(io.Reader), nil
...
case func() (io.ReadCloser, error):
    return v()          // ← 只调一次就丢了，本该同时装到 req.GetBody
```

**问题链**（对照标准库 `request.go:933-962`）：
- 标准库 `NewRequest` 只对 `*bytes.Buffer` / `*bytes.Reader` / `*strings.Reader` 自动推断 `ContentLength` 并设置 `GetBody`；
- 其他 `io.Reader` 一律 `ContentLength = 0` → `outgoingLength()` 翻成 `-1`（未知）；
- POST/PUT → 走 chunked；
- GET/HEAD/DELETE 等 → 触发 `probeRequestBody()`，最多**阻塞 200ms** 偷读一字节（标准库 `transfer.go:216-247`）；
- `GetBody == nil` → 307/308 重定向与连接重试**无法重放 body**。

**修复方向**：
```go
func ContentLength(n int64) Option                       // 显式声明，避免 chunked / 200ms probe
func BodyFunc(f func() (io.ReadCloser, error)) Option    // 同时装到 req.GetBody，支持重放
```
对 `*os.File` / `io.ReadSeeker` 可 `Seek` 推断长度（务必 seek 回原位）。

---

### C4 缺客户端 SSE 解析器；现有解析不符规范

**位置**：`use.go:127-147`（`ServerSentEvents.Read`）

```go
name, value, _ := bytes.Cut(bytes.TrimRight(b, "\n"), []byte(":"))
```

**问题**：
1. 该方法挂在**服务端**类型 `ServerSentEvents` 上，客户端拿不到；
2. 不剥 `": "` 后的**一个**前导空格（SSE 规范要求）；
3. 不做多行 `data:` 拼接（规范要求用 `\n` 连接）；
4. 只处理单行，不以空行为事件边界；
5. 不处理 `\r\n` / `\r` 换行。

**修复方向**：见方案 7。

---

### C5 `isStreaming` 靠 Content-Type 猜，不可靠

**位置**：`trace.go:340-369`

漏判场景：
- `application/octet-stream` 的大文件下载；
- `application/json` 的 chunked 长流；
- 任何自定义 Content-Type 的流。

误判场景：`text/plain` + chunked 的普通响应会被当成流。

**修复方向**：**以显式的 `options.Stream` 标记为主判据**（方案 2），`isStreaming` 降级为补充启发式。

---

### C6 服务端未设 `ReadHeaderTimeout` / `IdleTimeout`

**位置**：`server.go:736`（只设了 Read/Write 两项）

标准库明确建议优先使用 `ReadHeaderTimeout`（`net/http/server.go:3066-3070`）：

> *Because ReadTimeout does not let Handlers make per-request decisions on each request body's acceptable deadline or upload rate, most users will prefer to use ReadHeaderTimeout. It is valid to use them both.*

**当前不是裸奔**：`IdleTimeout` 为 0 时回落到 `ReadTimeout`（30s），`ReadHeaderTimeout` 同理。

**但这是 A5 的连带风险**：一旦为支持流式把 `Timeout` 设为 0，**`ReadHeaderTimeout` 和 `IdleTimeout` 会一起变成「不限」**，此时才真正出现 Slowloris 慢速攻击敞口。

**修复方向**：修 A5 时必须同步显式设置 `ReadHeaderTimeout`（建议 10~30s）与 `IdleTimeout`，不能依赖回落。

---

### C7 `Timeout(0)` 三方语义不一致 + 测试用例名不副实

**语义分歧**：
- 标准库（客户端与服务端）：`0` = **不限**；
- 本库客户端：`newOptions` 默认 30s（`options.go:121`），用户不传即 30s；
- 本库服务端测试用例把 `0` 当**「使用默认值」**：

```go
// server_test.go:1123-1128
{
    name:          "零超时设置",
    timeout:       0,                // 显式设置为0
    expectedRead:  30 * time.Second, // 实际行为：使用默认值
    expectedWrite: 30 * time.Second, // 实际行为：使用默认值
```

**但实现（`server.go:736`）是直接赋值**，`Timeout(0)` 传进去就是 `0`（不限）。该用例的注释与实现不符 —— 它靠 `if tt.timeout > 0` 跳过了 0 的分支才没暴露（`server_test.go:1164`）。

**为什么现在重要**：`Timeout(0)` 正是流式场景「关掉总超时」的入口（A2/A5 都要用），语义必须先钉死。

**修复方向**：客户端与服务端统一定义 `Timeout(0)` = **不限**，与标准库对齐；修正 `server_test.go:1123` 那条名不副实的用例；在 `Timeout()` 注释中写清客户端/服务端语义差异（对照表见 §3）。

---

## 6. 改造方案

### 方案 1（S1）：`Stream()` 改造为开关，向后兼容

Go 无函数重载，用可变参数保住已发布 API：

```go
func Stream(fn ...func(int64, []byte) error) Option
```

- `Stream()` 无参 → `o.Stream = true`，走新的 pull 模式（Body 保持活流）；
- `Stream(fn)` → 保持旧 push 行为（内部转 `streamRoundTrip`），行为完全不变。

同时 `Options` 增加字段：`Stream bool`。

---

### 方案 2（S2）：流式标记穿透中间件

中间件签名只有 `*http.Request`，拿不到 `Options`。用 request context 传标记：

```go
type streamKey struct{}

// NewRequestWithContext 中：
// if options.Stream { ctx = context.WithValue(ctx, streamKey{}, true) }

func isStreamRequest(r *http.Request) bool
```

然后以下三处一律先判 `isStreamRequest(r) || isStreaming(resp)` 再决定是否 `CopyBody`：
- `setup.go` → `printRoundTripper`
- `stat.go` → `responseLoad`
- `trace.go` → `traceLv`（已有 `isStreaming`，补显式标记）

---

### 方案 3（S3）：`DoRequest` 分叉

```go
if options.Stream {
    resp.Response.Body = newAutoCloseBody(ctx, resp.Response.Body)
    resp.IsStream = true
    return resp, nil   // 不读 Body
}
```

---

### 方案 4（S4）：客户端 Timeout 重做 —— **最关键，见决策项 D1**

#### 方案 X（推荐）
`Client.Timeout` 永久置 0，超时统一改由 `context.WithTimeout` 在 `Do` / `DoRequest` 层施加。

- 优点：语义干净；请求级天然生效（顺手修掉 A3）；流式/非流式统一；不再滥用 `Client.Do` 当 `RoundTripper`。
- 风险：对已依赖 `Timeout()` 的用户是行为变更 —— 错误类型从 `*url.Error{Timeout:true}` 变为 `context.DeadlineExceeded`（`errors.Is` 判断基本兼容，但 `err.(*url.Error).Timeout()` 这类断言会失效）。

#### 方案 Y（保守）
只在 stream 模式下绕过 `Client` 直连 `s.transport`：

```go
if options.Stream {
    options.Transport = s.transport                     // 绕过 Client.Timeout
} else if options.Timeout != s.client.Timeout {
    options.Transport = RoundTripperFunc((&http.Client{
        Timeout: options.Timeout, Transport: s.transport,
    }).Do)
} else {
    options.Transport = RoundTripperFunc(s.client.Do)
}
```

- 优点：改动小。
- 风险：绕过 `Client` 会**同时丢掉重定向跟随和 CookieJar**；A3 的请求级 bug 仅部分缓解。

**配套（两方案通用）**：新增 `ResponseHeaderTimeout(d)`、`StreamIdleTimeout(d)`。

---

### 方案 5（S5）：自动关闭三层保险 —— 实现「免 defer close」

`go.mod` 为 `go 1.22.1`：`context.AfterFunc`（1.21+）**可用**；`runtime.AddCleanup`（1.24）**不可用**，只能用 `runtime.SetFinalizer`。

```go
type autoCloseBody struct {
    rc   io.ReadCloser
    once sync.Once
    stop func()   // context.AfterFunc 返回的取消句柄
}
```

三层：
1. **主**：`stop := context.AfterFunc(ctx, b.Close)` —— 用户写 `defer cancel()`（用 context 时本来就要写）即等于关闭 Body。这是 `defer resp.Body.Close()` 最干净的替代品。
2. **辅**：`Read` 返回 `io.EOF` 时自动 `Close()` —— 立刻归还连接。标准库 `body.Close()` 在 `sawEOF` 后是 no-op，完全安全。
3. **兜底**：`runtime.SetFinalizer` 打警告日志 + `Close()`，防彻底泄漏。**finalizer 必须挂在独立的小 wrapper 上，不能存在循环引用，否则永不触发。**

⚠️ **必须写进文档的语义边界**：若用户传 `context.Background()`（不可取消），第 1 层保险失效，只剩 EOF 与 finalizer。因此 stream 模式应在文档明确「必须传可取消的 ctx」，并考虑在 `Stream()` 且 ctx 不可取消时输出 warning。

---

### 方案 7（S7）：客户端 SSE `EventScanner`

1.22 无 range-over-func，采用 `Scanner` 风格：

```go
type Event struct {
    ID    string
    Event string
    Data  string
    Retry int
}

type EventScanner struct{ /* ... */ }

func NewEventScanner(r io.Reader) *EventScanner
func (s *EventScanner) Scan() bool
func (s *EventScanner) Event() Event
func (s *EventScanner) Err() error
```

必须正确处理（当前 `use.go:128` 这几条全不满足）：
- 多行 `data:` 用 `\n` 拼接；
- `:` 后剥掉**一个**空格；
- **空行**才表示一个事件完成；
- 以 `:` 开头的行是注释，忽略；
- `\r\n` / `\r` / `\n` 三种换行都要支持；
- 设置行缓冲上界，防 OOM。

---

### 方案 8（S8）：请求侧 Option 补齐

```go
func ContentLength(n int64) Option
func BodyFunc(f func() (io.ReadCloser, error)) Option
```

并让 `makeBody` 对 `*os.File` / `io.ReadSeeker` 推断长度（seek 后复位）。

---

### 方案 9（S9）：拆分客户端/服务端 Timeout Option

**动机**：Timeout 语义混用是 A2/A3/A5/C6/C7 的共同根因（详见 §3 对照表）。一个 `Timeout()` 同时驱动 `Client.Timeout` 和 `Server.Read/WriteTimeout`，导致「为支持流式放开服务端写超时」会连带影响客户端，反之亦然。

**做法（只增不改，`Timeout()` 现有行为保持）**：

```go
Timeout(d)              // 保留，客户端总超时（兼容）
ReadTimeout(d)          // 服务端
WriteTimeout(d)         // 服务端
ReadHeaderTimeout(d)    // 服务端，防 Slowloris（配合 C6）
IdleTimeout(d)          // 服务端 keep-alive
```

`Options` 相应新增字段；`server.go:736` 改为优先取专用字段，未设置时回落到 `Timeout` 以保持兼容。

**服务端流式的正解**：`WriteTimeout = 0` + `ReadHeaderTimeout` 保底。因 `WriteTimeout` 无法按 handler 区分，同一 Server 上混合普通接口与 SSE 时，只能整体放开 `WriteTimeout`，再用 `http.ResponseController.SetWriteDeadline` 在非流式 handler 内逐个收紧。

**兼容性影响**：纯新增 API，无破坏。唯一行为变化来自 C7（`Timeout(0)` 统一为「不限」），需与 D2 一并决策。

---

## 7. 回归测试清单（改造过程中逐步补齐）

| 用例 | 验证目标 | 状态 |
|---|---|---|
| 41 秒长流不被客户端 30s 砍断 | A2 / A3 | ☐ |
| 请求级 `Timeout` 双向生效（收紧＋放宽），且不丢 `Jar`/`CheckRedirect` | A3-a | ☐ |
| **SSE 推流 40 秒不被服务端 `WriteTimeout` 砍断** | **A5** | ☐ |
| **`SSE().Send` 写失败时错误可见（不被静默吞掉）** | **A5** | ☐ |
| 开启 `Logf` 时流不被吞（首个事件应在 ~即时抵达） | B2 | ☐ |
| 开启 `Trace` 时流不被吞 | C5 / S2 | ☐ |
| `ctx cancel` 后 Body 自动关闭且连接归还池 | S5 第 1 层 | ☐ |
| 读到 EOF 后连接立即可复用（对比 `Transport` 空闲连接数） | S5 第 2 层 | ☐ |
| 用户完全不 Close 也不 cancel 时不永久泄漏 | S5 第 3 层 | ☐ |
| `Stream(fn)` 旧回调行为不变（向后兼容） | S1 | ☐ |
| `streamRead` 遇超长无换行流返回错误而非 OOM | B4 | ☐ |
| SSE 多行 data / `": "` 空格 / `\r\n` / 注释行解析正确 | S7 | ☐ |
| 流式上传显式 `ContentLength` 不触发 chunked 与 200ms probe | C3 / S8 | ☐ |
| **`WriteTimeout=0` 时 `ReadHeaderTimeout` 仍生效（Slowloris 防护）** | **C6** | ☐ |
| **`Timeout(0)` 在客户端与服务端均表示「不限」** | **C7** | ☐ |

---

## 8. 待用户决策

### D1：客户端 Timeout 重做走哪个方案？
- **方案 X（推荐）**：`Client.Timeout=0` + 超时统一交给 context。语义干净、顺手修掉 A3、流式非流式统一；代价是超时错误类型变更。
- **方案 Y（保守）**：仅 stream 模式绕过 `Client`。改动小；代价是丢重定向与 CookieJar，且 A3 仅部分缓解。

> 状态：⏳ 未决策
>
> 关联子决策：A3-b（是否修正 `RoundTripperFunc(s.client.Do)` 违反 RoundTripper 契约的问题）。该项会影响 B2/C5 中间件改造的落点，**宜早定**。

### D2：是否拆分 Timeout Option（S9）？
- **拆分**：新增 `ReadTimeout` / `WriteTimeout` / `ReadHeaderTimeout` / `IdleTimeout`，只增不改。可从根上解开 A2/A5 的耦合。
- **不拆分**：继续共用 `Timeout`，则服务端为支持流式放开写超时时，无法与客户端超时独立配置。

> 状态：⏳ 未决策

---

## 9. 执行约定

- 用户逐条下达指令（如「处理 A2」「处理 S5」），**未指定的项一律不动**。
- 每完成一项：更新 §1 进度总表状态，必要时更新 §7 测试清单。
- 涉及公开 API 变更或行为变更的，先在对应条目下补「兼容性影响」小节，再动手。

---

## 10. 变更记录

### 2026-09-09：流式返回链路（A1 / A4 / B2 / C5 / C8 / S1 / S2 / S5）

**目标**：实现流式返回的行为一致性 —— 与标准库相同的使用体验，且 stream 模式不在内存中缓存数据。超时相关项（A2/A3/A5/C2/C6/C7/S9/D1/D2）按用户要求暂缓。

**改动文件**：

| 文件 | 改动 |
|---|---|
| `stream.go` | **新增**。`streamContextKey` / `withStream` / `isStreamRequest` / `autoCloseBody` |
| `options.go` | `Stream()` 改为可变参数开关；`Options` 新增 `Stream`、`streamPush` 字段 |
| `request.go` | `NewRequestWithContext` 注入流式 Context 标记 |
| `session.go` | `DoRequest` 分叉：流式不读 Body；defer 统一同步 `resp.Body` |
| `response.go` | `Response` 新增 `Body`、`IsStream` 字段；`String()`/`JSON()` 流式下给明确提示 |
| `setup.go` | `printRoundTripper` 标记流式，避免 `Stat()` 读 Body |
| `stat.go` | `responseLoad` 流式时跳过 `CopyBody` |
| `trace.go` | `traceLv` 优先使用显式标记，`isStreaming` 降为补充 |
| `stream_test.go` | **新增**。9 个验证用例 |

**关键设计**：

1. **推/拉双模式**（S1）：`Stream()` 无参为拉模式（新，Body 保持活流）；`Stream(fn)` 为推模式（旧行为，通过 `streamPush` 内部字段隔离，确保完全向后兼容）。
2. **Context 标记穿透**（S2）：中间件签名只有 `*http.Request`，通过 Context 传递流式标记，使 `Logf`/`Trace` 能跳过 `CopyBody`。
3. **三层自动关闭**（S5）：Context 取消 → 读到 EOF/错误 → GC finalizer 兜底。`context.AfterFunc` 的回调只捕获底层 `rc` 而非 `autoCloseBody`，否则 Context 会持有引用导致 finalizer 永不触发。

**改造中新发现的问题（C8）**：

`Response` 同时嵌入 `*http.Request` 和 `*http.Response`，两者都有 `Body` 字段，导致 `resp.Body` 触发 `ambiguous selector` 编译错误，用户被迫写 `resp.Response.Body`。这直接违背「与标准库相同体验」的目标。解决：在 `Response` 上显式声明 `Body io.ReadCloser` 字段遮蔽嵌入字段，并在 `DoRequest` 的 defer 中统一同步，覆盖所有返回路径（含错误路径）。

**实测结果**：

| 验证项 | 结果 |
|---|---|
| 增量投递（总时长 1s 的流） | 首条 **2~10ms** 到达，全部读完 1.02s |
| 开启 `Logf` 不缓存 | ✅ `stat.Response.Body="(streaming: body not buffered)"` |
| 开启 `Trace` 不缓存 | ✅ |
| ctx cancel 自动关闭 | ✅ `context canceled` |
| EOF 后连接复用 | ✅ 3 次流式请求仅拨号 **1 次** |
| GC finalizer 兜底 | ✅ 触发并关闭底层 Body |
| 推模式向后兼容 | ✅ 行为不变 |
| 非流式行为不变 | ✅ Content 缓存、JSON、Body 重放均正常 |

全量回归 `go test ./` 通过（53s）；`go vet` 通过；`go test -race` 通过。

**遗留**：
- **B1**（`streamRoundTrip` 错误路径 Body 泄漏）仍未修 —— 属推模式路径，未获指令，未改动。
- 流式下 `Response.Cost` 仅表示「拿到响应头」的耗时，不含流消费时长（符合流式语义，但需在 README 说明）。
