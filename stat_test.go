package requests

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStat_Methods 测试 Stat 的基础方法：String、Print、RequestBody、ResponseBody
func TestStat_Methods(t *testing.T) {
	stat := &Stat{
		RequestId: "test-req-123",
		StartAt:   "2023-05-01 12:00:00.000",
		Cost:      150,
	}
	stat.Request.Method = "POST"
	stat.Request.RemoteAddr = "192.168.1.1:8080"
	stat.Request.URL = "/api/v1/test"
	stat.Request.Body = map[string]any{"key": "value"}
	stat.Response.URL = "http://example.com"
	stat.Response.StatusCode = 201
	stat.Response.ContentLength = 2048
	stat.Response.Body = map[string]any{"status": "ok"}

	t.Run("String", func(t *testing.T) {
		jsonStr := stat.String()
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			t.Fatalf("无法解析 JSON: %v", err)
		}
		if parsed["RequestId"] != "test-req-123" {
			t.Errorf("RequestId 不匹配，期望 'test-req-123'，实际 %v", parsed["RequestId"])
		}
	})

	t.Run("Print", func(t *testing.T) {
		printStr := stat.Print()
		expected := "2023-05-01 12:00:00.000 POST \"192.168.1.1:8080 -> http://example.com/api/v1/test\" - 201 2048B in 150ms"
		if printStr != expected {
			t.Errorf("输出不匹配\n期望: %s\n实际: %s", expected, printStr)
		}
	})

	t.Run("RequestBody", func(t *testing.T) {
		body := stat.RequestBody()
		if !strings.Contains(body, "key") || !strings.Contains(body, "value") {
			t.Errorf("RequestBody 应包含 'key' 和 'value'，实际: %s", body)
		}
	})

	t.Run("ResponseBody", func(t *testing.T) {
		body := stat.ResponseBody()
		if !strings.Contains(body, "status") || !strings.Contains(body, "ok") {
			t.Errorf("ResponseBody 应包含 'status' 和 'ok'，实际: %s", body)
		}
	})
}

