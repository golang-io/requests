package requests

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

// generateTestCertificate 生成测试用的自签名证书
// Generates a self-signed certificate for testing
func generateTestCertificate(certFile, keyFile string) error {
	// 生成私钥
	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}

	// 证书模板
	// Certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Organization"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	// 创建自签名证书
	// Create self-signed certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %v", err)
	}

	// 写入证书文件
	// Write certificate file
	certOut, err := os.Create(certFile)
	if err != nil {
		return fmt.Errorf("failed to create cert file: %v", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write cert: %v", err)
	}

	// 写入私钥文件
	// Write private key file
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return fmt.Errorf("failed to create key file: %v", err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %v", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write key: %v", err)
	}

	return nil
}

// TestHTTP3Server 测试 HTTP/3 服务器
// Tests HTTP/3 server functionality
func TestHTTP3Server(t *testing.T) {
	// 生成测试证书
	// Generate test certificate
	certFile := "test_cert.pem"
	keyFile := "test_key.pem"
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	if err := generateTestCertificate(certFile, keyFile); err != nil {
		t.Fatalf("Failed to generate test certificate: %v", err)
	}

	// 创建服务器
	// Create server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mux := NewServeMux()
	mux.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "pong")
	})

	// 启动 HTTP/3 服务器
	// Start HTTP/3 server
	go func() {
		err := ListenAndServeHTTP3(
			ctx,
			mux,
			URL("127.0.0.1:8443"),
			CertKey(certFile, keyFile),
		)
		if err != nil && err != context.DeadlineExceeded {
			t.Logf("Server error: %v", err)
		}
	}()

	// 等待服务器启动
	// Wait for server to start
	time.Sleep(500 * time.Millisecond)

	// 创建 HTTP/3 客户端
	// Create HTTP/3 client
	sess := New(
		URL("https://127.0.0.1:8443"),
		EnableHTTP3(true),
		Verify(false), // 跳过证书验证（测试环境）/ Skip cert verification (test env)
	)

	// 发送请求
	// Send request
	resp, err := sess.DoRequest(context.TODO(), Path("/ping"))
	if err != nil {
		t.Fatalf("Failed to send HTTP/3 request: %v", err)
	}

	// 验证响应
	// Verify response
	if resp.Content.String() != "pong" {
		t.Errorf("Expected 'pong', got '%s'", resp.Content.String())
	}

	t.Logf("✓ HTTP/3 request successful")
}

// TestHTTP3Transport 测试 HTTP/3 传输层
// Tests HTTP/3 transport layer
func TestHTTP3Transport(t *testing.T) {
	transport := newHTTP3Transport(Verify(false))
	if transport == nil {
		t.Fatal("Failed to create HTTP/3 transport")
	}
	defer transport.Close()

	t.Logf("✓ HTTP/3 transport created successfully")
}

// TestHTTP3ClientWithPublicServer 测试使用公共 HTTP/3 服务器
// Tests HTTP/3 client with public HTTP/3 server
func TestHTTP3ClientWithPublicServer(t *testing.T) {
	// 跳过此测试，除非显式请求
	// Skip this test unless explicitly requested
	if testing.Short() {
		t.Skip("Skipping public server test in short mode")
	}

	// 使用支持 HTTP/3 的公共服务器（如 Cloudflare）
	// Use a public server that supports HTTP/3 (like Cloudflare)
	sess := New(
		URL("https://cloudflare-quic.com"),
		EnableHTTP3(true),
		Verify(true),
		Timeout(10*time.Second),
	)

	resp, err := sess.DoRequest(context.TODO())
	if err != nil {
		t.Logf("Public HTTP/3 request failed (this is OK if network is unavailable): %v", err)
		return
	}

	if resp.Response.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.Response.StatusCode)
	}

	t.Logf("✓ Public HTTP/3 server request successful")
}

// BenchmarkHTTP3vsHTTP2 比较 HTTP/3 和 HTTP/2 性能
// Benchmarks HTTP/3 vs HTTP/2 performance
func BenchmarkHTTP3vsHTTP2(b *testing.B) {
	// 生成测试证书
	// Generate test certificate
	certFile := "bench_cert.pem"
	keyFile := "bench_key.pem"
	defer os.Remove(certFile)
	defer os.Remove(keyFile)

	if err := generateTestCertificate(certFile, keyFile); err != nil {
		b.Fatalf("Failed to generate test certificate: %v", err)
	}

	// 创建服务器
	// Create server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mux := NewServeMux()
	mux.Route("/bench", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "benchmark")
	})

	// 启动 HTTP/3 服务器
	// Start HTTP/3 server
	go func() {
		ListenAndServeHTTP3(ctx, mux,
			URL("127.0.0.1:9443"),
			CertKey(certFile, keyFile),
		)
	}()

	// 启动 HTTP/2 服务器
	// Start HTTP/2 server
	go func() {
		ListenAndServe(ctx, mux,
			URL("127.0.0.1:9444"),
			CertKey(certFile, keyFile),
		)
	}()

	time.Sleep(500 * time.Millisecond)

	b.Run("HTTP3", func(b *testing.B) {
		sess := New(
			URL("https://127.0.0.1:9443"),
			EnableHTTP3(true),
			Verify(false),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sess.DoRequest(context.TODO(), Path("/bench"))
		}
	})

	b.Run("HTTP2", func(b *testing.B) {
		// 配置 HTTP/2 客户端
		// Configure HTTP/2 client
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		sess := New(
			URL("https://127.0.0.1:9444"),
			RoundTripper(transport),
		)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			sess.DoRequest(context.TODO(), Path("/bench"))
		}
	})
}

// Example_hTTP3Client 展示 HTTP/3 客户端使用示例
// Example of HTTP/3 client usage
func Example_hTTP3Client() {
	// 创建启用 HTTP/3 的客户端会话
	// Create a session with HTTP/3 enabled
	sess := New(
		URL("https://example.com"),
		EnableHTTP3(true),
		Timeout(10*time.Second),
	)

	// 发送请求
	// Send request
	resp, err := sess.DoRequest(context.TODO())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Status: %d\n", resp.Response.StatusCode)
	fmt.Printf("Protocol: %s\n", resp.Response.Proto)
}

// Example_hTTP3Server 展示 HTTP/3 服务器使用示例
// Example of HTTP/3 server usage
func Example_hTTP3Server() {
	// 创建路由
	// Create router
	mux := NewServeMux()
	mux.Route("/ping", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "pong\n")
	})
	mux.Route("/echo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "You sent: %s\n", r.URL.Query().Get("msg"))
	})

	// 启动 HTTP/3 服务器
	// Start HTTP/3 server
	// 注意：需要提供有效的 TLS 证书和密钥
	// Note: Valid TLS certificate and key are required
	ctx := context.Background()
	err := ListenAndServeHTTP3(
		ctx,
		mux,
		URL(":8443"),
		CertKey("cert.pem", "key.pem"),
	)
	if err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
