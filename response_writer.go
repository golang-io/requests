// Package requests 提供了增强的HTTP响应写入器
// Package requests provides enhanced HTTP response writer
package requests

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"sync"
)

// ResponseWriter 包装了http.ResponseWriter接口，提供了额外的功能
// 主要用于服务器端中间件，支持响应内容捕获和并发安全访问
//
// ResponseWriter wraps http.ResponseWriter interface with additional features
// Mainly used for server-side middleware, supporting response content capture and concurrent-safe access
//
// 核心功能 / Core Features:
//  1. 记录响应状态码 / Records response status code
//  2. 缓存响应内容 / Caches response content
//  3. 支持并发安全的读写操作 / Supports concurrent-safe read/write operations
//  4. 实现多个标准接口 / Implements multiple standard interfaces:
//     - http.ResponseWriter
//     - http.Flusher
//     - http.Pusher
//     - http.Hijacker
//     - io.Reader
//     - io.Writer
//
// 使用场景 / Use Cases:
//   - 中间件需要记录响应内容 / Middleware needs to record response content
//   - 日志记录完整响应 / Logging complete responses
//   - 响应内容分析和监控 / Response content analysis and monitoring
//   - 实现自定义响应处理逻辑 / Implementing custom response handling logic
//
// 并发安全 / Concurrency Safety:
//   - 所有方法都使用互斥锁保护 / All methods are protected by mutex
//   - 可以从多个goroutine安全访问 / Can be safely accessed from multiple goroutines
//
// 示例 / Example:
//
//	func middleware(next http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        ww := requests.NewResponseWriter(w)
//	        next.ServeHTTP(ww, r)
//	        // 可以访问响应状态码和内容
//	        // Can access response status code and content
//	        log.Printf("Status: %d, Body: %s", ww.StatusCode, ww.Content.String())
//	    })
//	}
type ResponseWriter struct {
	mu sync.Mutex // 互斥锁，保护并发访问 / Mutex lock for concurrent access protection
	http.ResponseWriter
	wroteHeader bool          // 是否已经写入响应头 / Whether response header has been written
	StatusCode  int           // HTTP响应状态码 / HTTP response status code
	Content     *bytes.Buffer // 响应内容的缓存 / Response content cache
}

// NewResponseWriter 创建一个新的ResponseWriter实例
// 默认状态码为200 OK
//
// NewResponseWriter creates a new ResponseWriter instance
// Default status code is 200 OK
//
// 参数 / Parameters:
//   - w: http.ResponseWriter - 原始的ResponseWriter / Original ResponseWriter
//
// 返回值 / Returns:
//   - *ResponseWriter: 包装后的ResponseWriter / Wrapped ResponseWriter
//
// 示例 / Example:
//
//	func handler(w http.ResponseWriter, r *http.Request) {
//	    ww := requests.NewResponseWriter(w)
//	    ww.Write([]byte("Hello, World!"))
//	    fmt.Printf("Response: %s\n", ww.Content.String())
//	}
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, StatusCode: 200, Content: &bytes.Buffer{}}
}

