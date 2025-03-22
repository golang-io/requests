package requests

import (
	"bytes"
	"context"
	"io"
	"net/http"
)

// printRoundTripper creates a middleware for printing HTTP client request and response information.
// Parameter f is a callback function for processing request statistics.
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
// 参数 f 是流处理回调函数，接收两个参数：
//   - i int64: 当前处理的数据块序号（从1开始）
//   - raw []byte: 原始数据块内容（按换行符分割）
//
// 返回值是可用于HTTP客户端中间件链的RoundTripper
// 适用场景：大文件下载、实时事件流处理等需要边接收边处理的场景
// 注意：与普通RoundTripper不同，此方法会流式处理响应体而不是缓存全部内容
func streamRoundTrip(fn func(i int64, raw []byte) error) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			resp := newResponse(r)
			if resp.Response, resp.Err = next.RoundTrip(r); resp.Err != nil {
				return resp.Response, resp.Err
			}

			if resp.Response.ContentLength, resp.Err = streamRead(resp.Response.Body, fn); resp.Err != nil {
				return resp.Response, resp.Err
			}
			resp.Response.Body = io.NopCloser(bytes.NewReader([]byte("[stream]")))
			return resp.Response, resp.Err
		})
	}
}