// TestA2S 测试 a2s 函数的各种类型转换场景
func TestA2S(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"nil", nil, "null"},
		{"string", "hello", `"hello"`},
		{"number", 42, "42"},
		{"boolean", true, "true"},
		{"map", map[string]any{"key": "value"}, `{"key":"value"}`},
		{"slice", []string{"a", "b"}, `["a","b"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := a2s(tt.input)
			if result != tt.expected {
				t.Errorf("期望 %s，实际 %s", tt.expected, result)
			}
		})
	}

	// 测试无法序列化的类型
	t.Run("unserializable", func(t *testing.T) {
		ch := make(chan int)
		result := a2s(ch)
		expected := fmt.Sprintf("%v", ch)
		if result != expected {
			t.Errorf("无法序列化的类型应使用 fmt.Sprintf，期望 %s，实际 %s", expected, result)
		}
	})
}

// TestResponseLoad 测试 responseLoad 函数
func TestResponseLoad(t *testing.T) {
	tests := []struct {
		name      string
		buildResp func() *Response
		checkFunc func(t *testing.T, stat *Stat)
	}{
		{
			name: "标准JSON响应",
			buildResp: func() *Response {
				httpResp := &http.Response{
					StatusCode: 200,
					Header: http.Header{
						"Content-Type": []string{"application/json"},
						"X-Test":       []string{"test-value"},
					},
					Body: io.NopCloser(strings.NewReader(`{"message":"success"}`)),
				}
				req, _ := http.NewRequest("GET", "http://example.com/test?param=value", nil)
				req.Header.Set(RequestId, "test-request-id")
				return &Response{
					Response: httpResp,
					Request:  req,
					StartAt:  time.Now().Add(-100 * time.Millisecond),
				}
			},
			checkFunc: func(t *testing.T, stat *Stat) {
				if stat.RequestId != "test-request-id" {
					t.Errorf("RequestId 不匹配")
				}
				if stat.Request.Method != "GET" {
					t.Errorf("Method 不匹配")
				}
				if stat.Response.StatusCode != 200 {
					t.Errorf("StatusCode 不匹配")
				}
				if responseBody, ok := stat.Response.Body.(map[string]interface{}); !ok {
					t.Errorf("Response.Body 应为 map")
				} else if responseBody["message"] != "success" {
					t.Errorf("message 字段不匹配")
				}
			},
		},
		{
			name: "带请求体",
			buildResp: func() *Response {
				httpResp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("{}")),
				}
				bodyStr := `{"test":"data"}`
				req, _ := http.NewRequest("POST", "http://example.com/test", strings.NewReader(bodyStr))
				req.GetBody = func() (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader(bodyStr)), nil
				}
				return &Response{
					Response: httpResp,
					Request:  req,
					StartAt:  time.Now(),
				}
			},
			checkFunc: func(t *testing.T, stat *Stat) {
				if stat.Request.Body == nil {
					t.Error("应该有请求体")
				}
			},
		},
		{
			name: "空响应",
			buildResp: func() *Response {
				httpResp := &http.Response{
					StatusCode:    200,
					Header:        http.Header{"Content-Type": []string{"text/plain"}},
					Body:          io.NopCloser(strings.NewReader("")),
					ContentLength: -1,
				}
				req, _ := http.NewRequest("GET", "http://example.com/empty", nil)
				return &Response{
					Response: httpResp,
					Request:  req,
					StartAt:  time.Now(),
					Content:  &bytes.Buffer{},
				}
			},
			checkFunc: func(t *testing.T, stat *Stat) {
				if stat.Response.ContentLength != 0 {
					t.Logf("ContentLength: %d", stat.Response.ContentLength)
				}
			},
		},
		{
			name: "带错误",
			buildResp: func() *Response {
				return &Response{
					Err:     fmt.Errorf("测试错误"),
					StartAt: time.Now().Add(-50 * time.Millisecond),
				}
			},
			checkFunc: func(t *testing.T, stat *Stat) {
				if stat.Err != "测试错误" {
					t.Errorf("错误信息不匹配，期望 '测试错误'，实际 '%s'", stat.Err)
				}
			},
		},
		{
			name: "nil请求",
			buildResp: func() *Response {
				httpResp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("test")),
				}
				return &Response{
					Response: httpResp,
					Request:  nil,
					StartAt:  time.Now(),
				}
			},
			checkFunc: func(t *testing.T, stat *Stat) {
				if stat.Request.URL != "" {
					t.Error("nil 请求时 URL 应为空")
				}
			},
		},
		{
			name: "nil响应",
			buildResp: func() *Response {
				req, _ := http.NewRequest("GET", "http://example.com/test", nil)
				return &Response{
					Response: nil,
					Request:  req,
					StartAt:  time.Now(),
				}
			},
			checkFunc: func(t *testing.T, stat *Stat) {
				if stat.Response.StatusCode != 0 {
					t.Errorf("nil 响应时状态码应为 0，实际 %d", stat.Response.StatusCode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.buildResp()
			stat := responseLoad(resp)
			tt.checkFunc(t, stat)
		})
	}
}

// TestResponseLoad_ErrorHandling 测试错误处理场景
func TestResponseLoad_ErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		buildResp      func() *Response
		expectedErrStr string
	}{
		{
			name: "读取响应体错误",
			buildResp: func() *Response {
				errorBody := &errorReader{err: fmt.Errorf("read body error")}
				httpResp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(errorBody),
				}
				req, _ := http.NewRequest("GET", "http://example.com/test", nil)
				return &Response{
					Response: httpResp,
					Request:  req,
					StartAt:  time.Now(),
					Content:  nil,
				}
			},
			expectedErrStr: "read response",
		},
		{
			name: "GetBody错误",
			buildResp: func() *Response {
				httpResp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("{}")),
				}
				req, _ := http.NewRequest("POST", "http://example.com/test", nil)
				req.GetBody = func() (io.ReadCloser, error) {
					return nil, fmt.Errorf("get body error")
				}
				return &Response{
					Response: httpResp,
					Request:  req,
					StartAt:  time.Now(),
				}
			},
			expectedErrStr: "read request1",
		},
		{
			name: "ParseBody错误",
			buildResp: func() *Response {
				httpResp := &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader("{}")),
				}
				req, _ := http.NewRequest("POST", "http://example.com/test", nil)
				req.GetBody = func() (io.ReadCloser, error) {
					return io.NopCloser(&errorReader{err: fmt.Errorf("parse body error")}), nil
				}
				return &Response{
					Response: httpResp,
					Request:  req,
					StartAt:  time.Now(),
				}
			},
			expectedErrStr: "read request2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.buildResp()
			stat := responseLoad(resp)
			if !strings.Contains(stat.Err, tt.expectedErrStr) {
				t.Errorf("期望错误信息包含 '%s'，实际 '%s'", tt.expectedErrStr, stat.Err)
			}
		})
	}
}

// TestServeLoad 测试 serveLoad 函数
func TestServeLoad(t *testing.T) {
	tests := []struct {
		name      string
		buildReq  func() (*http.Request, *ResponseWriter, *bytes.Buffer)
		checkFunc func(t *testing.T, stat *Stat)
	}{
		{
			name: "标准JSON请求",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("POST", "/api/v1/test?param=value", strings.NewReader(`{"data":"test"}`))
				req.Header.Set("Content-Type", "application/json")
				req.RemoteAddr = "192.168.1.1:8080"
				w := &ResponseWriter{
					StatusCode: 201,
					Content:    bytes.NewBufferString(`{"status":"created"}`),
				}
				buf := bytes.NewBufferString(`{"data":"test"}`)
				return req, w, buf
			},
			checkFunc: func(t *testing.T, stat *Stat) {
				if stat.Request.Method != "POST" {
					t.Errorf("Method 不匹配")
				}
				if stat.Request.RemoteAddr != "192.168.1.1:8080" {
					t.Errorf("RemoteAddr 不匹配")
				}
				if stat.Response.StatusCode != 201 {
					t.Errorf("StatusCode 不匹配")
				}
				if requestBody, ok := stat.Request.Body.(map[string]interface{}); !ok {
					t.Errorf("Request.Body 应为 map")
				} else if requestBody["data"] != "test" {
					t.Errorf("data 字段不匹配")
				}
			},
		},
		{
			name: "无效JSON请求",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("POST", "/test", strings.NewReader("invalid json"))
				req.RemoteAddr = "127.0.0.1:8080"
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				buf := bytes.NewBufferString("invalid json")
				return req, w, buf
			},
			checkFunc: func(t *testing.T, stat *Stat) {
				if bodyStr, ok := stat.Request.Body.(string); !ok {
					t.Error("无效 JSON 应存储为字符串")
				} else if bodyStr != "invalid json" {
					t.Errorf("请求体不匹配，期望 'invalid json'，实际 '%s'", bodyStr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, w, buf := tt.buildReq()
			start := time.Now().Add(-200 * time.Millisecond)
			stat := serveLoad(w, req, start, buf)
			tt.checkFunc(t, stat)
		})
	}
}

// TestServeLoad_TLS 测试 TLS 相关的 serveLoad 功能
func TestServeLoad_TLS(t *testing.T) {
	tests := []struct {
		name        string
		buildReq    func() (*http.Request, *ResponseWriter, *bytes.Buffer)
		expectHTTPS bool
		description string
	}{
		{
			name: "HTTP请求",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1.1:8080"
				req.Host = "example.com:8080"
				// 不设置 TLS，模拟 HTTP 请求
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				return req, w, nil
			},
			expectHTTPS: false,
			description: "普通 HTTP 请求应该使用 http:// 协议",
		},
		{
			name: "HTTPS请求",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1.1:8443"
				req.Host = "example.com:8443"
				// 模拟 TLS 连接
				req.TLS = &tls.ConnectionState{
					Version:           0x0303, // TLS 1.2
					HandshakeComplete: true,
				}
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				return req, w, nil
			},
			expectHTTPS: true,
			description: "TLS 请求应该使用 https:// 协议",
		},
		{
			name: "HTTPS请求_默认端口",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1.1:443"
				req.Host = "example.com"
				// 模拟 TLS 连接
				req.TLS = &tls.ConnectionState{
					Version:           0x0303, // TLS 1.2
					HandshakeComplete: true,
				}
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				return req, w, nil
			},
			expectHTTPS: true,
			description: "TLS 请求在默认端口 443 应该使用 https:// 协议",
		},
		{
			name: "HTTPS请求_自定义端口",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1.1:9443"
				req.Host = "example.com:9443"
				// 模拟 TLS 连接
				req.TLS = &tls.ConnectionState{
					Version:           0x0303, // TLS 1.2
					HandshakeComplete: true,
				}
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				return req, w, nil
			},
			expectHTTPS: true,
			description: "TLS 请求在自定义端口应该使用 https:// 协议",
		},
		{
			name: "HTTP请求_自定义端口",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1.1:8080"
				req.Host = "example.com:8080"
				// 不设置 TLS，模拟 HTTP 请求
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				return req, w, nil
			},
			expectHTTPS: false,
			description: "HTTP 请求在自定义端口应该使用 http:// 协议",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, w, buf := tt.buildReq()
			start := time.Now().Add(-100 * time.Millisecond)
			stat := serveLoad(w, req, start, buf)

			// 验证协议选择
			expectedScheme := "http://"
			if tt.expectHTTPS {
				expectedScheme = "https://"
			}

			expectedURL := expectedScheme + req.Host
			if stat.Response.URL != expectedURL {
				t.Errorf("URL 不匹配\n期望: %s\n实际: %s\n描述: %s",
					expectedURL, stat.Response.URL, tt.description)
			}

			// 验证其他基本字段
			if stat.Request.Method != req.Method {
				t.Errorf("Method 不匹配，期望 %s，实际 %s", req.Method, stat.Request.Method)
			}

			if stat.Request.RemoteAddr != req.RemoteAddr {
				t.Errorf("RemoteAddr 不匹配，期望 %s，实际 %s", req.RemoteAddr, stat.Request.RemoteAddr)
			}

			if stat.Response.StatusCode != w.StatusCode {
				t.Errorf("StatusCode 不匹配，期望 %d，实际 %d", w.StatusCode, stat.Response.StatusCode)
			}

			// 验证时间相关字段
			if stat.Cost < 0 {
				t.Error("Cost 应该大于等于 0")
			}

			if stat.StartAt != start.Format("2006-01-02 15:04:05.000") {
				t.Errorf("StartAt 格式不正确，期望 %s，实际 %s",
					start.Format("2006-01-02 15:04:05.000"), stat.StartAt)
			}
		})
	}
}

// TestServeLoad_TLS_EdgeCases 测试 TLS 相关的边界情况
func TestServeLoad_TLS_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		buildReq    func() (*http.Request, *ResponseWriter, *bytes.Buffer)
		expectURL   string
		description string
	}{
		{
			name: "TLS为nil但Host包含https",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1.1:8080"
				req.Host = "https://example.com:8080" // 错误的 Host 格式
				// TLS 为 nil
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				return req, w, nil
			},
			expectURL:   "http://https://example.com:8080", // 应该使用 http://
			description: "即使 Host 包含 https，但 TLS 为 nil 时仍应使用 http://",
		},
		{
			name: "空Host",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1.1:8080"
				req.Host = "" // 空 Host
				req.TLS = &tls.ConnectionState{
					Version:           0x0303,
					HandshakeComplete: true,
				}
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				return req, w, nil
			},
			expectURL:   "https://", // 空 Host 的情况
			description: "空 Host 时应该使用 https:// 前缀",
		},
		{
			name: "TLS未完成握手",
			buildReq: func() (*http.Request, *ResponseWriter, *bytes.Buffer) {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = "192.168.1.1:8443"
				req.Host = "example.com:8443"
				// TLS 存在但握手未完成
				req.TLS = &tls.ConnectionState{
					Version:           0x0303,
					HandshakeComplete: false, // 握手未完成
				}
				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				return req, w, nil
			},
			expectURL:   "https://example.com:8443", // 仍然应该使用 https://
			description: "TLS 存在但握手未完成时仍应使用 https://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, w, buf := tt.buildReq()
			start := time.Now().Add(-50 * time.Millisecond)
			stat := serveLoad(w, req, start, buf)

			if stat.Response.URL != tt.expectURL {
				t.Errorf("URL 不匹配\n期望: %s\n实际: %s\n描述: %s",
					tt.expectURL, stat.Response.URL, tt.description)
			}
		})
	}
}

// TestServeLoad_TLS_Performance 测试 TLS 检测的性能
func TestServeLoad_TLS_Performance(t *testing.T) {
	// 创建大量请求来测试性能
	req, _ := http.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:8443"
	req.Host = "example.com:8443"
	req.TLS = &tls.ConnectionState{
		Version:           0x0303,
		HandshakeComplete: true,
	}
	w := &ResponseWriter{
		StatusCode: 200,
		Content:    bytes.NewBufferString("ok"),
	}
	start := time.Now()

	// 运行多次测试
	for i := range 1000 {
		stat := serveLoad(w, req, start, nil)
		if !strings.HasPrefix(stat.Response.URL, "https://") {
			t.Errorf("第 %d 次测试失败，URL 应为 https:// 开头，实际: %s", i+1, stat.Response.URL)
		}
	}
}

// TestServeLoad_TLS_Concurrent 测试 TLS 检测的并发安全性
func TestServeLoad_TLS_Concurrent(t *testing.T) {
	const numGoroutines = 100
	const numRequests = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numRequests)

	for i := range numGoroutines {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numRequests; j++ {
				req, _ := http.NewRequest("GET", "/api/test", nil)
				req.RemoteAddr = fmt.Sprintf("192.168.1.%d:8443", goroutineID%255+1)
				req.Host = fmt.Sprintf("example%d.com:8443", goroutineID)

				// 交替使用 HTTP 和 HTTPS
				if j%2 == 0 {
					req.TLS = &tls.ConnectionState{
						Version:           0x0303,
						HandshakeComplete: true,
					}
				}

				w := &ResponseWriter{
					StatusCode: 200,
					Content:    bytes.NewBufferString("ok"),
				}
				start := time.Now()

				stat := serveLoad(w, req, start, nil)

				// 验证结果
				expectedScheme := "http://"
				if j%2 == 0 {
					expectedScheme = "https://"
				}

				if !strings.HasPrefix(stat.Response.URL, expectedScheme) {
					errors <- fmt.Errorf("goroutine %d, request %d: 期望 %s 开头，实际 %s",
						goroutineID, j, expectedScheme, stat.Response.URL)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// 检查错误
	for err := range errors {
		t.Error(err)
	}
}

// BenchmarkServeLoad_TLS 基准测试 TLS 检测性能
func BenchmarkServeLoad_TLS(b *testing.B) {
	req, _ := http.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:8443"
	req.Host = "example.com:8443"
	req.TLS = &tls.ConnectionState{
		Version:           0x0303,
		HandshakeComplete: true,
	}
	w := &ResponseWriter{
		StatusCode: 200,
		Content:    bytes.NewBufferString("ok"),
	}
	start := time.Now()

	b.ResetTimer()
	for range b.N {
		serveLoad(w, req, start, nil)
	}
}

// BenchmarkServeLoad_HTTP 基准测试 HTTP 检测性能
func BenchmarkServeLoad_HTTP(b *testing.B) {
	req, _ := http.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:8080"
	req.Host = "example.com:8080"
	// 不设置 TLS，模拟 HTTP 请求
	w := &ResponseWriter{
		StatusCode: 200,
		Content:    bytes.NewBufferString("ok"),
	}
	start := time.Now()

	b.ResetTimer()
	for range b.N {
		serveLoad(w, req, start, nil)
	}
}

// BenchmarkServeLoad_TLS_Detection 基准测试 TLS 检测逻辑的性能
func BenchmarkServeLoad_TLS_Detection(b *testing.B) {
	// 测试 TLS 检测逻辑的性能
	scheme := "http://"
	r := &http.Request{
		TLS: &tls.ConnectionState{
			Version:           0x0303,
			HandshakeComplete: true,
		},
		Host: "example.com:8443",
	}

	b.ResetTimer()
	for range b.N {
		if r.TLS != nil {
			scheme = "https://"
		}
		_ = scheme + r.Host
		// 重置 scheme 用于下次测试
		scheme = "http://"
	}
}
