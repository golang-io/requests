package requests

import (
	"context"
	"net"
	"net/url"
	"time"
)

// socket 创建一个网络连接（内部函数）
// socket creates a network connection (internal function)
//
// 参数 / Parameters:
//   - ctx: 上下文（用于控制连接超时和取消）/ Context (for controlling timeout and cancellation)
//   - src: 本地地址（可选，用于绑定出口IP）/ Local address (optional, for binding outbound IP)
//   - network: 网络类型（"tcp", "tcp4", "tcp6", "unix"）/ Network type ("tcp", "tcp4", "tcp6", "unix")
//   - address: 目标地址 / Target address
//   - timeout: 连接超时时间 / Connection timeout
//
// 返回值 / Returns:
//   - net.Conn: 网络连接 / Network connection
//   - error: 连接错误 / Connection error
//
// 配置说明 / Configuration:
//   - Timeout: 连接超时（默认 10秒）/ Connection timeout (default 10s)
//   - KeepAlive: TCP 保活间隔（60秒）/ TCP keep-alive interval (60s)
//   - LocalAddr: 本地地址绑定 / Local address binding
//   - Resolver: DNS 解析器配置 / DNS resolver configuration
//
// 支持的网络类型 / Supported Network Types:
//   - tcp: TCP 网络（IPv4 或 IPv6）/ TCP network (IPv4 or IPv6)
//   - tcp4: 仅 TCP IPv4 / TCP IPv4 only
//   - tcp6: 仅 TCP IPv6 / TCP IPv6 only
//   - unix: Unix Domain Socket / Unix Domain Socket
//
// 示例 / Example:
//
//	// TCP 连接 / TCP connection
//	conn, _ := socket(ctx, nil, "tcp", "example.com:80", 10*time.Second)
//
//	// Unix Socket 连接 / Unix Socket connection
//	conn, _ := socket(ctx, nil, "unix", "/tmp/app.sock", 10*time.Second)
func socket(ctx context.Context, src net.Addr, network, address string, timeout time.Duration) (net.Conn, error) {
	dialer := net.Dialer{
		Timeout:   timeout,          // TCP 连接超时 / TCP connection timeout
		KeepAlive: 60 * time.Second, // TCP 保活间隔（维持连接活性）/ TCP keep-alive interval (maintain connection)
		LocalAddr: src,              // 本地地址绑定（可选）/ Local address binding (optional)
		Resolver: &net.Resolver{ // DNS 解析器配置 / DNS resolver configuration
			PreferGo:     true,  // 优先使用 Go 的 DNS 解析器 / Prefer Go's DNS resolver
			StrictErrors: false, // 容忍 DNS 解析错误 / Tolerate DNS resolution errors
		},
	}
	return dialer.DialContext(ctx, network, address)
}

// Socket 创建一个网络连接（公开函数）
// Socket creates a network connection (public function)
//
// 参数 / Parameters:
//   - ctx: 上下文 / Context
//   - opts: 配置选项 / Configuration options
//
// 返回值 / Returns:
//   - net.Conn: 网络连接 / Network connection
//   - error: 连接错误 / Connection error
//
// 使用说明 / Usage:
//   - 通过 requests.URL() 指定连接地址 / Specify connection address via requests.URL()
//   - 支持 TCP 和 Unix Socket / Supports TCP and Unix Socket
//   - 可以通过 requests.LocalAddr() 绑定本地地址 / Can bind local address via requests.LocalAddr()
//   - 可以通过 requests.Timeout() 设置超时 / Can set timeout via requests.Timeout()
//
// 示例 / Example:
//
//	// TCP 连接 / TCP connection
//	conn, err := requests.Socket(context.Background(),
//	    requests.URL("tcp://example.com:80"),
//	    requests.Timeout(10*time.Second),
//	)
//
//	// Unix Socket 连接 / Unix Socket connection
//	conn, err := requests.Socket(context.Background(),
//	    requests.URL("unix:///tmp/app.sock"),
//	)
//
//	// 绑定本地地址 / Bind local address
//	localAddr := &net.TCPAddr{IP: net.ParseIP("192.168.1.100")}
//	conn, err := requests.Socket(context.Background(),
//	    requests.URL("tcp://example.com:80"),
//	    requests.LocalAddr(localAddr),
//	)
func Socket(ctx context.Context, opts ...Option) (net.Conn, error) {
	options := newOptions(opts)

	// 解析 URL 获取网络类型和地址 / Parse URL to get network type and address
	u, err := url.Parse(options.URL)
	if err != nil {
		return nil, err
	}

	// 创建连接 / Create connection
	// u.Scheme: 网络类型（tcp, unix）/ Network type (tcp, unix)
	// u.Host: 目标地址 / Target address
	return socket(ctx, options.LocalAddr, u.Scheme, u.Host, options.Timeout)
}
