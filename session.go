package requests

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// Session 是 HTTP 客户端会话管理器
// Session is the HTTP client session manager
//
// 核心特性 / Core Features:
//   - 线程安全：可被多个goroutine并发使用 / Thread-safe: can be used concurrently by multiple goroutines
//   - 连接复用：自动管理连接池，提高性能 / Connection reuse: automatic connection pooling for better performance
//   - 配置持久化：会话级别的配置对所有请求生效 / Configuration persistence: session-level config applies to all requests
//   - 中间件支持：灵活的请求/响应处理链 / Middleware support: flexible request/response processing chain
//
// 设计原则 / Design Principles:
//   - 应该只创建一次并重复使用（遵循 net/http 的最佳实践）
//   - Should be created once and reused (follows net/http best practices)
//   - 每个会话维护独立的连接池和配置
//   - Each session maintains independent connection pool and configuration
//
// 示例 / Example:
//
//	// 创建一个会话，配置公共参数
//	// Create a session with common configuration
//	sess := requests.New(
//	    requests.URL("https://api.example.com"),
//	    requests.Header("Authorization", "Bearer token123"),
//	    requests.Timeout(30*time.Second),
//	)
//
//	// 所有请求都会继承会话配置
//	// All requests will inherit session configuration
//	resp1, _ := sess.DoRequest(context.Background(), requests.Path("/users"))
//	resp2, _ := sess.DoRequest(context.Background(), requests.Path("/posts"))
type Session struct {
	opts      []Option        // 会话级别的配置选项 / Session-level configuration options
	transport *http.Transport // 底层传输层，管理连接池 / Underlying transport layer, manages connection pool
	client    *http.Client    // HTTP客户端实例 / HTTP client instance
}

// New 创建一个新的会话实例
// New creates a new session instance
//
// 参数 / Parameters:
//   - opts: 可选的配置选项（会话级别）/ Optional configuration options (session-level)
//
// 返回值 / Returns:
//   - *Session: 初始化的会话对象 / Initialized session object
//
// 配置说明 / Configuration Notes:
//   - 会话级配置对所有请求生效 / Session-level config applies to all requests
//   - 请求级配置可以覆盖会话配置 / Request-level config can override session config
//
// 示例 / Example:
//
//	// 基础会话
//	// Basic session
//	sess := requests.New()
//
//	// 带配置的会话
//	// Session with configuration
//	sess := requests.New(
//	    requests.URL("https://api.example.com"),
//	    requests.Header("User-Agent", "MyApp/1.0"),
//	    requests.Timeout(30*time.Second),
//	    requests.MaxConns(100),
//	)
func New(opts ...Option) *Session {
	options := newOptions(opts)
	transport := newTransport(opts...)
	client := &http.Client{Timeout: options.Timeout, Transport: transport}
	s := &Session{opts: opts, transport: transport, client: client}
	return s
}

// HTTPClient 返回配置好的 http.Client 实例
// HTTPClient returns the configured http.Client instance
//
// 参数 / Parameters:
//   - opts: 可选的额外配置选项 / Optional additional configuration options
//
// 返回值 / Returns:
//   - *http.Client: 配置好的HTTP客户端 / Configured HTTP client
//
// 使用场景 / Use Cases:
//   - 需要使用标准库的 http.Client 接口 / Need to use standard library's http.Client interface
//   - 与其他库集成时 / When integrating with other libraries
func (s *Session) HTTPClient(opts ...Option) *http.Client {
	return &http.Client{Timeout: s.client.Timeout, Transport: s.RoundTripper(opts...)}
}

// Transport 返回底层的 http.Transport 实例
// Transport returns the underlying http.Transport instance
//
// 返回值 / Returns:
//   - *http.Transport: 传输层对象 / Transport object
//
// 使用场景 / Use Cases:
//   - 需要访问或修改传输层配置 / Need to access or modify transport configuration
//   - 需要获取连接池状态 / Need to get connection pool status
func (s *Session) Transport() *http.Transport {
	return s.transport
}

// RoundTrip 实现 http.RoundTripper 接口
// RoundTrip implements the http.RoundTripper interface
//
// 这使得 Session 可以作为 http.Client 的 Transport 使用
// This allows Session to be used as http.Client's Transport
//
// 参数 / Parameters:
//   - r: HTTP 请求对象 / HTTP request object
//
// 返回值 / Returns:
//   - *http.Response: HTTP 响应对象 / HTTP response object
//   - error: 请求过程中的错误 / Error during request
func (s *Session) RoundTrip(r *http.Request) (*http.Response, error) {
	return s.RoundTripper().RoundTrip(r)
}

