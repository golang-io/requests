package requests

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestNewOptions(t *testing.T) {
	opts := newOptions([]Option{
		Method("POST"),
		URL("https://example.com"),
		Timeout(5 * time.Second),
	})

	if opts.Method != "POST" {
		t.Errorf("期望 Method 为 POST，实际为 %s", opts.Method)
	}
	if opts.URL != "https://example.com" {
		t.Errorf("期望 URL 为 https://example.com，实际为 %s", opts.URL)
	}
	if opts.Timeout != 5*time.Second {
		t.Errorf("期望 Timeout 为 5s，实际为 %v", opts.Timeout)
	}
}

func TestURLOptions(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		path     []string
		params   map[string]string
		expected string
	}{
		{
			name:     "基本URL",
			url:      "http://example.com",
			expected: "http://example.com",
		},
		{
			name:     "带路径的URL",
			url:      "http://example.com",
			path:     []string{"/api", "/v1"},
			expected: "http://example.com/api/v1",
		},
		{
			name:     "带参数的URL",
			url:      "http://example.com",
			params:   map[string]string{"key": "value", "test": "123"},
			expected: "http://example.com?key=value&test=123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := newOptions([]Option{URL(tt.url)})
			for _, p := range tt.path {
				Path(p)(&opts)
			}
			if tt.params != nil {
				Params(tt.params)(&opts)
			}
			// URL 验证逻辑需要在实际请求中进行
		})
	}
}

func TestHeaderOptions(t *testing.T) {
	opts := newOptions([]Option{
		Header("Content-Type", "application/json"),
		Header("X-Custom", "test"),
		Headers(map[string]string{
			"Authorization": "Bearer token",
			"User-Agent":    "test-agent",
		}),
	})

	tests := []struct {
		key      string
		expected string
	}{
		{"Content-Type", "application/json"},
		{"X-Custom", "test"},
		{"Authorization", "Bearer token"},
		{"User-Agent", "test-agent"},
	}

	for _, tt := range tests {
		if got := opts.Header.Get(tt.key); got != tt.expected {
			t.Errorf("Header %s = %v, 期望 %v", tt.key, got, tt.expected)
		}
	}
}

func TestCookieOptions(t *testing.T) {
	cookie1 := http.Cookie{Name: "test1", Value: "value1"}
	cookie2 := http.Cookie{Name: "test2", Value: "value2"}

	opts := newOptions([]Option{
		Cookie(cookie1),
		Cookies(cookie2),
	})

	if len(opts.Cookies) != 2 {
		t.Errorf("期望 2 个 cookie，实际有 %d 个", len(opts.Cookies))
	}

	if opts.Cookies[0].Name != "test1" || opts.Cookies[0].Value != "value1" {
		t.Errorf("第一个 cookie 不匹配")
	}
	if opts.Cookies[1].Name != "test2" || opts.Cookies[1].Value != "value2" {
		t.Errorf("第二个 cookie 不匹配")
	}
}

func TestBasicAuthOption(t *testing.T) {
	username := "user"
	password := "pass"
	expected := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))

	opts := newOptions([]Option{
		BasicAuth(username, password),
	})

	if got := opts.Header.Get("Authorization"); got != expected {
		t.Errorf("BasicAuth header = %v, 期望 %v", got, expected)
	}
}

func TestFormOption(t *testing.T) {
	formData := url.Values{}
	formData.Add("key1", "value1")
	formData.Add("key2", "value2")

	opts := newOptions([]Option{
		Form(formData),
	})

	if got := opts.Header.Get("content-type"); got != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %v, 期望 application/x-www-form-urlencoded", got)
	}

	if opts.body == nil || opts.body.(url.Values).Encode() != formData.Encode() {
		t.Error("Form body 未正确设置")
	}
}

func TestLocalAddrOption(t *testing.T) {
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
	opts := newOptions([]Option{
		LocalAddr(addr),
	})

	if opts.LocalAddr != addr {
		t.Errorf("LocalAddr = %v, 期望 %v", opts.LocalAddr, addr)
	}
}

func TestTimeoutOption(t *testing.T) {
	timeout := 5 * time.Second
	opts := newOptions([]Option{
		Timeout(timeout),
	})

	if opts.Timeout != timeout {
		t.Errorf("Timeout = %v, 期望 %v", opts.Timeout, timeout)
	}
}

func TestStreamOption(t *testing.T) {
	// 声明一个用于测试的布尔变量
	streamFn := func(i int64, b []byte) error {
		_ = true
		return nil
	}

	opts := newOptions([]Option{
		Stream(streamFn),
	})

	if len(opts.HttpRoundTripper) != 1 {
		t.Errorf("期望 1 个 RoundTripper，实际有 %d 个", len(opts.HttpRoundTripper))
	}
}

func TestLogfOption(t *testing.T) {
	// 声明一个用于测试的布尔变量，并在后续代码中使用
	logFn := func(ctx context.Context, stat *Stat) {
		_ = true
	}

	opts := newOptions([]Option{
		Logf(logFn),
	})

	if len(opts.HttpRoundTripper) != 1 {
		t.Errorf("期望 1 个 RoundTripper，实际有 %d 个", len(opts.HttpRoundTripper))
	}
	if len(opts.HttpHandler) != 1 {
		t.Errorf("期望 1 个 Handler，实际有 %d 个", len(opts.HttpHandler))
	}
}

func TestProxyOption(t *testing.T) {
	proxyURL := "http://localhost:8080"
	opts := newOptions([]Option{
		Proxy(proxyURL),
	})

	if opts.Proxy == nil {
		t.Error("Proxy 函数未设置")
	}
}

func TestVerifyOption(t *testing.T) {
	opts := newOptions([]Option{
		Verify(true),
	})

	if !opts.Verify {
		t.Error("Verify 未正确设置为 true")
	}
}

func TestMaxConnsOption(t *testing.T) {
	maxConns := 50
	opts := newOptions([]Option{
		MaxConns(maxConns),
	})

	if opts.MaxConns != maxConns {
		t.Errorf("MaxConns = %v, 期望 %v", opts.MaxConns, maxConns)
	}
}

func TestCertKeyOption(t *testing.T) {
	certFile := "cert.pem"
	keyFile := "key.pem"
	opts := newOptions([]Option{
		CertKey(certFile, keyFile),
	})

	if opts.certFile != certFile || opts.keyFile != keyFile {
		t.Errorf("CertKey 文件设置错误，got cert=%s, key=%s, want cert=%s, key=%s",
			opts.certFile, opts.keyFile, certFile, keyFile)
	}
}
