package requests

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
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

			gotLen, err := streamRead(reader, func(_ int64, data []byte) error {
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

	_, err := streamRead(reader, func(_ int64, _ []byte) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// 测试读取错误的情况
	errorReader := &errorReader{err: errors.New("read error")}
	_, err = streamRead(errorReader, func(_ int64, _ []byte) error {
		return nil
	})

	if err == nil || !strings.Contains(err.Error(), "read error") {
		t.Errorf("Expected read error, got %v", err)
	}
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
