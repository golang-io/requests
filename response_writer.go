package requests

import (
	"bufio"
	"bytes"
	"net"
	"net/http"
	"sync"
)

// ResponseWriter 包装了 http.ResponseWriter 接口，提供了额外的功能：
// 1. 记录响应状态码
// 2. 缓存响应内容
// 3. 支持并发安全的读写操作
type ResponseWriter struct {
	mu sync.Mutex // 互斥锁，保护并发访问
	http.ResponseWriter
	wroteHeader bool          // 是否已经写入响应头
	StatusCode  int           // HTTP 响应状态码
	Content     *bytes.Buffer // 响应内容的缓存
}

// newResponseWriter 创建一个新的 ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, StatusCode: 200, Content: &bytes.Buffer{}}
}

// WriteHeader 设置 HTTP 响应状态码
// 该方法确保状态码只被设置一次
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

// Write 实现 io.Writer 接口
// 将数据同时写入原始 ResponseWriter 和内容缓存
func (w *ResponseWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if n, err := w.ResponseWriter.Write(b); err != nil {
		return n, err
	}
	return w.Content.Write(b)
}

// Read 实现 io.Reader 接口
// 从内容缓存中读取数据
func (w *ResponseWriter) Read(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.Content.Read(b)
}

// Flush 实现 http.Flusher 接口
// 将缓冲的数据立即发送到客户端
func (w *ResponseWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wroteHeader = true
	w.ResponseWriter.(http.Flusher).Flush()
}

// Push 实现 http.Pusher 接口
// 支持 HTTP/2 服务器推送功能
func (w *ResponseWriter) Push(target string, opts *http.PushOptions) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ResponseWriter.(http.Pusher).Push(target, opts)
}

// Hijack 实现 http.Hijacker 接口
// 允许接管 HTTP 连接
func (w *ResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	hj := w.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}
