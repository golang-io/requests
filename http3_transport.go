package requests

import (
	"crypto/tls"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// HTTP3RoundTripper HTTP/3 客户端传输层实现
// HTTP/3 uses QUIC as transport protocol, providing better performance than TCP-based HTTP/2
type HTTP3RoundTripper struct {
	transport *http3.Transport
}

// newHTTP3Transport 创建一个新的 HTTP/3 传输层
// Creates a new HTTP/3 transport with QUIC configuration
func newHTTP3Transport(opts ...Option) *HTTP3RoundTripper {
	options := newOptions(opts)

	// 配置 QUIC 参数
	// Configure QUIC parameters for optimal performance
	quicConfig := &quic.Config{
		// 最大空闲超时时间
		// Maximum idle timeout for connections
		MaxIdleTimeout: 120 * time.Second,

		// 启用数据报支持（用于某些扩展）
		// Enable datagram support for extensions
		EnableDatagrams: false,

		// 初始流窗口大小
		// Initial stream window size
		InitialStreamReceiveWindow: 1 << 20, // 1 MB

		// 初始连接窗口大小
		// Initial connection window size
		InitialConnectionReceiveWindow: 1 << 21, // 2 MB

		// 最大接收流缓冲大小
		// Maximum receive stream buffer
		MaxStreamReceiveWindow: 6 << 20, // 6 MB

		// 最大接收连接缓冲大小
		// Maximum receive connection buffer
		MaxConnectionReceiveWindow: 15 << 20, // 15 MB

		// 允许的并发流数量
		// Number of concurrent streams allowed
		MaxIncomingStreams:    100,
		MaxIncomingUniStreams: 100,

		// 保持连接活跃
		// Keep connections alive
		KeepAlivePeriod: 10 * time.Second,
	}

	// 配置 TLS
	// Configure TLS for secure connections
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !options.Verify,  // 是否验证服务器证书 / Whether to verify server certificates
		NextProtos:         []string{"h3"},   // HTTP/3 ALPN 协议标识 / HTTP/3 ALPN protocol identifier
		MinVersion:         tls.VersionTLS13, // HTTP/3 requires TLS 1.3
	}

	// 创建 HTTP/3 Transport
	// Create HTTP/3 Transport with configuration
	transport := &http3.Transport{
		TLSClientConfig: tlsConfig,
		QUICConfig:      quicConfig,
		// 禁用压缩以保持与现有行为一致
		// Disable compression to maintain consistency with existing behavior
		DisableCompression: true,
		// 连接池配置
		// Connection pool configuration
		MaxResponseHeaderBytes: 10 << 20, // 10 MB
	}

	return &HTTP3RoundTripper{transport: transport}
}

// RoundTrip 实现 http.RoundTripper 接口
// Implements the http.RoundTripper interface for HTTP/3 requests
func (t *HTTP3RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.transport.RoundTrip(req)
}

// Close 关闭 HTTP/3 传输层并释放资源
// Closes the HTTP/3 transport and releases resources
func (t *HTTP3RoundTripper) Close() error {
	return t.transport.Close()
}
