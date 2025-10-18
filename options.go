package requests

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Options 是请求和会话的配置选项集合
// Options is the collection of configuration options for requests and sessions
//
// 设计理念 / Design Philosophy:
//   - 客户端配置（Client）：URL、Header、Timeout、Proxy 等
//   - 服务器配置（Server）：Handler、OnStart、OnShutdown 等
//   - 传输层配置（Transport）：MaxConns、Verify、LocalAddr 等
//   - 中间件配置（Middleware）：HttpRoundTripper、HttpHandler 等
//
// 两级配置系统 / Two-Level Configuration System:
//   - 会话级别（Session-level）：创建Session时设置，对所有请求生效
//   - 请求级别（Request-level）：单次请求时设置，可覆盖会话配置
//
// 示例 / Example:
//
//	// 会话级配置
//	// Session-level configuration
//	opts := []requests.Option{
//	    requests.URL("https://api.example.com"),
//	    requests.Timeout(30*time.Second),
//	    requests.Header("Authorization", "Bearer token"),
//	}
//	sess := requests.New(opts...)
//
//	// 请求级配置（覆盖会话配置）
//	// Request-level configuration (overrides session config)
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.Path("/users"),
//	    requests.Timeout(10*time.Second), // 覆盖会话的30秒超时 / Override session's 30s timeout
//	)
type Options struct {
	// ===== HTTP 请求基础配置 / HTTP Request Basic Configuration =====
	Method   string        // HTTP方法（GET、POST等）/ HTTP method (GET, POST, etc.)
	URL      string        // 目标URL / Target URL
	Path     []string      // URL路径片段（会追加到URL后）/ URL path segments (appended to URL)
	RawQuery url.Values    // URL查询参数 / URL query parameters
	body     any           // 请求体（支持多种类型）/ Request body (supports multiple types)
	Header   http.Header   // HTTP请求头 / HTTP headers
	Cookies  []http.Cookie // HTTP Cookies

	// ===== 客户端配置 / Client Configuration =====
	Timeout  time.Duration // 请求超时时间 / Request timeout
	MaxConns int           // 最大连接数（连接池大小）/ Maximum connections (connection pool size)
	Verify   bool          // 是否验证TLS证书 / Whether to verify TLS certificates

	// ===== 传输层配置 / Transport Layer Configuration =====
	Transport        http.RoundTripper                           // 自定义传输层 / Custom transport
	HttpRoundTripper []func(http.RoundTripper) http.RoundTripper // 客户端中间件链 / Client middleware chain

	// ===== 服务器配置 / Server Configuration =====
	Handler     http.Handler                      // HTTP处理器 / HTTP handler
	HttpHandler []func(http.Handler) http.Handler // 服务器中间件链 / Server middleware chain
	OnStart     func(*http.Server)                // 服务器启动回调 / Server start callback
	OnShutdown  func(*http.Server)                // 服务器关闭回调 / Server shutdown callback

	// ===== TLS/SSL 配置 / TLS/SSL Configuration =====
	certFile string // 证书文件路径 / Certificate file path
	keyFile  string // 密钥文件路径 / Key file path

	// ===== 网络配置 / Network Configuration =====
	// client session used
	LocalAddr net.Addr                              // 本地地址绑定 / Local address binding
	Proxy     func(*http.Request) (*url.URL, error) // 代理配置函数 / Proxy configuration function

	// HTTP/3 (QUIC) support
	// EnableHTTP3 enables HTTP/3 protocol using QUIC
	EnableHTTP3 bool
}

// Option 是用于配置 Options 的函数类型
// Option is a function type for configuring Options
//
// 这是一种常见的函数式选项模式（Functional Options Pattern）
// This is a common Functional Options Pattern
//
// 优点 / Advantages:
//   - 可选参数：不需要的参数可以不传 / Optional parameters: no need to pass unused parameters
//   - 可扩展性：添加新选项不影响现有代码 / Extensibility: adding new options doesn't affect existing code
//   - 可读性：每个选项的意义一目了然 / Readability: each option's meaning is clear
//
// 示例 / Example:
//
//	// 定义一个自定义选项
//	// Define a custom option
//	func MyCustomOption(value string) requests.Option {
//	    return func(o *requests.Options) {
//	        o.Header.Set("X-Custom", value)
//	    }
//	}
//
//	// 使用自定义选项
//	// Use custom option
//	sess := requests.New(MyCustomOption("my-value"))
type Option func(*Options)

