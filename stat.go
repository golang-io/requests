// Package requests 提供了HTTP请求统计和性能监控功能
// Package requests provides HTTP request statistics and performance monitoring
package requests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// RequestId 是用于跟踪请求的HTTP头字段名称
// RequestId is the HTTP header field name used for request tracking
const RequestId = "Request-Id"

// dateTime 是统计信息中使用的时间格式
// dateTime is the time format used in statistics
const dateTime = "2006-01-02 15:04:05.000"

// Stat 是HTTP请求统计信息结构，记录了请求和响应的完整信息
// 该结构在客户端和服务器端都可以使用，但某些字段的含义略有不同
//
// Stat is the HTTP request statistics structure that records complete request and response information
// This structure can be used on both client and server side, but some fields have slightly different meanings
//
// 使用场景 / Use Cases:
//   - 性能监控和分析 / Performance monitoring and analysis
//   - 请求日志记录 / Request logging
//   - 调试和故障排查 / Debugging and troubleshooting
//   - API调用统计 / API call statistics
//
// 示例 / Example:
//
//	// 客户端获取统计信息
//	// Client getting statistics
//	resp, _ := session.DoRequest(ctx, requests.URL("http://example.com"))
//	stat := resp.Stat()
//	fmt.Printf("Request took %dms\n", stat.Cost)
//
//	// 服务器端使用Setup中间件记录统计
//	// Server using Setup middleware to record statistics
//	mux := requests.NewServeMux()
//	mux.Use(requests.Setup(func(ctx context.Context, stat *requests.Stat) {
//	    log.Printf("Request: %s %s - %dms", stat.Request.Method, stat.Request.URL, stat.Cost)
//	}))
type Stat struct {
	// RequestId 请求唯一标识符，用于跟踪和关联请求
	// RequestId is the unique identifier for request tracking and correlation
	RequestId string `json:"RequestId"`

	// StartAt 请求开始时间，格式为 "2006-01-02 15:04:05.000"
	// StartAt is the request start time in format "2006-01-02 15:04:05.000"
	StartAt string `json:"StartAt"`

	// Cost 请求总耗时，单位：毫秒
	// Cost is the total request duration in milliseconds
	Cost int64 `json:"Cost"`

	// Request 包含请求相关的所有信息
	// Request contains all request-related information
	Request struct {
		// RemoteAddr 远程地址（仅服务器端使用）
		// 客户端请求时此字段为空
		// RemoteAddr is the remote address (server side only)
		// For client requests, this field is unused
		RemoteAddr string `json:"RemoteAddr"`

		// URL 请求的URL地址
		// 客户端：完整的请求地址，包含 schema://ip:port/path/xx
		// 服务器端：仅包含路径，例如：/api/v1/xxx
		// URL is the request URL
		// Client: Full request address containing schema://ip:port/path/xx
		// Server: Only path, e.g., /api/v1/xxx
		URL string `json:"URL"`

		// Method HTTP请求方法（GET, POST, PUT, DELETE等）
		// Method is the HTTP request method (GET, POST, PUT, DELETE, etc.)
		Method string `json:"Method"`

		// Header 请求头信息（简化版，每个key只保留第一个value）
		// Header is the request headers (simplified, only first value per key)
		Header map[string]string `json:"Header"`

		// Body 请求体内容（可能是字符串或JSON对象）
		// Body is the request body content (may be string or JSON object)
		Body any `json:"Body"`
	} `json:"Request"`

	// Response 包含响应相关的所有信息
	// Response contains all response-related information
	Response struct {
		// URL 服务器地址，例如：http://127.0.0.1:8080（仅服务器端使用）
		// 客户端请求时此字段为空
		// URL is the server address, e.g., http://127.0.0.1:8080 (server side only)
		// For client requests, this field is unused
		URL string `json:"URL"`

		// Header 响应头信息（简化版，每个key只保留第一个value）
		// Header is the response headers (simplified, only first value per key)
		Header map[string]string `json:"Header"`

		// Body 响应体内容（可能是字符串或JSON对象）
		// Body is the response body content (may be string or JSON object)
		Body any `json:"Body"`

		// StatusCode HTTP响应状态码
		// StatusCode is the HTTP response status code
		StatusCode int `json:"StatusCode"`

		// ContentLength 响应体长度（字节）
		// ContentLength is the response body length in bytes
		ContentLength int64 `json:"ContentLength"`
	} `json:"Response"`

	// Err 错误信息（如果有）
	// Err is the error message (if any)
	Err string `json:"Err"`
}

