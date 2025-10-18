// Package requests 提供了HTTP请求追踪和调试功能
// Package requests provides HTTP request tracing and debugging functionality
package requests

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/textproto"
)

// trace 是HTTP客户端追踪的钩子集合，用于在HTTP请求的各个阶段运行回调函数
// 这些钩子函数提供了对HTTP请求生命周期的详细观察能力
//
// trace is a set of hooks to run at various stages of an outgoing HTTP request
// These hooks provide detailed observation capabilities for the HTTP request lifecycle
//
// 特性 / Features:
//   - 任何特定钩子都可以为nil / Any particular hook may be nil
//   - 函数可能从不同goroutine并发调用 / Functions may be called concurrently from different goroutines
//   - 某些函数可能在请求完成或失败后调用 / Some may be called after the request has completed or failed
//   - 当前追踪单个HTTP请求和响应的往返过程 / Currently traces a single HTTP request & response during a single round trip
//   - 没有跨越一系列重定向请求的钩子 / Has no hooks that span a series of redirected requests
//
// 追踪阶段 / Trace Stages:
//  1. DNS解析 (DNSStart/DNSDone)
//  2. 连接建立 (GetConn/GotConn/ConnectStart/ConnectDone)
//  3. TLS握手 (TLSHandshakeStart/TLSHandshakeDone)
//  4. 请求写入 (WroteHeaderField/WroteHeaders/WroteRequest)
//  5. 响应接收 (GotFirstResponseByte/Got100Continue/Got1xxResponse)
//  6. 连接释放 (PutIdleConn)
//
// 参考文档 / Reference:
//   - https://blog.golang.org/http-tracing
//
// 使用场景 / Use Cases:
//   - 性能分析和诊断 / Performance analysis and diagnostics
//   - 调试网络问题 / Debugging network issues
//   - 监控HTTP请求细节 / Monitoring HTTP request details
//   - 了解连接复用情况 / Understanding connection reuse
var trace = &httptrace.ClientTrace{
	// GetConn is called before a connection is created or
	// retrieved from an idle pool. The hostPort is the
	// "host:port" of the target or proxy. GetConn is called even
	// if there's already an idle cached connection available.
	GetConn: func(hostPort string) {
		Log("* Connect: %v", hostPort)
	},

	// GotConn is called after a successful connection is
	// obtained. There is no hook for failure to obtain a
	// connection; instead, use the error from
	// Transport.RoundTrip.
	GotConn: func(connInfo httptrace.GotConnInfo) {
		Log("* Got Conn: %v -> %v", connInfo.Conn.LocalAddr(), connInfo.Conn.RemoteAddr())
	},
	// PutIdleConn is called when the connection is returned to
	// the idle pool. If err is nil, the connection was
	// successfully returned to the idle pool. If err is non-nil,
	// it describes why not. PutIdleConn is not called if
	// connection reuse is disabled via Transport.DisableKeepAlives.
	// PutIdleConn is called before the caller's Response.Body.Close
	// call returns.
	// For HTTP/2, this hook is not currently used.
	PutIdleConn: func(err error) {},

	// GotFirstResponseByte is called when the first byte of the response
	// headers is available.
	GotFirstResponseByte: func() {},

	// Got100Continue is called if the server replies with a "100
	// Continue" response.
	Got100Continue: func() {},

	// Got1xxResponse is called for each 1xx informational response header
	// returned before the final non-1xx response. Got1xxResponse is called
	// for "100 Continue" responses, even if Got100Continue is also defined.
	// If it returns an error, the client request is aborted with that error value.
	Got1xxResponse: func(code int, header textproto.MIMEHeader) error { return nil },

	// DNSStart is called when a DNS lookup begins.
	DNSStart: func(dnsInfo httptrace.DNSStartInfo) {
		Log("* Resolved Host: %v", dnsInfo.Host)
	},
	// DNSDone is called when a DNS lookup ends.
	DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
		var ipaddrs []string
		for _, ipaddr := range dnsInfo.Addrs {
			ipaddrs = append(ipaddrs, ipaddr.String())
		}
		Log("* Resolved DNS: %v, Coalesced: %v, err=%v", ipaddrs, dnsInfo.Coalesced, dnsInfo.Err)
	},
	// ConnectStart is called when a new connection's Dial begins.
	// If net.Dialer.DualStack (IPv6 "Happy Eyeballs") support is
	// enabled, this may be called multiple times.
	ConnectStart: func(network, addr string) {
		Log("* Trying ConnectStart %v %v...", network, addr)
	},
	// ConnectDone is called when a new connection's Dial
	// completes. The provided err indicates whether the
	// connection completed successfully.
	// If net.Dialer.DualStack ("Happy Eyeballs") support is
	// enabled, this may be called multiple times.
	ConnectDone: func(network, addr string, err error) {
		Log("* Completed connection: %v %v, err=%v", network, addr, err)
	},
	// TLSHandshakeStart is called when the TLS handshake is started. When
	// connecting to an HTTPS site via an HTTP proxy, the handshake happens
	// after the CONNECT request is processed by the proxy.
	TLSHandshakeStart: func() {},

	// TLSHandshakeDone is called after the TLS handshake with either the
	// successful handshake's connection state, or a non-nil error on handshake
	// failure.
	TLSHandshakeDone: func(state tls.ConnectionState, err error) {
		Log("* SSL HandshakeComplete: %v", state.HandshakeComplete)
	},
	// WroteHeaderField is called after the Transport has written
	// each request header. At the time of this call the values
	// might be buffered and not yet written to the network.
	WroteHeaderField: func(key string, value []string) {},

	// WroteHeaders is called after the Transport has written
	// all request headers.
	WroteHeaders: func() {},

	// Wait100Continue is called if the Request specified
	// "Expect: 100-continue" and the Transport has written the
	// request headers but is waiting for "100 Continue" from the
	// server before writing the request body.
	Wait100Continue: func() {},

	// WroteRequest is called with the result of writing the
	// request and any body. It may be called multiple times
	// in the case of retried requests.
	WroteRequest: func(reqInfo httptrace.WroteRequestInfo) {
		//Log("* WroteRequest, err=%v", reqInfo.Err)
	},
}

