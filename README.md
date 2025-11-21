# Requests - Elegant HTTP Client and Server Library for Go

<div align="center">

**Requests is a simple, yet elegant, Go HTTP client and server library for Humans™ ✨🍰✨**

[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/golang-io/requests)
[![Go Reference](https://pkg.go.dev/badge/github.com/golang-io/requests.svg)](https://pkg.go.dev/github.com/golang-io/requests)
[![Apache V2 License](https://img.shields.io/badge/license-Apache%20V2-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Build Status](https://github.com/golang-io/requests/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/golang-io/requests/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/golang-io/requests)](https://goreportcard.com/report/github.com/golang-io/requests)
[![Sourcegraph](https://sourcegraph.com/github.com/golang-io/requests/-/badge.svg)](https://sourcegraph.com/github.com/golang-io/requests?badge)
[![codecov](https://codecov.io/gh/golang-io/requests/graph/badge.svg?token=T8MZ92JL1T)](https://codecov.io/gh/golang-io/requests)

English | [简体中文](README_CN.md)

</div>

---

## 📖 Overview

Requests is inspired by Python's famous `requests` library, bringing a more elegant and intuitive approach to HTTP in Go. This library simplifies common HTTP tasks while remaining fully compatible with Go's standard `net/http` library.

### ✨ Key Features

- 🔒 **Automatic Safe Body Close** - No more `resp.Body.Close()` concerns
- 📦 **Zero External Dependencies** - Only depends on Go standard library
- 🌊 **Streaming Downloads** - Efficient handling of large files
- 🔄 **Chunked HTTP Requests** - Support for streaming uploads
- 🔗 **Keep-Alive & Connection Pooling** - Automatic connection reuse management
- 🍪 **Sessions with Cookie Persistence** - Easy session management
- 🔐 **Basic & Digest Authentication** - Built-in authentication support
- 🎯 **Full http.RoundTripper Implementation** - Fully compatible with `net/http`
- 📁 **File System Support** - Easy file upload and download
- 🔌 **Middleware System** - Flexible request/response processing chain
- 🖥️ **HTTP Server** - Built-in routing and middleware support
- 🎯 **Path Parameters** - Support for `:id` and `{id}` syntax (compatible with Gin, Echo, and Go 1.22+)
- 🐛 **Debug Tracing** - Built-in HTTP request tracing

### 🎯 Design Philosophy

```
Simple is better than complex
Beautiful is better than ugly
Explicit is better than implicit
Practical beats purity
```

---

## 📥 Installation

```bash
go get github.com/golang-io/requests
```

**Requirements:** Go 1.22.1 or higher

---

## 🚀 Quick Start

### Hello World

```go
package main

import (
    "context"
    "fmt"
    "github.com/golang-io/requests"
)

func main() {
    // Create a session
    sess := requests.New(requests.URL("https://httpbin.org"))
    
    // Send request (Body is automatically closed)
    resp, _ := sess.DoRequest(context.Background(), 
        requests.Path("/get"),
    )
    
    // Content is automatically cached
    fmt.Println(resp.Content.String())
}
```

### Why Requests?

**Traditional way** (using `net/http`):
```go
resp, err := http.Get("https://api.example.com/users")
if err != nil {
    return err
}
defer resp.Body.Close() // Easy to forget!

body, err := io.ReadAll(resp.Body)
if err != nil {
    return err
}

var users []User
json.Unmarshal(body, &users) // Lots of boilerplate
```

**With Requests**:
```go
sess := requests.New(requests.URL("https://api.example.com"))
resp, _ := sess.DoRequest(ctx, requests.Path("/users"))

var users []User
resp.JSON(&users) // Clean and elegant!
```

---

## 📚 Core Concepts

### 1. Session

Session is the core concept in Requests, managing configuration, connection pools, and middleware:

```go
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Header("Authorization", "Bearer token123"),
    requests.Timeout(30*time.Second),
    requests.MaxConns(100),
)

// All requests inherit session configuration
resp1, _ := sess.DoRequest(ctx, requests.Path("/users"))
resp2, _ := sess.DoRequest(ctx, requests.Path("/posts"))
```

**Features:**
- ✅ Thread-safe (can be used concurrently by multiple goroutines)
- ✅ Connection reuse (automatic connection pool management)
- ✅ Configuration persistence (session-level config applies to all requests)

### 2. Two-Level Configuration System

Requests supports a flexible two-level configuration system:

```go
// Session-level configuration (applies to all requests)
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Timeout(30*time.Second),
)

// Request-level configuration (applies only to this request, can override session config)
resp, _ := sess.DoRequest(ctx,
    requests.Path("/long-task"),
    requests.Timeout(60*time.Second), // Override session's 30-second timeout
)
```

### 3. Enhanced Response

Requests provides an enhanced `*Response` type:

```go
resp, _ := sess.DoRequest(ctx)

// Automatically cached content
fmt.Println(resp.Content.String())

// Parse JSON
var data map[string]any
resp.JSON(&data)

// Request statistics
fmt.Printf("Duration: %v\n", resp.Cost)
stat := resp.Stat()
```

**Benefits:**
- ✅ Automatic safe closing of `resp.Body`
- ✅ Content automatically cached in `Content`
- ✅ Support for multiple reads of response content
- ✅ Request timing and statistics tracking

---

## 💡 Usage Examples

### GET Requests

```go
// Simple GET
resp, _ := requests.Get("https://httpbin.org/get")

// GET with query parameters
sess := requests.New(requests.URL("https://api.example.com"))
resp, _ := sess.DoRequest(ctx,
    requests.Path("/users"),
    requests.Params(map[string]string{
        "page": "1",
        "size": "20",
    }),
)
```

### POST Requests

```go
// POST JSON (automatically serialized)
data := map[string]string{
    "name": "John",
    "email": "john@example.com",
}

resp, _ := sess.DoRequest(ctx,
    requests.MethodPost,
    requests.Path("/users"),
    requests.Body(data), // Automatically serialized to JSON
    requests.Header("Content-Type", "application/json"),
)

// POST form data
form := url.Values{}
form.Set("username", "john")
form.Set("password", "secret")

resp, _ := sess.DoRequest(ctx,
    requests.MethodPost,
    requests.Form(form), // Automatically sets Content-Type
)
```

### Setting Headers

```go
sess := requests.New(
    requests.URL("https://api.example.com"),
    // Session-level headers
    requests.Header("Accept", "application/json"),
    requests.Header("User-Agent", "MyApp/1.0"),
)

// Request-level headers
resp, _ := sess.DoRequest(ctx,
    requests.Header("X-Request-ID", "abc-123"),
    requests.Headers(map[string]string{
        "X-Custom-1": "value1",
        "X-Custom-2": "value2",
    }),
)
```

### Authentication

```go
// Basic Authentication
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

### File Download

```go
// Small file: one-time read
resp, _ := sess.DoRequest(ctx, requests.Path("/file.txt"))
os.WriteFile("downloaded.txt", resp.Content.Bytes(), 0644)

// Large file: streaming download
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

### Timeout Control

```go
// Session-level timeout
sess := requests.New(
    requests.Timeout(10*time.Second),
)

// Request-level timeout
resp, _ := sess.DoRequest(ctx,
    requests.Timeout(30*time.Second), // Override session config
)

// Context-based timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
resp, _ := sess.DoRequest(ctx)
```

### Proxy Settings

```go
// HTTP proxy
sess := requests.New(
    requests.Proxy("http://proxy.company.com:8080"),
)

// SOCKS5 proxy
sess := requests.New(
    requests.Proxy("socks5://127.0.0.1:1080"),
)
```

### Custom Middleware

```go
// Request ID middleware
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

### HTTP Server

```go
// Create server
mux := requests.NewServeMux(
    requests.Addr("0.0.0.0:8080"),
    requests.Use(loggingMiddleware), // Global middleware
)

// Register routes
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    fmt.Fprintf(w, "Hello, World!")
})

mux.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
    // Handle request
}, requests.Use(authMiddleware)) // Route-specific middleware

// Path parameters with :id syntax (compatible with Gin, Echo, etc.)
mux.GET("/api/users/:id", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // Get path parameter value
    fmt.Fprintf(w, "User ID: %s", id)
})

// Path parameters with {id} syntax (compatible with Go 1.22+ standard library)
mux.PUT("/api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id") // Get path parameter value
    fmt.Fprintf(w, "Post ID: %s", id)
})

// Start server
requests.ListenAndServe(context.Background(), mux)
```

---

## 📊 Feature Comparison

| Feature | net/http | requests |
|---------|----------|----------|
| Ease of Use | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| Auto Body Close | ❌ | ✅ |
| Session Management | Manual | Automatic |
| Connection Pool | Need Configuration | Built-in |
| JSON Support | Manual | Built-in |
| Middleware System | DIY | Built-in |
| Streaming | Manual | Built-in |
| Debug Tracing | External Tools | Built-in |
| Server Support | Basic | Enhanced |

---

## 🎓 Advanced Topics

### Path Parameters

Requests supports two path parameter syntaxes for flexible routing:

**`:id` syntax** (compatible with Gin, Echo, etc.):
```go
mux := requests.NewServeMux()

// Register route with :id parameter
mux.GET("/api/users/:id", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    fmt.Fprintf(w, "User ID: %s", id)
})