// String 实现 fmt.Stringer 接口，将Stat对象序列化为JSON字符串
//
// # String implements the fmt.Stringer interface, serializes Stat object to JSON string
//
// 返回值 / Returns:
//   - string: JSON格式的统计信息字符串 / JSON formatted statistics string
//
// 示例 / Example:
//
//	stat := resp.Stat()
//	fmt.Println(stat.String()) // 输出完整的JSON格式统计信息 / Output full JSON statistics
func (stat *Stat) String() string {
	return a2s(stat)
}

// RequestBody 返回请求体内容的字符串表示
//
// # RequestBody returns the request body content as a string
//
// 返回值 / Returns:
//   - string: 请求体的字符串表示（JSON格式） / String representation of request body (JSON format)
//
// 示例 / Example:
//
//	stat := resp.Stat()
//	fmt.Printf("Request body: %s\n", stat.RequestBody())
func (stat *Stat) RequestBody() string {
	return a2s(stat.Request.Body)
}

// ResponseBody 返回响应体内容的字符串表示
//
// # ResponseBody returns the response body content as a string
//
// 返回值 / Returns:
//   - string: 响应体的字符串表示（JSON格式） / String representation of response body (JSON format)
//
// 示例 / Example:
//
//	stat := resp.Stat()
//	fmt.Printf("Response body: %s\n", stat.ResponseBody())
func (stat *Stat) ResponseBody() string {
	return a2s(stat.Response.Body)
}

// Print 返回格式化的日志字符串（主要用于服务器端）
// 输出格式：时间 方法 "客户端地址 -> 服务器地址+路径" - 状态码 响应大小 耗时
//
// Print returns a formatted log string (mainly used for server side)
// Format: Time Method "ClientAddr -> ServerAddr+Path" - StatusCode ResponseSize Duration
//
// 返回值 / Returns:
//   - string: 格式化的日志字符串 / Formatted log string
//
// 示例 / Example:
//
//	// 服务器端日志输出示例
//	// Server side log output example
//	// 2024-01-01 12:00:00.000 POST "192.168.1.100:8080 -> http://127.0.0.1:8080/api/v1/users" - 200 1024B in 150ms
//	log.Println(stat.Print())
func (stat *Stat) Print() string {
	return fmt.Sprintf("%s %s \"%s -> %s%s\" - %d %dB in %dms",
		stat.StartAt, stat.Request.Method,
		stat.Request.RemoteAddr, stat.Response.URL, stat.Request.URL,
		stat.Response.StatusCode, stat.Response.ContentLength, stat.Cost)
}

