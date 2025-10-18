# HTTP/3 (QUIC) 使用示例

## 概述

本库已完全支持 HTTP/3 协议，基于 QUIC 传输层。HTTP/3 相比传统 HTTP/1.1 和 HTTP/2 具有以下优势：

- ⚡ **更快的连接建立**：0-RTT 或 1-RTT 握手，减少延迟
- 🔒 **内置加密**：强制使用 TLS 1.3，更安全
- 🚀 **多路复用无阻塞**：解决队头阻塞问题
- 🔄 **连接迁移**：支持网络切换时保持连接
- 📦 **基于 UDP**：更好的移动网络性能

## HTTP/3 客户端示例

### 基础用法

```go
package main

import (
    "context"
    "fmt"
    "github.com/golang-io/requests"
)

func main() {
    // 创建启用 HTTP/3 的客户端
    sess := requests.New(
        requests.URL("https://cloudflare-quic.com"),
        requests.EnableHTTP3(true),
        requests.Timeout(10*time.Second),
    )
    
    // 发送请求
    resp, err := sess.DoRequest(context.TODO())
    if err != nil {
        fmt.Printf("请求失败: %v\n", err)
        return
    }
    
    fmt.Printf("状态码: %d\n", resp.Response.StatusCode)
    fmt.Printf("协议版本: %s\n", resp.Response.Proto)
    fmt.Printf("响应内容: %s\n", resp.Content.String())
}
```

### 带参数的请求

```go
package main

import (
    "context"
    "fmt"
    "github.com/golang-io/requests"
    "time"
)

func main() {
    // 创建客户端
    sess := requests.New(
        requests.URL("https://api.example.com"),
        requests.EnableHTTP3(true),
        requests.Header("User-Agent", "MyApp/1.0"),
        requests.Timeout(30*time.Second),
    )
    
    // GET 请求
    resp, _ := sess.DoRequest(
        context.TODO(),
        requests.Path("/api/v1/users"),
        requests.Param("page", "1"),
        requests.Param("limit", "10"),
    )
    fmt.Println("GET 响应:", resp.Content.String())
    
    // POST 请求
    resp, _ = sess.DoRequest(
        context.TODO(),
        requests.MethodPost,
        requests.Path("/api/v1/users"),
        requests.JSON(map[string]interface{}{
            "name": "张三",
            "age": 25,
        }),
    )
    fmt.Println("POST 响应:", resp.Content.String())
}
```

### 跳过证书验证（测试环境）

```go
package main

import (
    "context"
    "github.com/golang-io/requests"
)

func main() {
    // 在测试环境中跳过证书验证
    sess := requests.New(
        requests.URL("https://localhost:8443"),
        requests.EnableHTTP3(true),
        requests.Verify(false),  // ⚠️ 仅用于测试环境
    )
    
    resp, _ := sess.DoRequest(context.TODO())
    println(resp.Content.String())
}
```

## HTTP/3 服务器示例

### 基础服务器

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "github.com/golang-io/requests"
)

func main() {
    // 创建路由
    mux := requests.NewServeMux()
    
    mux.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "pong\n")
    })
    
    mux.Route("/hello", func(w http.ResponseWriter, r *http.Request) {
        name := r.URL.Query().Get("name")
        fmt.Fprintf(w, "你好, %s!\n", name)
    })
    
    // 启动 HTTP/3 服务器
    // 注意：需要提供有效的 TLS 证书和密钥
    ctx := context.Background()
    err := requests.ListenAndServeHTTP3(
        ctx,
        mux,
        requests.URL(":8443"),
        requests.CertKey("server.crt", "server.key"),
    )
    if err != nil {
        fmt.Printf("服务器错误: %v\n", err)
    }
}
```

### RESTful API 服务器

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "github.com/golang-io/requests"
)

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    Age  int    `json:"age"`
}

func main() {
    mux := requests.NewServeMux()
    
    // 获取用户列表
    mux.GET("/api/users", func(w http.ResponseWriter, r *http.Request) {
        users := []User{
            {ID: 1, Name: "张三", Age: 25},
            {ID: 2, Name: "李四", Age: 30},
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(users)
    })
    
    // 创建用户
    mux.POST("/api/users", func(w http.ResponseWriter, r *http.Request) {
        var user User
        if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        user.ID = 3 // 模拟分配 ID
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        json.NewEncoder(w).Encode(user)
    })
    
    // 启动服务器
    ctx := context.Background()
    fmt.Println("HTTP/3 服务器启动在 :8443")
    requests.ListenAndServeHTTP3(
        ctx,
        mux,
        requests.URL(":8443"),
        requests.CertKey("server.crt", "server.key"),
    )
}
```