// Log 打印日志信息（用于调试追踪）
// 格式化输出字符串并添加换行符
//
// Log prints log information (for debugging traces)
// Formats output string and adds newline
//
// 参数 / Parameters:
//   - format: string - 格式化字符串（类似fmt.Printf） / Format string (similar to fmt.Printf)
//   - v: ...any - 格式化参数 / Format arguments
//
// 使用场景 / Use Cases:
//   - 调试HTTP请求过程 / Debugging HTTP request process
//   - 追踪网络连接状态 / Tracing network connection status
func Log(format string, v ...any) {
	print(fmt.Sprintf(format+"\n", v...))
}

// show 格式化显示字节切片内容，每行添加前缀，并支持截断
// 用于美化显示HTTP请求和响应内容
//
// show formats and displays byte slice content, adding prefix to each line, with truncation support
// Used to beautify HTTP request and response content display
//
// 参数 / Parameters:
//   - prompt: string - 每行的前缀（例如："> "表示请求，"< "表示响应） / Prefix for each line (e.g., "> " for request, "< " for response)
//   - b: []byte - 要显示的内容 / Content to display
//   - mLimit: int - 最大显示长度，超过则截断 / Maximum display length, truncate if exceeded
//
// 返回值 / Returns:
//   - string: 格式化后的字符串 / Formatted string
//
// 示例输出 / Example Output:
//
//	> POST /api/users HTTP/1.1
//	> Host: example.com
//	> Content-Type: application/json
//	> {"name": "alice"}
func show(prompt string, b []byte, mLimit int) string {
	var buf bytes.Buffer
	for _, line := range bytes.Split(b, []byte("\n")) {
		buf.Write([]byte(prompt))
		buf.Write(line)
		buf.WriteString("\n")
	}
	str := buf.String()
	if len(str) > mLimit {
		return fmt.Sprintf("%s...[Len=%d, Truncated[%d]]", str[:mLimit], len(str), mLimit)
	}
	return str
}

