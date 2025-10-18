package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// makeBody 将各种输入类型转换为适用于HTTP请求体的 io.Reader
// makeBody converts various input types to an io.Reader suitable for HTTP request bodies
//
// 支持的类型 / Supported types:
//   - nil: 返回 nil（无请求体）/ returns nil (no body)
//   - []byte: 返回 bytes.Reader / returns a bytes.Reader
//   - string: 返回 strings.Reader / returns a strings.Reader
//   - *bytes.Buffer, bytes.Buffer: 返回 io.Reader / returns as io.Reader
//   - io.Reader, io.ReadSeeker, *bytes.Reader, *strings.Reader: 直接返回 / returns as is
//   - url.Values: 返回编码后的表单值（strings.Reader）/ returns encoded form values as strings.Reader
//   - func() (io.ReadCloser, error): 调用函数并返回结果 / calls the function and returns the result
//   - 其他类型: 序列化为JSON并返回 bytes.Reader / any other type: marshals to JSON and returns as bytes.Reader
//
// 参数 / Parameters:
//   - body: 请求体数据，可以是多种类型 / Request body data, can be various types
//
// 返回值 / Returns:
//   - io.Reader: 转换后的请求体读取器 / Converted request body reader
//   - error: 转换过程中的错误（如JSON序列化失败）/ Error during conversion (e.g., JSON marshaling failure)
//
// 示例 / Example:
//
//	// 使用字符串 / Using string
//	reader, _ := makeBody("hello world")
//
//	// 使用结构体（自动JSON序列化）/ Using struct (automatic JSON serialization)
//	type User struct {
//	    Name string `json:"name"`
//	}
//	reader, _ := makeBody(User{Name: "John"})
//
//	// 使用字节切片 / Using byte slice
//	reader, _ := makeBody([]byte("data"))
func makeBody(body any) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}

	switch v := body.(type) {
	case []byte:
		return bytes.NewReader(v), nil
	case string:
		return strings.NewReader(v), nil
	case *bytes.Buffer:
		return body.(io.Reader), nil
	case io.Reader, io.ReadSeeker, *bytes.Reader, *strings.Reader:
		return body.(io.Reader), nil
	case url.Values:
		return strings.NewReader(v.Encode()), nil
	case func() (io.ReadCloser, error):
		return v()
	default:
		// 尝试将其他类型序列化为JSON
		// Try to serialize other types as JSON
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(b), nil
	}
}

// NewRequestWithContext 使用给定的上下文和选项创建一个新的HTTP请求
// NewRequestWithContext creates a new HTTP request with the given context and options
//
// 功能 / Features:
//   - 将请求体转换为适当的 io.Reader / Converting the request body to an appropriate io.Reader
//   - 设置请求方法和URL / Setting the request method and URL
//   - 追加路径段 / Appending path segments
//   - 设置查询参数 / Setting query parameters
//   - 设置请求头和Cookie / Setting headers and cookies
//
// 参数 / Parameters:
//   - ctx: 请求上下文，用于控制超时和取消 / Request context for controlling timeout and cancellation
//   - options: 请求选项配置 / Request options configuration
//
// 返回值 / Returns:
//   - *http.Request: 构造好的HTTP请求对象 / The constructed http.Request object
//   - error: 创建过程中的错误 / Error encountered during creation
//
// 示例 / Example:
//
//	options := Options{
//	    Method: "POST",
//	    URL: "https://api.example.com",
//	    Path: []string{"/users", "/123"},
//	    body: map[string]string{"name": "John"},
//	}
//	req, err := NewRequestWithContext(context.Background(), options)
func NewRequestWithContext(ctx context.Context, options Options) (*http.Request, error) {
	// 转换请求体
	// Convert request body
	body, err := makeBody(options.body)
	if err != nil {
		return nil, err
	}

	// 创建HTTP请求
	// Create HTTP request
	r, err := http.NewRequestWithContext(ctx, options.Method, options.URL, body)
	if err != nil {
		return nil, err
	}

	// 追加路径段
	// Append path segments
	for _, p := range options.Path {
		r.URL.Path += p
	}

	// 设置查询参数
	// Set query parameters
	r.URL.RawQuery = options.RawQuery.Encode()

	// 设置请求头
	// Set headers
	r.Header = options.Header

	// 添加Cookie
	// Add cookies
	for _, cookie := range options.Cookies {
		r.AddCookie(&cookie)
	}

	return r, nil
}