// responseLoad 从Response对象中提取并构建统计信息（客户端使用）
// 该函数会读取响应体内容并尝试解析为JSON，如果失败则保存为字符串
//
// responseLoad extracts and builds statistics from Response object (used by client)
// This function reads response body content and tries to parse it as JSON, or saves it as string if parsing fails
//
// 参数 / Parameters:
//   - resp: *Response - 响应对象 / Response object
//
// 返回值 / Returns:
//   - *Stat: 统计信息对象 / Statistics object
func responseLoad(resp *Response) *Stat {
	stat := &Stat{
		StartAt: resp.StartAt.Format(dateTime),
		Cost:    time.Since(resp.StartAt).Milliseconds(),
	}
	if resp.Response != nil {
		var err error
		if resp.Content == nil || resp.Content.Len() == 0 {
			if resp.Content, resp.Response.Body, err = CopyBody(resp.Response.Body); err != nil {
				stat.Err += fmt.Sprintf("read response: %s", err)
				return stat
			}
		}
		stat.Response.Body = make(map[string]any)
		if err := json.Unmarshal(resp.Content.Bytes(), &stat.Response.Body); err != nil {
			stat.Response.Body = resp.Content.String()
		}

		stat.Response.Header = make(map[string]string)
		for k, v := range resp.Response.Header {
			stat.Response.Header[k] = v[0]
		}
		stat.Response.ContentLength = resp.Response.ContentLength
		if stat.Response.ContentLength == -1 && resp.Content.Len() != 0 {
			stat.Response.ContentLength = int64(resp.Content.Len())
		}
		stat.Response.StatusCode = resp.StatusCode
	}
	if resp.Request != nil {
		stat.RequestId = resp.Request.Header.Get(RequestId)
		stat.Request.Method = resp.Request.Method
		stat.Request.URL = resp.Request.URL.String()
		if resp.Request.GetBody != nil {
			body, err := resp.Request.GetBody()
			if err != nil {
				stat.Err += fmt.Sprintf("read request1: %s", err)
				return stat
			}

			buf, err := ParseBody(body)
			if err != nil {
				stat.Err += fmt.Sprintf("read request2: %s", err)
				return stat
			}

			m := make(map[string]any)

			if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
				stat.Request.Body = buf.String()
			} else {
				stat.Request.Body = m
			}
		}

		stat.Request.Header = make(map[string]string)

		for k, v := range resp.Request.Header {
			stat.Request.Header[k] = v[0]
		}
	}

	if resp.Err != nil {
		stat.Err = resp.Err.Error()
	}
	return stat
}

// serveLoad 从HTTP请求和响应中提取并构建统计信息（服务器端使用）
// 该函数会记录服务器端的请求处理统计信息
//
// serveLoad extracts and builds statistics from HTTP request and response (used by server)
// This function records server-side request processing statistics
//
// 参数 / Parameters:
//   - w: *ResponseWriter - 响应写入器，包含响应内容和状态码 / Response writer with response content and status code
//   - r: *http.Request - HTTP请求对象 / HTTP request object
//   - start: time.Time - 请求开始时间 / Request start time
//   - buf: *bytes.Buffer - 请求体缓冲区 / Request body buffer
//
// 返回值 / Returns:
//   - *Stat: 统计信息对象 / Statistics object
func serveLoad(w *ResponseWriter, r *http.Request, start time.Time, buf *bytes.Buffer) *Stat {
	stat := &Stat{
		StartAt: start.Format("2006-01-02 15:04:05.000"),
		Cost:    time.Since(start).Milliseconds(),
	}
	stat.Request.RemoteAddr = r.RemoteAddr
	stat.Request.Method = r.Method
	stat.Request.Header = make(map[string]string)
	for k, v := range r.Header {
		stat.Request.Header[k] = v[0]
	}
	stat.Request.URL = r.URL.String()

	if buf != nil {
		m := make(map[string]any)
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			stat.Request.Body = buf.String()
		} else {
			stat.Request.Body = m
		}
	}
	scheme := "http://"
	if r.TLS != nil {
		scheme = "https://"
	}
	stat.Response.URL = scheme + r.Host
	stat.Response.StatusCode = w.StatusCode
	stat.Response.ContentLength = int64(w.Content.Len())
	stat.Response.Header = make(map[string]string)
	for k, v := range r.Header {
		stat.Response.Header[k] = v[0]
	}
	stat.Response.Body = w.Content.String()
	return stat
}

// a2s (any to string) 将任意类型的值转换为JSON字符串
// 如果JSON序列化失败，则使用fmt.Sprintf格式化输出
//
// a2s (any to string) converts any type value to JSON string
// If JSON serialization fails, use fmt.Sprintf for formatted output
//
// 参数 / Parameters:
//   - v: any - 需要转换的值 / Value to convert
//
// 返回值 / Returns:
//   - string: JSON字符串或格式化字符串 / JSON string or formatted string
func a2s(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