// Trace 启用HTTP请求追踪，打印请求和响应的详细信息
// 包括HTTP头、请求体、响应体等，适用于调试和开发环境
//
// Trace enables HTTP request tracing, prints detailed request and response information
// Includes HTTP headers, request body, response body, etc., suitable for debugging and development
//
// 参数 / Parameters:
//   - mLimit: ...int - 可选的最大显示长度（默认10240字节），超过则截断 / Optional maximum display length (default 10240 bytes), truncate if exceeded
//
// 返回值 / Returns:
//   - Option: 配置选项函数 / Configuration option function
//
// 追踪内容 / Traced Content:
//   - DNS解析过程 / DNS resolution process
//   - TCP连接建立 / TCP connection establishment
//   - TLS握手过程 / TLS handshake process
//   - HTTP请求完整内容 / Complete HTTP request content
//   - HTTP响应完整内容 / Complete HTTP response content
//
// 性能注意 / Performance Note:
//   - 会复制并缓存请求和响应体，增加内存开销 / Copies and caches request and response bodies, increasing memory overhead
//   - 仅建议在开发和调试环境使用 / Recommended only for development and debugging environments
//   - 生产环境建议使用Setup()进行轻量级日志 / Use Setup() for lightweight logging in production
//
// 使用场景 / Use Cases:
//   - 调试API调用问题 / Debugging API call issues
//   - 分析HTTP通信细节 / Analyzing HTTP communication details
//   - 开发环境请求监控 / Request monitoring in development
//   - 学习HTTP协议 / Learning HTTP protocol
//
// 示例 / Example:
//
//	// 启用请求追踪
//	// Enable request tracing
//	session := requests.New(requests.Trace())
//	resp, _ := session.DoRequest(ctx, requests.URL("https://api.example.com/users"))
//
//	// 自定义截断长度为5000字节
//	// Custom truncate length to 5000 bytes
//	session := requests.New(requests.Trace(5000))
func Trace(mLimit ...int) Option {
	return func(o *Options) {
		o.HttpRoundTripper = append([]func(http.RoundTripper) http.RoundTripper{traceLv(true, mLimit...)}, o.HttpRoundTripper...)
	}
}

// traceLv 创建追踪级别的RoundTripper中间件
// 内部函数，用于实现Trace()的核心逻辑
//
// traceLv creates a trace-level RoundTripper middleware
// Internal function to implement the core logic of Trace()
//
// 参数 / Parameters:
//   - used: bool - 是否启用追踪（false则快速返回） / Whether to enable tracing (fast path if false)
//   - mLimit: ...int - 最大显示长度 / Maximum display length
//
// 返回值 / Returns:
//   - func(http.RoundTripper) http.RoundTripper: RoundTripper中间件函数 / RoundTripper middleware function
func traceLv(used bool, mLimit ...int) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if !used {
				return next.RoundTrip(req) // fast path
			}
			maxLimit := 10240
			if len(mLimit) != 0 {
				maxLimit = mLimit[0]
			}
			ctx := httptrace.WithClientTrace(req.Context(), trace)
			req2 := req.WithContext(ctx)

			// 使用 recover 来捕获 DumpRequestOut 可能的 panic
			var reqLog []byte
			var err error

			defer func() {
				if r := recover(); r != nil {
					Log("! request dump panic: %v", r)
					reqLog = []byte("(request dump failed due to panic)")
					err = fmt.Errorf("request dump panic: %v", r)
				}
			}()
			reqLog, err = httputil.DumpRequestOut(req2, true)

			if err != nil {
				Log("! request error: %v", err)
				// 即使 DumpRequestOut 失败，我们仍然可以继续处理请求
				// 只是不记录请求日志
				reqLog = []byte("(request dump failed)")
			}
			resp, err := next.RoundTrip(req2)

			Log("%s", show("> ", reqLog, maxLimit))

			if err != nil {
				return nil, err
			}

			// 答应响应头和响应体长度
			Log("< %s %s", resp.Proto, resp.Status)
			for k, vs := range resp.Header {
				for _, v := range vs {
					Log("< %s: %s", k, v)
				}
			}

			buf, r, err := CopyBody(resp.Body)
			if err != nil {
				Log("! response error: %w", err)
				return nil, err
			}
			resp.Body = r
			Log("%s", show("", buf.Bytes(), maxLimit))

			return resp, nil
		})
	}
}
