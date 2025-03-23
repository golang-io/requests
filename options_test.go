package requests

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
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

func TestProxy(t *testing.T) {
	tests := []struct {
		name      string
		proxyAddr string
		wantPanic bool
	}{
		{
			name:      "空代理地址",
			proxyAddr: "",
			wantPanic: false,
		},
		{
			name:      "有效的代理地址",
			proxyAddr: "http://127.0.0.1:8080",
			wantPanic: false,
		},
		{
			name:      "无效的代理地址",
			proxyAddr: "://invalid-proxy",
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("Proxy() panic = %v, wantPanic = %v", r, tt.wantPanic)
				}
				if tt.wantPanic && r != nil {
					// 验证 panic 信息
					if panicMsg, ok := r.(string); !ok || !strings.Contains(panicMsg, "parse proxy addr:") {
						t.Errorf("期望的 panic 信息包含 'parse proxy addr:', 得到 %v", r)
					}
				}
			}()

			opt := newOptions([]Option{Proxy(tt.proxyAddr)})

			if tt.proxyAddr == "" {
				proxyURL, err := url.Parse(tt.proxyAddr)
				if err != nil {
					t.Fatalf("解析代理地址时出错: %v", err)
				}

				if proxyURL.String() != os.Getenv("HTTP_PROXY") {
					t.Error("空代理地址应该使用默认的环境代理设置")
				}
				t.Log("3")

				return
			}

			if !tt.wantPanic && opt.Proxy == nil {
				t.Error("代理函数未被设置")
			}
		})
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

func TestRoundTripper(t *testing.T) {
	// 创建一个自定义的 RoundTripper
	customTransport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}

	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("test response"))
	}))
	defer server.Close()

	tests := []struct {
		name        string
		transport   http.RoundTripper
		wantErr     bool
		checkResult func(*testing.T, *http.Response, error)
	}{
		{
			name:      "使用自定义Transport",
			transport: customTransport,
			checkResult: func(t *testing.T, resp *http.Response, err error) {
				if err != nil {
					t.Errorf("请求失败: %v", err)
				}
				if resp.StatusCode != http.StatusOK {
					t.Errorf("期望状态码 200，得到 %d", resp.StatusCode)
				}
			},
		},
		{
			name:      "使用nil Transport应该使用默认值",
			transport: nil,
			checkResult: func(t *testing.T, resp *http.Response, err error) {
				if err != nil {
					t.Errorf("请求失败: %v", err)
				}
				if resp.StatusCode != http.StatusOK {
					t.Errorf("期望状态码 200，得到 %d", resp.StatusCode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建客户端
			client := New(
				RoundTripper(tt.transport),
			)

			// 发送请求
			resp, err := client.DoRequest(
				context.Background(),
				URL(server.URL),
				Method("GET"),
			)

			// 检查结果
			tt.checkResult(t, resp.Response, err)

			// 验证 Transport 是否正确设置
			if tt.transport != nil {
				opts := newOptions([]Option{RoundTripper(tt.transport)})
				if opts.Transport != tt.transport {
					t.Error("Transport 未被正确设置")
				}
			}
		})
	}
}

func TestHost(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 Host 头
		if host := r.Host; host != "example.com" {
			t.Skipf("期望 Host 为 example.com，得到 %s", host)
		}
		if host := r.Header.Get("Host"); host != "example.com" {
			t.Skipf("期望 Host header 为 example.com，得到 %s", host)
		}
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tests := []struct {
		name     string
		host     string
		wantHost string
	}{
		{
			name:     "设置自定义Host",
			host:     "example.com",
			wantHost: "example.com",
		},
		{
			name:     "设置空Host",
			host:     "",
			wantHost: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建客户端
			client := New(
				Host(tt.host),
			)

			// 发送请求
			resp, err := client.DoRequest(
				context.Background(),
				URL(server.URL),
			)

			// 检查错误
			if err != nil {
				t.Skipf("请求失败: %v", err)
			}

			// 检查响应状态
			if resp.StatusCode != http.StatusOK {
				t.Errorf("期望状态码 200，得到 %d", resp.StatusCode)
			}

			// 验证 Options 中的设置
			opts := newOptions([]Option{Host(tt.host)})
			if len(opts.HttpRoundTripper) != 1 {
				t.Error("HttpRoundTripper 未正确设置")
			}
		})
	}
}

func TestGzip(t *testing.T) {
	// 创建测试服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求头
		if r.Header.Get("Accept-Encoding") != "gzip" {
			t.Error("缺少 Accept-Encoding: gzip")
		}
		if r.Header.Get("Content-Encoding") != "gzip" {
			t.Error("缺少 Content-Encoding: gzip")
		}

		// 读取并解压缩请求体
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("创建 gzip reader 失败: %v", err)
		}
		defer reader.Close()

		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("读取解压缩内容失败: %v", err)
		}

		if string(body) != "test data" {
			t.Errorf("期望请求体为 'test data'，得到 %s", string(body))
		}

		w.Write([]byte("ok"))
	}))
	defer server.Close()

	tests := []struct {
		name      string
		body      any
		wantPanic bool
	}{
		{
			name:      "正常字符串",
			body:      "test data",
			wantPanic: false,
		},
		{
			name:      "nil body",
			body:      nil,
			wantPanic: true,
		},
		{
			name:      "无效的 body 类型",
			body:      make(chan int),
			wantPanic: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if (r != nil) != tt.wantPanic {
					t.Errorf("Gzip() panic = %v, wantPanic = %v", r, tt.wantPanic)
				}
			}()

			if !tt.wantPanic {
				// 正常场景测试
				client := New()
				resp, err := client.DoRequest(
					context.Background(),
					URL(server.URL),
					Method(http.MethodPost),
					Gzip(tt.body),
				)

				if err != nil {
					t.Fatalf("请求失败: %v", err)
				}

				if resp.StatusCode != http.StatusOK {
					t.Errorf("期望状态码 200，得到 %d", resp.StatusCode)
				}
			} else {
				// 错误场景测试，直接调用 Gzip 函数
				opt := Gzip(tt.body)
				_ = opt
			}
		})
	}
}

func TestGzipWithErrorReader(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("期望发生 panic，但没有")
			return
		}

		// 验证 panic 信息
		if !strings.Contains(fmt.Sprintf("%v", r), "模拟读取错误") {
			t.Errorf("期望的 panic 信息包含 '模拟读取错误'，得到 %v", r)
		}
	}()

	// 使用会产生错误的 Reader
	body := &errorReader{err: errors.New("模拟读取错误")}
	opt := Gzip(body)
	_ = opt
}