// Do 发送请求并返回标准的 http.Response
// Do sends a request and returns standard http.Response
//
// 重要提示 / Important Note:
//   - 必须手动关闭 resp.Body！ / Must manually close resp.Body!
//   - 建议使用 DoRequest 方法，它会自动处理 Body 关闭
//   - Recommend using DoRequest method, which auto-handles Body closing
//
// 参数 / Parameters:
//   - ctx: 请求上下文 / Request context
//   - opts: 请求级别的配置选项（可覆盖会话配置）/ Request-level options (can override session config)
//
// 返回值 / Returns:
//   - *http.Response: HTTP 响应对象（需要手动关闭Body）/ HTTP response object (Body must be closed manually)
//   - error: 请求过程中的错误 / Error during request
//
// 示例 / Example:
//
//	resp, err := sess.Do(context.Background(),
//	    requests.MethodPost,
//	    requests.Path("/api/users"),
//	    requests.Body(`{"name": "John"}`),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer resp.Body.Close() // 必须手动关闭！ / Must close manually!
//
//	body, _ := io.ReadAll(resp.Body)
//	fmt.Println(string(body))
func (s *Session) Do(ctx context.Context, opts ...Option) (*http.Response, error) {
	options := newOptions(s.opts, opts...)
	req, err := NewRequestWithContext(ctx, options)
	if err != nil {
		return &http.Response{}, err
	}
	return s.RoundTripper(opts...).RoundTrip(req)
}

// DoRequest 发送请求并返回增强的 Response 对象
// DoRequest sends a request and returns enhanced Response object
//
// 相比 Do 方法的优势 / Advantages over Do method:
//   - 自动安全关闭 resp.Body / Automatically and safely closes resp.Body
//   - 自动缓存响应内容到 Content / Auto-caches response content to Content
//   - 记录请求耗时和统计信息 / Records request duration and statistics
//   - 支持多次读取响应内容 / Supports multiple reads of response content
//   - 无需担心资源泄漏 / No need to worry about resource leaks
//
// 参数 / Parameters:
//   - ctx: 请求上下文 / Request context
//   - opts: 请求级别的配置选项 / Request-level configuration options
//
// 返回值 / Returns:
//   - *Response: 增强的响应对象（Content已缓存）/ Enhanced response object (Content cached)
//   - error: 请求过程中的错误 / Error during request
//
// 推荐使用场景 / Recommended Use Cases:
//   - 大多数日常HTTP请求 / Most daily HTTP requests
//   - 需要记录请求统计信息 / Need to record request statistics
//   - 需要多次读取响应内容 / Need to read response content multiple times
//
// 示例 / Example:
//
//	sess := requests.New(requests.URL("https://api.example.com"))
//	resp, err := sess.DoRequest(context.Background(),
//	    requests.MethodPost,
//	    requests.Path("/users"),
//	    requests.Body(map[string]string{"name": "John"}),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// 无需关闭Body，Content已自动缓存
//	// No need to close Body, Content is auto-cached
//	fmt.Println(resp.Content.String())
//
//	// 可以多次读取
//	// Can read multiple times
//	var user User
//	resp.JSON(&user)
//
//	// 查看统计信息
//	// View statistics
//	fmt.Printf("Request took %v\n", resp.Cost)
func (s *Session) DoRequest(ctx context.Context, opts ...Option) (*Response, error) {
	options, resp := newOptions(s.opts, opts...), newResponse(nil)

	// 创建 HTTP 请求
	// Create HTTP request
	resp.Request, resp.Err = NewRequestWithContext(ctx, options)
	if resp.Err != nil {
		return resp, resp.Err
	}

	// 发送请求
	// Send request
	resp.Response, resp.Err = s.RoundTripper(opts...).RoundTrip(resp.Request)
	if resp.Err != nil {
		return resp, resp.Err
	}

	// 确保响应体不为 nil
	// Ensure response body is not nil
	if resp.Response == nil {
		resp.Response = &http.Response{Body: http.NoBody}
	} else if resp.Response.Body == nil {
		resp.Response.Body = http.NoBody
	}

	// 自动读取并缓存响应内容，然后安全关闭 Body
	// Auto-read and cache response content, then safely close Body
	defer resp.Response.Body.Close()
	_, resp.Err = resp.Content.ReadFrom(resp.Response.Body)

	// 重新包装 Body，使其仍然可读
	// Re-wrap Body to make it still readable
	resp.Response.Body = io.NopCloser(bytes.NewReader(resp.Content.Bytes()))

	// 计算请求耗时
	// Calculate request duration
	resp.Cost = resp.StartAt.Sub(resp.StartAt)

	return resp, resp.Err
}

// RoundTripper 返回配置好的 http.RoundTripper
// RoundTripper returns the configured http.RoundTripper
//
// 功能 / Features:
//   - 应用所有注册的中间件（按注册顺序的反序）/ Applies all registered middleware (in reverse order of registration)
//   - 支持中间件链式调用 / Supports middleware chaining
//
// 参数 / Parameters:
//   - opts: 可选的额外配置选项 / Optional additional configuration options
//
// 返回值 / Returns:
//   - http.RoundTripper: 配置好的 RoundTripper（包含中间件链）/ Configured RoundTripper (with middleware chain)
//
// 使用场景 / Use Cases:
//   - 需要自定义请求/响应处理逻辑 / Need custom request/response processing logic
//   - 与其他库集成时 / When integrating with other libraries
func (s *Session) RoundTripper(opts ...Option) http.RoundTripper {
	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		options := newOptions(s.opts, opts...)

		// 设置默认传输层
		// Set default transport
		if options.Transport == nil {
			options.Transport = RoundTripperFunc(s.client.Do)
		}

		// 应用所有中间件（装饰器模式）
		// Apply all middleware (decorator pattern)
		for _, tr := range options.HttpRoundTripper {
			options.Transport = tr(options.Transport)
		}

		return options.Transport.RoundTrip(r)
	})
}