### 带中间件的服务器

```go
package main

import (
    "context"
    "fmt"
    "log"
    "net/http"
    "time"
    "github.com/golang-io/requests"
)

// 日志中间件
func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("[%s] %s - %v", r.Method, r.URL.Path, time.Since(start))
    })
}

// 认证中间件
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, "未授权", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}

func main() {
    mux := requests.NewServeMux(
        requests.Use(loggingMiddleware),
    )
    
    mux.Route("/public", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "公开内容\n")
    })
    
    mux.Route("/private", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "私有内容\n")
    }, requests.Use(authMiddleware))
    
    ctx := context.Background()
    requests.ListenAndServeHTTP3(
        ctx,
        mux,
        requests.URL(":8443"),
        requests.CertKey("server.crt", "server.key"),
    )
}
```

## 生成测试证书

在开发和测试环境中，你可以使用以下命令生成自签名证书：

```bash
# 生成私钥
openssl ecparam -genkey -name prime256v1 -out server.key

# 生成证书签名请求
openssl req -new -key server.key -out server.csr \
    -subj "/C=CN/ST=Beijing/L=Beijing/O=Test/CN=localhost"

# 生成自签名证书
openssl x509 -req -days 365 -in server.csr -signkey server.key \
    -out server.crt -extfile <(echo "subjectAltName=DNS:localhost,IP:127.0.0.1")
```

## 性能对比测试

```go
package main

import (
    "context"
    "fmt"
    "testing"
    "time"
    "github.com/golang-io/requests"
)

func BenchmarkHTTP3(b *testing.B) {
    sess := requests.New(
        requests.URL("https://localhost:8443"),
        requests.EnableHTTP3(true),
        requests.Verify(false),
    )
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        sess.DoRequest(context.TODO(), requests.Path("/ping"))
    }
}

func BenchmarkHTTP2(b *testing.B) {
    sess := requests.New(
        requests.URL("https://localhost:8443"),
        requests.Verify(false),
    )
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        sess.DoRequest(context.TODO(), requests.Path("/ping"))
    }
}
```

## 注意事项

### 客户端

- ✅ 自动使用 HTTP/3，无需额外配置
- ✅ 自动回退到 HTTP/2 或 HTTP/1.1（如果服务器不支持）
- ⚠️ 必须使用 HTTPS（HTTP/3 强制加密）
- ⚠️ 某些防火墙可能阻止 UDP 流量

### 服务端

- ⚠️ 必须提供有效的 TLS 证书和密钥
- ⚠️ 监听 UDP 端口，而非 TCP
- ⚠️ 需要防火墙允许 UDP 流量
- ✅ 默认端口 443（HTTPS）

## 常见问题

### Q: 如何验证是否真的使用了 HTTP/3？

A: 检查响应的协议版本：

```go
resp, _ := sess.DoRequest(context.TODO())
fmt.Printf("协议: %s\n", resp.Response.Proto)  // 应该显示 "HTTP/3.0"
```

### Q: 为什么客户端连接失败？

A: 可能的原因：
1. 服务器没有启用 HTTP/3
2. 防火墙阻止了 UDP 流量
3. 证书验证失败（可以用 `Verify(false)` 跳过）

### Q: 可以同时支持 HTTP/2 和 HTTP/3 吗？

A: 可以！客户端会自动协商最佳协议。服务端需要分别启动：

```go
// HTTP/2 服务器 (TCP)
go requests.ListenAndServe(ctx, mux,
    requests.URL(":443"),
    requests.CertKey("cert.pem", "key.pem"),
)

// HTTP/3 服务器 (UDP)
go requests.ListenAndServeHTTP3(ctx, mux,
    requests.URL(":443"),
    requests.CertKey("cert.pem", "key.pem"),
)
```

## 更多资源

- [HTTP/3 RFC 9114](https://www.rfc-editor.org/rfc/rfc9114.html)
- [QUIC RFC 9000](https://www.rfc-editor.org/rfc/rfc9000.html)
- [quic-go 文档](https://github.com/quic-go/quic-go)

