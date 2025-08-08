package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestStat_String(t *testing.T) {
	stat := &Stat{
		RequestId: "test-request-id",
		StartAt:   "2023-05-01 12:00:00.000",
		Cost:      100,
	}
	stat.Request.Method = "GET"
	stat.Request.URL = "http://example.com/test"
	stat.Response.StatusCode = 200
	stat.Response.ContentLength = 1024

	jsonStr := stat.String()
	var parsedStat map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsedStat); err != nil {
		t.Errorf("无法解析 Stat.String() 的输出: %v", err)
	}

	if parsedStat["RequestId"] != "test-request-id" {
		t.Errorf("期望 RequestId 为 'test-request-id'，实际为 %v", parsedStat["RequestId"])
	}
}

func TestStat_Print(t *testing.T) {
	stat := &Stat{
		StartAt: "2023-05-01 12:00:00.000",
		Cost:    100,
	}
	stat.Request.Method = "GET"
	stat.Request.RemoteAddr = "192.168.1.1:8080"
	stat.Request.URL = "/api/v1/test"
	stat.Response.URL = "http://example.com"
	stat.Response.StatusCode = 200
	stat.Response.ContentLength = 1024

	printStr := stat.Print()
	expected := "2023-05-01 12:00:00.000 GET \"192.168.1.1:8080 -> http://example.com/api/v1/test\" - 200 1024B in 100ms"
	if printStr != expected {
		t.Errorf("期望输出为 '%s'，实际为 '%s'", expected, printStr)
	}
}

func TestResponseLoad(t *testing.T) {
	// 创建一个模拟的 HTTP 响应
	httpResp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Test":       []string{"test-value"},
		},
		Body: io.NopCloser(strings.NewReader(`{"message":"success"}`)),
	}

	// 创建一个模拟的请求
	req, _ := http.NewRequest("GET", "http://example.com/test?param=value", nil)
	req.Header.Set(RequestId, "test-request-id")
	req.Header.Set("User-Agent", "test-agent")

	// 创建响应对象
	resp := &Response{
		Response: httpResp,
		Request:  req,
		StartAt:  time.Now().Add(-100 * time.Millisecond), // 100ms 前
	}

	// 测试 responseLoad 函数
	stat := responseLoad(resp)

	// 验证基本字段
	if stat.RequestId != "test-request-id" {
		t.Errorf("期望 RequestId 为 'test-request-id'，实际为 %s", stat.RequestId)
	}

	if stat.Request.Method != "GET" {
		t.Errorf("期望 Method 为 'GET'，实际为 %s", stat.Request.Method)
	}

	if !strings.Contains(stat.Request.URL, "http://example.com/test?param=value") {
		t.Errorf("期望 URL 包含 'http://example.com/test?param=value'，实际为 %s", stat.Request.URL)
	}

	if stat.Response.StatusCode != 200 {
		t.Errorf("期望 StatusCode 为 200，实际为 %d", stat.Response.StatusCode)
	}

	if stat.Response.Header["Content-Type"] != "application/json" {
		t.Errorf("期望 Content-Type 为 'application/json'，实际为 %s", stat.Response.Header["Content-Type"])
	}

	// 验证响应体解析
	responseBody, ok := stat.Response.Body.(map[string]interface{})
	if !ok {
		t.Errorf("期望 Response.Body 为 map[string]interface{}，实际为 %T", stat.Response.Body)
	} else if responseBody["message"] != "success" {
		t.Errorf("期望 message 为 'success'，实际为 %v", responseBody["message"])
	}
}

