package requests

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// Response 包装了 http.Response 结构体，提供了额外的功能
// Response wraps http.Response struct with additional features
//
// 主要功能 / Main Features:
//  1. 记录请求开始时间和耗时 / Record request start time and duration
//  2. 自动缓存响应内容 / Automatically cache response content
//  3. 统一的错误处理 / Unified error handling
//  4. 统计信息收集 / Statistics collection
//  5. 自动安全关闭响应体 / Auto-safe close response body
//
// 使用场景 / Use Cases:
//   - 需要记录请求耗时 / Need to track request duration
//   - 需要多次读取响应内容 / Need to read response content multiple times
//   - 需要收集请求统计信息 / Need to collect request statistics
//   - 希望自动处理响应体关闭 / Want automatic response body closing
//
// 示例 / Example:
//
//	sess := requests.New(requests.URL("https://api.example.com"))
//	resp, err := sess.DoRequest(context.Background())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	// Content已经自动缓存，无需担心Body关闭问题
//	// Content is auto-cached, no need to worry about Body closing
//	fmt.Println(resp.Content.String())
//	fmt.Printf("Request took %v\n", resp.Cost)
type Response struct {
	*http.Request                // 原始 HTTP 请求 / Original HTTP request
	*http.Response               // 原始 HTTP 响应 / Original HTTP response
	StartAt        time.Time     // 请求开始时间 / Request start time
	Cost           time.Duration // 请求耗时 / Request duration
	Content        *bytes.Buffer // 响应内容缓存（已自动读取）/ Response content cache (auto-read)
	Err            error         // 请求过程中的错误 / Error during request
}

// newResponse 创建一个新的 Response 实例
// newResponse creates a new Response instance
//
// 参数 / Parameters:
//   - r: 原始的 HTTP 请求对象 / Original HTTP request object
//
// 返回值 / Returns:
//   - *Response: 初始化的响应对象 / Initialized response object
func newResponse(r *http.Request) *Response {
	return &Response{
		Request:  r,
		StartAt:  time.Now(),
		Response: &http.Response{},
		Content:  &bytes.Buffer{},
	}
}

// String 实现 fmt.Stringer 接口
// String implements the fmt.Stringer interface
//
// 返回响应内容的字符串形式，便于打印和调试
// Returns the string representation of response content for easy printing and debugging
//
// 返回值 / Returns:
//   - string: 响应内容的字符串 / String representation of response content
//
// 示例 / Example:
//
//	resp, _ := sess.DoRequest(context.Background())
//	fmt.Println(resp.String()) // 打印完整响应内容 / Print full response content
func (resp *Response) String() string {
	return resp.Content.String()
}

// Error 实现 error 接口
// Error implements the error interface
//
// 允许 Response 对象作为 error 类型使用
// Allows Response object to be used as an error type
//
// 返回值 / Returns:
//   - string: 错误信息，如果没有错误则返回空字符串 / Error message, or empty string if no error
func (resp *Response) Error() string {
	if resp.Err == nil {
		return ""
	}
	return resp.Err.Error()
}

// JSON 将响应内容反序列化为指定的类型
// JSON deserializes the response content into the specified type
//
// 参数 / Parameters:
//   - v: 接收反序列化结果的指针 / Pointer to receive deserialized result
//
// 返回值 / Returns:
//   - error: 反序列化错误 / Deserialization error
//
// 示例 / Example:
//
//	type User struct {
//	    Name string `json:"name"`
//	    Age  int    `json:"age"`
//	}
//	var user User
//	if err := resp.JSON(&user); err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("User: %s, Age: %d\n", user.Name, user.Age)
func (resp *Response) JSON(v any) error {
	return json.Unmarshal(resp.Content.Bytes(), v)
}

// Stat 返回请求的统计信息
// Stat returns the request statistics
//
// 包括 / Includes:
//   - 请求和响应的详细信息 / Detailed request and response information
//   - 请求耗时 / Request duration
//   - 请求和响应的头部信息 / Request and response headers
//   - 请求和响应的内容 / Request and response content
//
// 返回值 / Returns:
//   - *Stat: 统计信息对象 / Statistics object
//
// 示例 / Example:
//
//	resp, _ := sess.DoRequest(context.Background())
//	stat := resp.Stat()
//	fmt.Printf("Request took %dms\n", stat.Cost)
//	fmt.Printf("Status code: %d\n", stat.Response.StatusCode)
func (resp *Response) Stat() *Stat {
	return responseLoad(resp)
}

// streamRead 按行读取数据流（用于流式处理大文件或实时数据）
// streamRead reads data stream line by line (for streaming large files or real-time data)
//
// 特点 / Features:
//   - 使用 1MB 缓冲，提高读取效率 / Uses 1MB buffer for efficient reading
//   - 按换行符分割数据 / Splits data by newline character
//   - 支持边读边处理，减少内存占用 / Supports read-and-process, reducing memory usage
//   - 支持 Context 取消，可中断长时间运行的流式处理 / Supports Context cancellation to interrupt long-running stream processing
//   - 适用于大文件下载、日志流等场景 / Suitable for large file downloads, log streams, etc.
//
// 参数 / Parameters:
//   - ctx: 上下文对象，用于取消操作 / Context for cancellation
//   - reader: 输入的数据流 / Input data stream
//   - fn: 处理每一行数据的回调函数，参数为(行号, 行内容) / Callback function to process each line, parameters: (line number, line content)
//
// 返回值 / Returns:
//   - int64: 读取的总字节数 / Total bytes read
//   - error: 读取或处理过程中的错误，如果 Context 被取消则返回 context.Canceled / Error during reading or processing, returns context.Canceled if context is cancelled
//
// 示例 / Example:
//
//	sess := requests.New(requests.URL("https://example.com/large-file.txt"))
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	_, err := sess.DoRequest(ctx,
//	    requests.Stream(func(lineNum int64, line []byte) error {
//	        fmt.Printf("Line %d: %s", lineNum, line)
//	        return nil
//	    }),
//	)
func streamRead(ctx context.Context, reader io.Reader, fn func(int64, []byte) error) (int64, error) {
	// 创建一个 1MB 缓冲的读取器，提高大文件读取性能
	// Create a 1MB buffered reader for better performance with large files
	i, cnt, r := int64(0), int64(0), bufio.NewReaderSize(reader, 1024*1024)
	for {
		// 检查 Context 是否已取消
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return cnt, ctx.Err()
		default:
		}

		// 读取直到遇到换行符 (ASCII 10 = '\n')
		// Read until newline character (ASCII 10 = '\n')
		raw, err1 := r.ReadBytes(10)
		if err1 != nil && err1 != io.EOF {
			return cnt, err1
		}

		// 累计行号和字节数
		// Accumulate line number and byte count
		i, cnt = i+1, cnt+int64(len(raw))

		// 调用回调函数处理当前行
		// Call callback function to process current line
		if err2 := fn(i, raw); err1 == io.EOF || err2 != nil {
			// 确保最后一行能被处理，并且可以正常返回
			// Ensure the last line is processed and return properly
			return cnt, err2
		}
	}
}