// newOptions 创建并初始化配置选项
// newOptions creates and initializes configuration options
//
// 参数 / Parameters:
//   - opts: 主要的配置选项列表 / Main configuration options list
//   - extends: 扩展的配置选项（会覆盖主要选项）/ Extended configuration options (override main options)
//
// 返回值 / Returns:
//   - Options: 合并后的配置选项 / Merged configuration options
func newOptions(opts []Option, extends ...Option) Options {
	opt := Options{
		URL:      "http://127.0.0.1:80",
		RawQuery: make(url.Values),
		Header:   make(http.Header),
		Timeout:  30 * time.Second,
		MaxConns: 100,
		Proxy:    http.ProxyFromEnvironment,

		OnStart:    func(s *http.Server) { log.Printf("http(s) serve %s", s.Addr) },
		OnShutdown: func(s *http.Server) { log.Printf("http shutdown") },
	}
	for _, o := range opts {
		o(&opt)
	}
	for _, o := range extends {
		o(&opt)
	}
	return opt
}

// 预定义的常用 HTTP 方法
// Pre-defined common HTTP methods
var (
	MethodGet  = Method("GET")  // GET 方法 / GET method
	MethodPost = Method("POST") // POST 方法 / POST method
)

// CertKey 设置 TLS/SSL 证书和密钥文件路径（用于HTTPS服务器）
// CertKey sets TLS/SSL certificate and key file paths (for HTTPS server)
//
// 参数 / Parameters:
//   - cert: 证书文件路径 / Certificate file path
//   - key: 密钥文件路径 / Key file path
//
// 示例 / Example:
//
//	// 创建 HTTPS 服务器
//	// Create HTTPS server
//	mux := requests.NewServeMux(
//	    requests.URL("0.0.0.0:443"),
//	    requests.CertKey("/path/to/cert.pem", "/path/to/key.pem"),
//	)
//	requests.ListenAndServe(context.Background(), mux)
func CertKey(cert, key string) Option {
	return func(o *Options) {
		o.certFile, o.keyFile = cert, key
	}
}

// MaxConns 设置最大连接数（连接池大小）
// MaxConns sets the maximum number of connections (connection pool size)
//
// 参数 / Parameters:
//   - conn: 最大连接数 / Maximum number of connections
//
// 说明 / Notes:
//   - 该值同时设置 MaxIdleConns 和 MaxIdleConnsPerHost
//   - This value sets both MaxIdleConns and MaxIdleConnsPerHost
//   - 默认值为 100 / Default is 100
//   - 适当增大可以提高并发性能 / Increasing properly can improve concurrent performance
//
// 示例 / Example:
//
//	sess := requests.New(
//	    requests.URL("https://api.example.com"),
//	    requests.MaxConns(200), // 支持200个并发连接 / Support 200 concurrent connections
//	)
func MaxConns(conn int) Option {
	return func(o *Options) {
		o.MaxConns = conn
	}
}

// Method 设置 HTTP 请求方法
// Method sets the HTTP request method
//
// 参数 / Parameters:
//   - method: HTTP 方法（GET、POST、PUT、DELETE等）/ HTTP method (GET, POST, PUT, DELETE, etc.)
//
// 示例 / Example:
//
//	// 使用预定义常量
//	// Use pre-defined constants
//	resp, _ := sess.Do(context.Background(), requests.MethodGet)
//
//	// 使用自定义方法
//	// Use custom method
//	resp, _ := sess.Do(context.Background(), requests.Method("PATCH"))
func Method(method string) Option {
	return func(o *Options) {
		o.Method = method
	}
}

// URL 设置请求的目标 URL
// URL sets the target URL for the request
//
// 支持的协议 / Supported Protocols:
//   - http:// - 标准 HTTP 协议 / Standard HTTP protocol
//   - https:// - 安全 HTTPS 协议 / Secure HTTPS protocol
//   - unix:// - Unix Domain Socket / Unix Domain Socket
//
// Unix Socket 使用说明 / Unix Socket Usage:
//
//	// 会话级别设置 socket 路径
//	// Set socket path at session level
//	sess := requests.New(requests.URL("unix:///tmp/requests.sock"))
//
//	// 请求级别设置 HTTP URL
//	// Set HTTP URL at request level
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.URL("http://localhost/api/users"),
//	    requests.Body("data"),
//	)
//
// 参数 / Parameters:
//   - url: 目标 URL 地址 / Target URL address
//
// 示例 / Example:
//
//	sess := requests.New(requests.URL("https://api.example.com"))
func URL(url string) Option {
	return func(o *Options) {
		o.URL = url
	}
}

