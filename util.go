// Package requests 提供了HTTP客户端和服务器的工具函数
// Package requests provides utility functions for HTTP client and server
package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// ParseBody 从 Request.Body 中解析并读取所有内容到内存缓冲区
// 该函数会自动处理空body和http.NoBody的情况
//
// ParseBody parses and reads all content from Request.Body into a memory buffer
// This function automatically handles empty body and http.NoBody cases
//
// 参数 / Parameters:
//   - r: io.ReadCloser - 需要解析的请求体 / The request body to parse
//
// 返回值 / Returns:
//   - *bytes.Buffer: 包含请求体内容的缓冲区 / Buffer containing the request body content
//   - error: 读取过程中的错误 / Error during reading
//
// 使用场景 / Use Cases:
//   - 需要多次读取请求体内容时 / When you need to read request body multiple times
//   - 需要缓存请求体以便后续处理 / When you need to cache request body for later processing
//   - 中间件需要检查请求体内容 / When middleware needs to inspect request body
//
// 示例 / Example:
//
//	// 读取并缓存请求体
//	// Read and cache request body
//	buf, err := requests.ParseBody(req.Body)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Body content: %s\n", buf.String())
func ParseBody(r io.ReadCloser) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if r == nil || r == http.NoBody {
		// No copying needed. Preserve the magic sentinel meaning of NoBody.
		// 无需复制，保留 NoBody 的特殊含义
		return &buf, nil
	}
	if _, err := buf.ReadFrom(r); err != nil {
		return &buf, err
	}
	return &buf, r.Close()
}

// CopyBody 将ReadCloser的所有内容读取到内存，然后返回两个等价的ReadCloser
// 两个返回的ReadCloser都能读取相同的字节内容
//
// CopyBody reads all of b to memory and then returns two equivalent
// ReadClosers yielding the same bytes.
//
// 参数 / Parameters:
//   - b: io.ReadCloser - 需要复制的原始body / The original body to copy
//
// 返回值 / Returns:
//   - *bytes.Buffer: 包含body内容的缓冲区 / Buffer containing the body content
//   - io.ReadCloser: 可以重新读取body内容的ReadCloser / ReadCloser that can read the body content again
//   - error: 读取失败时的错误 / Error if reading fails
//
// 注意事项 / Notes:
//   - 如果初始读取所有字节失败，则返回错误 / Returns error if the initial slurp of all bytes fails
//   - 不保证返回的ReadCloser具有相同的错误匹配行为 / Does not guarantee identical error-matching behavior
//   - 主要用于需要多次读取body的场景 / Mainly used for scenarios requiring multiple reads of the body
//
// 使用场景 / Use Cases:
//   - 中间件需要检查请求体但不能消耗它 / Middleware needs to inspect request body without consuming it
//   - 需要记录请求体同时还要转发给下一个处理器 / Need to log request body while forwarding to next handler
//   - 实现请求重试功能时保留原始请求体 / Preserve original request body for retry functionality
//
// 示例 / Example:
//
//	// 复制请求体以便多次使用
//	// Copy request body for multiple uses
//	buf, bodyReader, err := requests.CopyBody(req.Body)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// 可以从buf读取内容 / Can read content from buf
//	fmt.Printf("Body: %s\n", buf.String())
//	// 可以将bodyReader赋值回req.Body / Can assign bodyReader back to req.Body
//	req.Body = bodyReader
func CopyBody(b io.ReadCloser) (*bytes.Buffer, io.ReadCloser, error) {
	buf, err := ParseBody(b)
	if err != nil {
		return nil, nil, err
	}
	return buf, io.NopCloser(bytes.NewReader(buf.Bytes())), nil
}

// LogS 是默认的Stat统计信息处理函数，将统计信息打印到标准输出
// 该函数会格式化输出请求和响应的详细信息
//
// LogS is the default Stat handler function that prints statistics to stdout
// This function formats and outputs detailed request and response information
//
// 参数 / Parameters:
//   - ctx: context.Context - 上下文对象（未使用，为了接口一致性） / Context object (unused, for interface consistency)
//   - stat: *Stat - 请求统计信息对象 / Request statistics object
//
// 输出格式 / Output Format:
//   - 如果没有响应URL，则输出完整的JSON格式的stat / If no response URL, outputs full JSON formatted stat
//   - 否则输出简化的日志格式，包含请求体和响应体 / Otherwise outputs simplified log format with request and response body
//
// 使用场景 / Use Cases:
//   - 作为Setup()的默认回调函数 / As the default callback function for Setup()
//   - 调试HTTP请求和响应 / Debugging HTTP requests and responses
//   - 监控请求性能和错误 / Monitoring request performance and errors
//
// 示例 / Example:
//
//	// 使用默认日志输出
//	// Use default logging output
//	session := requests.New(
//	    requests.Setup(requests.LogS),
//	)
//
//	// 或者作为服务器中间件
//	// Or as server middleware
//	mux := requests.NewServeMux()
//	mux.Use(requests.Setup(requests.LogS))
func LogS(ctx context.Context, stat *Stat) {
	if stat.Response.URL == "" {
		_, _ = fmt.Printf("%s\n", stat)
		return
	}
	if b, err := json.Marshal(stat.Request.Body); err != nil {
		log.Printf(`%s # body=%v, resp="%v", err=%v`, stat.Print(), stat.Request.Body, stat.Response.Body, err)
	} else {
		log.Printf(`%s # body=%s, resp="%v"`, stat.Print(), b, stat.Response.Body)
	}
}
