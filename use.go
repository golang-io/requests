// Package requests 提供了HTTP服务器中间件，包括SSE、CORS等功能
// Package requests provides HTTP server middleware including SSE, CORS, etc.
package requests

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// ServerSentEvents 实现了Server-Sent Events (SSE)流式传输的http.ResponseWriter包装器
// 它在标准ResponseWriter基础上提供SSE特定的功能
//
// ServerSentEvents implements http.Handler interface for Server-Sent Events (SSE) streaming
// It wraps a http.ResponseWriter to provide SSE-specific functionality
//
// SSE协议规范 / SSE Protocol Specification:
//   - 使用"text/event-stream"内容类型 / Uses "text/event-stream" content type
//   - 每个事件由一个或多个字段组成 / Each event consists of one or more fields
//   - 字段格式：name:value\n / Field format: name:value\n
//   - 空行表示事件结束 / Empty line marks event completion
//
// 使用场景 / Use Cases:
//   - 实时通知推送 / Real-time notification push
//   - 服务器到客户端的单向数据流 / One-way data stream from server to client
//   - 聊天应用消息推送 / Chat application message push
//   - 实时日志流 / Real-time log streaming
type ServerSentEvents struct {
	w http.ResponseWriter
}

// WriteHeader 实现http.ResponseWriter接口，写入HTTP响应状态码
//
// WriteHeader implements http.ResponseWriter interface
// It writes the HTTP status code to the response
//
// 参数 / Parameters:
//   - statusCode: int - HTTP状态码 / HTTP status code
func (s *ServerSentEvents) WriteHeader(statusCode int) {
	s.w.WriteHeader(statusCode)
}

// Write 实现http.ResponseWriter接口，将字节切片作为data事件写入SSE流
// 这是一个便捷方法，相当于调用 Send("data", b)
//
// Write implements http.ResponseWriter interface
// It writes the byte slice as a data event to the SSE stream
//
// 参数 / Parameters:
//   - b: []byte - 要发送的数据 / Data to send
//
// 返回值 / Returns:
//   - int: 写入的字节数 / Number of bytes written
//   - error: 写入错误 / Write error
func (s *ServerSentEvents) Write(b []byte) (int, error) {
	return s.Send("data", b)
}

// Header 实现http.ResponseWriter接口，返回将要被WriteHeader发送的响应头
//
// Header implements http.ResponseWriter interface
// It returns the header map that will be sent by WriteHeader
//
// 返回值 / Returns:
//   - http.Header: 响应头映射 / Response header map
func (s *ServerSentEvents) Header() http.Header {
	return s.w.Header()
}

// Send 向SSE流中写入一个命名的事件，并自动刷新响应
// 格式：name:value\n
//
// Send writes a named SSE event with formatted data to the stream
// It automatically flushes the response after writing
//
// 参数 / Parameters:
//   - name: string - 事件名称（例如："data", "event", "id", "retry"等） / Event name (e.g., "data", "event", "id", "retry", etc.)
//   - b: []byte - 事件数据 / Event data
//
// 返回值 / Returns:
//   - int: 写入的字节数 / Number of bytes written
//   - error: 写入错误 / Write error
//
// 示例 / Example:
//
//	// 发送数据事件
//	// Send data event
//	sse.Send("data", []byte(`{"message": "hello"}`))
//
//	// 发送自定义事件类型
//	// Send custom event type
//	sse.Send("event", []byte("user-login"))
//	sse.Send("data", []byte(`{"user": "alice"}`))
func (s *ServerSentEvents) Send(name string, b []byte) (int, error) {
	defer s.w.(http.Flusher).Flush()
	return s.w.Write([]byte(name + ":" + string(b) + "\n"))
}

// End 通过写入两个换行符来终止SSE流
// 当流完成时应该调用此方法
//
// End terminates the SSE stream by writing two newlines
// This should be called when the stream is complete
func (s *ServerSentEvents) End() {
	_, _ = s.Write([]byte("\n\n"))
}

// Read 从给定的字节切片中解析SSE消息
// 处理不同类型的SSE事件（空行、事件类型、数据）
//
// Read parses an SSE message from the given byte slice
// It handles different types of SSE events (empty, event, data)
//
// 参数 / Parameters:
//   - b: []byte - 原始SSE消息行 / Raw SSE message line
//
// 返回值 / Returns:
//   - []byte: 事件数据（仅对data事件） / Event data (only for data events)
//   - error: 解析错误 / Parsing error
//
// 返回逻辑 / Return Logic:
//   - data事件：返回事件值 / For data events: returns the event value
//   - 空行或event行：返回nil, nil / For empty or event lines: returns nil, nil
//   - 未知事件：返回nil和错误 / For unknown events: returns nil and an error
func (s *ServerSentEvents) Read(b []byte) ([]byte, error) {
	name, value, _ := bytes.Cut(bytes.TrimRight(b, "\n"), []byte(":"))
	switch string(name) {
	case "":
		// Empty lines or comments (": something") should be ignored
		// 空行或注释（": something"）应该被忽略
		return nil, nil
	case "event":
		// Event type declarations are processed but not returned
		// 事件类型声明被处理但不返回
		return nil, nil
	case "data":
		// Data events return their value
		// 数据事件返回其值
		return value, nil
	default:
		// Unknown event types return an error
		// 未知事件类型返回错误
		return nil, fmt.Errorf("unknown event: %s", name)
	}
}