// Path 追加 URL 路径片段
// Path appends URL path segment
//
// 特点 / Features:
//   - 可以多次调用，路径会依次追加 / Can be called multiple times, paths are appended sequentially
//   - 自动处理路径拼接 / Automatically handles path concatenation
//
// 参数 / Parameters:
//   - path: 路径片段 / Path segment
//
// 示例 / Example:
//
//	sess := requests.New(requests.URL("https://api.example.com"))
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.Path("/users"),    // https://api.example.com/users
//	    requests.Path("/123"),      // https://api.example.com/users/123
//	    requests.Path("/profile"),  // https://api.example.com/users/123/profile
//	)
func Path(path string) Option {
	return func(o *Options) {
		o.Path = append(o.Path, path)
	}
}

// Params 批量添加 URL 查询参数
// Params adds multiple URL query parameters at once
//
// 参数 / Parameters:
//   - query: 查询参数映射表 / Query parameters map
//
// 示例 / Example:
//
//	params := map[string]string{
//	    "page": "1",
//	    "size": "20",
//	    "sort": "created_at",
//	}
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.Path("/users"),
//	    requests.Params(params), // /users?page=1&size=20&sort=created_at
//	)
func Params(query map[string]string) Option {
	return func(o *Options) {
		for k, v := range query {
			o.RawQuery.Add(k, v)
		}
	}
}

// Param 添加单个 URL 查询参数（支持同名参数多值）
// Param adds a single URL query parameter (supports multiple values for same key)
//
// 参数 / Parameters:
//   - k: 参数名 / Parameter name
//   - v: 参数值（可以有多个）/ Parameter values (can be multiple)
//
// 示例 / Example:
//
//	// 添加单个值
//	// Add single value
//	requests.Param("page", "1")
//
//	// 添加多个值（同名参数）
//	// Add multiple values (same parameter name)
//	requests.Param("tags", "go", "http", "client") // tags=go&tags=http&tags=client
func Param(k string, v ...string) Option {
	return func(o *Options) {
		for _, x := range v {
			o.RawQuery.Add(k, x)
		}
	}
}

// Body 设置请求体内容
// Body sets the request body content
//
// 支持的类型 / Supported Types:
//   - string: 字符串内容 / String content
//   - []byte: 字节数组 / Byte array
//   - io.Reader: 任何读取器 / Any reader
//   - url.Values: 表单数据 / Form data
//   - struct/map: 自动序列化为JSON / Auto-serialized to JSON
//
// 参数 / Parameters:
//   - body: 请求体数据 / Request body data
//
// 示例 / Example:
//
//	// 字符串
//	// String
//	requests.Body("plain text")
//
//	// JSON（自动序列化）
//	// JSON (auto-serialized)
//	requests.Body(map[string]string{"name": "John"})
//
//	// 表单
//	// Form
//	form := url.Values{}
//	form.Set("username", "john")
//	requests.Body(form)
func Body(body any) Option {
	return func(o *Options) {
		o.body = body
	}
}

// Gzip 对请求体进行 gzip 压缩
// Gzip compresses the request body using gzip
//
// 自动设置的头部 / Automatically set headers:
//   - Accept-Encoding: gzip
//   - Content-Encoding: gzip
//
// 参数 / Parameters:
//   - body: 要压缩的内容 / Content to compress
//
// 使用场景 / Use Cases:
//   - 发送大量数据时，减少网络传输量 / Reduce network transmission when sending large data
//   - 服务器支持 gzip 解压时 / When server supports gzip decompression
//
// 示例 / Example:
//
//	largeData := strings.Repeat("data", 10000)
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.MethodPost,
//	    requests.Gzip(largeData), // 自动压缩 / Auto-compressed
//	)
func Gzip(body any) Option {
	reader, err := makeBody(body)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	defer w.Close()

	if _, err := io.Copy(w, reader); err != nil {
		panic(err)
	}

	return func(o *Options) {
		o.body = &buf
		o.Header.Add("Accept-Encoding", "gzip")
		o.Header.Add("Content-Encoding", "gzip")
	}
}

