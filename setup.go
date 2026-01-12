// Package requests 提供了HTTP客户端的中间件设置功能
// Package requests provides middleware setup functionality for HTTP client
package requests

import (
	"context"
	"net/http"
)

// printRoundTripper 创建一个用于打印HTTP客户端请求和响应信息的中间件
// 该中间件会在请求完成后调用回调函数，传递统计信息
//
// printRoundTripper creates a middleware for printing HTTP client request and response information
// This middleware calls the callback function after request completion, passing statistics
//
// 参数 / Parameters:
//   - f: func(ctx context.Context, stat *Stat) - 处理请求统计信息的回调函数 / Callback function for processing request statistics
//
// 返回值 / Returns:
//   - func(http.RoundTripper) http.RoundTripper: RoundTripper中间件函数 / RoundTripper middleware function
//
// 内部实现 / Implementation:
//   - 拦截HTTP请求 / Intercepts HTTP requests
//   - 调用下一个RoundTripper处理请求 / Calls next RoundTripper to handle request
//   - 构建统计信息并调用回调函数 / Builds statistics and calls callback function
//   - 返回原始响应 / Returns original response
//
// 使用场景 / Use Cases:
//   - 请求日志记录 / Request logging
//   - 性能监控 / Performance monitoring
//   - 调试和审计 / Debugging and auditing
func printRoundTripper(f func(ctx context.Context, stat *Stat)) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := newResponse(r)
			resp.Response, resp.Err = next.RoundTrip(r)
			f(r.Context(), resp.Stat())
			return resp.Response, resp.Err
		})
	}
}

// streamRoundTrip 创建一个流式处理的RoundTripper中间件
// 该中间件按行读取响应体，并对每一行调用回调函数，适用于大文件或实时流
//
// streamRoundTrip creates a streaming RoundTripper middleware
// This middleware reads response body line by line and calls callback for each line, suitable for large files or real-time streams
//
// 参数 / Parameters:
//   - fn: func(i int64, raw []byte) error - 流处理回调函数 / Stream processing callback function
//   - i: 当前处理的数据块序号（从1开始） / Current data chunk number (starting from 1)
//   - raw: 原始数据块内容（按换行符分割） / Raw data chunk content (split by newline)
//
// 返回值 / Returns:
//   - func(http.RoundTripper) http.RoundTripper: RoundTripper中间件函数 / RoundTripper middleware function
//
// 行为特性 / Behavior:
//   - 按行流式读取响应体 / Reads response body line by line in streaming mode
//   - 不缓存完整响应体，节省内存 / Does not cache full response body, saves memory
//   - 响应体被替换为 http.NoBody，表示已被流式处理 / Response body is replaced with http.NoBody to indicate it has been streamed
//   - 回调函数返回错误时立即停止处理 / Stops processing immediately if callback returns error
//
// 适用场景 / Use Cases:
//   - 大文件下载（逐块处理） / Large file downloads (process by chunks)
//   - Server-Sent Events (SSE)实时事件流 / Server-Sent Events (SSE) real-time event streams
//   - 日志流实时处理 / Real-time log stream processing
//   - 减少内存占用的流式API / Memory-efficient streaming APIs
//
// 注意事项 / Notes:
//   - 响应体会被完全消费 / Response body will be fully consumed
//   - 原始响应体不可再次读取 / Original response body cannot be read again
//   - 适合单向流式处理场景 / Suitable for one-way streaming scenarios
//
// 示例 / Example:
//
//	// 流式下载大文件
//	// Stream download large file
//	session := requests.New(
//	    requests.Stream(func(i int64, line []byte) error {
//	        fmt.Printf("Line %d: %s\n", i, string(line))
//	        return nil
//	    }),
//	)
func streamRoundTrip(fn func(i int64, raw []byte) error) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := newResponse(r)
			if resp.Response, resp.Err = next.RoundTrip(r); resp.Err != nil {
				return resp.Response, resp.Err
			}

			// 传递请求的 Context 给 streamRead，支持取消操作
			// Pass request's Context to streamRead to support cancellation
			if resp.Response.ContentLength, resp.Err = streamRead(r.Context(), resp.Response.Body, fn); resp.Err != nil {
				return resp.Response, resp.Err
			}

			// 关闭原始 Body（streamRead 已经消费完）
			// Close original Body (streamRead has already consumed it)
			resp.Response.Body.Close()

			// 使用 http.NoBody 表示 Body 已被流式处理，没有可读内容
			// Use http.NoBody to indicate Body has been streamed and has no readable content
			resp.Response.Body = http.NoBody

			return resp.Response, resp.Err
		})
	}
}