func TestServeLoad(t *testing.T) {
	// 创建一个模拟的 HTTP 请求
	req, _ := http.NewRequest("POST", "/api/v1/test?param=value", strings.NewReader(`{"data":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "192.168.1.1:8080"

	// 创建一个模拟的响应写入器
	w := &ResponseWriter{
		StatusCode: 201,
		Content:    bytes.NewBufferString(`{"status":"created"}`),
	}

	// 创建请求体缓冲区
	buf := bytes.NewBufferString(`{"data":"test"}`)

	// 测试 serveLoad 函数
	start := time.Now().Add(-200 * time.Millisecond) // 200ms 前
	stat := serveLoad(w, req, start, buf)

	// 验证基本字段
	if stat.Request.Method != "POST" {
		t.Errorf("期望 Method 为 'POST'，实际为 %s", stat.Request.Method)
	}

	if stat.Request.RemoteAddr != "192.168.1.1:8080" {
		t.Errorf("期望 RemoteAddr 为 '192.168.1.1:8080'，实际为 %s", stat.Request.RemoteAddr)
	}

	if !strings.Contains(stat.Request.URL, "/api/v1/test?param=value") {
		t.Errorf("期望 URL 包含 '/api/v1/test?param=value'，实际为 %s", stat.Request.URL)
	}

	if stat.Response.StatusCode != 201 {
		t.Errorf("期望 StatusCode 为 201，实际为 %d", stat.Response.StatusCode)
	}

	if stat.Response.ContentLength != int64(w.Content.Len()) { // `{"status":"created"}` 的长度
		t.Errorf("期望 ContentLength 为 %d，实际为 %d", int64(w.Content.Len()), stat.Response.ContentLength)
	}

	// 验证请求体解析
	requestBody, ok := stat.Request.Body.(map[string]interface{})
	if !ok {
		t.Errorf("期望 Request.Body 为 map[string]interface{}，实际为 %T", stat.Request.Body)
	} else if requestBody["data"] != "test" {
		t.Errorf("期望 data 为 'test'，实际为 %v", requestBody["data"])
	}

	// 验证响应体
	if stat.Response.Body != `{"status":"created"}` {
		t.Errorf("期望 Response.Body 为 '{\"status\":\"created\"}'，实际为 %v", stat.Response.Body)
	}
}

func TestStat_WithError(t *testing.T) {
	// 测试带有错误的情况
	resp := &Response{
		Err:     fmt.Errorf("测试错误"),
		StartAt: time.Now().Add(-50 * time.Millisecond),
	}

	stat := responseLoad(resp)
	if stat.Err != "测试错误" {
		t.Errorf("期望错误信息为 '测试错误'，实际为 '%s'", stat.Err)
	}
}

// TestStat_RequestBody 测试RequestBody方法
func TestStat_RequestBody(t *testing.T) {
	tests := []struct {
		name     string
		body     any
		expected string
		desc     string
	}{
		{
			name:     "nil_body",
			body:     nil,
			expected: "null",
			desc:     "nil body应该返回null字符串",
		},
		{
			name:     "string_body",
			body:     "hello world",
			expected: `"hello world"`,
			desc:     "字符串body应该正确序列化",
		},
		{
			name:     "map_body",
			body:     map[string]any{"key": "value", "number": 123},
			expected: `{"key":"value","number":123}`,
			desc:     "map body应该正确序列化为JSON",
		},
		{
			name:     "slice_body",
			body:     []string{"a", "b", "c"},
			expected: `["a","b","c"]`,
			desc:     "slice body应该正确序列化为JSON",
		},
		{
			name:     "number_body",
			body:     42,
			expected: "42",
			desc:     "数字body应该正确序列化",
		},
		{
			name:     "boolean_body",
			body:     true,
			expected: "true",
			desc:     "布尔值body应该正确序列化",
		},
		{
			name: "struct_body",
			body: struct {
				Name string
				Age  int
			}{"Alice", 30},
			expected: `{"Name":"Alice","Age":30}`,
			desc:     "结构体body应该正确序列化为JSON",
		},
		{
			name:     "empty_map",
			body:     map[string]any{},
			expected: "{}",
			desc:     "空map应该序列化为空JSON对象",
		},
		{
			name:     "empty_slice",
			body:     []any{},
			expected: "[]",
			desc:     "空slice应该序列化为空JSON数组",
		},
		{
			name: "nested_structure",
			body: map[string]any{
				"user": map[string]any{
					"name": "Bob",
					"age":  25,
					"hobbies": []string{
						"reading",
						"swimming",
					},
				},
				"active": true,
			},
			expected: `{"user":{"age":25,"hobbies":["reading","swimming"],"name":"Bob"},"active":true}`,
			desc:     "嵌套结构应该正确序列化",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stat := &Stat{}
			stat.Request.Body = tt.body

			result := stat.RequestBody()

			// 对于可以JSON序列化的数据，验证JSON格式
			if tt.body != nil {
				// 尝试解析结果是否为有效JSON
				var parsed any
				if err := json.Unmarshal([]byte(result), &parsed); err != nil {
					// 如果不是有效JSON，检查是否为fmt.Sprintf的输出
					expectedFallback := fmt.Sprintf("%v", tt.body)
					if result != expectedFallback {
						t.Errorf("测试 '%s': 期望 '%s' 或 '%s'，实际为 '%s'",
							tt.name, tt.expected, expectedFallback, result)
					}
				} else {
					// 如果是有效JSON，验证内容（不依赖字段顺序）
					var expectedParsed any
					if err := json.Unmarshal([]byte(tt.expected), &expectedParsed); err != nil {
						t.Errorf("测试 '%s': 期望值不是有效JSON: %v", tt.name, err)
					} else {
						// 比较解析后的JSON对象
						if !reflect.DeepEqual(parsed, expectedParsed) {
							t.Errorf("测试 '%s': JSON内容不匹配，期望 %v，实际 %v",
								tt.name, expectedParsed, parsed)
						}
					}
				}
			} else {
				// 对于nil，应该返回"null"
				if result != tt.expected {
					t.Errorf("测试 '%s': 期望 '%s'，实际为 '%s'",
						tt.name, tt.expected, result)
				}
			}
		})
	}
}

// TestStat_ResponseBody 测试ResponseBody方法
func TestStat_ResponseBody(t *testing.T) {
	tests := []struct {
		name     string
		body     any
		expected string
		desc     string
	}{
		{
			name:     "nil_body",
			body:     nil,
			expected: "null",
			desc:     "nil body应该返回null字符串",
		},
		{
			name:     "string_body",
			body:     "response data",
			expected: `"response data"`,
			desc:     "字符串body应该正确序列化",
		},
		{
			name:     "map_body",
			body:     map[string]any{"status": "success", "data": "test"},
			expected: `{"data":"test","status":"success"}`,
			desc:     "map body应该正确序列化为JSON",
		},
		{
			name:     "slice_body",
			body:     []int{1, 2, 3, 4, 5},
			expected: `[1,2,3,4,5]`,
			desc:     "slice body应该正确序列化为JSON",
		},
		{
			name:     "number_body",
			body:     3.14159,
			expected: "3.14159",
			desc:     "浮点数body应该正确序列化",
		},
		{
			name:     "boolean_body",
			body:     false,
			expected: "false",
			desc:     "布尔值body应该正确序列化",
		},
		{
			name: "struct_body",
			body: struct {
				Message string
				Code    int
			}{"OK", 200},
			expected: `{"Code":200,"Message":"OK"}`,
			desc:     "结构体body应该正确序列化为JSON",
		},
		{
			name:     "empty_map",
			body:     map[string]any{},
			expected: "{}",
			desc:     "空map应该序列化为空JSON对象",
		},
		{
			name:     "empty_slice",
			body:     []any{},
			expected: "[]",
			desc:     "空slice应该序列化为空JSON数组",
		},
		{
			name: "complex_nested_structure",
			body: map[string]any{
				"api": map[string]any{
					"version": "1.0",
					"endpoints": []map[string]any{
						{"name": "users", "method": "GET"},
						{"name": "posts", "method": "POST"},
					},
				},
				"timestamp": "2023-05-01T12:00:00Z",
			},
			expected: `{"api":{"endpoints":[{"method":"GET","name":"users"},{"method":"POST","name":"posts"}],"version":"1.0"},"timestamp":"2023-05-01T12:00:00Z"}`,
			desc:     "复杂嵌套结构应该正确序列化",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stat := &Stat{}
			stat.Response.Body = tt.body

			result := stat.ResponseBody()

			// 对于可以JSON序列化的数据，验证JSON格式
			if tt.body != nil {
				// 尝试解析结果是否为有效JSON
				var parsed any
				if err := json.Unmarshal([]byte(result), &parsed); err != nil {
					// 如果不是有效JSON，检查是否为fmt.Sprintf的输出
					expectedFallback := fmt.Sprintf("%v", tt.body)
					if result != expectedFallback {
						t.Errorf("测试 '%s': 期望 '%s' 或 '%s'，实际为 '%s'",
							tt.name, tt.expected, expectedFallback, result)
					}
				} else {
					// 如果是有效JSON，验证内容（不依赖字段顺序）
					var expectedParsed any
					if err := json.Unmarshal([]byte(tt.expected), &expectedParsed); err != nil {
						t.Errorf("测试 '%s': 期望值不是有效JSON: %v", tt.name, err)
					} else {
						// 比较解析后的JSON对象
						if !reflect.DeepEqual(parsed, expectedParsed) {
							t.Errorf("测试 '%s': JSON内容不匹配，期望 %v，实际 %v",
								tt.name, expectedParsed, parsed)
						}
					}
				}
			} else {
				// 对于nil，应该返回"null"
				if result != tt.expected {
					t.Errorf("测试 '%s': 期望 '%s'，实际为 '%s'",
						tt.name, tt.expected, result)
				}
			}
		})
	}
}

// TestStat_BodyMethods_EdgeCases 测试RequestBody和ResponseBody的边界情况
func TestStat_BodyMethods_EdgeCases(t *testing.T) {
	t.Run("RequestBody边界情况", func(t *testing.T) {
		stat := &Stat{}

		// 测试无法JSON序列化的类型
		ch := make(chan int)
		stat.Request.Body = ch
		result := stat.RequestBody()
		expected := fmt.Sprintf("%v", ch)
		if result != expected {
			t.Errorf("无法序列化的类型应该使用fmt.Sprintf，期望 '%s'，实际为 '%s'", expected, result)
		}

		// 测试函数类型
		testFunc := func() {}
		stat.Request.Body = testFunc
		result = stat.RequestBody()
		// 函数类型无法直接使用fmt.Sprintf，但RequestBody应该返回一个字符串表示
		if result == "" {
			t.Errorf("函数类型应该返回非空字符串，实际为空")
		}

		// 测试循环引用
		type CircularRef struct {
			Self *CircularRef
		}
		circular := &CircularRef{}
		circular.Self = circular
		stat.Request.Body = circular
		result = stat.RequestBody()
		expected = fmt.Sprintf("%v", circular)
		if result != expected {
			t.Errorf("循环引用应该使用fmt.Sprintf，期望 '%s'，实际为 '%s'", expected, result)
		}
	})

	t.Run("ResponseBody边界情况", func(t *testing.T) {
		stat := &Stat{}

		// 测试无法JSON序列化的类型
		ch := make(chan string)
		stat.Response.Body = ch
		result := stat.ResponseBody()
		expected := fmt.Sprintf("%v", ch)
		if result != expected {
			t.Errorf("无法序列化的类型应该使用fmt.Sprintf，期望 '%s'，实际为 '%s'", expected, result)
		}

		// 测试接口类型
		var iface any = "interface value"
		stat.Response.Body = iface
		result = stat.ResponseBody()
		expected = `"interface value"`
		if result != expected {
			t.Errorf("接口类型应该正确序列化，期望 '%s'，实际为 '%s'", expected, result)
		}

		// 测试指针类型
		str := "pointer value"
		stat.Response.Body = &str
		result = stat.ResponseBody()
		expected = `"pointer value"`
		if result != expected {
			t.Errorf("指针类型应该正确序列化，期望 '%s'，实际为 '%s'", expected, result)
		}
	})
}

// TestStat_BodyMethods_Integration 测试RequestBody和ResponseBody的集成场景
func TestStat_BodyMethods_Integration(t *testing.T) {
	t.Run("完整Stat对象的RequestBody和ResponseBody", func(t *testing.T) {
		stat := &Stat{
			RequestId: "test-integration",
			StartAt:   "2023-05-01 12:00:00.000",
			Cost:      150,
		}

		// 设置请求数据
		stat.Request.Method = "POST"
		stat.Request.URL = "http://example.com/api/users"
		stat.Request.Header = map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
		}
		stat.Request.Body = map[string]any{
			"name":  "John Doe",
			"email": "john@example.com",
			"age":   30,
		}

		// 设置响应数据
		stat.Response.StatusCode = 201
		stat.Response.ContentLength = 256
		stat.Response.Header = map[string]string{
			"Content-Type": "application/json",
			"Location":     "http://example.com/api/users/123",
		}
		stat.Response.Body = map[string]any{
			"id":      123,
			"name":    "John Doe",
			"email":   "john@example.com",
			"created": "2023-05-01T12:00:00Z",
		}

		// 测试RequestBody
		requestBody := stat.RequestBody()
		expectedRequestBody := `{"age":30,"email":"john@example.com","name":"John Doe"}`
		if requestBody != expectedRequestBody {
			t.Errorf("RequestBody集成测试失败，期望 '%s'，实际为 '%s'", expectedRequestBody, requestBody)
		}

		// 测试ResponseBody
		responseBody := stat.ResponseBody()
		expectedResponseBody := `{"created":"2023-05-01T12:00:00Z","email":"john@example.com","id":123,"name":"John Doe"}`
		if responseBody != expectedResponseBody {
			t.Errorf("ResponseBody集成测试失败，期望 '%s'，实际为 '%s'", expectedResponseBody, responseBody)
		}

		// 验证String方法包含正确的body信息
		jsonStr := stat.String()
		if !strings.Contains(jsonStr, expectedRequestBody) {
			t.Errorf("String方法应该包含正确的RequestBody，但未找到期望的内容")
		}
		if !strings.Contains(jsonStr, expectedResponseBody) {
			t.Errorf("String方法应该包含正确的ResponseBody，但未找到期望的内容")
		}
	})
}