// Form 设置表单数据（自动设置 Content-Type）
// Form sets form data (automatically sets Content-Type)
//
// 自动设置的头部 / Automatically set headers:
//   - Content-Type: application/x-www-form-urlencoded
//
// 参数 / Parameters:
//   - form: 表单数据 / Form data
//
// 示例 / Example:
//
//	form := url.Values{}
//	form.Set("username", "john")
//	form.Set("password", "secret123")
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.MethodPost,
//	    requests.Path("/login"),
//	    requests.Form(form),
//	)
func Form(form url.Values) Option {
	return func(o *Options) {
		o.Header.Add("content-type", "application/x-www-form-urlencoded")
		o.body = form
	}
}

// Header 添加单个 HTTP 请求头
// Header adds a single HTTP header
//
// 参数 / Parameters:
//   - k: 请求头名称 / Header name
//   - v: 请求头值 / Header value
//
// 示例 / Example:
//
//	sess := requests.New(
//	    requests.Header("Authorization", "Bearer token123"),
//	    requests.Header("Accept", "application/json"),
//	)
func Header(k, v string) Option {
	return func(o *Options) {
		o.Header.Add(k, v)
	}
}

// Headers 批量添加 HTTP 请求头
// Headers adds multiple HTTP headers at once
//
// 参数 / Parameters:
//   - kv: 请求头映射表 / Headers map
//
// 示例 / Example:
//
//	headers := map[string]string{
//	    "Authorization": "Bearer token123",
//	    "Accept": "application/json",
//	    "X-Request-ID": "abc-123",
//	}
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.Headers(headers),
//	)
func Headers(kv map[string]string) Option {
	return func(o *Options) {
		for k, v := range kv {
			o.Header.Add(k, v)
		}
	}
}

// Cookie 添加单个 Cookie
// Cookie adds a single cookie
//
// 参数 / Parameters:
//   - cookie: Cookie 对象 / Cookie object
//
// 示例 / Example:
//
//	cookie := http.Cookie{
//	    Name:  "session_id",
//	    Value: "abc123",
//	}
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.Cookie(cookie),
//	)
func Cookie(cookie http.Cookie) Option {
	return func(o *Options) {
		o.Cookies = append(o.Cookies, cookie)
	}
}

// Cookies 批量添加 Cookies
// Cookies adds multiple cookies at once
//
// 参数 / Parameters:
//   - cookies: Cookie 对象列表 / Cookie objects list
//
// 示例 / Example:
//
//	cookies := []http.Cookie{
//	    {Name: "session_id", Value: "abc123"},
//	    {Name: "user_id", Value: "user456"},
//	}
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.Cookies(cookies...),
//	)
func Cookies(cookies ...http.Cookie) Option {
	return func(o *Options) {
		o.Cookies = append(o.Cookies, cookies...)
	}
}

