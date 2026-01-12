package requests

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestNewResponse 测试newResponse函数的基本功能
func TestNewResponse(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	resp := newResponse(r)

	if resp.Request != r {
		t.Error("Request not set correctly")
	}
	if resp.Content == nil {
		t.Error("Content buffer not initialized")
	}
	if resp.StartAt.IsZero() {
		t.Error("StartAt not initialized")
	}
}

// TestResponseString 测试Response.String()方法
func TestResponseString(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	resp := newResponse(r)
	resp.Content.WriteString("test content")

	if resp.String() != "test content" {
		t.Errorf("Expected 'test content', got '%s'", resp.String())
	}
}

// TestResponseError 测试Response.Error()方法
func TestResponseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"no error", nil, ""},
		{"with error", errors.New("test error"), "test error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "http://example.com", nil)
			resp := newResponse(r)
			resp.Err = tt.err
			if got := resp.Error(); got != tt.want {
				t.Errorf("Response.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStreamRead 测试streamRead函数的基本功能
func TestStreamRead(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int64
		wantErr bool
	}{
		{"empty input", "", 0, false},
		{"single line", "hello\n", 6, false},
		{"multiple lines", "hello\nworld\n", 12, false},
		{"no newline at end", "hello", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			var lines []string

			gotLen, err := streamRead(context.Background(), reader, func(_ int64, data []byte) error {
				lines = append(lines, string(bytes.TrimRight(data, "\n")))
				return nil
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("streamRead() error = %v, wantErr %v", err, tt.wantErr)
			}
			if gotLen != tt.wantLen {
				t.Errorf("streamRead() length = %v, want %v", gotLen, tt.wantLen)
			}
		})
	}
}

// TestStreamReadError 测试streamRead函数的错误处理
func TestStreamReadError(t *testing.T) {
	// 测试回调函数返回错误的情况
	reader := strings.NewReader("test\ndata\n")
	expectedErr := errors.New("callback error")

	_, err := streamRead(context.Background(), reader, func(_ int64, _ []byte) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// 测试读取错误的情况
	errorReader := &errorReader{err: errors.New("read error")}
	_, err = streamRead(context.Background(), errorReader, func(_ int64, _ []byte) error {
		return nil
	})

	if err == nil || !strings.Contains(err.Error(), "read error") {
		t.Errorf("Expected read error, got %v", err)
	}
}

// TestStreamReadContextCancel 测试streamRead函数的Context取消功能
// TestStreamReadContextCancel tests Context cancellation in streamRead function
func TestStreamReadContextCancel(t *testing.T) {
	// 创建一个会持续产生数据的读取器
	// Create a reader that continuously produces data
	infiniteReader := &infiniteStringReader{data: "line\n"}

	// 创建可取消的 Context
	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 在另一个 goroutine 中延迟取消
	// Cancel after a delay in another goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// 开始流式读取，应该会被 Context 取消中断
	// Start streaming read, should be interrupted by context cancellation
	processedLines := 0
	_, err := streamRead(ctx, infiniteReader, func(_ int64, _ []byte) error {
		processedLines++
		time.Sleep(10 * time.Millisecond) // 模拟处理时间
		return nil
	})

	// 验证返回了 context.Canceled 错误
	// Verify that context.Canceled error is returned
	if err == nil {
		t.Error("Expected context.Canceled error, got nil")
	} else if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	// 验证确实处理了一些行（在取消之前）
	// Verify that some lines were processed before cancellation
	if processedLines == 0 {
		t.Error("Expected at least some lines to be processed before cancellation")
	}
}

// infiniteStringReader 是一个无限读取器，用于测试
// infiniteStringReader is an infinite reader for testing
type infiniteStringReader struct {
	data string
	pos  int
}

func (r *infiniteStringReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	// 循环返回数据
	// Loop and return data
	for i := 0; i < len(p); i++ {
		p[i] = r.data[r.pos%len(r.data)]
		r.pos++
		if r.pos >= len(r.data) {
			r.pos = 0
		}
	}
	return len(p), nil
}

// 用于测试的错误读取器
type errorReader struct {
	err error
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, r.err
}

// TestResponseJSON 测试 Response.JSON() 方法
// TestResponseJSON tests the Response.JSON() method
func TestResponseJSON(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		target      any
		wantErr     bool
		description string
	}{
		{
			name:    "有效JSON到结构体",
			content: `{"name":"Alice","age":30}`,
			target: &struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{},
			wantErr:     false,
			description: "正常情况：有效的JSON对象反序列化到结构体",
		},
		{
			name:        "有效JSON到map",
			content:     `{"key1":"value1","key2":123}`,
			target:      &map[string]any{},
			wantErr:     false,
			description: "正常情况：有效的JSON对象反序列化到map",
		},
		{
			name:        "有效JSON数组到slice",
			content:     `[1,2,3,4,5]`,
			target:      &[]int{},
			wantErr:     false,
			description: "正常情况：有效的JSON数组反序列化到slice",
		},
		{
			name:    "有效JSON数组到结构体slice",
			content: `[{"name":"Alice"},{"name":"Bob"}]`,
			target: &[]struct {
				Name string `json:"name"`
			}{},
			wantErr:     false,
			description: "正常情况：有效的JSON对象数组反序列化到结构体slice",
		},
		{
			name:        "空JSON对象",
			content:     `{}`,
			target:      &map[string]any{},
			wantErr:     false,
			description: "边界情况：空JSON对象",
		},
		{
			name:        "空JSON数组",
			content:     `[]`,
			target:      &[]int{},
			wantErr:     false,
			description: "边界情况：空JSON数组",
		},
		{
			name:        "JSON null值",
			content:     `null`,
			target:      &map[string]any{},
			wantErr:     false,
			description: "边界情况：JSON null值",
		},
		{
			name:        "JSON字符串值",
			content:     `"hello world"`,
			target:      new(string),
			wantErr:     false,
			description: "正常情况：JSON字符串值",
		},
		{
			name:        "JSON数字值",
			content:     `42`,
			target:      new(int),
			wantErr:     false,
			description: "正常情况：JSON数字值",
		},
		{
			name:        "JSON布尔值",
			content:     `true`,
			target:      new(bool),
			wantErr:     false,
			description: "正常情况：JSON布尔值",
		},
		{
			name:        "嵌套JSON对象",
			content:     `{"user":{"name":"Alice","profile":{"age":30}}}`,
			target:      &map[string]any{},
			wantErr:     false,
			description: "正常情况：嵌套的JSON对象",
		},
		{
			name:        "无效JSON格式",
			content:     `{"name":"Alice",}`,
			target:      &map[string]any{},
			wantErr:     true,
			description: "错误情况：无效的JSON格式（尾随逗号）",
		},
		{
			name:        "无效JSON语法",
			content:     `{name:"Alice"}`,
			target:      &map[string]any{},
			wantErr:     true,
			description: "错误情况：无效的JSON语法（缺少引号）",
		},
		{
			name:        "空内容",
			content:     ``,
			target:      &map[string]any{},
			wantErr:     true,
			description: "错误情况：空内容",
		},
		{
			name:        "不完整JSON",
			content:     `{"name":`,
			target:      &map[string]any{},
			wantErr:     true,
			description: "错误情况：不完整的JSON",
		},
		{
			name:        "类型不匹配-对象到slice",
			content:     `{"name":"Alice"}`,
			target:      &[]int{},
			wantErr:     true,
			description: "错误情况：JSON对象无法反序列化到slice",
		},
		{
			name:        "类型不匹配-数组到map",
			content:     `[1,2,3]`,
			target:      &map[string]int{},
			wantErr:     true,
			description: "错误情况：JSON数组无法反序列化到map",
		},
		{
			name:        "类型不匹配-字符串到数字",
			content:     `"not a number"`,
			target:      new(int),
			wantErr:     true,
			description: "错误情况：JSON字符串无法反序列化到数字类型",
		},
		{
			name:    "复杂嵌套结构",
			content: `{"users":[{"id":1,"name":"Alice","tags":["admin","user"]},{"id":2,"name":"Bob","tags":["user"]}]}`,
			target: &struct {
				Users []struct {
					ID   int      `json:"id"`
					Name string   `json:"name"`
					Tags []string `json:"tags"`
				} `json:"users"`
			}{},
			wantErr:     false,
			description: "正常情况：复杂的嵌套JSON结构",
		},
		{
			name:        "包含特殊字符的JSON",
			content:     `{"message":"Hello\nWorld\tTab","unicode":"中文测试"}`,
			target:      &map[string]string{},
			wantErr:     false,
			description: "正常情况：包含转义字符和Unicode的JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 Response 对象
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			resp := newResponse(req)

			// 设置响应内容
			resp.Content.WriteString(tt.content)

			// 执行 JSON 反序列化
			err := resp.JSON(tt.target)

			// 验证错误
			if (err != nil) != tt.wantErr {
				t.Errorf("Response.JSON() error = %v, wantErr %v", err, tt.wantErr)
				t.Logf("Description: %s", tt.description)
				return
			}

			// 如果没有错误，验证反序列化结果
			if !tt.wantErr && err == nil {
				// 对于基本类型，验证值是否正确
				switch v := tt.target.(type) {
				case *string:
					if tt.content == `"hello world"` && *v != "hello world" {
						t.Errorf("Expected string 'hello world', got %q", *v)
					}
				case *int:
					if tt.content == `42` && *v != 42 {
						t.Errorf("Expected int 42, got %d", *v)
					}
				case *bool:
					if tt.content == `true` && *v != true {
						t.Errorf("Expected bool true, got %v", *v)
					}
				case *map[string]any:
					switch tt.content {
					case `{"key1":"value1","key2":123}`:
						if (*v)["key1"] != "value1" {
							t.Errorf("Expected key1='value1', got %v", (*v)["key1"])
						}
						if (*v)["key2"] != float64(123) {
							t.Errorf("Expected key2=123, got %v", (*v)["key2"])
						}
					case `{"message":"Hello\nWorld\tTab","unicode":"中文测试"}`:
						if (*v)["message"] != "Hello\nWorld\tTab" {
							t.Errorf("Expected message with escape chars, got %q", (*v)["message"])
						}
						if (*v)["unicode"] != "中文测试" {
							t.Errorf("Expected unicode='中文测试', got %q", (*v)["unicode"])
						}
					}
				case *[]int:
					if tt.content == `[1,2,3,4,5]` {
						expected := []int{1, 2, 3, 4, 5}
						if len(*v) != len(expected) {
							t.Errorf("Expected slice length %d, got %d", len(expected), len(*v))
						} else {
							for i, val := range expected {
								if (*v)[i] != val {
									t.Errorf("Expected [%d]=%d, got %d", i, val, (*v)[i])
								}
							}
						}
					}
				case *[]struct {
					Name string `json:"name"`
				}:
					if tt.content == `[{"name":"Alice"},{"name":"Bob"}]` {
						if len(*v) != 2 {
							t.Errorf("Expected slice length 2, got %d", len(*v))
						} else {
							if (*v)[0].Name != "Alice" {
								t.Errorf("Expected first name='Alice', got %q", (*v)[0].Name)
							}
							if (*v)[1].Name != "Bob" {
								t.Errorf("Expected second name='Bob', got %q", (*v)[1].Name)
							}
						}
					}
				case *struct {
					Name string `json:"name"`
					Age  int    `json:"age"`
				}:
					if tt.content == `{"name":"Alice","age":30}` {
						if v.Name != "Alice" {
							t.Errorf("Expected Name='Alice', got %q", v.Name)
						}
						if v.Age != 30 {
							t.Errorf("Expected Age=30, got %d", v.Age)
						}
					}
				case *struct {
					Users []struct {
						ID   int      `json:"id"`
						Name string   `json:"name"`
						Tags []string `json:"tags"`
					} `json:"users"`
				}:
					if tt.content == `{"users":[{"id":1,"name":"Alice","tags":["admin","user"]},{"id":2,"name":"Bob","tags":["user"]}]}` {
						if len(v.Users) != 2 {
							t.Errorf("Expected 2 users, got %d", len(v.Users))
						} else {
							if v.Users[0].ID != 1 || v.Users[0].Name != "Alice" {
								t.Errorf("Expected first user id=1, name='Alice', got id=%d, name=%q", v.Users[0].ID, v.Users[0].Name)
							}
							if len(v.Users[0].Tags) != 2 || v.Users[0].Tags[0] != "admin" {
								t.Errorf("Expected first user tags=['admin','user'], got %v", v.Users[0].Tags)
							}
						}
					}
				}
			}
		})
	}
}

// TestResponseJSON_NilTarget 测试 nil 目标的情况
// TestResponseJSON_NilTarget tests nil target case
func TestResponseJSON_NilTarget(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp := newResponse(req)
	resp.Content.WriteString(`{"name":"Alice"}`)

	// 测试 nil 指针
	var nilMap *map[string]any
	err := resp.JSON(nilMap)
	if err == nil {
		t.Error("Expected error for nil target, got nil")
	}
}

// TestResponseJSON_NonPointerTarget 测试非指针目标的情况
// TestResponseJSON_NonPointerTarget tests non-pointer target case
func TestResponseJSON_NonPointerTarget(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp := newResponse(req)
	resp.Content.WriteString(`{"name":"Alice"}`)

	// 测试非指针目标（应该失败）
	var m map[string]any
	err := resp.JSON(m)
	if err == nil {
		t.Error("Expected error for non-pointer target, got nil")
	}
}

// TestResponseJSON_EmptyContent 测试空内容的情况
// TestResponseJSON_EmptyContent tests empty content case
func TestResponseJSON_EmptyContent(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp := newResponse(req)
	// Content 为空，不写入任何内容

	var result map[string]any
	err := resp.JSON(&result)
	if err == nil {
		t.Error("Expected error for empty content, got nil")
	}
}

// TestResponseJSON_WhitespaceContent 测试只有空白字符的内容
// TestResponseJSON_WhitespaceContent tests whitespace-only content
func TestResponseJSON_WhitespaceContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{"只有空格", "   ", true},
		{"只有换行符", "\n\n", true},
		{"只有制表符", "\t\t", true},
		{"混合空白字符", " \n\t ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			resp := newResponse(req)
			resp.Content.WriteString(tt.content)

			var result map[string]any
			err := resp.JSON(&result)
			if (err != nil) != tt.wantErr {
				t.Errorf("Response.JSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestStreamContextWithDoRequest 测试完整的 DoRequest + Stream + Context 流程
// TestStreamContextWithDoRequest tests the complete DoRequest + Stream + Context flow
func TestStreamContextWithDoRequest(t *testing.T) {
	// 创建测试服务器，返回多行数据
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 发送 50 行数据
		for i := 0; i < 50; i++ {
			if _, err := w.Write([]byte("line\n")); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()

	// 测试场景 1: 正常完成（Context 不取消）
	t.Run("正常完成流程", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sess := New(URL(server.URL))

		var processedLines int64
		resp, err := sess.DoRequest(ctx,
			Stream(func(lineNum int64, data []byte) error {
				processedLines = lineNum
				return nil
			}),
		)

		// 应该成功完成
		if err != nil {
			t.Errorf("Expected no error for normal completion, got %v", err)
		}

		// 应该处理了所有行（可能多一行，因为最后一行可能没有换行符）
		// 实际处理的行数应该接近 50
		if processedLines < 45 {
			t.Errorf("Expected at least 45 lines processed, got %d", processedLines)
		}

		// 验证响应对象
		if resp == nil {
			t.Error("Expected response object, got nil")
		}

		// 在流式模式下，Body 应该是 http.NoBody（在 streamRoundTrip 中设置）
		// 但 DoRequest 可能会重新包装，所以这里只验证响应对象存在
		if resp.Response == nil {
			t.Error("Expected response.Response to exist")
		}

		t.Logf("正常完成测试通过: 处理了 %d 行", processedLines)
	})

	// 测试场景 2: 超时中断
	t.Run("超时中断流程", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		sess := New(URL(server.URL))

		var processedLines int64
		resp, err := sess.DoRequest(ctx,
			Stream(func(lineNum int64, data []byte) error {
				processedLines = lineNum
				time.Sleep(2 * time.Millisecond)
				return nil
			}),
		)

		// 应该返回超时错误
		if err != context.DeadlineExceeded {
			t.Errorf("Expected context.DeadlineExceeded, got %v", err)
		}

		// 应该处理了部分行（因为超时）
		if processedLines == 0 {
			t.Error("Expected at least some lines to be processed before timeout")
		}

		if processedLines >= 50 {
			t.Errorf("Expected processed lines < 50 (due to timeout), got %d", processedLines)
		}

		// 验证响应对象存在
		if resp == nil {
			t.Error("Expected response object, got nil")
		}

		t.Logf("超时中断测试通过: 处理了 %d 行后超时", processedLines)
	})

	// 测试场景 3: 手动取消
	t.Run("手动取消流程", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sess := New(URL(server.URL))

		var processedLines int64

		// 在另一个 goroutine 中延迟取消
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		resp, err := sess.DoRequest(ctx,
			Stream(func(lineNum int64, data []byte) error {
				processedLines = lineNum
				time.Sleep(2 * time.Millisecond)
				return nil
			}),
		)

		// 应该返回取消错误
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}

		// 应该处理了部分行
		if processedLines == 0 {
			t.Error("Expected at least some lines to be processed before cancellation")
		}

		// 验证响应对象存在
		if resp == nil {
			t.Error("Expected response object, got nil")
		}

		t.Logf("手动取消测试通过: 处理了 %d 行后取消", processedLines)
	})
}

// TestStreamContextEdgeCases tests edge cases for stream context.
func TestStreamContextEdgeCases(t *testing.T) {
	t.Run("immediately cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("line1\nline2\n"))
		}))
		defer server.Close()

		sess := New(URL(server.URL))

		var callbackCalled bool
		_, err := sess.DoRequest(ctx,
			Stream(func(_ int64, _ []byte) error {
				callbackCalled = true
				return nil
			}),
		)

		// 应该返回取消错误（可能是 context.Canceled 或包含 "context canceled" 的错误）
		if err == nil {
			t.Error("Expected error for immediately cancelled context, got nil")
		} else if err != context.Canceled && err.Error() != "context canceled" {
			// HTTP 客户端可能会包装错误，所以检查错误信息
			if !strings.Contains(err.Error(), "context canceled") {
				t.Errorf("Expected context.Canceled or 'context canceled' error, got %v", err)
			}
		}

		// 回调可能被调用（如果 HTTP 请求已经完成），也可能不被调用（如果 HTTP 请求还没开始）
		// 这个行为取决于 HTTP 请求的时机
		t.Logf("回调是否被调用: %v", callbackCalled)
	})

	// 测试空响应体
	t.Run("空响应体", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 不写入任何数据
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		sess := New(URL(server.URL))

		var callbackCalled bool
		resp, err := sess.DoRequest(ctx,
			Stream(func(_ int64, _ []byte) error {
				callbackCalled = true
				return nil
			}),
		)

		// 应该成功完成（没有数据，所以不会超时）
		if err != nil {
			t.Errorf("Expected no error for empty response, got %v", err)
		}

		// 回调可能被调用（如果响应体有换行符等），也可能不被调用
		// 这个行为取决于响应体的具体内容
		t.Logf("空响应体测试: 回调是否被调用: %v", callbackCalled)

		// 验证响应对象存在
		if resp == nil {
			t.Error("Expected response object, got nil")
		}
	})

	// 测试单行数据
	t.Run("单行数据", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("single line\n"))
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		sess := New(URL(server.URL))

		var processedLines int64
		resp, err := sess.DoRequest(ctx,
			Stream(func(lineNum int64, data []byte) error {
				processedLines = lineNum
				return nil
			}),
		)

		// 应该成功完成
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		// 应该处理了 1-2 行（取决于最后是否有换行符）
		if processedLines < 1 || processedLines > 2 {
			t.Errorf("Expected 1-2 lines processed, got %d", processedLines)
		}

		// 验证响应对象存在
		if resp == nil {
			t.Error("Expected response object, got nil")
		}
	})
}