// SSE 返回一个启用Server-Sent Events支持的中间件函数
// 该中间件会自动设置SSE所需的响应头并处理流的生命周期
//
// SSE returns a middleware function that enables Server-Sent Events support
// The middleware automatically sets required SSE response headers and manages stream lifecycle
//
// 中间件功能 / Middleware Features:
//   - 设置适当的SSE响应头（Content-Type, Cache-Control等） / Sets appropriate SSE headers (Content-Type, Cache-Control, etc.)
//   - 创建ServerSentEvents包装器 / Creates ServerSentEvents wrapper for the response writer
//   - 通过defer确保流正确终止 / Ensures proper stream termination via deferred End() call
//   - 启用CORS跨域支持 / Enables CORS support for cross-origin requests
//
// 返回值 / Returns:
//   - func(next http.Handler) http.Handler: HTTP中间件函数 / HTTP middleware function
//
// 使用场景 / Use Cases:
//   - 实时数据推送 / Real-time data push
//   - 服务器主动通知客户端 / Server-initiated notifications to clients
//   - 长连接事件流 / Long-lived event streams
//
// 示例 / Example:
//
//	mux := requests.NewServeMux()
//	mux.GET("/events", requests.SSE()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	    sse := w.(*requests.ServerSentEvents)
//	    for i := 0; i < 10; i++ {
//	        sse.Send("data", []byte(fmt.Sprintf(`{"count": %d}`, i)))
//	        time.Sleep(time.Second)
//	    }
//	})))
func SSE() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sse := &ServerSentEvents{w: w}
			defer sse.End()
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			next.ServeHTTP(sse, r)
		})
	}
}

// CORS 返回一个跨域资源共享(Cross-Origin Resource Sharing)中间件
// 该中间件允许来自任何源的跨域请求，并处理预检请求
//
// CORS returns a Cross-Origin Resource Sharing middleware
// This middleware allows cross-origin requests from any origin and handles preflight requests
//
// 返回值 / Returns:
//   - func(next http.Handler) http.Handler: HTTP中间件函数 / HTTP middleware function
//
// CORS配置 / CORS Configuration:
//   - Access-Control-Allow-Origin: * （允许所有源） / * (allows all origins)
//   - Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
//   - Access-Control-Allow-Headers: Content-Type, Authorization
//
// 预检请求处理 / Preflight Request Handling:
//   - 自动响应OPTIONS请求 / Automatically responds to OPTIONS requests
//   - 返回204 No Content状态码 / Returns 204 No Content status code
//
// 使用场景 / Use Cases:
//   - 允许前端跨域访问API / Allow frontend cross-origin API access
//   - 公共API服务 / Public API services
//   - 微服务间通信 / Inter-microservice communication
//
// 示例 / Example:
//
//	mux := requests.NewServeMux()
//	mux.Use(requests.CORS())
//	mux.GET("/api/data", handler)
func CORS() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// printHandler 创建一个用于打印HTTP服务器请求和响应信息的中间件
// 该中间件会记录请求处理时间和相关统计信息
//
// printHandler creates a middleware for printing HTTP server request and response information
// It records the request processing time and related statistics
//
// 参数 / Parameters:
//   - f: func(ctx context.Context, stat *Stat) - 处理统计信息的回调函数 / Callback function for processing statistics
//
// 返回值 / Returns:
//   - func(handler http.Handler) http.Handler: HTTP中间件函数 / HTTP middleware function
//
// 内部实现 / Implementation:
//   - 记录请求开始时间 / Records request start time
//   - 包装ResponseWriter以捕获响应内容 / Wraps ResponseWriter to capture response content
//   - 复制请求体以便多次读取 / Copies request body for multiple reads
//   - 计算请求处理耗时 / Calculates request processing duration
//   - 调用回调函数传递统计信息 / Calls callback function with statistics
//
// 使用场景 / Use Cases:
//   - 服务器端请求日志 / Server-side request logging
//   - 性能监控 / Performance monitoring
//   - 审计追踪 / Audit trail
func printHandler(f func(ctx context.Context, stat *Stat)) func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := NewResponseWriter(w)
			buf, body, _ := CopyBody(r.Body)
			r.Body = body
			next.ServeHTTP(ww, r)
			f(r.Context(), serveLoad(ww, r, start, buf))
		})
	}
}