// BasicAuth 设置 HTTP 基本认证
// BasicAuth sets HTTP Basic Authentication
//
// 自动设置的头部 / Automatically set headers:
//   - Authorization: Basic <base64(username:password)>
//
// 参数 / Parameters:
//   - username: 用户名 / Username
//   - password: 密码 / Password
//
// 示例 / Example:
//
//	sess := requests.New(
//	    requests.URL("https://api.example.com"),
//	    requests.BasicAuth("admin", "secret123"),
//	)
func BasicAuth(username, password string) Option {
	return Header("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}

// Timeout 设置请求超时时间
// Timeout sets the request timeout duration
//
// 参数 / Parameters:
//   - timeout: 超时时间 / Timeout duration
//
// 说明 / Notes:
//   - 默认值为 30 秒 / Default is 30 seconds
//   - 超时会返回 context deadline exceeded 错误
//   - Timeout returns context deadline exceeded error
//
// 示例 / Example:
//
//	// 会话级超时
//	// Session-level timeout
//	sess := requests.New(requests.Timeout(10*time.Second))
//
//	// 请求级超时（覆盖会话配置）
//	// Request-level timeout (overrides session config)
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.Timeout(5*time.Second),
//	)
func Timeout(timeout time.Duration) Option {
	return func(o *Options) {
		o.Timeout = timeout
	}
}

// Verify 设置是否验证 TLS/SSL 证书
// Verify sets whether to verify TLS/SSL certificates
//
// 参数 / Parameters:
//   - verify: true-验证证书，false-跳过验证 / true-verify certificates, false-skip verification
//
// 警告 / Warning:
//   - 生产环境应始终验证证书（verify=true）
//   - Production should always verify certificates (verify=true)
//   - 仅在测试或开发环境中跳过验证
//   - Skip verification only in testing or development environments
//
// 示例 / Example:
//
//	// 开发环境，跳过证书验证
//	// Development environment, skip certificate verification
//	sess := requests.New(
//	    requests.URL("https://localhost:8443"),
//	    requests.Verify(false),
//	)
func Verify(verify bool) Option {
	return func(o *Options) {
		o.Verify = verify
	}
}

// LocalAddr 绑定本地网络地址
// LocalAddr binds local network address
//
// 参数 / Parameters:
//   - addr: 本地地址 / Local address
//   - TCP: &net.TCPAddr{IP: net.ParseIP("192.168.1.100")}
//   - Unix: &net.UnixAddr{Net: "unix", Name: "/tmp/socket"}
//
// 使用场景 / Use Cases:
//   - 服务器有多个网卡时，指定出口IP / Specify outbound IP when server has multiple NICs
//   - 需要使用Unix Domain Socket / Need to use Unix Domain Socket
//
// 示例 / Example:
//
//	// 绑定特定IP
//	// Bind to specific IP
//	localAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.100")}
//	sess := requests.New(
//	    requests.URL("https://api.example.com"),
//	    requests.LocalAddr(localAddr),
//	)
func LocalAddr(addr net.Addr) Option {
	return func(o *Options) {
		o.LocalAddr = addr
	}
}

// Stream 启用流式处理模式（用于大文件下载或实时数据流）
// Stream enables streaming mode (for large file downloads or real-time data streams)
//
// 参数 / Parameters:
//   - stream: 流处理回调函数 func(lineNumber int64, lineContent []byte) error
//
// 特点 / Features:
//   - 边接收边处理，不缓存全部内容 / Process while receiving, no full content caching
//   - 按行分割数据 / Split data by lines
//   - 适合大文件和实时流 / Suitable for large files and real-time streams
//
// 示例 / Example:
//
//	// 下载大文件并实时处理
//	// Download large file and process in real-time
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.Stream(func(lineNum int64, line []byte) error {
//	        fmt.Printf("Line %d: %s", lineNum, line)
//	        return nil
//	    }),
//	)
func Stream(stream func(int64, []byte) error) Option {
	return func(o *Options) {
		o.HttpRoundTripper = append(o.HttpRoundTripper, streamRoundTrip(stream))
	}
}

// Host 设置请求的 Host 头（用于虚拟主机或代理场景）
// Host sets the request Host header (for virtual host or proxy scenarios)
//
// 参数 / Parameters:
//   - host: 主机名 / Host name
//
// 说明 / Notes:
//   - 在客户端，Host字段（可选地）用来重写请求的Host头
//   - On client side, Host field (optionally) overrides the request Host header
//   - 如果为空，Request.Write 方法会使用 URL 字段的 Host
//   - If empty, Request.Write method uses Host from URL field
//
// 使用场景 / Use Cases:
//   - 通过IP访问，但需要指定域名（绕过DNS）
//   - Access by IP but need to specify domain name (bypass DNS)
//   - 虚拟主机环境 / Virtual host environment
//
// 示例 / Example:
//
//	// 直接访问IP，但设置Host头
//	// Access by IP directly but set Host header
//	resp, _ := sess.DoRequest(context.Background(),
//	    requests.URL("http://192.168.1.100"),
//	    requests.Host("api.example.com"),
//	)
func Host(host string) Option {
	return func(o *Options) {
		o.HttpRoundTripper = append(o.HttpRoundTripper, func(next http.RoundTripper) http.RoundTripper {
			return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				r.Host = host
				r.Header.Set("Host", host)
				return next.RoundTrip(r)
			})
		})
	}
}

// Proxy 设置代理服务器地址
// Proxy sets the proxy server address
//
// 参数 / Parameters:
//   - addr: 代理地址 / Proxy address
//   - HTTP代理: "http://proxy.example.com:8080"
//   - HTTPS代理: "https://proxy.example.com:8080"
//   - SOCKS5代理: "socks5://proxy.example.com:1080"
//
// 环境变量方式 / Environment Variable Alternative:
//   - os.Setenv("HTTP_PROXY", "http://127.0.0.1:8080")
//   - os.Setenv("HTTPS_PROXY", "https://127.0.0.1:8080")
//
// 示例 / Example:
//
//	// 使用HTTP代理
//	// Use HTTP proxy
//	sess := requests.New(
//	    requests.URL("https://api.example.com"),
//	    requests.Proxy("http://proxy.company.com:8080"),
//	)
//
//	// 使用SOCKS5代理
//	// Use SOCKS5 proxy
//	sess := requests.New(
//	    requests.Proxy("socks5://127.0.0.1:1080"),
//	)
func Proxy(addr string) Option {
	return func(o *Options) {
		if addr == "" {
			return
		}
		if proxyURL, err := url.Parse(addr); err == nil {
			o.Proxy = http.ProxyURL(proxyURL)
		} else {
			panic("parse proxy addr: " + err.Error())
		}
	}
}

// Setup 注册客户端中间件（用于请求/响应处理）
// Setup registers client middleware (for request/response processing)
//
// 参数 / Parameters:
//   - fn: 中间件函数列表 / Middleware function list
//
// 中间件执行顺序 / Middleware Execution Order:
//   - 按照注册的反序执行 / Executed in reverse order of registration
//   - 最先注册的最外层，最后注册的最内层 / First registered is outermost, last is innermost
//
// 示例 / Example:
//
//	// 自定义中间件：添加请求ID
//	// Custom middleware: add request ID
//	requestIDMiddleware := func(next http.RoundTripper) http.RoundTripper {
//	    return requests.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
//	        req.Header.Set("X-Request-ID", uuid.New().String())
//	        return next.RoundTrip(req)
//	    })
//	}
//
//	sess := requests.New(
//	    requests.URL("https://api.example.com"),
//	    requests.Setup(requestIDMiddleware),
//	)
func Setup(fn ...func(tripper http.RoundTripper) http.RoundTripper) Option {
	return func(o *Options) {
		for _, f := range fn {
			o.HttpRoundTripper = append([]func(http.RoundTripper) http.RoundTripper{f}, o.HttpRoundTripper...)
		}
	}
}

// Use 注册服务器中间件（用于HTTP服务器请求处理）
// Use registers server middleware (for HTTP server request processing)
//
// 参数 / Parameters:
//   - fn: 中间件函数列表 / Middleware function list
//
// 常见用途 / Common Uses:
//   - 日志记录 / Logging
//   - 认证授权 / Authentication/Authorization
//   - CORS处理 / CORS handling
//   - 请求限流 / Rate limiting
//
// 示例 / Example:
//
//	// 自定义日志中间件
//	// Custom logging middleware
//	loggingMiddleware := func(next http.Handler) http.Handler {
//	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//	        log.Printf("Request: %s %s", r.Method, r.URL.Path)
//	        next.ServeHTTP(w, r)
//	    })
//	}
//
//	mux := requests.NewServeMux(
//	    requests.URL("0.0.0.0:8080"),
//	    requests.Use(loggingMiddleware),
//	)
func Use(fn ...func(http.Handler) http.Handler) Option {
	return func(o *Options) {
		for _, f := range fn {
			o.HttpHandler = append([]func(http.Handler) http.Handler{f}, o.HttpHandler...)
		}
	}
}

// RoundTripper 设置自定义的 HTTP 传输层
// RoundTripper sets custom HTTP transport layer
//
// 参数 / Parameters:
//   - tr: 自定义的 RoundTripper 实现 / Custom RoundTripper implementation
//
// 使用场景 / Use Cases:
//   - 完全自定义传输行为 / Fully customize transport behavior
//   - 使用第三方传输实现 / Use third-party transport implementation
//   - 高级连接池管理 / Advanced connection pool management
//
// 示例 / Example:
//
//	// 使用自定义传输层
//	// Use custom transport
//	customTransport := &http.Transport{
//	    MaxIdleConns:        200,
//	    MaxIdleConnsPerHost: 100,
//	    IdleConnTimeout:     90 * time.Second,
//	}
//
//	sess := requests.New(
//	    requests.RoundTripper(customTransport),
//	)
func RoundTripper(tr http.RoundTripper) Option {
	return func(o *Options) {
		o.Transport = tr
	}
}

// Logf 启用请求/响应日志记录
// Logf enables request/response logging
//
// 参数 / Parameters:
//   - f: 日志处理函数 / Log handler function
//
// 功能 / Features:
//   - 同时记录客户端和服务器端的请求 / Logs both client and server requests
//   - 记录详细的统计信息 / Records detailed statistics
//
// 示例 / Example:
//
//	// 使用默认日志函数
//	// Use default log function
//	sess := requests.New(
//	    requests.URL("https://api.example.com"),
//	    requests.Logf(requests.LogS),
//	)
//
//	// 自定义日志函数
//	// Custom log function
//	customLog := func(ctx context.Context, stat *requests.Stat) {
//	    log.Printf("Request to %s took %dms", stat.Request.URL, stat.Cost)
//	}
//	sess := requests.New(requests.Logf(customLog))
func Logf(f func(context.Context, *Stat)) Option {
	return func(o *Options) {
		o.HttpRoundTripper = append([]func(http.RoundTripper) http.RoundTripper{printRoundTripper(f)}, o.HttpRoundTripper...)
		o.HttpHandler = append([]func(http.Handler) http.Handler{printHandler(f)}, o.HttpHandler...)
	}
}

// OnStart 设置服务器启动时的回调函数
// OnStart sets the callback function when server starts
//
// 参数 / Parameters:
//   - f: 启动回调函数 / Start callback function
//
// 示例 / Example:
//
//	mux := requests.NewServeMux(
//	    requests.URL("0.0.0.0:8080"),
//	    requests.OnStart(func(s *http.Server) {
//	        log.Printf("Server started on %s", s.Addr)
//	    }),
//	)
func OnStart(f func(*http.Server)) Option {
	return func(o *Options) {
		o.OnStart = f
	}
}

// OnShutdown 设置服务器关闭时的回调函数
// OnShutdown sets the callback function when server shuts down
//
// 参数 / Parameters:
//   - f: 关闭回调函数 / Shutdown callback function
//
// 示例 / Example:
//
//	mux := requests.NewServeMux(
//	    requests.URL("0.0.0.0:8080"),
//	    requests.OnShutdown(func(s *http.Server) {
//	        log.Printf("Server %s shutting down", s.Addr)
//	        // 清理资源 / Clean up resources
//	    }),
//	)
func OnShutdown(f func(*http.Server)) Option {
	return func(o *Options) {
		o.OnShutdown = f
	}
}

// EnableHTTP3 启用 HTTP/3 协议支持
// Enable HTTP/3 protocol using QUIC transport
//
// HTTP/3 特性 (HTTP/3 Features):
//   - 基于 UDP 的 QUIC 协议 (QUIC protocol over UDP)
//   - 内置 TLS 1.3 加密 (Built-in TLS 1.3 encryption)
//   - 0-RTT 连接建立 (0-RTT connection establishment)
//   - 多路复用无队头阻塞 (Multiplexing without head-of-line blocking)
//   - 连接迁移支持 (Connection migration support)
//
// 客户端使用示例 (Client usage example):
//
//	sess := requests.New(
//	    requests.URL("https://example.com"),
//	    requests.EnableHTTP3(true),
//	)
//	resp, err := sess.DoRequest(context.TODO())
//
// 服务端使用示例 (Server usage example):
//
//	mux := requests.NewServeMux()
//	mux.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
//	    fmt.Fprintf(w, "pong\n")
//	})
//	err := requests.ListenAndServeHTTP3(
//	    context.Background(),
//	    mux,
//	    requests.URL(":8443"),
//	    requests.CertKey("cert.pem", "key.pem"),
//	)
//
// 注意事项 (Important notes):
//   - 客户端：自动使用 HTTP/3，无需证书 (Client: Auto uses HTTP/3, no cert needed)
//   - 服务端：必须提供 TLS 证书和密钥 (Server: Must provide TLS cert and key)
//   - HTTP/3 默认使用 443 端口 (HTTP/3 uses port 443 by default)
func EnableHTTP3(enable bool) Option {
	return func(o *Options) {
		o.EnableHTTP3 = enable
	}
}
