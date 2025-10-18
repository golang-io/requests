package requests

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// HTTP3Server HTTP/3 服务器实现
// HTTP/3 server implementation using QUIC protocol
type HTTP3Server struct {
	options Options
	server  *http3.Server
}

// NewHTTP3Server 创建一个新的 HTTP/3 服务器
// Creates a new HTTP/3 server with the given handler and options
//
// 注意事项 (Important notes):
// 1. HTTP/3 需要 TLS 证书和密钥 (HTTP/3 requires TLS certificate and key)
// 2. HTTP/3 使用 UDP 协议而非 TCP (HTTP/3 uses UDP instead of TCP)
// 3. 必须使用 HTTPS 地址 (Must use HTTPS address)
func NewHTTP3Server(ctx context.Context, h http.Handler, opts ...Option) *HTTP3Server {
	s := &HTTP3Server{}

	// 获取 ServeMux 配置
	// Get ServeMux configuration
	mux, ok := h.(*ServeMux)
	if !ok {
		mux = NewServeMux()
	}
	s.options = newOptions(mux.opts, opts...)

	// 检查证书配置
	// Check certificate configuration
	if s.options.certFile == "" || s.options.keyFile == "" {
		panic("HTTP/3 requires TLS certificate and key files. Use CertKey() option to set them.")
	}

	// 配置 QUIC 参数
	// Configure QUIC parameters for server
	quicConfig := &quic.Config{
		// 最大空闲超时时间
		// Maximum idle timeout
		MaxIdleTimeout: 120 * time.Second,

		// 启用数据报支持
		// Enable datagram support
		EnableDatagrams: false,

		// 流控制窗口大小
		// Stream flow control window size
		InitialStreamReceiveWindow:     1 << 20,  // 1 MB
		InitialConnectionReceiveWindow: 1 << 21,  // 2 MB
		MaxStreamReceiveWindow:         6 << 20,  // 6 MB
		MaxConnectionReceiveWindow:     15 << 20, // 15 MB

		// 允许的并发流数量
		// Number of concurrent streams
		MaxIncomingStreams:    int64(s.options.MaxConns),
		MaxIncomingUniStreams: int64(s.options.MaxConns),

		// 保持活跃
		// Keep alive
		KeepAlivePeriod: 10 * time.Second,
	}

	// 加载 TLS 证书
	// Load TLS certificate
	cert, err := tls.LoadX509KeyPair(s.options.certFile, s.options.keyFile)
	if err != nil {
		panic("failed to load TLS certificate: " + err.Error())
	}

	// 配置 TLS
	// Configure TLS for HTTP/3
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3"}, // HTTP/3 ALPN 标识 / HTTP/3 ALPN identifier
		MinVersion:   tls.VersionTLS13,
	}

	// 解析服务器地址
	// Parse server address
	addr := s.options.URL
	if addr == "" {
		addr = ":443" // HTTP/3 默认端口 / Default HTTP/3 port
	}

	// 创建 HTTP/3 服务器
	// Create HTTP/3 server instance
	s.server = &http3.Server{
		Addr:       addr,
		Handler:    h,
		TLSConfig:  tlsConfig,
		QUICConfig: quicConfig,
		// 最大响应头大小
		// Maximum response header size
		MaxHeaderBytes: 10 << 20, // 10 MB
	}

	// 启动和关闭回调
	// Start and shutdown callbacks
	s.options.OnStart(&http.Server{Addr: addr, Handler: h})
	go s.Shutdown(ctx)

	return s
}

// Shutdown 优雅关闭 HTTP/3 服务器
// Gracefully shuts down the HTTP/3 server
func (s *HTTP3Server) Shutdown(ctx context.Context) error {
	<-ctx.Done()
	err := s.server.Close()
	// 执行关闭回调
	// Execute shutdown callback
	s.options.OnShutdown(&http.Server{Addr: s.server.Addr})
	return err
}

// ListenAndServe 启动 HTTP/3 服务器并监听请求
// Starts the HTTP/3 server and listens for requests
//
// 注意：HTTP/3 服务器监听 UDP 端口，而非 TCP
// Note: HTTP/3 server listens on UDP port, not TCP
func (s *HTTP3Server) ListenAndServe() error {
	// ListenAndServe 会自动使用配置的 TLS 证书
	// ListenAndServe automatically uses the configured TLS certificate
	return s.server.ListenAndServe()
}

// ListenAndServeHTTP3 启动 HTTP/3 服务器的便捷函数
// Convenience function to start an HTTP/3 server
//
// 使用示例 (Usage example):
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
func ListenAndServeHTTP3(ctx context.Context, h http.Handler, opts ...Option) error {
	s := NewHTTP3Server(ctx, h, opts...)
	return s.ListenAndServe()
}
