package requests

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WarpRoundTripper 包装一个 http.RoundTripper 实例
// WarpRoundTripper wraps an http.RoundTripper instance
//
// 参数 / Parameters:
//   - next: 下一个 RoundTripper / Next RoundTripper
//
// 返回值 / Returns:
//   - func(http.RoundTripper) http.RoundTripper: 装饰器函数 / Decorator function
//
// 说明 / Notes:
//   - 这是一个装饰器工厂函数 / This is a decorator factory function
//   - 用于为现有 RoundTripper 添加额外功能 / Used to add additional functionality to existing RoundTripper
//
// 示例 / Example:
//
//	middleware := requests.WarpRoundTripper(customRoundTripper)
//	sess := requests.New(requests.Setup(middleware))
func WarpRoundTripper(next http.RoundTripper) func(http.RoundTripper) http.RoundTripper {
	return func(http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return next.RoundTrip(r)
		})
	}
}

// RoundTripperFunc 是 http.RoundTripper 接口的函数式实现
// RoundTripperFunc is a functional implementation of the http.RoundTripper interface
//
// 说明 / Notes:
//   - 允许将普通函数转换为 RoundTripper 接口 / Allows converting regular functions to RoundTripper interface
//   - 便于函数式扩展 / Facilitates functional extensions
//   - 是实现中间件的核心类型 / Core type for implementing middleware
//
// 示例 / Example:
//
//	rt := requests.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
//	    // 添加自定义请求头 / Add custom header
//	    req.Header.Set("X-Custom", "value")
//	    // 调用下一个处理器 / Call next handler
//	    return next.RoundTrip(req)
//	})
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip 实现 http.RoundTripper 接口
// RoundTrip implements the http.RoundTripper interface
//
// 参数 / Parameters:
//   - r: HTTP 请求 / HTTP request
//
// 返回值 / Returns:
//   - *http.Response: HTTP 响应 / HTTP response
//   - error: 错误信息 / Error
//
// 说明 / Notes:
//   - 直接调用底层函数完成请求发送和响应接收
//   - Directly calls the underlying function to complete request/response
func (fn RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

// newTransport 创建一个新的 Transport 实例
// newTransport creates a new Transport instance
//
// 参数 / Parameters:
//   - opts: 配置选项 / Configuration options
//
// 返回值 / Returns:
//   - *http.Transport: 配置好的传输层 / Configured transport
//
// 配置说明 / Configuration:
//   - 连接池管理 / Connection pool management
//   - 超时设置 / Timeout settings
//   - TLS 配置 / TLS configuration
//   - 代理支持 / Proxy support
//   - Keep-Alive / Keep-Alive
//   - Unix Socket 支持 / Unix Socket support
//
// 示例 / Example:
//
//	// 内部使用，通常不需要直接调用
//	// Internal use, usually no need to call directly
//	transport := newTransport(
//	    requests.MaxConns(200),
//	    requests.Verify(true),
//	)
func newTransport(opts ...Option) *http.Transport {
	options := newOptions(opts)
	return &http.Transport{
		// Proxy 设置代理函数 / Proxy sets the proxy function
		// 默认从环境变量读取 / Defaults to reading from environment variables
		Proxy: options.Proxy,

		// DialContext 自定义连接创建逻辑 / DialContext customizes connection creation logic
		// 支持 Unix domain sockets 和 TCP 连接 / Supports Unix domain sockets and TCP connections
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 处理 Unix domain socket 连接 / Handle Unix domain socket connections
			if strings.HasPrefix(options.URL, "unix://") {
				u, err := url.Parse(options.URL)
				if err != nil {
					return nil, err
				}
				// 覆盖网络类型和地址 / Override network type and address
				network, addr = u.Scheme, u.Path
			}
			// 创建连接 / Create connection
			return socket(ctx, options.LocalAddr, network, addr, 10*time.Second)
		},

		// 连接池配置 / Connection pool configuration
		MaxIdleConns:        options.MaxConns,  // 最大空闲连接数 / Maximum number of idle connections
		MaxIdleConnsPerHost: options.MaxConns,  // 每个主机最大空闲连接数 / Maximum idle connections per host
		IdleConnTimeout:     120 * time.Second, // 空闲连接超时时间 / Idle connection timeout

		// 连接行为配置 / Connection behavior configuration
		DisableCompression: true,  // 禁用压缩（由应用层控制）/ Disable compression (controlled by app layer)
		DisableKeepAlives:  false, // 启用 Keep-Alive（连接复用）/ Enable Keep-Alive (connection reuse)

		// TLS 配置 / TLS configuration
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: !options.Verify, // 是否验证服务器证书 / Whether to verify server certificates
		},
	}
}
