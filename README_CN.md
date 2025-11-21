# Requests - 优雅的 Go HTTP 客户端和服务器库

<div align="center">

**Requests 是一个简单而优雅的 Go HTTP 客户端和服务器库，专为人类设计 ✨🍰✨**

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/golang-io/requests)
[![Go Reference](https://pkg.go.dev/badge/github.com/golang-io/requests.svg)](https://pkg.go.dev/github.com/golang-io/requests)
[![Apache V2 License](https://img.shields.io/badge/license-Apache%20V2-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Build Status](https://github.com/golang-io/requests/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/golang-io/requests/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/golang-io/requests)](https://goreportcard.com/report/github.com/golang-io/requests)
[![Sourcegraph](https://sourcegraph.com/github.com/golang-io/requests/-/badge.svg)](https://sourcegraph.com/github.com/golang-io/requests?badge)
[![codecov](https://codecov.io/gh/golang-io/requests/graph/badge.svg?token=T8MZ92JL1T)](https://codecov.io/gh/golang-io/requests)

[English](README.md) | 简体中文

</div>

---

## 📖 概述

Requests 受 Python 著名的 `requests` 库启发，为 Go 带来了更优雅、直观的 HTTP 体验。这个库简化了常见的 HTTP 任务，同时与 Go 标准库 `net/http` 完全兼容。

### ✨ 核心特性

- 🔒 **自动安全关闭** `resp.Body`（无需担心资源泄漏）
- 📦 **零外部依赖**（仅依赖 Go 标准库）
- 🌊 **流式下载支持**（高效处理大文件）
- 🔄 **分块 HTTP 请求**（支持流式上传）
- 🔗 **Keep-Alive 和连接池**（自动管理连接复用）
- 🍪 **持久化 Cookie 会话**（会话管理简单易用）
- 🔐 **基础和摘要认证**（内置认证支持）
- 🎯 **完全实现 http.RoundTripper**（与 `net/http` 完全兼容）
- 📁 **文件系统支持**（轻松上传和下载文件）
- 🔌 **中间件系统**（灵活的请求/响应处理链）
- 🖥️ **HTTP 服务器**（内置路由和中间件支持）
- 🎯 **路径参数支持**（支持 `:id` 和 `{id}` 两种语法，兼容 Gin、Echo 和 Go 1.22+）
- 🐛 **调试追踪**（内置 HTTP 请求追踪）

### 🎯 设计理念

```
简单 > 复杂
优雅 > 丑陋
明确 > 隐晦
实用 > 完美
```

---

## 📥 安装

```bash
go get github.com/golang-io/requests
```

**要求：** Go 1.22.1 或更高版本

---

## 🚀 快速开始

### Hello World

```go
package main

import (
    "context"
    "fmt"
    "github.com/golang-io/requests"
)

func main() {
    // 创建会话
    sess := requests.New(requests.URL("https://httpbin.org"))
    
    // 发送请求（自动处理 Body 关闭）
    resp, _ := sess.DoRequest(context.Background(), 
        requests.Path("/get"),
    )
    
    // 直接使用缓存的内容
    fmt.Println(resp.Content.String())
}
```

### 为什么选择 Requests？

**传统方式** (使用 `net/http`):
```go
resp, err := http.Get("https://api.example.com/users")
if err != nil {
    return err
}
defer resp.Body.Close() // 容易忘记！

body, err := io.ReadAll(resp.Body)
if err != nil {
    return err
}

var users []User
json.Unmarshal(body, &users) // 大量样板代码
```

**使用 Requests**:
```go
sess := requests.New(requests.URL("https://api.example.com"))
resp, _ := sess.DoRequest(ctx, requests.Path("/users"))

var users []User
resp.JSON(&users) // 简洁优雅！
```

---

## 📚 核心概念

### 1. 会话 (Session)

会话是 Requests 的核心概念，管理配置、连接池和中间件：

```go
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Header("Authorization", "Bearer token123"),
    requests.Timeout(30*time.Second),
    requests.MaxConns(100),
)

// 所有请求继承会话配置
resp1, _ := sess.DoRequest(ctx, requests.Path("/users"))
resp2, _ := sess.DoRequest(ctx, requests.Path("/posts"))
```

**特点：**
- ✅ 线程安全（可被多个 goroutine 并发使用）
- ✅ 连接复用（自动管理连接池）
- ✅ 配置持久化（会话级配置对所有请求生效）

### 2. 两级配置系统

Requests 支持灵活的两级配置：

```go
// 会话级配置（所有请求生效）
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Timeout(30*time.Second),
)

// 请求级配置（仅当前请求生效，可覆盖会话配置）
resp, _ := sess.DoRequest(ctx,
    requests.Path("/long-task"),
    requests.Timeout(60*time.Second), // 覆盖会话的 30 秒超时
)
```

### 3. 增强的 Response

Requests 提供增强的 `*Response` 类型：

```go
resp, _ := sess.DoRequest(ctx)

// 自动缓存的内容
fmt.Println(resp.Content.String())

// 解析 JSON
var data map[string]any
resp.JSON(&data)

// 请求统计
fmt.Printf("耗时: %v\n", resp.Cost)
stat := resp.Stat()
```

**优势：**
- ✅ 自动安全关闭 `resp.Body`
- ✅ 内容自动缓存到 `Content`
- ✅ 支持多次读取响应内容
- ✅ 记录请求耗时和统计信息

---

## 💡 使用示例

### GET 请求

```go
// 简单 GET
resp, _ := requests.Get("https://httpbin.org/get")

// 带查询参数
sess := requests.New(requests.URL("https://api.example.com"))
resp, _ := sess.DoRequest(ctx,
    requests.Path("/users"),
    requests.Params(map[string]string{
        "page": "1",
        "size": "20",
    }),
)
```

### POST 请求

```go
// POST JSON（自动序列化）
data := map[string]string{
    "name": "John",
    "email": "john@example.com",
}

resp, _ := sess.DoRequest(ctx,
    requests.MethodPost,
    requests.Path("/users"),
    requests.Body(data), // 自动序列化为 JSON
    requests.Header("Content-Type", "application/json"),
)

// POST 表单
form := url.Values{}
form.Set("username", "john")
form.Set("password", "secret")

resp, _ := sess.DoRequest(ctx,
    requests.MethodPost,
    requests.Form(form), // 自动设置 Content-Type
)
```

### 设置请求头

```go
sess := requests.New(
    requests.URL("https://api.example.com"),
    // 会话级请求头
    requests.Header("Accept", "application/json"),
    requests.Header("User-Agent", "MyApp/1.0"),
)

// 请求级请求头
resp, _ := sess.DoRequest(ctx,
    requests.Header("X-Request-ID", "abc-123"),
    requests.Headers(map[string]string{
        "X-Custom-1": "value1",
        "X-Custom-2": "value2",
    }),
)
```

### HTTP 认证

```go
// Basic Auth
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.BasicAuth("username", "password"),
)

// Bearer Token
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Header("Authorization", "Bearer token123"),
)
```

### 文件下载

```go
// 小文件：一次性读取
resp, _ := sess.DoRequest(ctx, requests.Path("/file.txt"))
os.WriteFile("downloaded.txt", resp.Content.Bytes(), 0644)

// 大文件：流式下载
file, _ := os.Create("large-file.zip")
defer file.Close()

resp, _ := sess.DoRequest(ctx,
    requests.Path("/large-file.zip"),
    requests.Stream(func(lineNum int64, data []byte) error {
        file.Write(data)
        return nil
    }),
)
```

### 超时控制

```go
// 会话级超时
sess := requests.New(
    requests.Timeout(10*time.Second),
)

// 请求级超时
resp, _ := sess.DoRequest(ctx,
    requests.Timeout(30*time.Second), // 覆盖会话配置
)

// 使用 Context 超时
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
resp, _ := sess.DoRequest(ctx)
```

### 代理设置

```go
// HTTP 代理
sess := requests.New(
    requests.Proxy("http://proxy.company.com:8080"),
)

// SOCKS5 代理
sess := requests.New(
    requests.Proxy("socks5://127.0.0.1:1080"),
)
```

### 自定义中间件

```go
// 请求 ID 中间件
requestIDMiddleware := func(next http.RoundTripper) http.RoundTripper {
    return requests.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
        req.Header.Set("X-Request-ID", uuid.New().String())
        return next.RoundTrip(req)
    })
}

sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Use(requestIDMiddleware),
)
```

### HTTP 服务器

```go
// 创建服务器
mux := requests.NewServeMux(
    requests.Addr("0.0.0.0:8080"),
    requests.Use(loggingMiddleware), // 全局中间件
)

// 注册路由
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, World!")
})

mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
    // 处理请求
}, requests.Use(authMiddleware)) // 路由级中间件

// 路径参数 - :id 语法（兼容 Gin、Echo 等框架）
mux.GET("/api/users/:id", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // 获取路径参数值
    fmt.Fprintf(w, "用户 ID: %s", id)
})

// 路径参数 - {id} 语法（兼容 Go 1.22+ 标准库）
mux.PUT("/api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // 获取路径参数值
    fmt.Fprintf(w, "文章 ID: %s", id)
})

// 启动服务器
requests.ListenAndServe(context.Background(), mux)
```

---

## 📊 功能对比

| 特性 | net/http | requests |
|------|----------|----------|
| 易用性 | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| 自动关闭 Body | ❌ | ✅ |
| 会话管理 | 手动 | 自动 |
| 连接池 | 需配置 | 内置 |
| JSON 支持 | 需手动处理 | 内置 |
| 中间件系统 | 需自己实现 | 内置 |
| 流式处理 | 需手动处理 | 内置 |
| 调试追踪 | 需额外工具 | 内置 |
| 服务器支持 | 基础 | 增强 |

---

## 🎓 进阶主题

### 路径参数

Requests 支持两种路径参数语法，提供灵活的路由功能：

**`:id` 语法**（兼容 Gin、Echo 等框架）：
```go
mux := requests.NewServeMux()

// 注册带 :id 参数的路由
mux.GET("/api/users/:id", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    fmt.Fprintf(w, "用户 ID: %s", id)
})

// 请求: GET /api/users/123
// 响应: "用户 ID: 123"
```

**`{id}` 语法**（兼容 Go 1.22+ 标准库）：
```go
mux := requests.NewServeMux()

// 注册带 {id} 参数的路由
mux.PUT("/api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    fmt.Fprintf(w, "文章 ID: %s", id)
})

// 请求: PUT /api/posts/456
// 响应: "文章 ID: 456"
```

**匹配规则：**
- 精确匹配优先于参数匹配
- 静态路径优先于参数路径
- 参数值自动提取，可通过 `r.PathValue(name)` 获取

**多参数示例：**
```go
mux.GET("/api/users/:userId/posts/:postId", func(w http.ResponseWriter, r *http.Request) {
    userId := r.PathValue("userId")
    postId := r.PathValue("postId")
    fmt.Fprintf(w, "用户: %s, 文章: %s", userId, postId)
})
```

### Unix Domain Socket

```go
sess := requests.New(
    requests.URL("unix:///var/run/docker.sock"),
)

resp, _ := sess.DoRequest(ctx,
    requests.URL("http://localhost/version"),
)
```

### 自定义传输层

```go
transport := &http.Transport{
    MaxIdleConns:        200,
    MaxIdleConnsPerHost: 100,
    IdleConnTimeout:     90 * time.Second,
}

sess := requests.New(
    requests.RoundTripper(transport),
)
```

### 调试和追踪

```go
sess := requests.New(
    requests.URL("https://httpbin.org"),
    requests.Trace(), // 启用详细追踪
)

resp, _ := sess.DoRequest(ctx)
// 输出会显示：DNS 解析、连接建立、TLS 握手、请求/响应详情
```

### 请求统计

```go
resp, _ := sess.DoRequest(ctx)

// 获取详细统计信息
stat := resp.Stat()
fmt.Printf("请求耗时: %dms\n", stat.Cost)
fmt.Printf("状态码: %d\n", stat.Response.StatusCode)
fmt.Printf("请求URL: %s\n", stat.Request.URL)
```

---

## 🌟 最佳实践

### 1. 使用会话管理连接

```go
// ✅ 推荐：创建一次，重复使用
var apiClient = requests.New(
    requests.URL("https://api.example.com"),
    requests.Timeout(30*time.Second),
)

// ❌ 不推荐：每次请求都创建新会话
func badExample() {
    sess := requests.New(...)  // 浪费资源
    sess.DoRequest(...)
}
```

### 2. 使用 DoRequest 而不是 Do

```go
// ✅ 推荐：DoRequest 自动处理 Body 关闭
resp, _ := sess.DoRequest(ctx)
fmt.Println(resp.Content.String()) // 安全

// ❌ 不推荐：需要手动关闭 Body
resp, _ := sess.Do(ctx)
defer resp.Body.Close() // 容易忘记
```

### 3. 利用配置继承

```go
// ✅ 推荐：会话级配置 + 请求级覆盖
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Timeout(10*time.Second), // 默认 10 秒
)

// 大部分请求使用默认配置
resp1, _ := sess.DoRequest(ctx)

// 特殊请求覆盖配置
resp2, _ := sess.DoRequest(ctx,
    requests.Timeout(60*time.Second), // 特殊任务 60 秒
)
```

### 4. 使用中间件处理通用逻辑

```go
// ✅ 推荐：使用中间件
sess := requests.New(
    requests.Use(
        requestIDMiddleware,   // 添加请求 ID
        retryMiddleware,       // 自动重试
        loggingMiddleware,     // 日志记录
    ),
)

// ❌ 不推荐：每次请求重复代码
func badExample() {
    req.Header.Set("X-Request-ID", ...)  // 重复代码
    // 手动重试逻辑
    // 手动日志记录
}
```

### 5. 错误处理

```go
// ✅ 推荐：完整的错误处理
resp, err := sess.DoRequest(ctx)
if err != nil {
    log.Printf("请求失败: %v", err)
    return err
}

if resp.StatusCode != http.StatusOK {
    log.Printf("HTTP 错误: %d", resp.StatusCode)
    return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}

// ❌ 不推荐：忽略错误
resp, _ := sess.DoRequest(ctx) // 忽略错误
```

---

## 📖 完整示例

### 构建 REST API 客户端

```go
package main

import (
    "context"
    "fmt"
    "github.com/golang-io/requests"
    "time"
)

type APIClient struct {
    sess *requests.Session
}

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func NewAPIClient(baseURL, token string) *APIClient {
    sess := requests.New(
        requests.URL(baseURL),
        requests.Header("Authorization", "Bearer "+token),
        requests.Header("Accept", "application/json"),
        requests.Header("Content-Type", "application/json"),
        requests.Timeout(30*time.Second),
        requests.MaxConns(100),
    )
    
    return &APIClient{sess: sess}
}

func (c *APIClient) GetUser(ctx context.Context, userID int) (*User, error) {
    resp, err := c.sess.DoRequest(ctx,
        requests.Path(fmt.Sprintf("/users/%d", userID)),
    )
    if err != nil {
        return nil, err
    }
    
    if resp.StatusCode != 200 {
        return nil, fmt.Errorf("API 错误: %d", resp.StatusCode)
    }
    
    var user User
    if err := resp.JSON(&user); err != nil {
        return nil, err
    }
    
    return &user, nil
}

func (c *APIClient) CreateUser(ctx context.Context, user *User) (*User, error) {
    resp, err := c.sess.DoRequest(ctx,
        requests.MethodPost,
        requests.Path("/users"),
        requests.Body(user),
    )
    if err != nil {
        return nil, err
    }
    
    var created User
    resp.JSON(&created)
    return &created, nil
}

func main() {
    client := NewAPIClient("https://api.example.com", "your-token")
    
    user, _ := client.GetUser(context.Background(), 123)
    fmt.Printf("用户: %s\n", user.Name)
}
```

---

## 🔧 配置选项速查表

### 客户端配置

| 选项 | 说明 | 示例 |
|-----|------|------|
| `URL(string)` | 设置目标URL | `requests.URL("https://api.example.com")` |
| `Path(string)` | 追加路径 | `requests.Path("/users")` |
| `Method(string)` | 设置HTTP方法 | `requests.MethodPost` |
| `Timeout(duration)` | 设置超时时间 | `requests.Timeout(30*time.Second)` |
| `Header(k, v)` | 添加请求头 | `requests.Header("Accept", "application/json")` |
| `BasicAuth(user, pass)` | 基础认证 | `requests.BasicAuth("admin", "secret")` |
| `Body(any)` | 设置请求体 | `requests.Body(map[string]string{"key": "value"})` |
| `Form(values)` | 表单数据 | `requests.Form(url.Values{...})` |
| `Params(map)` | 查询参数 | `requests.Params(map[string]string{...})` |
| `Proxy(addr)` | 设置代理 | `requests.Proxy("http://proxy:8080")` |
| `MaxConns(int)` | 最大连接数 | `requests.MaxConns(100)` |
| `Verify(bool)` | 验证证书 | `requests.Verify(false)` |

### 服务器配置

| 选项 | 说明 | 示例 |
|-----|------|------|
| `Use(middleware...)` | 注册中间件 | `requests.Use(loggingMiddleware)` |
| `CertKey(cert, key)` | TLS证书 | `requests.CertKey("cert.pem", "key.pem")` |
| `OnStart(func)` | 启动回调 | `requests.OnStart(func(s *http.Server){...})` |
| `OnShutdown(func)` | 关闭回调 | `requests.OnShutdown(func(s *http.Server){...})` |

---

## 🤝 贡献

我们欢迎各种形式的贡献！

- 🐛 报告 Bug
- 💡 提出新功能
- 📖 改进文档
- 🔧 提交 Pull Request

请查看 [贡献指南](CONTRIBUTING.md) 了解详情。

---

## 📄 许可证

本项目采用 [Apache License 2.0](LICENSE) 许可证。

---

## 🙏 致谢

- 受 Python [requests](https://github.com/psf/requests) 库启发
- 感谢所有贡献者

---

## 📚 更多资源

- [API 文档](https://pkg.go.dev/github.com/golang-io/requests)
- [GitHub 仓库](https://github.com/golang-io/requests)
- [问题反馈](https://github.com/golang-io/requests/issues)
- [讨论区](https://github.com/golang-io/requests/discussions)

---

<div align="center">

**如果这个项目对你有帮助，请给我们一个 ⭐ Star！**

Made with ❤️ by the Requests Team

</div>