// WriteHeader 设置HTTP响应状态码
// 该方法确保状态码只被设置一次（幂等性）
//
// WriteHeader sets the HTTP response status code
// This method ensures the status code is set only once (idempotent)
//
// 参数 / Parameters:
//   - statusCode: int - HTTP状态码（例如：200, 404, 500等） / HTTP status code (e.g., 200, 404, 500, etc.)
//
// 并发安全 / Concurrency Safety:
//   - 方法内部使用互斥锁保护 / Protected by mutex internally
//
// 注意事项 / Notes:
//   - 如果已经调用过WriteHeader，再次调用会被忽略 / Subsequent calls are ignored if WriteHeader was already called
//   - 调用Write()会自动触发WriteHeader(200) / Calling Write() automatically triggers WriteHeader(200)
//
// 示例 / Example:
//
//	ww.WriteHeader(http.StatusNotFound)
//	ww.Write([]byte("Not Found"))
func (w *ResponseWriter) WriteHeader(statusCode int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.StatusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write 实现io.Writer接口，写入响应体数据
// 数据会同时写入原始ResponseWriter和内容缓存
//
// Write implements io.Writer interface, writes response body data
// Data is written to both original ResponseWriter and content cache
//
// 参数 / Parameters:
//   - b: []byte - 要写入的数据 / Data to write
//
// 返回值 / Returns:
//   - int: 写入的字节数 / Number of bytes written
//   - error: 写入错误 / Write error
//
// 并发安全 / Concurrency Safety:
//   - 方法内部使用互斥锁保护 / Protected by mutex internally
//
// 行为特性 / Behavior:
//   - 首次调用会自动触发WriteHeader(200) / First call automatically triggers WriteHeader(200)
//   - 数据会被复制到内部缓存 / Data is copied to internal cache
//   - 如果写入原始ResponseWriter失败，不会写入缓存 / If writing to original ResponseWriter fails, cache is not updated
//
// 示例 / Example:
//
//	ww.Write([]byte("Hello, World!"))
//	fmt.Printf("Cached content: %s\n", ww.Content.String())
func (w *ResponseWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n, err := w.ResponseWriter.Write(b); err != nil {
		return n, err
	}
	return w.Content.Write(b)
}

// Read 实现io.Reader接口，从内容缓存中读取数据
// 通常用于中间件读取已写入的响应内容
//
// Read implements io.Reader interface, reads data from content cache
// Typically used by middleware to read written response content
//
// 参数 / Parameters:
//   - b: []byte - 读取数据的目标缓冲区 / Target buffer for reading data
//
// 返回值 / Returns:
//   - int: 读取的字节数 / Number of bytes read
//   - error: 读取错误（io.EOF表示读取完毕） / Read error (io.EOF indicates end of data)
//
// 并发安全 / Concurrency Safety:
//   - 方法内部使用互斥锁保护 / Protected by mutex internally
//
// 注意事项 / Notes:
//   - 读取操作会消耗缓存内容 / Read operation consumes cache content
//   - 如需多次读取，应先保存Content / To read multiple times, save Content first
//
// 示例 / Example:
//
//	data := make([]byte, 1024)
//	n, err := ww.Read(data)
//	fmt.Printf("Read %d bytes: %s\n", n, string(data[:n]))
func (w *ResponseWriter) Read(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Content.Read(b)
}

// Flush 实现http.Flusher接口，立即将缓冲的数据发送到客户端
// 用于实时流式响应（如Server-Sent Events）
//
// Flush implements http.Flusher interface, immediately sends buffered data to client
// Used for real-time streaming responses (e.g., Server-Sent Events)
//
// 并发安全 / Concurrency Safety:
//   - 方法内部使用互斥锁保护 / Protected by mutex internally
//
// 使用场景 / Use Cases:
//   - Server-Sent Events (SSE)
//   - 流式响应 / Streaming responses
//   - 长轮询 / Long polling
//   - 实时数据推送 / Real-time data push
//
// 注意事项 / Notes:
//   - 调用Flush后响应头会被立即发送 / Response headers are sent immediately after calling Flush
//   - 如果底层ResponseWriter不支持Flush，会panic / Panics if underlying ResponseWriter doesn't support Flush
//
// 示例 / Example:
//
//	ww.Write([]byte("data: message 1\n\n"))
//	ww.Flush() // 立即发送给客户端 / Send to client immediately
func (w *ResponseWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wroteHeader = true
	w.ResponseWriter.(http.Flusher).Flush()
}

// Push 实现http.Pusher接口，支持HTTP/2服务器推送功能
// 允许服务器主动向客户端推送资源
//
// Push implements http.Pusher interface, supports HTTP/2 server push
// Allows server to proactively push resources to client
//
// 参数 / Parameters:
//   - target: string - 要推送的资源路径 / Path of resource to push
//   - opts: *http.PushOptions - 推送选项（可为nil） / Push options (can be nil)
//
// 返回值 / Returns:
//   - error: 推送错误 / Push error
//
// 并发安全 / Concurrency Safety:
//   - 方法内部使用互斥锁保护 / Protected by mutex internally
//
// 使用场景 / Use Cases:
//   - 预加载关键资源（CSS, JS） / Preload critical resources (CSS, JS)
//   - 优化页面加载性能 / Optimize page load performance
//   - HTTP/2服务器推送 / HTTP/2 server push
//
// 注意事项 / Notes:
//   - 仅在HTTP/2连接上有效 / Only effective on HTTP/2 connections
//   - 如果底层不支持Push，会panic / Panics if underlying doesn't support Push
//
// 示例 / Example:
//
//	// 推送CSS文件
//	// Push CSS file
//	ww.Push("/static/style.css", &http.PushOptions{
//	    Header: http.Header{"Content-Type": []string{"text/css"}},
//	})
func (w *ResponseWriter) Push(target string, opts *http.PushOptions) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ResponseWriter.(http.Pusher).Push(target, opts)
}

// Hijack 实现http.Hijacker接口，允许接管HTTP连接
// 用于WebSocket或其他需要直接操作底层连接的场景
//
// Hijack implements http.Hijacker interface, allows taking over the HTTP connection
// Used for WebSocket or other scenarios requiring direct connection manipulation
//
// 返回值 / Returns:
//   - net.Conn: 底层网络连接 / Underlying network connection
//   - *bufio.ReadWriter: 带缓冲的读写器 / Buffered reader/writer
//   - error: 接管错误 / Hijack error
//
// 并发安全 / Concurrency Safety:
//   - 方法内部使用互斥锁保护 / Protected by mutex internally
//
// 使用场景 / Use Cases:
//   - WebSocket协议升级 / WebSocket protocol upgrade
//   - 自定义协议实现 / Custom protocol implementation
//   - 直接TCP连接操作 / Direct TCP connection operations
//
// 注意事项 / Notes:
//   - Hijack后ResponseWriter不再可用 / ResponseWriter is no longer usable after Hijack
//   - 调用者负责关闭连接 / Caller is responsible for closing the connection
//   - 如果底层不支持Hijack，会panic / Panics if underlying doesn't support Hijack
//
// 示例 / Example:
//
//	// WebSocket升级示例
//	// WebSocket upgrade example
//	conn, rw, err := ww.Hijack()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
//	// 现在可以直接操作conn和rw
//	// Now can directly operate on conn and rw
func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	hj := w.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}