// Request: GET /api/users/123
// Response: "User ID: 123"
```

**`{id}` syntax** (compatible with Go 1.22+ standard library):
```go
mux := requests.NewServeMux()

// Register route with {id} parameter
mux.PUT("/api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    fmt.Fprintf(w, "Post ID: %s", id)
})

// Request: PUT /api/posts/456
// Response: "Post ID: 456"
```

**Matching Rules:**
- Exact match takes priority over parameter match
- Static paths are matched before parameter paths
- Parameters are automatically extracted and available via `r.PathValue(name)`

**Example with multiple parameters:**
```go
mux.GET("/api/users/:userId/posts/:postId", func(w http.ResponseWriter, r *http.Request) {
    userId := r.PathValue("userId")
    postId := r.PathValue("postId")
    fmt.Fprintf(w, "User: %s, Post: %s", userId, postId)
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

### Custom Transport

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

### Debug and Tracing

```go
sess := requests.New(
    requests.URL("https://httpbin.org"),
    requests.Trace(), // Enable detailed tracing
)

resp, _ := sess.DoRequest(ctx)
// Output shows: DNS resolution, connection establishment, TLS handshake, request/response details
```

### Request Statistics

```go
resp, _ := sess.DoRequest(ctx)

// Get detailed statistics
stat := resp.Stat()
fmt.Printf("Request duration: %dms\n", stat.Cost)
fmt.Printf("Status code: %d\n", stat.Response.StatusCode)
fmt.Printf("Request URL: %s\n", stat.Request.URL)
```

---

## 🌟 Best Practices

### 1. Use Sessions for Connection Management

```go
// ✅ Recommended: Create once, reuse many times
var apiClient = requests.New(
    requests.URL("https://api.example.com"),
    requests.Timeout(30*time.Second),
)

// ❌ Not recommended: Create new session for each request
func badExample() {
    sess := requests.New(...)  // Waste of resources
    sess.DoRequest(...)
}
```

### 2. Use DoRequest Instead of Do

```go
// ✅ Recommended: DoRequest automatically handles Body closing
resp, _ := sess.DoRequest(ctx)
fmt.Println(resp.Content.String()) // Safe

// ❌ Not recommended: Need to manually close Body
resp, _ := sess.Do(ctx)
defer resp.Body.Close() // Easy to forget
```

### 3. Leverage Configuration Inheritance

```go
// ✅ Recommended: Session-level config + request-level override
sess := requests.New(
    requests.URL("https://api.example.com"),
    requests.Timeout(10*time.Second), // Default 10 seconds
)

// Most requests use default config
resp1, _ := sess.DoRequest(ctx)

// Special requests override config
resp2, _ := sess.DoRequest(ctx,
    requests.Timeout(60*time.Second), // Special task needs 60 seconds
)
```

### 4. Use Middleware for Common Logic

```go
// ✅ Recommended: Use middleware
sess := requests.New(
    requests.Use(
        requestIDMiddleware,   // Add request ID
        retryMiddleware,       // Auto retry
        loggingMiddleware,     // Logging
    ),
)

// ❌ Not recommended: Repeat code in every request
func badExample() {
    req.Header.Set("X-Request-ID", ...)  // Repeated code
    // Manual retry logic
    // Manual logging
}
```

### 5. Error Handling

```go
// ✅ Recommended: Complete error handling
resp, err := sess.DoRequest(ctx)
if err != nil {
    log.Printf("Request failed: %v", err)
    return err
}

if resp.StatusCode != http.StatusOK {
    log.Printf("HTTP error: %d", resp.StatusCode)
    return fmt.Errorf("unexpected status: %d", resp.StatusCode)
}

// ❌ Not recommended: Ignore errors
resp, _ := sess.DoRequest(ctx) // Ignoring error
```

---

## 📖 Complete Example

### Building a REST API Client

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
        return nil, fmt.Errorf("API error: %d", resp.StatusCode)
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
    fmt.Printf("User: %s\n", user.Name)
}
```

---

## 🔧 Configuration Options Quick Reference

### Client Configuration

| Option | Description | Example |
|--------|-------------|---------|
| `URL(string)` | Set target URL | `requests.URL("https://api.example.com")` |
| `Path(string)` | Append path | `requests.Path("/users")` |
| `Method(string)` | Set HTTP method | `requests.MethodPost` |
| `Timeout(duration)` | Set timeout | `requests.Timeout(30*time.Second)` |
| `Header(k, v)` | Add header | `requests.Header("Accept", "application/json")` |
| `BasicAuth(user, pass)` | Basic authentication | `requests.BasicAuth("admin", "secret")` |
| `Body(any)` | Set request body | `requests.Body(map[string]string{"key": "value"})` |
| `Form(values)` | Form data | `requests.Form(url.Values{...})` |
| `Params(map)` | Query parameters | `requests.Params(map[string]string{...})` |
| `Proxy(addr)` | Set proxy | `requests.Proxy("http://proxy:8080")` |
| `MaxConns(int)` | Max connections | `requests.MaxConns(100)` |
| `Verify(bool)` | Verify certificate | `requests.Verify(false)` |

### Server Configuration

| Option | Description | Example |
|--------|-------------|---------|
| `Use(middleware...)` | Register middleware | `requests.Use(loggingMiddleware)` |
| `CertKey(cert, key)` | TLS certificate | `requests.CertKey("cert.pem", "key.pem")` |
| `OnStart(func)` | Start callback | `requests.OnStart(func(s *http.Server){...})` |
| `OnShutdown(func)` | Shutdown callback | `requests.OnShutdown(func(s *http.Server){...})` |

---

## 🤝 Contributing

We welcome all forms of contributions!

- 🐛 Report bugs
- 💡 Suggest new features
- 📖 Improve documentation
- 🔧 Submit pull requests

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

---

## 📄 License

This project is licensed under the [Apache License 2.0](LICENSE).

---

## 🙏 Acknowledgments

- Inspired by Python's [requests](https://github.com/psf/requests) library
- Thanks to all contributors

---

## 📚 Resources

- [API Documentation](https://pkg.go.dev/github.com/golang-io/requests)
- [GitHub Repository](https://github.com/golang-io/requests)
- [Issue Tracker](https://github.com/golang-io/requests/issues)
- [Discussions](https://github.com/golang-io/requests/discussions)

---

<div align="center">

**If this project helps you, please give us a ⭐ Star!**

Made with ❤️ by the Requests Team

</div>
